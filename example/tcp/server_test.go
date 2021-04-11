package tcp

import (
	"bytes"
	"net"
	"reflect"
	"testing"
	"time"

	"github.com/kklab-com/gone/channel"
	"github.com/kklab-com/gone/tcp"
	"github.com/kklab-com/goth-kklogger"
)

func TestServer_Start(t *testing.T) {
	kklogger.SetLogLevel("TRACE")
	go func() {
		time.Sleep(time.Millisecond * 500)

		bootstrap := channel.NewBootstrap()
		bootstrap.ChannelType(reflect.TypeOf(tcp.DefaultTCPClientChannel{}))
		bootstrap.Handler(channel.NewInitializer(func(ch channel.Channel) {
			ch.Pipeline().AddLast("DECODE_HANDLER", NewDecodeHandler())
			ch.Pipeline().AddLast("HANDLER", &ClientHandler{})
		}))

		ch := bootstrap.Connect(&net.TCPAddr{IP: nil, Port: 18080}).Sync().Channel().(channel.ClientChannel)
		ch.Write(bytes.NewBufferString("o12b32c49"))
		time.Sleep(time.Second)
		ch.Write(bytes.NewBufferString("a42d22e41"))
		time.Sleep(time.Second)
		ch.Disconnect()
	}()

	(&Server{}).Start(&net.TCPAddr{IP: nil, Port: 18080})
}
