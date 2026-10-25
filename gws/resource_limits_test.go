package gws

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
	"github.com/yetiz-org/gone/channel"
	"github.com/yetiz-org/gone/ghttp"
)

func resourceWebSocketClient(t *testing.T, params map[channel.ParamKey]any, serve func(*websocket.Conn, <-chan struct{}), consume ...func(channel.HandlerContext, any)) (client *Channel) {
	t.Helper()
	done := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{WriteBufferSize: 128}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		serve(conn, done)
	}))
	bootstrap := channel.NewBootstrap().ChannelType(&Channel{})
	for key, value := range params {
		bootstrap.SetParams(key, value)
	}
	bootstrap.Handler(channel.NewInitializer(func(ch channel.Channel) {
		read := func(channel.HandlerContext, any) {}
		if len(consume) > 0 {
			read = consume[0]
		}
		ch.Pipeline().AddLast("consume", channel.NewRWHandler(read, nil))
	}))
	future := bootstrap.Connect(nil, &WSCustomConnectConfig{Url: "ws" + strings.TrimPrefix(server.URL, "http")})
	future.Sync()
	require.NoError(t, future.Error())
	client = future.Channel().(*Channel)
	t.Cleanup(func() { close(done); client.Close().Sync(); server.Close() })
	return client
}

func TestWebSocketAssemblyDeadlineIgnoresPing(t *testing.T) {
	client := resourceWebSocketClient(t, map[channel.ParamKey]any{
		channel.ParamKey("frame_assembly_timeout"): 150,
		channel.ParamReadIdleTimeout:               1000,
	}, func(conn *websocket.Conn, done <-chan struct{}) {
		writer, err := conn.NextWriter(websocket.TextMessage)
		if err != nil {
			return
		}
		if _, err := writer.Write(make([]byte, 1024)); err != nil {
			return
		}
		for {
			select {
			case <-done:
				return
			case <-time.After(25 * time.Millisecond):
				if err := conn.WriteControl(websocket.PingMessage, []byte("alive"), time.Now().Add(time.Second)); err != nil {
					return
				}
			}
		}
	})
	select {
	case <-client.CloseFuture().Done():
	case <-time.After(700 * time.Millisecond):
		t.Fatal("Ping extended an incomplete WebSocket message assembly")
	}
}

func TestWebSocketWriteDeadlineAndQueue(t *testing.T) {
	t.Run("blocked peer", func(t *testing.T) {
		client := resourceWebSocketClient(t, map[channel.ParamKey]any{
			channel.ParamWriteTimeout:                  100,
			channel.ParamKey("max_queued_write_bytes"): 10 << 20,
		}, func(conn *websocket.Conn, done <-chan struct{}) { <-done })
		var futures []channel.Future
		for _, n := range []int{8 << 20, 1 << 20, 1 << 20, 1 << 20} {
			futures = append(futures, client.Write(&DefaultMessage{MessageType: BinaryMessageType, Message: make([]byte, n)}))
		}
		select {
		case <-futures[3].Done():
		case <-time.After(300 * time.Millisecond):
			t.Fatal("over-budget WebSocket output remained queued")
		}
		require.Error(t, futures[3].Error())
		for _, future := range futures {
			select {
			case <-future.Done():
			case <-time.After(time.Second):
				t.Fatal("blocked peer left a pending output Future")
			}
		}
		select {
		case <-client.CloseFuture().Done():
		case <-time.After(time.Second):
			t.Fatal("write deadline did not close channel")
		}
	})
	t.Run("disabled deadline clears prior deadline", func(t *testing.T) {
		received := make(chan string, 1)
		client := resourceWebSocketClient(t, map[channel.ParamKey]any{channel.ParamWriteTimeout: 0}, func(conn *websocket.Conn, done <-chan struct{}) {
			_, bs, err := conn.ReadMessage()
			if err == nil {
				received <- string(bs)
			}
			<-done
		})
		require.NoError(t, client.wsConn.SetWriteDeadline(time.Now().Add(-time.Second)))
		future := client.Write(&DefaultMessage{MessageType: TextMessageType, Message: []byte("valid")})
		select {
		case <-future.Done():
		case <-time.After(time.Second):
			t.Fatal("write did not settle")
		}
		require.NoError(t, future.Error())
		select {
		case value := <-received:
			require.Equal(t, "valid", value)
		case <-time.After(time.Second):
			t.Fatal("cleared deadline prevented valid write")
		}
	})
}

type resourceServerTask struct {
	DefaultServerHandlerTask
	connected chan channel.Channel
	pings     chan channel.Channel
	completed chan int
}

func (h *resourceServerTask) WSConnected(ch channel.Channel, req *ghttp.Request, resp *ghttp.Response, params map[string]any) {
	h.connected <- ch
}

func (h *resourceServerTask) WSPing(ctx channel.HandlerContext, message *PingMessage, params map[string]any) {
	h.pings <- ctx.Channel()
}

func (h *resourceServerTask) WSText(ctx channel.HandlerContext, message *DefaultMessage, params map[string]any) {
	h.completed <- len(message.Message)
}

func TestWebSocketSharedBudgetAndRelease(t *testing.T) {
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := probe.Addr().(*net.TCPAddr)
	require.NoError(t, probe.Close())
	task := &resourceServerTask{connected: make(chan channel.Channel, 4), pings: make(chan channel.Channel, 4), completed: make(chan int, 2)}
	route := ghttp.NewSimpleRoute().SetEndpoint("/ws", task)
	bootstrap := channel.NewServerBootstrap()
	bootstrap.ChannelType(&ghttp.ServerChannel{})
	bootstrap.SetParams(channel.ParamKey("max_in_progress_bytes"), 1700)
	bootstrap.SetParams(channel.ParamMaxConnections, 2)
	bootstrap.SetChildParams(channel.ParamKey("frame_assembly_timeout"), 2000)
	bootstrap.ChildHandler(channel.NewInitializer(func(ch channel.Channel) {
		ch.Pipeline().AddLast("dispatch", ghttp.NewDispatchHandler(route))
		ch.Pipeline().AddLast("upgrade", &UpgradeProcessor{})
	}))
	future := bootstrap.Bind(addr)
	future.Sync()
	require.NoError(t, future.Error())
	server := future.Channel()
	t.Cleanup(func() { server.Close().Sync() })
	dial := func() (*websocket.Conn, channel.Channel) {
		dialer := websocket.Dialer{WriteBufferSize: 128, HandshakeTimeout: time.Second}
		peer, _, err := dialer.Dial("ws://"+addr.String()+"/ws", nil)
		require.NoError(t, err)
		t.Cleanup(func() { peer.Close() })
		select {
		case ch := <-task.connected:
			return peer, ch
		case <-time.After(time.Second):
			t.Fatal("upgrade did not publish channel")
			return nil, nil
		}
	}
	fragment := func(peer *websocket.Conn) io.WriteCloser {
		writer, err := peer.NextWriter(websocket.TextMessage)
		require.NoError(t, err)
		_, err = writer.Write(make([]byte, 1024))
		require.NoError(t, err)
		return writer
	}
	pin := func(peer *websocket.Conn) {
		require.NoError(t, peer.WriteControl(websocket.PingMessage, []byte("p"), time.Now().Add(time.Second)))
		select {
		case <-task.pings:
		case <-time.After(time.Second):
			t.Fatal("incomplete message was not retained")
		}
	}
	first, _ := dial()
	firstWriter := fragment(first)
	pin(first)
	second, secondChannel := dial()
	fragment(second)
	select {
	case <-secondChannel.CloseFuture().Done():
	case <-time.After(400 * time.Millisecond):
		t.Fatal("another upgraded peer exceeded the server's aggregate budget")
	}
	require.NoError(t, firstWriter.Close())
	select {
	case n := <-task.completed:
		require.Equal(t, 1024, n)
	case <-time.After(time.Second):
		t.Fatal("completed WS message was not delivered")
	}
	third, thirdChannel := dial()
	fragment(third)
	pin(third)
	third.Close()
	select {
	case <-thirdChannel.CloseFuture().Done():
	case <-time.After(time.Second):
		t.Fatal("closed WS peer retained its channel")
	}
	fourth, _ := dial()
	fragment(fourth)
	pin(fourth)
}

func TestWebSocketLogURIExcludesQuery(t *testing.T) {
	ch := &channel.DefaultChannel{}
	ch.Init()
	ch.SetParam(ghttp.ParamLogURI, func(req *ghttp.Request) string { return req.Url().EscapedPath() })
	request := ghttp.WrapRequest(ch, httptest.NewRequest(http.MethodGet, "/ws%20path?code=secret", nil))
	client := &Channel{DefaultNetChannel: &channel.DefaultNetChannel{}, Request: request, Response: ghttp.NewResponse(request)}
	entry := client._NewWSLog(&DefaultMessage{}, io.ErrClosedPipe)
	require.Equal(t, "/ws%20path", entry.RequestURI)
	require.NotContains(t, entry.RequestURI, "secret")
}

func TestWebSocketSmallMessageFitsSmallBudget(t *testing.T) {
	delivered := make(chan string, 1)
	start := make(chan struct{})
	resourceWebSocketClient(t, map[channel.ParamKey]any{channel.ParamMaxInProgressBytes: 512}, func(conn *websocket.Conn, done <-chan struct{}) {
		<-start
		conn.WriteMessage(websocket.TextMessage, []byte("x"))
		<-done
	}, func(ctx channel.HandlerContext, obj any) {
		delivered <- obj.(*DefaultMessage).StringMessage()
	})
	close(start)
	select {
	case message := <-delivered:
		require.Equal(t, "x", message)
	case <-time.After(time.Second):
		t.Fatal("one-byte message exceeded 512-byte retention budget")
	}
}

func TestWebSocketReadCapacityBoundaries(t *testing.T) {
	for _, size := range []int{512, 513, 1024} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			start := make(chan struct{})
			delivered := make(chan string, 1)
			payload := strings.Repeat("x", size)
			resourceWebSocketClient(t, map[channel.ParamKey]any{ParamWSReadLimit: size}, func(conn *websocket.Conn, done <-chan struct{}) {
				<-start
				conn.WriteMessage(websocket.BinaryMessage, []byte(payload))
				<-done
			}, func(ctx channel.HandlerContext, obj any) {
				delivered <- obj.(*DefaultMessage).StringMessage()
			})
			close(start)
			select {
			case message := <-delivered:
				require.Equal(t, payload, message)
			case <-time.After(time.Second):
				t.Fatal("exact-limit message did not complete")
			}
		})
	}
}

func TestWebSocketAssemblyDeadlineInterruptsBlockedControlHandler(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	peerClosed := make(chan error, 1)
	resourceWebSocketClient(t, map[channel.ParamKey]any{channel.ParamFrameAssemblyTimeout: 50}, func(conn *websocket.Conn, done <-chan struct{}) {
		writer, err := conn.NextWriter(websocket.BinaryMessage)
		if err != nil {
			return
		}
		if _, err := writer.Write(make([]byte, 1024)); err != nil {
			return
		}
		conn.WriteControl(websocket.PingMessage, []byte("p"), time.Now().Add(time.Second))
		_, err = conn.UnderlyingConn().Read(make([]byte, 1))
		peerClosed <- err
		<-done
	}, func(ctx channel.HandlerContext, obj any) {
		if _, ok := obj.(*PingMessage); ok {
			close(entered)
			<-release
		}
	})
	<-entered
	t.Cleanup(func() { close(release) })
	select {
	case err := <-peerClosed:
		require.ErrorIs(t, err, io.EOF)
	case <-time.After(300 * time.Millisecond):
		t.Fatal("assembly remained open inside control callback")
	}
}
