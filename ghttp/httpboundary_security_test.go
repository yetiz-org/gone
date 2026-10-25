package ghttp

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/yetiz-org/gone/channel"
	"github.com/yetiz-org/gone/ghttp/httpsession"
	"github.com/yetiz-org/gone/ghttp/httpsession/memory"
	"github.com/yetiz-org/goth-util/hash"
)

type boundaryLookupProvider struct {
	httpsession.SessionProvider
	session httpsession.Session
	deleted bool
}

func (p *boundaryLookupProvider) Session(key string) (session httpsession.Session) {
	return p.session
}

func (p *boundaryLookupProvider) Delete(key string) {
	p.deleted = true
	p.session = nil
}

func TestBoundaryExpiredServerSession(t *testing.T) {
	previous := SessionProvider()
	t.Cleanup(func() { defaultSessionProvider = previous })
	for _, expired := range []bool{false, true} {
		provider := &boundaryLookupProvider{SessionProvider: memory.NewSessionProvider()}
		session := httpsession.NewDefaultSession(provider)
		session.SetExpire(time.Now().Add(time.Hour))
		if expired {
			session.SetExpire(time.Now().Add(-time.Hour))
		}

		provider.session = session
		defaultSessionProvider = provider
		req := WrapRequest(channel.NewMockChannel(), httptest.NewRequest(MethodGet, "/session", nil))
		req.AddCookie(&http.Cookie{Name: SessionKey, Value: hash.TimeHash([]byte(session.Id()), time.Now().Add(24*time.Hour).Unix())})
		stored := _StoredSession(req)
		if expired {
			require.Nil(t, stored, "client timestamp cannot extend server expiry")
			require.False(t, provider.deleted, "key-only cleanup must remain provider-owned")
		} else {
			require.Same(t, session, stored)
			require.False(t, provider.deleted)
		}
	}
}

func TestBoundaryCookieScope(t *testing.T) {
	previous := SessionDomain
	t.Cleanup(func() { SessionDomain = previous })
	for _, host := range []string{"example.test", "api.example.test:8080", "127.0.0.1:8080", "[::1]:8080"} {
		for _, refresh := range []bool{false, true} {
			SessionDomain = ""
			req := WrapRequest(channel.NewMockChannel(), httptest.NewRequest(MethodGet, "http://"+host+"/session", nil))
			req.session = httpsession.NewDefaultSession(memory.NewSessionProvider())
			if refresh {
				req.AddCookie(&http.Cookie{Name: SessionKey, Value: "expired"})
			}

			resp := NewResponse(req)
			NewDispatchHandler(NewSimpleRoute())._UpdateSessionCookie(resp)
			require.NotNil(t, resp.Cookie(SessionKey))
			require.Empty(t, resp.Cookie(SessionKey).Domain, host)
			require.NotContains(t, resp.Cookie(SessionKey).String(), "Domain=")
		}
	}

	SessionDomain = "example.test"
	req := WrapRequest(channel.NewMockChannel(), httptest.NewRequest(MethodGet, "http://api.example.test/session", nil))
	req.session = httpsession.NewDefaultSession(memory.NewSessionProvider())
	resp := NewResponse(req)
	NewDispatchHandler(NewSimpleRoute())._UpdateSessionCookie(resp)
	require.Equal(t, "example.test", resp.Cookie(SessionKey).Domain)
}

func TestBoundaryForwardedTrust(t *testing.T) {
	previous := TrustForwardedFor
	t.Cleanup(func() { TrustForwardedFor = previous })
	for _, peer := range []string{"10.0.0.5:1234", "127.0.0.1:1234", "[::1]:1234", "1.1.1.1:1234"} {
		req := httptest.NewRequest(MethodGet, "/", nil)
		req.RemoteAddr = peer
		req.Header.Set("X-Forwarded-For", "8.8.8.8")
		req.Header.Set("X-Forwarded-Proto", "https")
		wrapped := WrapRequest(channel.NewMockChannel(), req)
		require.Equal(t, peer, wrapped.Request().RemoteAddr)
		require.False(t, wrapped.forwarded)
		require.Equal(t, []string{peer}, wrapped.RemoteAddrs())
	}

	TrustForwardedFor = func(peer net.IP) bool { return peer.Equal(net.ParseIP("10.0.0.5")) }
	for _, scenario := range []struct {
		header string
		want   string
	}{
		{header: "8.8.8.8", want: "8.8.8.8"},
		{header: "8.8.8.8, 10.0.0.2", want: "10.0.0.2"},
		{header: "8.8.8.8, malformed", want: "10.0.0.5:1234"},
		{header: "2001:4860:4860::8888", want: "2001:4860:4860::8888"},
	} {
		req := httptest.NewRequest(MethodGet, "/", nil)
		req.RemoteAddr = "10.0.0.5:1234"
		req.Header.Set("X-Forwarded-For", scenario.header)
		wrapped := WrapRequest(channel.NewMockChannel(), req)
		require.Equal(t, scenario.want, wrapped.Request().RemoteAddr, scenario.header)
		require.Equal(t, "10.0.0.5:1234", wrapped.RemoteAddrs()[len(wrapped.RemoteAddrs())-1])
	}
}

func TestBoundaryLogURIOmitsQuery(t *testing.T) {
	ch := &channel.DefaultChannel{}
	ch.Init()
	ch.SetParam(ParamLogURI, func(req *Request) string { return req.Url().EscapedPath() })
	for _, query := range []string{"code=secret", "CODE=secret&code=&code=%73ecret", "token=secret&safe=value", "secret"} {
		req := WrapRequest(ch, httptest.NewRequest(MethodGet, "/oauth%2Fcallback?"+query, nil))
		handler := NewLogHandler(false)
		requestLog := handler.constructReq(req)
		responseLog := handler.constructResp(NewResponse(req))
		require.Equal(t, "/oauth%2Fcallback", requestLog.URI)
		require.Equal(t, requestLog.URI, responseLog.URI)
		require.NotContains(t, requestLog.URI, "secret")
	}
}

func TestBoundaryNotFoundIsLocal(t *testing.T) {
	req := WrapRequest(channel.NewMockChannel(), httptest.NewRequest(MethodGet, "/missing", nil))
	resp := NewResponse(req)
	NewDispatchHandler(NewSimpleRoute()).defaultNotFound404(req, resp, nil)
	require.Equal(t, http.StatusNotFound, resp.StatusCode())
	require.Equal(t, "text/plain; charset=utf-8", resp.GetHeader("Content-Type"))
	require.Equal(t, "404 Not Found\n", string(resp.Body().Bytes()))
	require.False(t, strings.Contains(string(resp.Body().Bytes()), "http"))
}

func TestBoundaryDirectPeerAddress(t *testing.T) {
	previous := TrustForwardedFor
	TrustForwardedFor = nil
	t.Cleanup(func() { TrustForwardedFor = previous })
	for _, test := range []struct {
		peer string
		ip   string
		port string
	}{
		{peer: "192.0.2.8", ip: "192.0.2.8"},
		{peer: "192.0.2.8:443", ip: "192.0.2.8", port: "443"},
		{peer: "2001:db8::abcd", ip: "2001:db8::abcd"},
		{peer: "[2001:db8::abcd]:443", ip: "2001:db8::abcd", port: "443"},
		{peer: "[::1]:443", ip: "::1", port: "443"},
		{peer: "[fe80::abcd%en0]:443", ip: "<nil>", port: "443"},
	} {
		t.Run(test.peer, func(t *testing.T) {
			req := httptest.NewRequest(MethodGet, "/", nil)
			req.RemoteAddr = test.peer
			wrapped := WrapRequest(channel.NewMockChannel(), req)
			ip, port := wrapped.RemoteAddr()
			require.Equal(t, test.ip, ip.String())
			require.Equal(t, test.port, port)
			require.Equal(t, test.peer, wrapped.SocketRemoteAddr())
		})
	}
}

func TestBoundaryIPv6ForwardedTrust(t *testing.T) {
	previousTrust, previousSecure := TrustForwardedFor, SessionSecure
	t.Cleanup(func() { TrustForwardedFor, SessionSecure = previousTrust, previousSecure })
	SessionSecure = false
	for _, proxy := range []string{"2001:db8::abcd", "2001:db8::abc"} {
		t.Run(proxy, func(t *testing.T) {
			TrustForwardedFor = func(peer net.IP) (trusted bool) { return peer.Equal(net.ParseIP(proxy)) }
			req := httptest.NewRequest(MethodGet, "/", nil)
			req.RemoteAddr = "[2001:db8::abcd]:443"
			req.Header.Set("X-Forwarded-For", "192.0.2.8")
			req.Header.Set("X-Forwarded-Proto", "https")
			wrapped := WrapRequest(channel.NewMockChannel(), req)
			wrapped.session = httpsession.NewDefaultSession(memory.NewSessionProvider())
			resp := NewResponse(wrapped)
			NewDispatchHandler(NewSimpleRoute())._UpdateSessionCookie(resp)
			trusted := proxy == "2001:db8::abcd"
			require.Equal(t, trusted, wrapped.forwarded)
			require.Equal(t, trusted, resp.Cookie(SessionKey).Secure)
			require.Equal(t, "[2001:db8::abcd]:443", wrapped.SocketRemoteAddr())
			if trusted {
				require.Equal(t, "192.0.2.8", wrapped.Request().RemoteAddr)
				require.Equal(t, []string{"192.0.2.8", "[2001:db8::abcd]:443"}, wrapped.RemoteAddrs())
			} else {
				require.Equal(t, "[2001:db8::abcd]:443", wrapped.Request().RemoteAddr)
				require.Equal(t, []string{"[2001:db8::abcd]:443"}, wrapped.RemoteAddrs())
			}
		})
	}
}
