package example

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	http2 "net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yetiz-org/gone/channel"
	"github.com/yetiz-org/gone/ghttp"
	"github.com/yetiz-org/gone/ghttp/httpheadername"
)

func gUnzipData(data []byte) (resData []byte, err error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}

func startExampleHTTPServer(t *testing.T, bootstrap channel.ServerBootstrap) (server channel.Channel, baseURL string, client *http2.Client) {
	t.Helper()
	reservation, err := net.Listen("tcp4", "127.0.0.1:0")
	require.NoError(t, err)
	addr := reservation.Addr().(*net.TCPAddr)
	require.NoError(t, reservation.Close())
	bound := bootstrap.Bind(addr)
	require.True(t, bound.AwaitTimeout(5*time.Second).IsSuccess(), "bind: %v", bound.Error())
	server = bound.Channel()
	transport := &http2.Transport{DisableCompression: true}
	client = &http2.Client{Transport: transport, Timeout: 10 * time.Second}
	t.Cleanup(func() {
		transport.CloseIdleConnections()
		assert.True(t, server.Close().AwaitTimeout(5*time.Second).IsSuccess())
		assert.True(t, server.CloseFuture().AwaitTimeout(5*time.Second).IsSuccess())
	})
	return server, "http://" + addr.String(), client
}

func TestServer_Start(t *testing.T) {
	bootstrap := channel.NewServerBootstrap()
	bootstrap.ChannelType(&ghttp.ServerChannel{})
	bootstrap.Handler(channel.NewInitializer(func(ch channel.Channel) {
		ch.Pipeline().AddLast("INDICATE_HANDLER_INBOUND", &channel.IndicateHandlerInbound{}).AddLast("INDICATE_HANDLER_OUTBOUND", &channel.IndicateHandlerOutbound{})
	}))
	clientCountHandler := &ServerChildCountHandler{}
	bootstrap.ChildHandler(channel.NewInitializer(func(ch channel.Channel) {
		ch.Pipeline().AddLast("CLIENT_COUNT_HANDLER", clientCountHandler)
		ch.Pipeline().AddLast("INDICATE_HANDLER_INBOUND", &channel.IndicateHandlerInbound{})
		ch.Pipeline().AddLast("NET_STATUS_INBOUND", &channel.NetStatusInbound{})
		ch.Pipeline().AddLast("GZIP_HANDLER", new(ghttp.GZipHandler))
		ch.Pipeline().AddLast("LOG_HANDLER", ghttp.NewLogHandler(false))
		ch.Pipeline().AddLast("DISPATCHER", ghttp.NewDispatchHandler(NewRoute()))
		ch.Pipeline().AddLast("NET_STATUS_OUTBOUND", &channel.NetStatusOutbound{})
		ch.Pipeline().AddLast("INDICATE_HANDLER_OUTBOUND", &channel.IndicateHandlerOutbound{})
	}))
	server, baseURL, client := startExampleHTTPServer(t, bootstrap)
	request := func(method, path, wantBody string, status int, gzipRequested, authorize, sse bool) {
		req, err := http2.NewRequestWithContext(t.Context(), method, baseURL+path, nil)
		if !assert.NoError(t, err) {
			return
		}
		if gzipRequested {
			req.Header.Set(httpheadername.AcceptEncoding, "gzip")
		}
		if authorize {
			req.Header.Set(httpheadername.Authorization, "!!!!")
		}
		resp, err := client.Do(req)
		if !assert.NoError(t, err) {
			return
		}
		defer resp.Body.Close()
		data, err := io.ReadAll(resp.Body)
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, status, resp.StatusCode)
		if resp.Header.Get(httpheadername.ContentEncoding) == "gzip" {
			assert.NotEmpty(t, data)
			data, err = gUnzipData(data)
			if !assert.NoError(t, err) {
				return
			}
		}
		if wantBody != "" {
			assert.Equal(t, wantBody, string(data))
		}
		if sse {
			assert.Equal(t, "true", resp.Header.Get("Validate"))
		}
	}
	var workers sync.WaitGroup
	for i := range 10 {
		workers.Go(func() {
			v := fmt.Sprintf("%d", rand.Uint64())
			request("GET", "/long?v="+v, longMsg+v, 200, i%2 == 0, false, false)
		})
	}
	for _, tt := range []struct {
		path, body    string
		count, status int
	}{
		{"", "feeling good", 10, 200},
		{"/home", "/home", 10, 200},
		{"/v1/home", "/v1/home", 5, 200},
		{"/homes", "", 10, 404},
		{"/400", "", 1, 400},
	} {
		for range tt.count {
			workers.Go(func() { request("GET", tt.path, tt.body, tt.status, false, false, false) })
		}
	}
	workers.Go(func() {
		expected := "event: event\ndata: 0\n\nevent: event\ndata: 1\n\nevent: event\ndata: 2\n\nevent: event2\ndata: 4\n\nevent: event2\ndata: 5\ndata: 5-1\n\nevent: event2\ndata: 6\n\n"
		request("GET", "/sse", expected, 200, false, false, true)
	})
	workers.Wait()
	for range 50 {
		workers.Go(func() { request("POST", "", "feeling good", 200, false, true, false) })
	}
	workers.Wait()
	request("GET", "/close", "/close", 200, false, false, false)
	client.CloseIdleConnections()
	require.True(t, server.CloseFuture().AwaitTimeout(5*time.Second).IsSuccess())
	assert.Zero(t, atomic.LoadInt32(&clientCountHandler.regTrigCount))
	assert.Zero(t, atomic.LoadInt32(&clientCountHandler.actTrigCount))
}

func TestServer_BodyLimit(t *testing.T) {
	bootstrap := channel.NewServerBootstrap()
	bootstrap.ChannelType(&ghttp.ServerChannel{})
	bootstrap.Handler(channel.NewInitializer(func(ch channel.Channel) {
		ch.Pipeline().AddLast("INDICATE_HANDLER_INBOUND", &channel.IndicateHandlerInbound{}).AddLast("INDICATE_HANDLER_OUTBOUND", &channel.IndicateHandlerOutbound{})
	}))
	bootstrap.SetParams(ghttp.ParamMaxBodyBytes, 10)
	clientCountHandler := &ServerChildCountHandler{}
	bootstrap.ChildHandler(channel.NewInitializer(func(ch channel.Channel) {
		ch.Pipeline().AddLast("CLIENT_COUNT_HANDLER", clientCountHandler)
		ch.Pipeline().AddLast("INDICATE_HANDLER_INBOUND", &channel.IndicateHandlerInbound{})
		ch.Pipeline().AddLast("NET_STATUS_INBOUND", &channel.NetStatusInbound{})
		ch.Pipeline().AddLast("GZIP_HANDLER", new(ghttp.GZipHandler))
		ch.Pipeline().AddLast("LOG_HANDLER", ghttp.NewLogHandler(false))
		ch.Pipeline().AddLast("DISPATCHER", ghttp.NewDispatchHandler(NewRoute()))
		ch.Pipeline().AddLast("NET_STATUS_OUTBOUND", &channel.NetStatusOutbound{})
		ch.Pipeline().AddLast("INDICATE_HANDLER_OUTBOUND", &channel.IndicateHandlerOutbound{})
	}))

	_, baseURL, client := startExampleHTTPServer(t, bootstrap)
	request, err := http2.NewRequestWithContext(t.Context(), "POST", baseURL, bytes.NewBufferString("this is more than 10 characters"))
	require.NoError(t, err)
	resp, err := client.Do(request)
	require.NoError(t, err)
	defer resp.Body.Close()
	_, err = io.Copy(io.Discard, resp.Body)
	require.NoError(t, err)
	assert.Equal(t, 413, resp.StatusCode)
}
