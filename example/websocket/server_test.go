package example

import (
	"net"
	http2 "net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/yetiz-org/gone/channel"
	"github.com/yetiz-org/gone/http"
	websocket "github.com/yetiz-org/gone/websocket"
	buf "github.com/yetiz-org/goth-bytebuf"
	concurrent "github.com/yetiz-org/goth-concurrent"
	"github.com/yetiz-org/goth-kklogger"
)

func TestServer_Start(t *testing.T) {
	kklogger.SetLogLevel("TRACE")
	serverBootstrap := channel.NewServerBootstrap()
	serverBootstrap.
		ChannelType(&http.ServerChannel{}).
		SetParams(websocket.ParamCheckOrigin, false)

	clientCountHandler := &ServerChildCountHandler{}
	server := serverBootstrap.
		ChildHandler(channel.NewInitializer(func(ch channel.Channel) {
			ch.Pipeline().
				AddLast("CLIENT_COUNT_HANDLER", clientCountHandler).
				AddLast("DISPATCHER", http.NewDispatchHandler(NewRoute())).
				AddLast("WS_UPGRADE", &websocket.WSUpgradeProcessor{})
		})).
		Bind(&net.TCPAddr{IP: nil, Port: 18081}).Sync().Channel()

	go func() {
		time.Sleep(time.Minute * 1)
		server.Close()
	}()

	if rtn, err := http2.DefaultClient.Get("http://localhost:18081"); err != nil {
		assert.Fail(t, err.Error())
	} else {
		assert.EqualValues(t, "feeling good", string(buf.EmptyByteBuf().WriteReader(rtn.Body).Bytes()))
	}

	http2.DefaultClient.CloseIdleConnections()
	if rtn, err := http2.DefaultClient.Get("http://localhost:18081/home"); err != nil {
		assert.Fail(t, err.Error())
	} else {
		assert.EqualValues(t, "/home", string(buf.EmptyByteBuf().WriteReader(rtn.Body).Bytes()))
	}

	http2.DefaultClient.CloseIdleConnections()
	if rtn, err := http2.DefaultClient.Get("http://localhost:18081/v1/home"); err != nil {
		assert.Fail(t, err.Error())
	} else {
		assert.EqualValues(t, "/v1/home", string(buf.EmptyByteBuf().WriteReader(rtn.Body).Bytes()))
	}

	http2.DefaultClient.CloseIdleConnections()
	if rtn, err := http2.DefaultClient.Get("http://localhost:18081/static/home"); err != nil {
		assert.Fail(t, err.Error())
	} else {
		assert.EqualValues(t, "/static/home", string(buf.EmptyByteBuf().WriteReader(rtn.Body).Bytes()))
	}

	http2.DefaultClient.CloseIdleConnections()
	if rtn, err := http2.DefaultClient.Get("http://localhost:18081/homes"); err != nil {
		assert.Fail(t, err.Error())
	} else {
		assert.EqualValues(t, 404, rtn.StatusCode)
	}

	bootstrap := channel.NewBootstrap()
	bootstrap.ChannelType(&websocket.Channel{})
	bootstrap.Handler(channel.NewInitializer(func(ch channel.Channel) {
		ch.Pipeline().AddLast("HANDLER", websocket.NewInvokeHandler(&ClientHandlerTask{}, nil))
	}))

	ch := bootstrap.Connect(nil, &websocket.WSCustomConnectConfig{Url: "ws://localhost:18081/echo", Header: nil}).Sync().Channel()
	ch.Write(&websocket.DefaultMessage{
		MessageType: websocket.TextMessageType,
		Message:     []byte("write data"),
	})

	ch.Write(&websocket.DefaultMessage{
		MessageType: websocket.BinaryMessageType,
	})

	bwg := concurrent.WaitGroup{}
	for i := 0; i < 50; i++ {
		bwg.Add(1)
		go func(i int) {
			chs := bootstrap.Connect(nil, &websocket.WSCustomConnectConfig{Url: "ws://localhost:18081/echo", Header: nil}).Sync().Channel()
			time.Sleep(time.Millisecond * 500)
			if i%2 == 0 {
				chs.Disconnect()
			}

			bwg.Done()
		}(i)
	}

	bwg.Wait()
	time.Sleep(time.Millisecond * 10)
	ch.Write(&websocket.CloseMessage{
		DefaultMessage: websocket.DefaultMessage{
			MessageType: websocket.CloseMessageType,
			Message:     []byte("text"),
		},
		CloseCode: websocket.CloseNormalClosure,
	})

	time.Sleep(time.Millisecond * 500)
	server.CloseFuture().Sync()
	assert.Equal(t, int32(0), clientCountHandler.regTrigCount)
	assert.Equal(t, int32(0), clientCountHandler.actTrigCount)
}
