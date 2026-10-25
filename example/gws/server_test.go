package example

import (
	"bytes"
	"encoding/json"
	"io"
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
	websocket "github.com/yetiz-org/gone/gws"
)

type observedClientTask struct {
	ClientHandlerTask
	pong chan struct{}
	text chan []byte
}

func (h *observedClientTask) WSPong(ctx channel.HandlerContext, message *websocket.PongMessage, params map[string]any) {
	h.pong <- struct{}{}
}

func (h *observedClientTask) WSText(ctx channel.HandlerContext, message *websocket.DefaultMessage, params map[string]any) {
	h.text <- bytes.Clone(message.Message)
}

func TestServer_Start(t *testing.T) {
	reservation, err := net.Listen("tcp4", "127.0.0.1:0")
	require.NoError(t, err)
	addr := reservation.Addr().(*net.TCPAddr)
	require.NoError(t, reservation.Close())
	serverBootstrap := channel.NewServerBootstrap()
	serverBootstrap.ChannelType(&ghttp.ServerChannel{}).SetParams(websocket.ParamCheckOrigin, false)
	counts := &ServerChildCountHandler{}
	serverBootstrap.ChildHandler(channel.NewInitializer(func(ch channel.Channel) {
		ch.Pipeline().AddLast("counts", counts).AddLast("dispatch", ghttp.NewDispatchHandler(NewRoute())).AddLast("upgrade", &websocket.UpgradeProcessor{})
	}))
	bound := serverBootstrap.Bind(addr)
	require.True(t, bound.AwaitTimeout(5*time.Second).IsSuccess(), "bind: %v", bound.Error())
	server := bound.Channel()
	transport := &http2.Transport{}
	client := &http2.Client{Transport: transport, Timeout: 10 * time.Second}
	t.Cleanup(func() {
		transport.CloseIdleConnections()
		server.Close().AwaitTimeout(5 * time.Second)
		assert.True(t, server.CloseFuture().AwaitTimeout(5*time.Second).IsSuccess())
	})
	for _, tt := range []struct {
		path, body string
		status     int
	}{
		{"", "feeling good", 200},
		{"/home", "/home", 200},
		{"/v1/home", "/v1/home", 200},
		{"/static/home", "/static/home", 200},
		{"/homes", "", 404},
	} {
		req, err := http2.NewRequestWithContext(t.Context(), "GET", "http://"+addr.String()+tt.path, nil)
		require.NoError(t, err)
		resp, err := client.Do(req)
		require.NoError(t, err)
		data, err := io.ReadAll(resp.Body)
		closeErr := resp.Body.Close()
		require.NoError(t, err)
		require.NoError(t, closeErr)
		assert.Equal(t, tt.status, resp.StatusCode)
		if tt.body != "" {
			assert.Equal(t, tt.body, string(data))
		}
		transport.CloseIdleConnections()
	}

	connect := func() (connected channel.Channel, task *observedClientTask) {
		task = &observedClientTask{pong: make(chan struct{}, 1), text: make(chan []byte, 1)}
		bootstrap := channel.NewBootstrap()
		bootstrap.ChannelType(&websocket.Channel{})
		bootstrap.Handler(channel.NewInitializer(func(ch channel.Channel) {
			ch.Pipeline().AddLast("handler", websocket.NewInvokeHandler(task, nil))
		}))
		future := bootstrap.Connect(nil, &websocket.WSCustomConnectConfig{Url: "ws://" + addr.String() + "/echo"})
		if !assert.True(t, future.AwaitTimeout(5*time.Second).IsSuccess(), "connect: %v", future.Error()) {
			return nil, task
		}
		connected = future.Channel()
		select {
		case <-task.pong:
		case <-time.After(5 * time.Second):
			t.Error("server did not answer client ping")
		}
		return connected, task
	}
	main, task := connect()
	require.NotNil(t, main)
	clients := make([]channel.Channel, 51)
	clients[0] = main
	defer func() {
		for _, ch := range clients {
			if ch == nil {
				continue
			}
			if !ch.CloseFuture().IsDone() {
				assert.True(t, ch.Disconnect().AwaitTimeout(5*time.Second).IsSuccess())
			}
			assert.True(t, ch.CloseFuture().AwaitTimeout(5*time.Second).IsSuccess())
		}
	}()
	require.True(t, main.Write(&websocket.DefaultMessage{MessageType: websocket.TextMessageType, Message: []byte("write data")}).AwaitTimeout(5*time.Second).IsSuccess())
	select {
	case data := <-task.text:
		var echo struct {
			Message string `json:"message"`
		}
		require.NoError(t, json.Unmarshal(data, &echo))
		assert.Equal(t, "write data", echo.Message)
	case <-time.After(5 * time.Second):
		t.Fatal("server did not echo text")
	}
	require.True(t, main.Write(&websocket.DefaultMessage{MessageType: websocket.BinaryMessageType}).AwaitTimeout(5*time.Second).IsSuccess())
	var workers sync.WaitGroup
	for i := range 50 {
		workers.Go(func() { clients[i+1], _ = connect() })
	}
	workers.Wait()
	for _, ch := range clients {
		assert.NotNil(t, ch)
	}
	require.True(t, main.Write(&websocket.CloseMessage{MessageType: websocket.CloseMessageType, Message: []byte("text"), CloseCode: websocket.CloseNormalClosure}).AwaitTimeout(5*time.Second).IsSuccess())
	for _, ch := range clients {
		if ch != nil && !ch.CloseFuture().IsDone() {
			assert.True(t, ch.Disconnect().AwaitTimeout(5*time.Second).IsSuccess())
		}
	}
	for _, ch := range clients {
		if ch != nil {
			assert.True(t, ch.CloseFuture().AwaitTimeout(5*time.Second).IsSuccess())
		}
	}
	require.True(t, server.CloseFuture().AwaitTimeout(5*time.Second).IsSuccess())
	assert.Zero(t, atomic.LoadInt32(&counts.regTrigCount))
	assert.Zero(t, atomic.LoadInt32(&counts.actTrigCount))
}
