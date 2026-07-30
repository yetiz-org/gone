package ghttp

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yetiz-org/gone/channel"
	"github.com/yetiz-org/gone/ghttp/httpheadername"
	"github.com/yetiz-org/gone/ghttp/httpstatus"
	buf "github.com/yetiz-org/goth-bytebuf"
)

func newGZipTestPack(t *testing.T, body string, contentType string) (*Pack, *channel.MockHandlerContext, channel.Future) {
	t.Helper()

	req, err := http.NewRequest(http.MethodGet, "http://example.test", nil)
	require.NoError(t, err)
	req.Header.Set(httpheadername.AcceptEncoding, "gzip")

	resp := &Response{
		request: &Request{request: req},
		header:  http.Header{},
		body:    buf.NewByteBufString(body),
	}
	resp.SetHeader(httpheadername.ContentType, contentType)

	pack := &Pack{
		Request:  resp.request,
		Response: resp,
		Params:   map[string]any{},
	}
	ctx := channel.NewMockHandlerContext()
	future := channel.NewFuture(nil)
	ctx.On("Write", pack, future).Return(future)

	return pack, ctx, future
}

func TestGZipHandler_UsesConfiguredThreshold(t *testing.T) {
	body := strings.Repeat("a", 256)
	pack, ctx, future := newGZipTestPack(t, body, "application/json")
	handler := &GZipHandler{CompressThreshold: 1024}

	handler.Write(ctx, pack, future)

	require.Empty(t, pack.Response.GetHeader(httpheadername.ContentEncoding))
	require.Equal(t, body, string(pack.Response.Body().Bytes()))
	ctx.AssertExpectations(t)
}

func TestGZipHandler_DefaultThresholdAppliesWhenWriteCalledDirectly(t *testing.T) {
	body := strings.Repeat("a", 64)
	pack, ctx, future := newGZipTestPack(t, body, "application/json")
	handler := &GZipHandler{}

	handler.Write(ctx, pack, future)

	require.Empty(t, pack.Response.GetHeader(httpheadername.ContentEncoding))
	require.Equal(t, body, string(pack.Response.Body().Bytes()))
	ctx.AssertExpectations(t)
}

func TestGZipHandler_SkipsBinaryContentTypes(t *testing.T) {
	body := strings.Repeat("a", 2048)
	pack, ctx, future := newGZipTestPack(t, body, "application/pdf")
	handler := &GZipHandler{CompressThreshold: 128}

	handler.Write(ctx, pack, future)

	require.Empty(t, pack.Response.GetHeader(httpheadername.ContentEncoding))
	require.Equal(t, body, string(pack.Response.Body().Bytes()))
	ctx.AssertExpectations(t)
}

func TestGZipHandler_CompressesTextContent(t *testing.T) {
	body := strings.Repeat("hello json ", 256)
	pack, ctx, future := newGZipTestPack(t, body, "application/json; charset=utf-8")
	handler := &GZipHandler{CompressThreshold: 128}

	handler.Write(ctx, pack, future)

	require.Equal(t, "gzip", pack.Response.GetHeader(httpheadername.ContentEncoding))
	require.True(t, headerHasToken(pack.Response.Header(), httpheadername.Vary, httpheadername.AcceptEncoding))
	require.Equal(t, pack.Response.Body().ReadableBytes(), mustAtoi(t, pack.Response.GetHeader(httpheadername.ContentLength)))
	require.NotEmpty(t, pack.Params["[gone-http]compress_time"])

	reader, err := gzip.NewReader(bytes.NewReader(pack.Response.Body().Bytes()))
	require.NoError(t, err)
	defer reader.Close()

	decompressed, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.Equal(t, body, string(decompressed))
	ctx.AssertExpectations(t)
}

func TestGZipHandler_AddsVaryWhenClientDoesNotAcceptGzip(t *testing.T) {
	body := strings.Repeat("hello json ", 256)
	pack, ctx, future := newGZipTestPack(t, body, "application/json")
	pack.Request.Header().Set(httpheadername.AcceptEncoding, "br")
	handler := &GZipHandler{CompressThreshold: 128}

	handler.Write(ctx, pack, future)

	require.Empty(t, pack.Response.GetHeader(httpheadername.ContentEncoding))
	require.True(t, headerHasToken(pack.Response.Header(), httpheadername.Vary, httpheadername.AcceptEncoding))
	ctx.AssertExpectations(t)
}

func TestGZipHandler_PreservesExistingVary(t *testing.T) {
	body := strings.Repeat("hello json ", 256)
	pack, ctx, future := newGZipTestPack(t, body, "application/json")
	pack.Response.SetHeader(httpheadername.Vary, "origin")
	handler := &GZipHandler{CompressThreshold: 128}

	handler.Write(ctx, pack, future)

	require.True(t, headerHasToken(pack.Response.Header(), httpheadername.Vary, "origin"))
	require.True(t, headerHasToken(pack.Response.Header(), httpheadername.Vary, httpheadername.AcceptEncoding))
	ctx.AssertExpectations(t)
}

func TestGZipHandler_DefaultThresholdFromAdded(t *testing.T) {
	handler := &GZipHandler{}

	handler.Added(channel.NewMockHandlerContext())

	require.Equal(t, 128, handler.CompressThreshold)
}

func TestGZipHandler_AcceptsGzipHeader(t *testing.T) {
	handler := &GZipHandler{}
	tests := []struct {
		name   string
		header string
		want   bool
	}{
		{name: "empty", header: "", want: false},
		{name: "gzip", header: "gzip", want: true},
		{name: "gzip with q", header: "br, gzip;q=0.8", want: true},
		{name: "gzip q zero", header: "gzip;q=0, br", want: false},
		{name: "wildcard", header: "br, *;q=0.5", want: true},
		{name: "explicit gzip beats wildcard", header: "gzip;q=0, *;q=1", want: false},
		{name: "substring not accepted", header: "notgzip", want: false},
		{name: "case insensitive", header: "GZip; Q=1", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, handler.acceptsGzip(tt.header))
		})
	}
}

func TestGZipHandler_SkipsWhenGzipNotAcceptable(t *testing.T) {
	body := strings.Repeat("hello json ", 256)
	pack, ctx, future := newGZipTestPack(t, body, "application/json")
	pack.Request.Header().Set(httpheadername.AcceptEncoding, "gzip;q=0, br")
	handler := &GZipHandler{CompressThreshold: 128}

	handler.Write(ctx, pack, future)

	require.Empty(t, pack.Response.GetHeader(httpheadername.ContentEncoding))
	require.Equal(t, body, string(pack.Response.Body().Bytes()))
	ctx.AssertExpectations(t)
}

func TestGZipHandler_SkipsAlreadyEncodedResponse(t *testing.T) {
	body := strings.Repeat("hello json ", 256)
	pack, ctx, future := newGZipTestPack(t, body, "application/json")
	pack.Response.SetHeader(httpheadername.ContentEncoding, "br")
	handler := &GZipHandler{CompressThreshold: 128}

	handler.Write(ctx, pack, future)

	require.Equal(t, "br", pack.Response.GetHeader(httpheadername.ContentEncoding))
	require.Equal(t, body, string(pack.Response.Body().Bytes()))
	ctx.AssertExpectations(t)
}

func TestGZipHandler_SkipsPartialContentRangeResponse(t *testing.T) {
	body := strings.Repeat("range body ", 256)
	pack, ctx, future := newGZipTestPack(t, body, "text/plain")
	pack.Response.SetStatusCode(httpstatus.PartialContent)
	pack.Response.SetHeader(httpheadername.ContentRange, "bytes 0-99/1000")
	pack.Response.SetHeader(httpheadername.ContentLength, strconv.Itoa(len(body)))
	handler := &GZipHandler{CompressThreshold: 128}

	handler.Write(ctx, pack, future)

	require.Empty(t, pack.Response.GetHeader(httpheadername.ContentEncoding))
	require.Equal(t, body, string(pack.Response.Body().Bytes()))
	require.Equal(t, strconv.Itoa(len(body)), pack.Response.GetHeader(httpheadername.ContentLength))
	ctx.AssertExpectations(t)
}

func TestGZipHandler_CompressesStructuredJsonContentType(t *testing.T) {
	body := strings.Repeat("problem json ", 256)
	pack, ctx, future := newGZipTestPack(t, body, "application/problem+json")
	pack.Params = nil
	handler := &GZipHandler{CompressThreshold: 128}

	handler.Write(ctx, pack, future)

	require.Equal(t, "gzip", pack.Response.GetHeader(httpheadername.ContentEncoding))
	ctx.AssertExpectations(t)
}

func TestGZipHandler_GzipWriteConcurrentPoolUse(t *testing.T) {
	handler := &GZipHandler{CompressThreshold: 128}
	const goroutines = 32
	const iterations = 50
	payload := strings.Repeat(`{"title":"sample","body":"compressible payload"}`, 64)
	errCh := make(chan error, goroutines*iterations)
	var wg sync.WaitGroup

	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range iterations {
				gzBody, err := handler.gzipWrite(buf.NewByteBufString(payload))
				if err != nil {
					errCh <- err
					continue
				}
				reader, err := gzip.NewReader(bytes.NewReader(gzBody.Bytes()))
				if err != nil {
					errCh <- err
					continue
				}
				decompressed, err := io.ReadAll(reader)
				closeErr := reader.Close()
				if err != nil {
					errCh <- err
					continue
				}
				if closeErr != nil {
					errCh <- closeErr
					continue
				}
				if string(decompressed) != payload {
					errCh <- io.ErrUnexpectedEOF
				}
			}
		}()
	}
	wg.Wait()
	close(errCh)

	for err := range errCh {
		require.NoError(t, err)
	}
}

func mustAtoi(t *testing.T, value string) int {
	t.Helper()

	n, err := strconv.Atoi(value)
	require.NoError(t, err)
	return n
}

func headerHasToken(header http.Header, name string, token string) bool {
	for _, value := range header.Values(name) {
		for part := range strings.SplitSeq(value, ",") {
			if strings.EqualFold(strings.TrimSpace(part), token) {
				return true
			}
		}
	}
	return false
}

var gzipBenchSink int

func BenchmarkGZipWrite_Current(b *testing.B) {
	payload := buf.NewByteBufString(strings.Repeat(`{"title":"sample","body":"compressible payload"}`, 128))
	handler := &GZipHandler{CompressThreshold: 128}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		gzBody, err := handler.gzipWrite(payload)
		if err != nil {
			b.Fatal(err)
		}
		gzipBenchSink += gzBody.ReadableBytes()
	}
}

func BenchmarkGZipWrite_Baseline(b *testing.B) {
	payload := buf.NewByteBufString(strings.Repeat(`{"title":"sample","body":"compressible payload"}`, 128))

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		gzBody := baselineGZipWrite(payload)
		gzipBenchSink += gzBody.ReadableBytes()
	}
}

func baselineGZipWrite(buffer buf.ByteBuf) buf.ByteBuf {
	est := buffer.ReadableBytes() / 3
	if est < 128 {
		est = 128
	}
	gzBuffer := buf.NewByteBuf(make([]byte, 0, est))
	writer, _ := gzip.NewWriterLevel(gzBuffer, gzip.BestSpeed)
	_, _ = writer.Write(buffer.Bytes())
	_ = writer.Flush()
	_ = writer.Close()
	return gzBuffer
}
