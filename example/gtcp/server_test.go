package gtcp

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/yetiz-org/gone/channel"
	"github.com/yetiz-org/gone/gtcp"
	buf "github.com/yetiz-org/goth-bytebuf"
	concurrent "github.com/yetiz-org/goth-concurrent"
	"github.com/yetiz-org/goth-kklogger"
)

func TestServer_Start(t *testing.T) {
	kklogger.SetLogLevel("DEBUG")
	serverChildHandler := &ServerChildHandler{}
	bootstrap := channel.NewServerBootstrap()
	bootstrap.ChannelType(&gtcp.ServerChannel{})
	bootstrap.Handler(channel.NewInitializer(func(ch channel.Channel) {
		ch.Pipeline().AddLast("INDICATE_HANDLER_INBOUND", &channel.IndicateHandlerInbound{})
		ch.Pipeline().AddLast("INDICATE_HANDLER_OUTBOUND", &channel.IndicateHandlerOutbound{})
	}))

	bootstrap.ChildHandler(channel.NewInitializer(func(ch channel.Channel) {
		ch.Pipeline().AddLast("INDICATE_HANDLER_INBOUND", &channel.IndicateHandlerInbound{})
		ch.Pipeline().AddLast("DECODE_HANDLER", NewDecodeHandler())
		ch.Pipeline().AddLast("HANDLER", serverChildHandler)
		ch.Pipeline().AddLast("INDICATE_HANDLER_OUTBOUND", &channel.IndicateHandlerOutbound{})
	}))

	ch := bootstrap.Bind(&net.TCPAddr{IP: nil, Port: 18082}).Sync().Channel()
	go func() {
		time.Sleep(time.Second * 1)
		ch.Close()
	}()

	clientHandler := &ClientHandler{}
	go func() {
		bootstrap := channel.NewBootstrap()
		bootstrap.ChannelType(&gtcp.Channel{})
		bootstrap.Handler(channel.NewInitializer(func(ch channel.Channel) {
			ch.Pipeline().AddLast("INDICATE_HANDLER_INBOUND", &channel.IndicateHandlerInbound{})
			ch.Pipeline().AddLast("DECODE_HANDLER", NewDecodeHandler())
			ch.Pipeline().AddLast("HANDLER", clientHandler)
			ch.Pipeline().AddLast("INDICATE_HANDLER_OUTBOUND", &channel.IndicateHandlerOutbound{})
		}))

		bwg := concurrent.WaitGroup{}
		for i := 0; i < 10; i++ {
			bwg.Add(1)
			go func(i int) {
				ch := bootstrap.Connect(nil, &net.TCPAddr{IP: nil, Port: 18082}).Sync().Channel()
				ch.Write(buf.NewByteBuf([]byte("o12b32c49")))
				time.Sleep(time.Millisecond * 10)
				ch.Write(buf.NewByteBuf([]byte("a42d22e41")))
				time.Sleep(time.Millisecond * 10)
				if i%2 == 0 {
					ch.Disconnect()
				}

				bwg.Done()
			}(i)
		}

		bwg.Wait()
		time.Sleep(time.Second * 111111)
		nch := bootstrap.Connect(nil, &net.TCPAddr{IP: nil, Port: 18082}).Sync().Channel()
		nch.Write(buf.NewByteBuf([]byte("ccc")))
	}()

	ch.CloseFuture().Sync()
	assert.Equal(t, int32(0), serverChildHandler.regTrigCount)
	assert.Equal(t, int32(0), serverChildHandler.actTrigCount)
	time.Sleep(time.Second)
	assert.Equal(t, int32(0), clientHandler.regTrigCount)
	assert.Equal(t, int32(0), clientHandler.actTrigCount)
}
