package gudp

import (
	"bytes"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yetiz-org/gone/channel"
	transport "github.com/yetiz-org/gone/gudp"
	buf "github.com/yetiz-org/goth-bytebuf"
)

type receivedBytesHandler struct {
	channel.DefaultHandler
	received chan []byte
}

func (h *receivedBytesHandler) Read(ctx channel.HandlerContext, obj any) {
	h.received <- bytes.Clone(obj.(buf.ByteBuf).Bytes())
	ctx.FireRead(obj)
}

func TestUDPServer_Start(t *testing.T) {
	reservation, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	require.NoError(t, err)
	addr := reservation.LocalAddr().(*net.UDPAddr)
	require.NoError(t, reservation.Close())

	serverHandler := &ServerChildHandler{}
	serverBootstrap := channel.NewServerBootstrap()
	serverBootstrap.ChannelType(&transport.ServerChannel{})
	serverBootstrap.Handler(channel.NewInitializer(func(ch channel.Channel) {
		ch.Pipeline().AddLast("INDICATE_HANDLER_INBOUND", &channel.IndicateHandlerInbound{}).AddLast("INDICATE_HANDLER_OUTBOUND", &channel.IndicateHandlerOutbound{})
	}))
	serverBootstrap.ChildHandler(channel.NewInitializer(func(ch channel.Channel) {
		ch.Pipeline().AddLast("INDICATE_HANDLER_INBOUND", &channel.IndicateHandlerInbound{}).AddLast("decoder", NewDecodeHandler()).AddLast("example", serverHandler).AddLast("INDICATE_HANDLER_OUTBOUND", &channel.IndicateHandlerOutbound{})
	}))
	bound := serverBootstrap.Bind(addr)
	require.True(t, bound.AwaitTimeout(5*time.Second).IsSuccess(), "bind: %v", bound.Error())
	server := bound.Channel()
	t.Cleanup(func() {
		server.Close().AwaitTimeout(5 * time.Second)
		assert.True(t, server.CloseFuture().AwaitTimeout(5*time.Second).IsSuccess())
	})

	clientHandler := &ClientHandler{}
	exchange := func(first, second, expected string) {
		received := make(chan []byte, 64)
		bootstrap := channel.NewBootstrap()
		bootstrap.ChannelType(&transport.Channel{})
		bootstrap.Handler(channel.NewInitializer(func(ch channel.Channel) {
			ch.Pipeline().AddLast("INDICATE_HANDLER_INBOUND", &channel.IndicateHandlerInbound{}).AddLast("received", &receivedBytesHandler{received: received}).AddLast("decoder", NewDecodeHandler()).AddLast("example", clientHandler).AddLast("INDICATE_HANDLER_OUTBOUND", &channel.IndicateHandlerOutbound{})
		}))
		connected := bootstrap.Connect(nil, addr)
		if !assert.True(t, connected.AwaitTimeout(5*time.Second).IsSuccess(), "connect: %v", connected.Error()) {
			return
		}
		client := connected.Channel()
		defer func() {
			if !client.CloseFuture().IsDone() {
				assert.True(t, client.Disconnect().AwaitTimeout(5*time.Second).IsSuccess())
			}
			assert.True(t, client.CloseFuture().AwaitTimeout(5*time.Second).IsSuccess())
		}()
		for _, payload := range []string{first, second} {
			if payload != "" {
				assert.True(t, client.Write(buf.NewByteBufString(payload)).AwaitTimeout(5*time.Second).IsSuccess())
			}
		}
		var actual bytes.Buffer
		timer := time.NewTimer(5 * time.Second)
		defer timer.Stop()
		for actual.Len() < len(expected) {
			select {
			case data := <-received:
				actual.Write(data)
			case <-timer.C:
				t.Errorf("echo incomplete: received %q, want %q", actual.String(), expected)
				return
			}
		}
		assert.Equal(t, expected, actual.String())
	}

	var workers sync.WaitGroup
	for range 5 {
		workers.Go(func() {
			exchange("o12b32c49", "a42d22e41", "udp_h:o udp_b:12udp_h:b udp_b:32udp_h:c udp_b:49udp_h:a udp_b:42udp_h:d udp_b:22udp_h:e udp_b:41")
		})
	}
	workers.Wait()
	exchange("ccc", "", "udp_h:c udp_b:cc")
	require.True(t, server.CloseFuture().AwaitTimeout(5*time.Second).IsSuccess(), "terminal ccc must close server")
	assert.Zero(t, atomic.LoadInt32(&serverHandler.regTrigCount))
	assert.Zero(t, atomic.LoadInt32(&serverHandler.actTrigCount))
	assert.Zero(t, atomic.LoadInt32(&clientHandler.regTrigCount))
	assert.Zero(t, atomic.LoadInt32(&clientHandler.actTrigCount))
}

type deregisterObserver struct {
	channel.DefaultHandler
	deregistered, disconnected int
}

func (h *deregisterObserver) Deregister(ctx channel.HandlerContext, future channel.Future) {
	h.deregistered++
	ctx.Deregister(future)
}

func (h *deregisterObserver) Disconnect(ctx channel.HandlerContext, future channel.Future) {
	h.disconnected++
	future.Completable().Complete(nil)
}

func TestServerChildHandler_Deregister(t *testing.T) {
	ch := new(channel.DefaultChannel).Init()
	before, after := new(deregisterObserver), new(deregisterObserver)
	ch.Pipeline().AddLast("before", before).AddLast("example", &ServerChildHandler{}).AddLast("after", after)
	future := ch.Deregister()
	require.True(t, future.IsDone(), "deregistration must reach its terminal handler")
	require.NoError(t, future.Error())
	assert.Equal(t, 1, after.deregistered)
	assert.Zero(t, before.disconnected)
}
