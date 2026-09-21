package ghttp

import (
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
)

type _RawModeTask struct {
	DefaultHTTPHandlerTask
}

func (t *_RawModeTask) Post(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) (errResponse ErrorResponse) {
	writer, ok := t.RawMode(req, resp, params)
	if !ok {
		return erresponse.ServerError
	}

	body, err := io.ReadAll(req.Request().Body)
	if err != nil || string(body) != "ping" {
		return erresponse.ServerError
	}

	writer.WriteHeader(http.StatusAccepted)
	if _, err := writer.Write([]byte("raw")); err != nil {
		return erresponse.ServerError
	}

	return nil
}

func TestRawModeStateTransitions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		arrange        func(task *DefaultHTTPHandlerTask, ctx channel.HandlerContext, req *Request, pack *Pack, params map[string]any)
		wantOK         bool
		wantWriter     bool
		wantSeparate   bool
		wantRaw        bool
		wantHeader     bool
		wantIdempotent bool
	}{
		{
			name: "missing pack",
			arrange: func(task *DefaultHTTPHandlerTask, ctx channel.HandlerContext, req *Request, pack *Pack, params map[string]any) {
				delete(params, "[gone-http]context_pack")
			},
		},
		{
			name: "invalid pack",
			arrange: func(task *DefaultHTTPHandlerTask, ctx channel.HandlerContext, req *Request, pack *Pack, params map[string]any) {
				params["[gone-http]context_pack"] = "invalid"
			},
		},
		{
			name: "missing writer",
			arrange: func(task *DefaultHTTPHandlerTask, ctx channel.HandlerContext, req *Request, pack *Pack, params map[string]any) {
				pack.Writer = nil
			},
		},
		{
			name: "SSE already active",
			arrange: func(task *DefaultHTTPHandlerTask, ctx channel.HandlerContext, req *Request, pack *Pack, params map[string]any) {
				task.SSEMode(ctx, req, pack.Response, params)
			},
			wantSeparate: true,
		},
		{
			name:           "raw mode is idempotent",
			wantOK:         true,
			wantWriter:     true,
			wantSeparate:   true,
			wantRaw:        true,
			wantHeader:     true,
			wantIdempotent: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ch := &channel.DefaultChannel{}
			ch.Init()
			req := &Request{request: httptest.NewRequest(MethodPost, "/raw", nil), channel: ch, trackID: "track"}
			pack := &Pack{Request: req, Response: NewResponse(req), Params: map[string]any{}, Writer: httptest.NewRecorder()}
			params := map[string]any{"[gone-http]context_pack": pack}
			task := &DefaultHTTPHandlerTask{}
			ctx := channel.NewMockHandlerContext()
			if tt.arrange != nil {
				tt.arrange(task, ctx, req, pack, params)
			}

			writer, ok := task.RawMode(req, pack.Response, params)

			require.Equal(t, tt.wantOK, ok, "RawMode success state should match the case")
			if tt.wantWriter {
				require.Same(t, pack.Writer, writer, "RawMode should return the pack writer")
			} else {
				require.Nil(t, writer, "RawMode should not expose a writer on rejection")
			}
			assert.Equal(t, tt.wantSeparate, pack.writeSeparateMode, "separate-write state should match the case")
			assert.Equal(t, tt.wantRaw, pack.rawMode, "raw-mode state should match the case")
			assert.Equal(t, tt.wantHeader, pack.Response.headerWritten, "header ownership should match the case")

			if tt.wantIdempotent {
				repeatedWriter, repeatedOK := task.RawMode(req, pack.Response, params)
				require.True(t, repeatedOK, "repeated RawMode should succeed")
				require.Same(t, writer, repeatedWriter, "repeated RawMode should return the same writer")
			}
		})
	}
}

func TestDispatchHandlerRawModeWritesThroughWriter(t *testing.T) {
	t.Parallel()

	ch := &channel.DefaultChannel{}
	ch.Init()
	req := &Request{request: httptest.NewRequest(MethodPost, "/raw", strings.NewReader("ping")), channel: ch, trackID: "track"}
	recorder := httptest.NewRecorder()
	pack := &Pack{Request: req, Response: NewResponse(req), Params: map[string]any{}, Writer: recorder}
	route := NewSimpleRoute().SetEndpoint("/raw", &_RawModeTask{})
	handler := NewDispatchHandler(route)
	ctx := channel.NewMockHandlerContext()
	ctx.On("Channel").Return(ch)
	ctx.On("Write", pack, pack.Response.done).Run(func(args mock.Arguments) {
		args.Get(1).(channel.Future).Completable().Complete(args.Get(0))
	}).Return(pack.Response.done).Maybe()

	handler.Read(ctx, pack)

	assert.Equal(t, http.StatusAccepted, recorder.Code, "the raw handler should control the response status")
	assert.Equal(t, "raw", recorder.Body.String(), "the raw handler should control the response body")
	assert.True(t, pack.writeSeparateMode, "dispatch should leave the pack in separate-write mode")
	assert.True(t, pack.rawMode, "dispatch should leave the pack in raw mode")
	ctx.AssertNotCalled(t, "Write", mock.Anything, mock.Anything)
	assert.False(t, (&GZipHandler{}).shouldCompress(pack), "raw responses should bypass gzip")
	ctx.AssertExpectations(t)
}
