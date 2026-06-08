package ghttp

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yetiz-org/gone/channel"
)

type _DispatchHeadTask struct {
	DefaultHTTPHandlerTask
	headCalled bool
	getCalled  bool
}

func (t *_DispatchHeadTask) Head(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) ErrorResponse {
	t.headCalled = true
	return nil
}

func (t *_DispatchHeadTask) Get(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) ErrorResponse {
	t.getCalled = true
	return nil
}

type _DispatchAllMethodsTask struct {
	DefaultHTTPHandlerTask
	called string
}

func (t *_DispatchAllMethodsTask) Index(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) ErrorResponse {
	return NotImplemented
}

func (t *_DispatchAllMethodsTask) Get(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) ErrorResponse {
	t.called = "Get"
	return nil
}

func (t *_DispatchAllMethodsTask) Head(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) ErrorResponse {
	t.called = "Head"
	return nil
}

func (t *_DispatchAllMethodsTask) Create(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) ErrorResponse {
	return NotImplemented
}

func (t *_DispatchAllMethodsTask) Post(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) ErrorResponse {
	t.called = "Post"
	return nil
}

func (t *_DispatchAllMethodsTask) Put(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) ErrorResponse {
	t.called = "Put"
	return nil
}

func (t *_DispatchAllMethodsTask) Patch(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) ErrorResponse {
	t.called = "Patch"
	return nil
}

func (t *_DispatchAllMethodsTask) Delete(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) ErrorResponse {
	t.called = "Delete"
	return nil
}

func (t *_DispatchAllMethodsTask) Connect(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) ErrorResponse {
	t.called = "Connect"
	return nil
}

func (t *_DispatchAllMethodsTask) Options(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) ErrorResponse {
	t.called = "Options"
	return nil
}

func (t *_DispatchAllMethodsTask) Trace(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) ErrorResponse {
	t.called = "Trace"
	return nil
}

func TestDispatchHandlerInvokeMethodHeadUsesHead(t *testing.T) {
	task := &_DispatchHeadTask{}
	req := &Request{request: httptest.NewRequest(MethodHead, "/probe", nil)}
	resp := &Response{header: http.Header{}}

	err := NewDispatchHandler(NewSimpleRoute()).invokeMethod(
		channel.NewMockHandlerContext(),
		task,
		req,
		resp,
		map[string]any{},
		true,
	)

	require.Nil(t, err)
	require.True(t, task.headCalled)
	require.False(t, task.getCalled)
}

func TestDispatchHandlerInvokeMethodSupportsHTTPMethods(t *testing.T) {
	tests := map[string]string{
		MethodGet:     "Get",
		MethodHead:    "Head",
		MethodPost:    "Post",
		MethodPut:     "Put",
		MethodPatch:   "Patch",
		MethodDelete:  "Delete",
		MethodConnect: "Connect",
		MethodOptions: "Options",
		MethodTrace:   "Trace",
	}

	for method, want := range tests {
		t.Run(method, func(t *testing.T) {
			task := &_DispatchAllMethodsTask{}
			req := &Request{request: httptest.NewRequest(method, "/probe", nil)}
			resp := &Response{header: http.Header{}}

			err := NewDispatchHandler(NewSimpleRoute()).invokeMethod(
				channel.NewMockHandlerContext(),
				task,
				req,
				resp,
				map[string]any{},
				true,
			)

			require.Nil(t, err)
			require.Equal(t, want, task.called)
		})
	}
}

func TestDispatchHandlerUpdateSessionCookieUsesRequestHostWhenSessionDomainEmpty(t *testing.T) {
	oldSessionDomain := SessionDomain
	oldSessionKey := SessionKey
	t.Cleanup(func() {
		SessionDomain = oldSessionDomain
		SessionKey = oldSessionKey
	})

	SessionDomain = ""
	SessionKey = "HOST_DOMAIN_TEST"

	req := &Request{
		request: httptest.NewRequest(MethodGet, "http://api.example.test:8080/probe", nil),
		channel: channel.NewMockChannel(),
	}
	req.Session()
	resp := NewResponse(req)

	NewDispatchHandler(NewSimpleRoute())._UpdateSessionCookie(resp)

	cookie := resp.Cookie(SessionKey)
	require.NotNil(t, cookie)
	require.Equal(t, "api.example.test", cookie.Domain)
}

func TestDispatchHandlerUpdateSessionCookieRefreshUsesRequestHostWhenSessionDomainEmpty(t *testing.T) {
	oldSessionDomain := SessionDomain
	oldSessionKey := SessionKey
	t.Cleanup(func() {
		SessionDomain = oldSessionDomain
		SessionKey = oldSessionKey
	})

	SessionDomain = ""
	SessionKey = "HOST_DOMAIN_REFRESH_TEST"

	req := &Request{
		request: httptest.NewRequest(MethodGet, "http://api.example.test:8080/probe", nil),
		channel: channel.NewMockChannel(),
	}
	req.Session()
	req.AddCookie(&http.Cookie{Name: SessionKey, Value: "expired"})
	resp := NewResponse(req)

	NewDispatchHandler(NewSimpleRoute())._UpdateSessionCookie(resp)

	cookie := resp.Cookie(SessionKey)
	require.NotNil(t, cookie)
	require.Equal(t, "api.example.test", cookie.Domain)
}

func TestDispatchHandlerUpdateSessionCookieKeepsExplicitSessionDomain(t *testing.T) {
	oldSessionDomain := SessionDomain
	oldSessionKey := SessionKey
	t.Cleanup(func() {
		SessionDomain = oldSessionDomain
		SessionKey = oldSessionKey
	})

	SessionDomain = "example.test"
	SessionKey = "EXPLICIT_DOMAIN_TEST"

	req := &Request{
		request: httptest.NewRequest(MethodGet, "http://api.example.test:8080/probe", nil),
		channel: channel.NewMockChannel(),
	}
	req.Session()
	resp := NewResponse(req)

	NewDispatchHandler(NewSimpleRoute())._UpdateSessionCookie(resp)

	cookie := resp.Cookie(SessionKey)
	require.NotNil(t, cookie)
	require.Equal(t, "example.test", cookie.Domain)
}

func TestDispatchHandlerUpdateSessionCookieSkipsIPv6HostDomain(t *testing.T) {
	oldSessionDomain := SessionDomain
	oldSessionKey := SessionKey
	t.Cleanup(func() {
		SessionDomain = oldSessionDomain
		SessionKey = oldSessionKey
	})

	SessionDomain = ""
	SessionKey = "IPV6_DOMAIN_TEST"

	req := &Request{
		request: httptest.NewRequest(MethodGet, "http://[::1]:8080/probe", nil),
		channel: channel.NewMockChannel(),
	}
	req.Session()
	resp := NewResponse(req)

	NewDispatchHandler(NewSimpleRoute())._UpdateSessionCookie(resp)

	cookie := resp.Cookie(SessionKey)
	require.NotNil(t, cookie)
	require.Empty(t, cookie.Domain)
}
