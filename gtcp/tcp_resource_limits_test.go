package gtcp

import (
	"io"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/yetiz-org/gone/channel"
	buf "github.com/yetiz-org/goth-bytebuf"
	"github.com/yetiz-org/goth-util/structs"
)

func newResourceTCPServer(t *testing.T, budget int, assemblyMS int, decode func(channel.HandlerContext, buf.ByteBuf, structs.Queue)) (addr string, accepted chan *Channel) {
	t.Helper()
	accepted = make(chan *Channel, 8)
	bootstrap := channel.NewServerBootstrap()
	bootstrap.ChannelType(&ServerChannel{})
	bootstrap.SetParams(channel.ParamKey("max_in_progress_bytes"), budget)
	bootstrap.SetChildParams(channel.ParamKey("frame_assembly_timeout"), assemblyMS)
	bootstrap.SetChildParams(channel.ParamReadTimeout, 20)
	bootstrap.SetChildParams(channel.ParamReadIdleTimeout, 1000)
	bootstrap.SetChildParams(channel.ParamReadBufferSize, 1024)
	bootstrap.ChildHandler(channel.NewInitializer(func(ch channel.Channel) {
		ch.Pipeline().AddLast("frame", channel.NewReplayDecoder(0, decode))
		ch.Pipeline().AddLast("consume", channel.NewRWHandler(func(ctx channel.HandlerContext, obj any) {
			if payload, ok := obj.(buf.ByteBuf); ok {
				ctx.Channel().Write(payload)
			}
		}, nil))
		accepted <- ch.(*Channel)
	}))
	future := bootstrap.Bind(&net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)})
	future.Sync()
	require.NoError(t, future.Error())
	server := future.Channel().(*ServerChannel)
	t.Cleanup(func() { server.Close().Sync() })
	return server.listen.Addr().String(), accepted
}

func TestTCPAssemblyDeadlineIgnoresTrickle(t *testing.T) {
	addr, accepted := newResourceTCPServer(t, 4096, 150, func(ctx channel.HandlerContext, in buf.ByteBuf, out structs.Queue) {
		panic(buf.ErrInsufficientSize)
	})
	peer, err := net.Dial("tcp", addr)
	require.NoError(t, err)
	defer peer.Close()
	child := <-accepted
	_, err = peer.Write([]byte("x"))
	require.NoError(t, err)
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		for {
			select {
			case <-stop:
				return
			case <-time.After(25 * time.Millisecond):
				if _, err := peer.Write([]byte("x")); err != nil {
					return
				}
			}
		}
	}()
	select {
	case <-child.CloseFuture().Done():
	case <-time.After(700 * time.Millisecond):
		t.Fatal("partial bytes extended the absolute frame assembly lifetime")
	}
}

func TestTCPSharedInputBudgetAndRelease(t *testing.T) {
	type inputEvent struct {
		child channel.Channel
		bytes int
	}
	retained := make(chan inputEvent, 16)
	complete := make(chan channel.Channel, 4)
	addr, accepted := newResourceTCPServer(t, 1700, 2000, func(ctx channel.HandlerContext, in buf.ByteBuf, out structs.Queue) {
		if in.ReadableBytes() >= 1024 {
			out.Push(in.ReadByteBuf(1024))
			complete <- ctx.Channel()
			return
		}
		retained <- inputEvent{child: ctx.Channel(), bytes: in.ReadableBytes()}
		panic(buf.ErrInsufficientSize)
	})
	children := map[net.Conn]*Channel{}
	dial := func() net.Conn {
		peer, err := net.Dial("tcp", addr)
		require.NoError(t, err)
		children[peer] = <-accepted
		t.Cleanup(func() { peer.Close() })
		return peer
	}
	assertRetained := func(peer net.Conn) {
		select {
		case event := <-retained:
			require.Same(t, children[peer], event.child, "retention must belong to the current peer, not a rejected predecessor")
			require.Equal(t, 900, event.bytes)
		case <-time.After(time.Second):
			t.Fatal("current peer's input not retained")
		}
	}
	finishFrame := func(peer net.Conn) {
		_, err := peer.Write(make([]byte, 124))
		require.NoError(t, err)
		select {
		case child := <-complete:
			require.Same(t, children[peer], child)
		case <-time.After(time.Second):
			t.Fatal("current peer's complete frame not decoded")
		}
		require.NoError(t, peer.SetReadDeadline(time.Now().Add(time.Second)))
		echo := make([]byte, 1024)
		_, err = io.ReadFull(peer, echo)
		require.NoError(t, err, "admission requires the current peer's complete frame to survive reservation and reach the echo handler")
		require.Equal(t, make([]byte, 1024), echo)
		require.True(t, children[peer].IsActive())
	}
	first := dial()
	_, err := first.Write(make([]byte, 900))
	require.NoError(t, err)
	assertRetained(first)
	second := dial()
	_, err = second.Write(make([]byte, 900))
	require.NoError(t, err)
	require.NoError(t, second.SetReadDeadline(time.Now().Add(300*time.Millisecond)))
	_, err = second.Read(make([]byte, 1))
	require.ErrorIs(t, err, io.EOF, "another connection exceeded the shared incomplete-input budget")
	assertRetained(second)
	finishFrame(first)
	third := dial()
	_, err = third.Write(make([]byte, 900))
	require.NoError(t, err)
	assertRetained(third)
	finishFrame(third)
	_, err = third.Write(make([]byte, 900))
	require.NoError(t, err)
	assertRetained(third)
	require.NoError(t, third.Close())
	select {
	case <-children[third].CloseFuture().Done():
	case <-time.After(time.Second):
		t.Fatal("peer did not close")
	}
	fourth := dial()
	_, err = fourth.Write(make([]byte, 900))
	require.NoError(t, err)
	assertRetained(fourth)
	finishFrame(fourth)
}

func TestTCPSuccessiveFramesHaveSeparateAssemblyDeadlines(t *testing.T) {
	retained := make(chan int, 4)
	addr, accepted := newResourceTCPServer(t, 4096, 400, func(ctx channel.HandlerContext, in buf.ByteBuf, out structs.Queue) {
		for in.ReadableBytes() >= 3 {
			in.Skip(3)
			out.Push(true)
		}
		retained <- in.ReadableBytes()
		panic(buf.ErrInsufficientSize)
	})
	peer, err := net.Dial("tcp", addr)
	require.NoError(t, err)
	defer peer.Close()
	child := <-accepted
	_, err = peer.Write([]byte("a"))
	require.NoError(t, err)
	select {
	case n := <-retained:
		require.Equal(t, 1, n)
	case <-time.After(time.Second):
		t.Fatal("first partial frame was not decoded")
	}
	time.Sleep(250 * time.Millisecond)
	_, err = peer.Write([]byte("bcd"))
	require.NoError(t, err)
	select {
	case n := <-retained:
		require.Equal(t, 1, n)
	case <-time.After(time.Second):
		t.Fatal("coalesced next frame was not retained")
	}
	select {
	case <-child.CloseFuture().Done():
		t.Fatal("next frame inherited the completed frame's assembly deadline")
	case <-time.After(200 * time.Millisecond):
	}
	_, err = peer.Write([]byte("ef"))
	require.NoError(t, err)
	select {
	case n := <-retained:
		require.Zero(t, n)
	case <-time.After(time.Second):
		t.Fatal("next frame did not complete within its own assembly window")
	}
	select {
	case <-child.CloseFuture().Done():
		t.Fatal("completed next frame left an assembly timer active")
	case <-time.After(500 * time.Millisecond):
	}
	_, err = peer.Write([]byte("ghi"))
	require.NoError(t, err)
	select {
	case n := <-retained:
		require.Zero(t, n)
	case <-time.After(time.Second):
		t.Fatal("connection could not decode a later complete frame")
	}
}
