package websocket

import (
	"net"
	"testing"
	"time"

	"github.com/kklab-com/gone/channel"
	"github.com/kklab-com/gone/websocket"
	"github.com/kklab-com/goth-kklogger"
)

var server = Server{}

func TestServer_Start(t *testing.T) {
	kklogger.SetLogLevel("TRACE")
	go func() {
		time.Sleep(time.Millisecond * 500)

		bootstrap := channel.NewBootstrap()
		bootstrap.ChannelType(&websocket.Channel{})
		bootstrap.Handler(channel.NewInitializer(func(ch channel.Channel) {
			ch.Pipeline().AddLast("HANDLER", &ClientHandler{})
		}))

		ch := bootstrap.Connect(nil, &websocket.WSCustomConnectConfig{Url: "ws://localhost:18081/echo", Header: nil}).Sync().Channel().(channel.ClientChannel)
		ch.Write(&websocket.DefaultMessage{
			MessageType: websocket.TextMessageType,
			Message:     []byte("write data"),
		})

		time.Sleep(time.Millisecond * 500)
		ch.Write(&websocket.CloseMessage{
			DefaultMessage: websocket.DefaultMessage{
				MessageType: websocket.CloseMessageType,
				Message:     []byte("text"),
			},
			CloseCode: websocket.CloseNormalClosure,
		})

		time.Sleep(time.Millisecond * 500)
		ch.Disconnect()
		time.Sleep(time.Millisecond * 500)
	}()

	server.Start(&net.TCPAddr{IP: nil, Port: 18081})
}
