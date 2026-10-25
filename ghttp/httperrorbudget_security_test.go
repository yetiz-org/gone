package ghttp

import (
	"encoding/json"
	"net/http/httptest"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yetiz-org/gone/channel"
	"github.com/yetiz-org/gone/erresponse"
)

type boundaryObservedError struct {
	erresponse.DefaultErrorResponse
	cloned    atomic.Int32
	formatted atomic.Int32
	mutated   atomic.Int32
}

func (e *boundaryObservedError) Clone() (cloned erresponse.ErrorResponse) {
	e.cloned.Add(1)
	return &boundaryObservedError{DefaultErrorResponse: *e.DefaultErrorResponse.Clone().(*erresponse.DefaultErrorResponse)}
}

func (e *boundaryObservedError) Error() (message string) {
	e.formatted.Add(1)
	data, _ := json.Marshal(struct {
		*erresponse.DefaultErrorResponse
		Custom bool `json:"custom"`
	}{DefaultErrorResponse: &e.DefaultErrorResponse, Custom: true})
	return string(data)
}

func (e *boundaryObservedError) ErrorData() (data map[string]any) {
	e.mutated.Add(1)
	return e.DefaultErrorResponse.ErrorData()
}

type boundaryErrorTask struct {
	DefaultHTTPHandlerTask
	failure ErrorResponse
	mode    string
}

func (t *boundaryErrorTask) Get(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) (failure ErrorResponse) {
	switch t.mode {
	case "direct":
		resp.ResponseError(t.failure)
		return nil
	case "panic":
		panic(t.failure)
	}

	return t.failure
}

type boundaryErrorAcceptance struct {
	DispatchAcceptance
	failure ErrorResponse
}

func (a *boundaryErrorAcceptance) Do(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) (rtErr error) {
	return a.failure
}

type boundaryErrorContext struct {
	channel.HandlerContext
	ch channel.Channel
}

func (c *boundaryErrorContext) Channel() (ch channel.Channel) {
	return c.ch
}

type boundaryErrorScenario struct {
	handler *DispatchHandler
	ctx     channel.HandlerContext
	node    RouteNode
	limit   int64
}

func newBoundaryErrorScenario(failure ErrorResponse, mode string, limit int64) (scenario *boundaryErrorScenario) {
	task := &boundaryErrorTask{failure: failure, mode: mode}
	route := NewSimpleRoute()
	if mode == "acceptance" {
		route.SetEndpoint("/failure", task, &boundaryErrorAcceptance{failure: failure})
	} else {
		route.SetEndpoint("/failure", task)
	}

	ch := &channel.DefaultChannel{}
	ch.Init()
	return &boundaryErrorScenario{handler: NewDispatchHandler(route), ctx: &boundaryErrorContext{ch: ch}, node: route.FindNode("/failure"), limit: limit}
}

func (s *boundaryErrorScenario) dispatch() (pack *Pack, rtErr error) {
	req := httptest.NewRequest(MethodGet, "/failure", nil)
	return s.handler.DispatchExact(s.ctx, req, nil, DispatchOptions{Operation: DispatchGet, Node: s.node, IDs: map[string]string{"failure": ""}, MaxResponseBytes: s.limit})
}

func TestBoundedCustomErrorWireAndCaps(t *testing.T) {
	for _, mode := range []string{"returned", "direct", "panic", "acceptance"} {
		for _, large := range []bool{false, true} {
			t.Run(mode+"/large="+strconv.FormatBool(large), func(t *testing.T) {
				failure := &boundaryObservedError{DefaultErrorResponse: erresponse.DefaultErrorResponse{StatusCode: 400, Name: "invalid_argument", Data: map[string]any{}}}
				if large {
					failure.Data["large"] = strings.Repeat("x", 4096)
				}

				pack, err := newBoundaryErrorScenario(failure, mode, 512).dispatch()
				require.NotNil(t, pack)
				require.Positive(t, failure.cloned.Load(), "custom Clone remains supported")
				if large {
					require.ErrorIs(t, err, ErrResponseBodyTooLarge)
					require.Zero(t, pack.Response.Body().ReadableBytes())
				} else {
					require.NoError(t, err)
					require.Equal(t, 400, pack.Response.StatusCode())
					var body map[string]any
					require.NoError(t, json.Unmarshal(pack.Response.Body().Bytes(), &body))
					require.Equal(t, true, body["custom"], "custom Error() owns the wire format")
					require.NotEmpty(t, body["data"].(map[string]any)["tid"])
				}
				require.LessOrEqual(t, pack.Response.Body().ReadableBytes(), 512)
				require.NotContains(t, failure.Data, "tid", "Clone isolates request metadata")
			})
		}
	}
}

func TestBoundedStandardErrorBodyLimit(t *testing.T) {
	for _, mode := range []string{"returned", "direct", "panic", "acceptance"} {
		for _, metadata := range []bool{false, true} {
			t.Run(mode+"/metadata="+strconv.FormatBool(metadata), func(t *testing.T) {
				failure := &erresponse.DefaultErrorResponse{StatusCode: 400, Name: "invalid_argument"}
				if metadata {
					failure.I18nParams = map[string]string{"large": strings.Repeat("x", 4096)}
				} else {
					failure.Data = map[string]any{"large": strings.Repeat("x", 4096)}
				}

				pack, err := newBoundaryErrorScenario(failure, mode, 512).dispatch()
				require.NotNil(t, pack)
				if metadata {
					require.NoError(t, err, "unserialized application metadata does not spend the body budget")
					require.NotEmpty(t, pack.Response.Body().Bytes())
				} else {
					require.ErrorIs(t, err, ErrResponseBodyTooLarge)
					require.Empty(t, pack.Response.Body().Bytes())
				}
				require.LessOrEqual(t, pack.Response.Body().ReadableBytes(), 512)
			})
		}
	}
}

func TestBoundedSmallErrorsPreserveSemantics(t *testing.T) {
	for _, mode := range []string{"returned", "direct", "panic", "acceptance"} {
		t.Run(mode, func(t *testing.T) {
			failure := &erresponse.DefaultErrorResponse{StatusCode: 400, Name: "invalid_argument", Title: "Bad request", Data: map[string]any{"field": "name"}, I18nParams: map[string]string{"name": "Display"}}
			scenario := newBoundaryErrorScenario(failure, mode, 4096)
			pack, err := scenario.dispatch()
			require.NoError(t, err)
			require.Equal(t, 400, pack.Response.StatusCode())
			var body map[string]any
			require.NoError(t, json.Unmarshal(pack.Response.Body().Bytes(), &body))
			require.Equal(t, "invalid_argument", body["error"])
			require.Equal(t, "Bad request", body["title"])
			data := body["data"].(map[string]any)
			require.Equal(t, "name", data["field"])
			require.NotEmpty(t, data["cid"])
			require.NotEmpty(t, data["tid"])
			require.Equal(t, map[string]any{"field": "name"}, failure.Data)
			require.Equal(t, map[string]string{"name": "Display"}, failure.I18nParams)
		})
	}

	failure := &boundaryObservedError{DefaultErrorResponse: erresponse.DefaultErrorResponse{StatusCode: 400, Name: "invalid_argument"}}
	scenario := newBoundaryErrorScenario(failure, "returned", 4096)
	pack := scenario.handler.Dispatch(scenario.ctx, httptest.NewRequest(MethodGet, "/failure", nil), nil)
	require.Equal(t, 400, pack.Response.StatusCode())
	require.Positive(t, failure.cloned.Load(), "ordinary Dispatch keeps legacy custom-error callbacks")
	require.NoError(t, pack.Response.BodyError())
}

type boundaryLargeError struct {
	erresponse.DefaultErrorResponse
	text string
}

func (e *boundaryLargeError) Clone() (cloned erresponse.ErrorResponse) {
	return e
}

func (e *boundaryLargeError) Error() (message string) {
	return e.text
}

func TestBoundedErrorTextCheckedBeforeCopy(t *testing.T) {
	failure := &boundaryLargeError{DefaultErrorResponse: erresponse.DefaultErrorResponse{StatusCode: 500, Name: "internal_error", Data: map[string]any{}}, text: strings.Repeat("x", 8<<20)}
	ch := channel.NewMockChannel()
	ch.On("ID").Return("cid")
	resp := NewResponse(WrapRequest(ch, httptest.NewRequest("GET", "/", nil)))
	resp.bodyLimit = 64
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	resp.ResponseError(failure)
	runtime.ReadMemStats(&after)

	require.ErrorIs(t, resp.BodyError(), ErrResponseBodyTooLarge)
	require.Zero(t, resp.Body().ReadableBytes())
	require.Less(t, after.TotalAlloc-before.TotalAlloc, uint64(1<<20), "oversized error text must be rejected before it is copied")
}
