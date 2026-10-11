package ghttp

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yetiz-org/gone/channel"
)

func TestLogHandlerRedactsCredentialHeaders(t *testing.T) {
	ch := &channel.DefaultChannel{}
	ch.Init()
	raw := httptest.NewRequest(MethodGet, "/", nil)
	raw.Header.Set("Authorization", "Bearer secret")
	raw.Header.Set("Cookie", "DEFAULT=secret")
	raw.Header.Set("Accept", "text/plain")
	req := WrapRequest(ch, raw)
	resp := NewResponse(req)
	resp.SetHeader("Set-Cookie", "DEFAULT=secret")
	resp.SetHeader("Content-Type", "text/plain")

	handler := NewLogHandler(false)
	reqLog, respLog := handler.constructReq(req), handler.constructResp(resp)
	require.Equal(t, LogHandlerRedacted, reqLog.Headers["Authorization"])
	require.Equal(t, LogHandlerRedacted, reqLog.Headers["Cookie"])
	require.Equal(t, "text/plain", reqLog.Headers["Accept"])
	require.Equal(t, LogHandlerRedacted, respLog.Headers["Set-Cookie"])
	require.Equal(t, "text/plain", respLog.Headers["Content-Type"])

	handler.RedactHeaders = nil
	require.Equal(t, "Bearer secret", handler.constructReq(req).Headers["Authorization"], "clearing RedactHeaders logs every value")
}
