package gmcp

import (
	"bufio"
	"context"
	"encoding/json/v2"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/yetiz-org/gone/channel"
	"github.com/yetiz-org/gone/erresponse"
	"github.com/yetiz-org/gone/ghttp"
)

type _LiveMCPTask struct {
	ghttp.DefaultHTTPHandlerTask
	_Server   *Server
	_Returned chan struct{}
}

func (h *_LiveMCPTask) Post(ctx channel.HandlerContext, req *ghttp.Request, resp *ghttp.Response, params map[string]any) (errResponse ghttp.ErrorResponse) {
	defer func() { h._Returned <- struct{}{} }()
	writer, ok := h.RawMode(req, resp, params)
	if !ok {
		return erresponse.ServerError
	}

	h._Server.Serve(writer, Caller{Request: req, HandlerContext: ctx})
	return nil
}

func _StartLiveMCP(t *testing.T, options Options) (endpoint string, client *http.Client, returned <-chan struct{}) {
	t.Helper()
	tools := ghttp.NewSimpleRoute().SetEndpoint("/empty", &_EmptyTask{})
	task := &_LiveMCPTask{_Server: _BindServer(options, tools), _Returned: make(chan struct{}, 1)}
	route := ghttp.NewSimpleRoute().SetEndpoint("/mcp", task)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	address := listener.Addr().(*net.TCPAddr)
	require.NoError(t, listener.Close())
	bootstrap := channel.NewServerBootstrap()
	bootstrap.ChannelType(&ghttp.ServerChannel{})
	bootstrap.ChildHandler(channel.NewInitializer(func(ch channel.Channel) {
		ch.Pipeline().AddLast("DISPATCHER", ghttp.NewDispatchHandler(route))
	}))
	bound := bootstrap.Bind(address).Sync()
	require.NoError(t, bound.Error())
	t.Cleanup(func() { bound.Channel().Close().Sync() })
	transport := &http.Transport{Proxy: nil}
	t.Cleanup(transport.CloseIdleConnections)
	return "http://" + address.String() + "/mcp", &http.Client{Transport: transport, Timeout: 5 * time.Second}, task._Returned
}

func _ListenRequest(t *testing.T, ctx context.Context, endpoint string, notifications string) (request *http.Request) {
	t.Helper()
	body := fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"subscriptions/listen","params":{"notifications":%s,"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientInfo":{"name":"live-test","version":"1"},"io.modelcontextprotocol/clientCapabilities":{}}}}`, notifications)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	request.Header.Set("MCP-Protocol-Version", "2026-07-28")
	request.Header.Set("Mcp-Method", "subscriptions/listen")
	return request
}

func _ReadLiveEvent(t *testing.T, reader *bufio.Reader) (event map[string]any, wireBytes int) {
	t.Helper()
	var data []string
	for {
		line, err := reader.ReadString('\n')
		require.NoError(t, err, "a complete SSE event must arrive before the response closes")
		wireBytes += len(line)
		line = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
		if line == "" {
			if len(data) == 0 {
				continue
			}

			require.NoError(t, json.Unmarshal([]byte(strings.Join(data, "\n")), &event))
			return event, wireBytes
		}

		if value, ok := strings.CutPrefix(line, "data:"); ok {
			data = append(data, strings.TrimPrefix(value, " "))
		}
	}
}

func TestServerLiveSubscription(t *testing.T) {
	endpoint, client, returned := _StartLiveMCP(t, Options{})
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	response, err := client.Do(_ListenRequest(t, ctx, endpoint, `{"toolsListChanged":true}`))
	require.NoError(t, err)
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(response.Body)
		t.Fatalf("HTTP %d: %s", response.StatusCode, data)
	}

	require.Equal(t, "text/event-stream", response.Header.Get("Content-Type"))
	ack, _ := _ReadLiveEvent(t, bufio.NewReader(response.Body))
	require.Equal(t, "notifications/subscriptions/acknowledged", ack["method"])
	params, ok := ack["params"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, map[string]any{"toolsListChanged": true}, params["notifications"])
	select {
	case <-returned:
		t.Fatal("subscription returned before the client disconnected")
	default:
	}

	cancel()
	require.NoError(t, response.Body.Close())
	select {
	case <-returned:
	case <-time.After(3 * time.Second):
		t.Fatal("client cancellation did not release the subscription handler")
	}
}

func TestServerLiveSubscriptionCompletion(t *testing.T) {
	const limit = 256
	endpoint, client, returned := _StartLiveMCP(t, Options{MaxResponseBytes: limit})
	response, err := client.Do(_ListenRequest(t, t.Context(), endpoint, `{}`))
	require.NoError(t, err)
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(response.Body)
		t.Fatalf("HTTP %d: %s", response.StatusCode, data)
	}

	require.Equal(t, "text/event-stream", response.Header.Get("Content-Type"))
	reader := bufio.NewReader(response.Body)
	ack, firstBytes := _ReadLiveEvent(t, reader)
	require.Equal(t, "notifications/subscriptions/acknowledged", ack["method"])
	result, secondBytes := _ReadLiveEvent(t, reader)
	require.Equal(t, float64(1), result["id"])
	require.Contains(t, result, "result")
	require.NotContains(t, result, "error")
	require.LessOrEqual(t, firstBytes, limit)
	require.LessOrEqual(t, secondBytes, limit)
	require.Greater(t, firstBytes+secondBytes, limit, "the limit applies per event, not to the stream lifetime")
	remaining, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.Empty(t, remaining, "completion must not append a second HTTP/JSON response")
	select {
	case <-returned:
	case <-time.After(3 * time.Second):
		t.Fatal("completed subscription handler did not return")
	}
}

func TestServerLiveSubscriptionLimit(t *testing.T) {
	endpoint, client, returned := _StartLiveMCP(t, Options{MaxResponseBytes: 64})
	response, err := client.Do(_ListenRequest(t, t.Context(), endpoint, `{"toolsListChanged":true}`))
	require.NoError(t, err)
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusInternalServerError, response.StatusCode)
	require.Equal(t, "application/json", response.Header.Get("Content-Type"))
	require.JSONEq(t, `{"error":{"category":"internal_error"}}`, string(data), "an oversized first event must be replaced before the stream is committed")
	require.LessOrEqual(t, len(data), 64)
	select {
	case <-returned:
	case <-time.After(3 * time.Second):
		t.Fatal("oversized event did not terminate the subscription handler")
	}
}
