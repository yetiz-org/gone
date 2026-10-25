package simpletcp

import (
	"bufio"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/yetiz-org/gone/channel"
	buf "github.com/yetiz-org/goth-bytebuf"
)

type reentrantActiveHandler struct {
	channel.DefaultHandler
	active func(channel.HandlerContext)
}

func (h *reentrantActiveHandler) Active(ctx channel.HandlerContext) {
	h.active(ctx)
}

func TestClientActiveCanUseCurrentChannelAcrossReconnect(t *testing.T) {
	listener, err := net.ListenTCP("tcp4", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)})
	require.NoError(t, err)
	t.Cleanup(func() { listener.Close() })

	type activeEvent struct {
		ch      channel.Channel
		current channel.Channel
		write   channel.Future
	}
	events := make(chan activeEvent, 2)
	completed := make(chan struct{}, 2)
	proceed := make(chan struct{}, 2)
	stop := make(chan struct{})
	t.Cleanup(func() { close(stop) })
	var activations atomic.Int32
	var client *Client
	client = NewClient(&reentrantActiveHandler{active: func(ctx channel.HandlerContext) {
		sequence := activations.Add(1)
		current := client.Channel()
		write := client.Write(buf.NewByteBuf([]byte{byte(sequence), 0x80, 0xff}))
		events <- activeEvent{ch: ctx.Channel(), current: current, write: write}
		select {
		case <-proceed:
		case <-stop:
		}
		if sequence == 2 {
			client.Disconnect().Sync()
		}
		completed <- struct{}{}
	}})
	var reconnects atomic.Int32
	client.AutoReconnect = func() bool { return reconnects.Add(1) == 1 }
	started := make(chan channel.Channel, 1)
	go func() { started <- client.Start(listener.Addr()) }()

	waitFuture := func(future channel.Future) {
		t.Helper()
		select {
		case <-future.Done():
			require.NoError(t, future.Error())
		case <-time.After(2 * time.Second):
			t.Fatal("channel operation did not complete")
		}
	}
	var first, second channel.Channel
	var reconnect channel.Future
	for sequence := range 2 {
		var event activeEvent
		select {
		case event = <-events:
		case <-time.After(2 * time.Second):
			t.Fatal("Active could not use the client current channel")
		}
		t.Cleanup(func() { event.ch.Disconnect() })
		require.Same(t, event.ch, event.current)
		require.True(t, event.current.IsActive())
		waitFuture(event.write)
		require.NoError(t, listener.SetDeadline(time.Now().Add(2*time.Second)))
		peer, acceptErr := listener.AcceptTCP()
		require.NoError(t, acceptErr)
		t.Cleanup(func() { peer.Close() })
		require.NoError(t, peer.SetReadDeadline(time.Now().Add(2*time.Second)))
		reader := bufio.NewReader(peer)
		var length byte
		for length == 0 {
			length, err = reader.ReadByte()
			require.NoError(t, err)
		}
		require.Equal(t, byte(3), length)
		payload := make([]byte, int(length))
		_, err = io.ReadFull(reader, payload)
		require.NoError(t, err)
		require.Equal(t, []byte{byte(sequence + 1), 0x80, 0xff}, payload)
		if sequence == 0 {
			first = event.ch
			reconnect = event.ch.Disconnect()
		} else {
			second = event.ch
			require.NotSame(t, first, second)
		}
	}
	for range 2 {
		proceed <- struct{}{}
	}
	for range 2 {
		select {
		case <-completed:
		case <-time.After(2 * time.Second):
			t.Fatal("Active did not return")
		}
	}
	select {
	case ch := <-started:
		require.Same(t, first, ch)
	case <-time.After(2 * time.Second):
		t.Fatal("initial Start did not return")
	}
	require.Same(t, second, client.Channel(), "initial Start must not overwrite a newer active connection")
	waitFuture(reconnect)
	waitFuture(first.CloseFuture())
	waitFuture(second.CloseFuture())
	require.Equal(t, int32(1), reconnects.Load())
	require.Nil(t, client.Start(nil))
	require.Nil(t, client.Channel(), "a failed Start must not retain the previous channel")
}
