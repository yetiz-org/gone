package ghttp

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/yetiz-org/gone/channel"
	"github.com/yetiz-org/gone/erresponse"
	"github.com/yetiz-org/gone/ghttp/httpstatus"
)

type _DispatchSeedAcceptance struct {
	DispatchAcceptance
}

func (a *_DispatchSeedAcceptance) Do(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) (err error) {
	switch params["seed"] {
	case "trusted":
		return nil
	case "interrupt":
		return AcceptanceInterrupt
	default:
		return erresponse.InvalidGrant
	}
}

type _DispatchProbeTask struct {
	DefaultHTTPHandlerTask
}

func (t *_DispatchProbeTask) Get(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) (errResponse ErrorResponse) {
	req.Session().PutString("lang", "en")
	if err := req.Session().Save(); err != nil {
		return erresponse.ServerError
	}

	_, raw := t.RawMode(req, resp, params)
	resp.JsonResponse(map[string]any{"id": t.GetID("items", params), "raw": raw, "sse": t.SSEMode(ctx, req, resp, params) != nil})
	return nil
}

func (t *_DispatchProbeTask) Post(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) (errResponse ErrorResponse) {
	panic("boom")
}

func TestDispatchHandlerDispatch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		method     string
		target     string
		body       io.Reader
		seed       map[string]any
		wantStatus int
		wantNode   bool
		wantBody   map[string]any
	}{
		{
			name:       "serves the handler in process",
			method:     MethodGet,
			target:     "/items/abc",
			seed:       map[string]any{"seed": "trusted"},
			wantStatus: http.StatusOK,
			wantNode:   true,
			wantBody:   map[string]any{"id": "abc", "raw": false, "sse": false},
		},
		{
			name:       "acceptances still apply",
			method:     MethodGet,
			target:     "/items/abc",
			wantStatus: httpstatus.Forbidden,
			wantNode:   true,
		},
		{
			name:       "interrupted acceptances skip the handler",
			method:     MethodGet,
			target:     "/items/abc",
			seed:       map[string]any{"seed": "interrupt"},
			wantStatus: http.StatusOK,
			wantNode:   true,
		},
		{
			name:       "panics become server errors",
			method:     MethodPost,
			target:     "/items",
			seed:       map[string]any{"seed": "trusted"},
			wantStatus: http.StatusInternalServerError,
			wantNode:   true,
		},
		{
			name:       "unreadable bodies are bad requests",
			method:     MethodPost,
			target:     "/items",
			body:       errorReadCloser{},
			seed:       map[string]any{"seed": "trusted"},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "unknown paths are not found",
			method:     MethodGet,
			target:     "/missing",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "non HTTP tasks are not found",
			method:     MethodGet,
			target:     "/plain",
			wantStatus: http.StatusNotFound,
			wantNode:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ch := &channel.DefaultChannel{}
			ch.Init()
			ctx := channel.NewMockHandlerContext()
			ctx.On("Channel").Return(ch)
			route := NewSimpleRoute().SetEndpoint("/items", &_DispatchProbeTask{}, &_DispatchSeedAcceptance{}).SetEndpoint("/plain", &DefaultHandlerTask{})
			handler := NewDispatchHandler(route)

			pack := handler.Dispatch(ctx, httptest.NewRequest(tt.method, tt.target, tt.body), tt.seed)

			require.NotNil(t, pack, "Dispatch should return a pack")
			assert.Equal(t, tt.wantStatus, pack.Response.StatusCode(), "status should match the case")
			assert.Equal(t, tt.wantNode, pack.RouteNode != nil, "matched node should match the case")
			assert.Empty(t, pack.Response.Cookies(), "internal dispatch should not issue cookies")
			assert.NotContains(t, pack.Response.Header(), "Access-Control-Allow-Origin", "internal dispatch should skip CORS")
			assert.Nil(t, SessionProvider().Session(pack.Request.Session().Id()), "internal sessions should not reach the shared store")
			ctx.AssertNotCalled(t, "Write", mock.Anything, mock.Anything)
			if tt.wantBody != nil {
				body := map[string]any{}
				require.NoError(t, json.Unmarshal(pack.Response.Body().Bytes(), &body), "body should be JSON")
				assert.Equal(t, tt.wantBody, body, "body should come from the handler")
			} else if tt.wantStatus == http.StatusOK {
				assert.Zero(t, pack.Response.Body().ReadableBytes(), "an interrupted dispatch should not run the handler")
			}
		})
	}
}

func TestDispatchHandlerDispatchOversizedBody(t *testing.T) {
	t.Parallel()

	ch := &channel.DefaultChannel{}
	ch.Init()
	ctx := channel.NewMockHandlerContext()
	ctx.On("Channel").Return(ch)
	handler := NewDispatchHandler(NewSimpleRoute().SetEndpoint("/items", &_DispatchProbeTask{}))
	req := httptest.NewRequest(MethodPost, "/items", nil)
	req.Body = http.MaxBytesReader(nil, io.NopCloser(strings.NewReader("exceeds")), 3)

	assert.Nil(t, handler.Dispatch(ctx, req, nil), "Dispatch should return nil when the body exceeds its limit")
}
