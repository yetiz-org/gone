package ghttp

import (
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yetiz-org/gone/channel"
	buf "github.com/yetiz-org/goth-bytebuf"
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

func TestLogHandlerResponseBodyPrefix(t *testing.T) {
	payload := []byte("decoded response body")
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	_, err := writer.Write(payload)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	for _, test := range []struct {
		name     string
		encoding string
		wire     []byte
		limit    int
		want     string
		panics   bool
	}{
		{name: "plain below cap", wire: payload, limit: len(payload) + 1, want: string(payload)},
		{name: "plain at cap", wire: payload, limit: len(payload), want: string(payload)},
		{name: "plain over cap", wire: payload, limit: 7, want: string(payload[:7])},
		{name: "gzip below cap", encoding: "gzip", wire: compressed.Bytes(), limit: len(payload) + 1, want: string(payload)},
		{name: "gzip at cap", encoding: "gzip", wire: compressed.Bytes(), limit: len(payload), want: string(payload)},
		{name: "gzip over cap", encoding: "gzip", wire: compressed.Bytes(), limit: 7, want: string(payload[:7])},
		{name: "gzip zero cap", encoding: "gzip", wire: compressed.Bytes(), limit: 0},
		{name: "invalid gzip header", encoding: "gzip", wire: payload, limit: len(payload)},
		{name: "plain negative cap", wire: payload, limit: -1, panics: true},
		{name: "gzip negative cap", encoding: "gzip", wire: compressed.Bytes(), limit: -1, panics: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			req := WrapRequest(nil, httptest.NewRequest(MethodGet, "/response?limit=body", nil))
			resp := NewResponse(req).SetStatusCode(http.StatusAccepted)
			resp.SetHeader("Content-Encoding", test.encoding)
			resp.SetHeader("X-Response", "retained")
			body := buf.NewByteBufString("unread prefix:")
			body.ReadBytes(body.ReadableBytes())
			body.WriteBytes(test.wire)
			resp.SetBody(body)
			readerIndex := body.ReaderIndex()
			headers := resp.Header().Clone()
			handler := NewLogHandler(true)
			handler.RespMaxBodySize = test.limit

			if test.panics {
				require.Panics(t, func() { handler.constructResp(resp) })
			} else {
				logged := handler.constructResp(resp)
				require.Equal(t, test.want, logged.Body)
				require.Equal(t, len(test.wire), logged.OutBodyLength)
				if test.encoding == "gzip" {
					require.Equal(t, len(test.want), logged.PreCompressLength)
				} else {
					require.Zero(t, logged.PreCompressLength)
				}

				require.Equal(t, http.StatusAccepted, logged.StatusCode)
				require.Equal(t, req.RequestURI(), logged.URI)
				require.Equal(t, test.encoding, logged.Headers["Content-Encoding"])
				require.Equal(t, "retained", logged.Headers["X-Response"])
			}

			require.Equal(t, test.wire, resp.Body().Bytes())
			require.Equal(t, readerIndex, resp.Body().ReaderIndex())
			require.Equal(t, headers, resp.Header())
		})
	}
}

func TestLogHandlerGzipBodyAllocationBound(t *testing.T) {
	const decodedSize = 4 << 20
	const logCap = 64
	const allocationBudget = decodedSize / 4
	payload := bytes.Repeat([]byte("x"), decodedSize)
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	_, err := writer.Write(payload)
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	req := WrapRequest(nil, httptest.NewRequest(MethodGet, "/compressed", nil))
	resp := NewResponse(req).SetHeader("Content-Encoding", "gzip")
	resp.SetBody(buf.NewByteBuf(compressed.Bytes()))
	handler := NewLogHandler(true)
	handler.RespMaxBodySize = logCap
	handler.constructResp(resp)

	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	logged := handler.constructResp(resp)
	runtime.ReadMemStats(&after)
	allocated := after.TotalAlloc - before.TotalAlloc
	t.Logf("decoded=%d cap=%d allocated=%d budget=%d", decodedSize, logCap, allocated, allocationBudget)
	require.Equal(t, string(payload[:logCap]), logged.Body)
	require.Equal(t, logCap, logged.PreCompressLength)
	require.Equal(t, len(compressed.Bytes()), logged.OutBodyLength)
	require.LessOrEqual(t, allocated, uint64(allocationBudget), "logging a prefix must not allocate the full decoded response")
}
