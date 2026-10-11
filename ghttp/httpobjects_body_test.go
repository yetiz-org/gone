package ghttp

import (
	"cmp"
	"net"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yetiz-org/gone/channel"
)

func TestWrapRequestPreservesBodyReadError(t *testing.T) {
	ch := &channel.DefaultChannel{}
	ch.Init()
	req := httptest.NewRequest("POST", "/upload", nil)
	req.Body = errorReadCloser{}

	wrapped := WrapRequest(ch, req)

	require.NotNil(t, wrapped)
	require.ErrorIs(t, wrapped.BodyReadError(), errBodyRead)
	require.Equal(t, 0, wrapped.Body().ReadableBytes())
}

func TestWrapRequestForwardedForTrust(t *testing.T) {
	tests := []struct {
		name      string
		peer      string
		header    string
		trust     func(net.IP) bool
		wantIP    string
		wantAddrs []string
	}{
		{name: "private proxy is trusted", peer: "10.0.0.5:1234", wantIP: "8.8.8.8", wantAddrs: []string{"8.8.8.8", "10.0.0.2", "10.0.0.5:1234"}},
		{name: "loopback proxy is trusted", peer: "127.0.0.1:1234", wantIP: "8.8.8.8", wantAddrs: []string{"8.8.8.8", "10.0.0.2", "127.0.0.1:1234"}},
		{name: "public peer cannot spoof", peer: "1.1.1.1:1234", wantIP: "1.1.1.1", wantAddrs: []string{"1.1.1.1:1234"}},
		{name: "custom trust restores the header", peer: "1.1.1.1:1234", trust: func(net.IP) bool { return true }, wantIP: "8.8.8.8", wantAddrs: []string{"8.8.8.8", "10.0.0.2", "1.1.1.1:1234"}},
		{name: "entries without spaces are split", peer: "10.0.0.5:1234", header: "8.8.4.4,10.0.0.2", wantIP: "8.8.4.4", wantAddrs: []string{"8.8.4.4", "10.0.0.2", "10.0.0.5:1234"}},
		{name: "non IP tokens are skipped", peer: "10.0.0.5:1234", header: "8.8.4.4, unknown", wantIP: "8.8.4.4", wantAddrs: []string{"8.8.4.4", "unknown", "10.0.0.5:1234"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.trust != nil {
				previous := TrustForwardedFor
				TrustForwardedFor = tt.trust
				t.Cleanup(func() { TrustForwardedFor = previous })
			}

			ch := &channel.DefaultChannel{}
			ch.Init()
			req := httptest.NewRequest(MethodGet, "/", nil)
			req.RemoteAddr = tt.peer
			req.Header.Set("X-Forwarded-For", cmp.Or(tt.header, "8.8.8.8, 10.0.0.2"))

			wrapped := WrapRequest(ch, req)

			require.Equal(t, tt.wantIP, wrapped.RemoteIP().String())
			require.Equal(t, tt.wantAddrs, wrapped.RemoteAddrs())
		})
	}
}
