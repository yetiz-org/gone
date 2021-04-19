package tcp

import (
	"net"
	"testing"
	"time"

	"github.com/kklab-com/gone/channel"
	"github.com/kklab-com/gone/tcp"
	"github.com/kklab-com/goth-kklogger"
	"github.com/kklab-com/goth-kkutil/buf"
	"github.com/kklab-com/goth-kkutil/sync"
)

func TestServer_Start(t *testing.T) {
	kklogger.SetLogLevel("TRACE")
	bootstrap := channel.NewServerBootstrap()
	bootstrap.ChannelType(&tcp.ServerChannel{})
	bootstrap.Handler(channel.NewInitializer(func(ch channel.Channel) {
		ch.Pipeline().AddLast("INDICATE_HANDLER_INBOUND", &channel.IndicateHandlerInbound{})
		ch.Pipeline().AddLast("INDICATE_HANDLER_OUTBOUND", &channel.IndicateHandlerOutbound{})
	}))

	bootstrap.ChildHandler(channel.NewInitializer(func(ch channel.Channel) {
		ch.Pipeline().AddLast("INDICATE_HANDLER_INBOUND", &channel.IndicateHandlerInbound{})
		ch.Pipeline().AddLast("DECODE_HANDLER", NewDecodeHandler())
		ch.Pipeline().AddLast("HANDLER", &ServerChildHandler{})
		ch.Pipeline().AddLast("INDICATE_HANDLER_OUTBOUND", &channel.IndicateHandlerOutbound{})
	}))

	ch := bootstrap.Bind(&net.TCPAddr{IP: nil, Port: 18082}).Sync().Channel()
	go func() {
		time.Sleep(time.Minute * 1)
		ch.Close()
	}()

	go func() {
		bootstrap := channel.NewBootstrap()
		bootstrap.ChannelType(&tcp.Channel{})
		bootstrap.Handler(channel.NewInitializer(func(ch channel.Channel) {
			ch.Pipeline().AddLast("INDICATE_HANDLER_INBOUND", &channel.IndicateHandlerInbound{})
			ch.Pipeline().AddLast("DECODE_HANDLER", NewDecodeHandler())
			ch.Pipeline().AddLast("HANDLER", &ClientHandler{})
			ch.Pipeline().AddLast("INDICATE_HANDLER_OUTBOUND", &channel.IndicateHandlerOutbound{})
		}))

		bwg := sync.BurstWaitGroup{}
		bwg.Add(100)
		for i := 0; i < 100; i++ {
			go func() {
				ch := bootstrap.Connect(nil, &net.TCPAddr{IP: nil, Port: 18082}).Sync().Channel().(channel.ClientChannel)
				ch.Write(buf.NewByteBuf([]byte("o12b32c49")))
				time.Sleep(time.Millisecond * 10)
				ch.Write(buf.NewByteBuf([]byte("a42d22e41")))
				time.Sleep(time.Millisecond * 10)
				ch.Disconnect()
				bwg.Done()
			}()
		}

		bwg.Wait()
		nch := bootstrap.Connect(nil, &net.TCPAddr{IP: nil, Port: 18082}).Sync().Channel().(channel.ClientChannel)
		nch.Write(buf.NewByteBuf([]byte("ccc")))
	}()

	ch.CloseFuture().Sync()
}
