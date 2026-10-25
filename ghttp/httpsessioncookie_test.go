package ghttp

import (
	"net"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/yetiz-org/gone/channel"
	"github.com/yetiz-org/gone/ghttp/httpsession"
	"github.com/yetiz-org/goth-util/hash"
)

type sessionCookieTask struct {
	DefaultHTTPHandlerTask
	previous chan string
}

func (t *sessionCookieTask) Get(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) (errResponse ErrorResponse) {
	req.Session().PutString("user", "u")
	return nil
}

func (t *sessionCookieTask) Post(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) (errResponse ErrorResponse) {
	t.previous <- req.Session().Id()
	req.RenewSession()
	return nil
}

func sessionCookie(t *testing.T, resp *http.Response) (cookie *http.Cookie) {
	t.Helper()
	for _, cookie = range resp.Cookies() {
		if cookie.Name == SessionKey {
			return cookie
		}
	}

	t.Fatal("the response must set the session cookie")
	return nil
}

func TestSessionCookieFlags(t *testing.T) {
	addr := startTestServer(t, nil, NewSimpleRoute().SetEndpoint("/session", &sessionCookieTask{}), false)
	tests := []struct {
		name       string
		proto      string
		trusted    bool
		wantSecure bool
	}{
		{name: "plain HTTP stays usable", wantSecure: false},
		{name: "HTTPS behind an explicitly trusted proxy is Secure", proto: "https", trusted: true, wantSecure: true},
		{name: "an untrusted peer cannot claim HTTPS", proto: "https", wantSecure: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			previous := TrustForwardedFor
			TrustForwardedFor = func(peer net.IP) bool { return tt.trusted && peer.IsLoopback() }
			t.Cleanup(func() { TrustForwardedFor = previous })

			req, err := http.NewRequest(http.MethodGet, "http://"+addr+"/session", nil)
			require.NoError(t, err)
			if tt.proto != "" {
				req.Header.Set("X-Forwarded-Proto", tt.proto)
			}

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			cookie := sessionCookie(t, resp)
			require.True(t, cookie.HttpOnly, "the session cookie must be hidden from scripts by default")
			require.Equal(t, tt.wantSecure, cookie.Secure)
		})
	}
}

type renewOnlyTask struct {
	DefaultHTTPHandlerTask
}

func (t *renewOnlyTask) Post(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) (errResponse ErrorResponse) {
	req.RenewSession()
	return nil
}

type countingSessionProvider struct {
	httpsession.SessionProvider
	created atomic.Int32
}

func (p *countingSessionProvider) NewSession(expire time.Time) (session httpsession.Session) {
	p.created.Add(1)
	return p.SessionProvider.NewSession(expire)
}

func TestRenewSessionWithoutCookieCreatesOneSession(t *testing.T) {
	provider := &countingSessionProvider{SessionProvider: SessionProvider()}
	defaultSessionProvider = provider
	t.Cleanup(func() { defaultSessionProvider = provider.SessionProvider })
	addr := startTestServer(t, nil, NewSimpleRoute().SetEndpoint("/renew", &renewOnlyTask{}), false)

	resp, err := http.Post("http://"+addr+"/renew", "text/plain", nil)
	require.NoError(t, err)
	resp.Body.Close()

	require.Equal(t, int32(1), provider.created.Load(), "renewing without a stored session must not create one just to delete it")
}

func TestRenewSessionDeletesPrevious(t *testing.T) {
	task := &sessionCookieTask{previous: make(chan string, 1)}
	addr := startTestServer(t, nil, NewSimpleRoute().SetEndpoint("/session", task), false)

	resp, err := http.Post("http://"+addr+"/session", "text/plain", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	previous := <-task.previous
	require.Nil(t, SessionProvider().Session(previous), "a renewed session must be removed from the store")
	renewed := string(hash.DataOfTimeHash(sessionCookie(t, resp).Value))
	require.NotEqual(t, previous, renewed)
	require.NotNil(t, SessionProvider().Session(renewed), "the new session must be stored")
}
