package ghttp

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yetiz-org/gone/channel"
)

type serverChildLifecycle struct {
	channel.DefaultHandler
	channel           channel.Channel
	peer              net.Conn
	activated         chan *serverChildLifecycle
	deregisterEntered chan struct{}
	deregisterRelease chan struct{}
	deregisters       atomic.Int32
	inactives         atomic.Int32
	unregistered      atomic.Int32
}

func (h *serverChildLifecycle) Active(ctx channel.HandlerContext) {
	h.channel = ctx.Channel()
	h.activated <- h
	ctx.FireActive()
}

func (h *serverChildLifecycle) Deregister(ctx channel.HandlerContext, future channel.Future) {
	h.deregisters.Add(1)
	if h.deregisterEntered != nil {
		close(h.deregisterEntered)
		<-h.deregisterRelease
	}
	ctx.Deregister(future)
}

func (h *serverChildLifecycle) Inactive(ctx channel.HandlerContext) {
	h.inactives.Add(1)
	ctx.FireInactive()
}

func (h *serverChildLifecycle) Unregistered(ctx channel.HandlerContext) {
	h.unregistered.Add(1)
	ctx.FireUnregistered()
}

func bindCleanupTestServer(t *testing.T, connectionCount int) (server *ServerChannel, children []*serverChildLifecycle, stateClosed func(net.Conn, http.ConnState)) {
	t.Helper()
	activated := make(chan *serverChildLifecycle, connectionCount)
	closed := make(chan net.Conn, connectionCount)
	idle := make(chan struct{}, connectionCount)
	bootstrap := channel.NewServerBootstrap()
	bootstrap.ChannelType(&ServerChannel{})
	bootstrap.ChildHandler(channel.NewInitializer(func(ch channel.Channel) {
		if stateClosed == nil {
			// ConnContext initializes the first child before Serve reads ConnState.
			httpServer := ch.Parent().(*ServerChannel).server
			originalState := httpServer.ConnState
			stateClosed = originalState
			httpServer.ConnState = func(conn net.Conn, state http.ConnState) {
				if state == http.StateClosed {
					closed <- conn
					return
				}
				originalState(conn, state)
				if state == http.StateIdle {
					idle <- struct{}{}
				}
			}
		}
		ch.Pipeline().AddLast("LIFECYCLE", &serverChildLifecycle{activated: activated})
		ch.Pipeline().AddLast("DISPATCHER", NewDispatchHandler(NewSimpleRoute()))
	}))
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := listener.Addr().(*net.TCPAddr)
	require.NoError(t, listener.Close())
	bound := bootstrap.Bind(addr).Sync()
	require.NoError(t, bound.Error())
	server = bound.Channel().(*ServerChannel)
	t.Cleanup(func() { server.Close().Sync() })

	// Defer only StateClosed delivery so UnsafeClose must run its fallback cleanup.
	connected := 0
	t.Cleanup(func() {
		server.server.Close()
		for range connected {
			stateClosed(<-closed, http.StateClosed)
		}
	})

	for range connectionCount {
		conn, err := net.Dial("tcp", addr.String())
		require.NoError(t, err)
		connected++
		t.Cleanup(func() { conn.Close() })
		child := <-activated
		_, err = fmt.Fprintf(conn, "GET /cleanup HTTP/1.1\r\nHost: localhost\r\n\r\n")
		require.NoError(t, err)
		response, err := http.ReadResponse(bufio.NewReader(conn), nil)
		require.NoError(t, err)
		_, err = io.Copy(io.Discard, response.Body)
		require.NoError(t, err)
		require.NoError(t, response.Body.Close())
		<-idle
		require.True(t, child.channel.IsActive())
		require.Zero(t, child.deregisters.Load())
		child.peer = conn
		children = append(children, child)
	}
	return server, children, stateClosed
}

func TestChannelRangeLoopDeregistration(t *testing.T) {
	server, children, _ := bindCleanupTestServer(t, 2)
	require.NoError(t, server.UnsafeClose())

	for _, child := range children {
		require.True(t, child.channel.CloseFuture().IsDone(), "close must finish child cleanup")
		assert.Equal(t, int32(1), child.deregisters.Load())
		assert.Equal(t, int32(1), child.inactives.Load())
		assert.Equal(t, int32(1), child.unregistered.Load())
		assert.False(t, child.channel.IsActive())
		_, err := child.peer.Read(make([]byte, 1))
		assert.ErrorIs(t, err, io.EOF)
	}
	server.chMap.Range(func(key, value any) bool {
		t.Error("closed server retained a child connection")
		return true
	})
}

func TestLoadAndDeleteRaceCondition(t *testing.T) {
	server, children, stateClosed := bindCleanupTestServer(t, 1)
	child := children[0]
	conn := child.channel.(channel.NetChannel).Conn().Conn()
	child.deregisterEntered = make(chan struct{})
	child.deregisterRelease = make(chan struct{})
	var wg sync.WaitGroup
	callbackDone := make(chan struct{})
	wg.Go(func() {
		stateClosed(conn, http.StateClosed)
		close(callbackDone)
	})
	select {
	case <-child.deregisterEntered:
	case <-callbackDone:
		t.Fatal("StateClosed returned without deregistering the active child")
	}
	wg.Go(func() {
		assert.NoError(t, server.UnsafeClose())
		close(child.deregisterRelease)
	})
	wg.Wait()
	require.True(t, child.channel.CloseFuture().IsDone(), "close must finish child cleanup")

	assert.GreaterOrEqual(t, child.deregisters.Load(), int32(1))
	assert.LessOrEqual(t, child.deregisters.Load(), int32(2))
	assert.Equal(t, int32(1), child.inactives.Load())
	assert.Equal(t, int32(1), child.unregistered.Load())
	assert.False(t, child.channel.IsActive())
	_, err := child.peer.Read(make([]byte, 1))
	assert.ErrorIs(t, err, io.EOF)
	server.chMap.Range(func(key, value any) bool {
		t.Error("closed server retained a child connection")
		return true
	})
}

func TestServerChannelMultipleDeregisterSafety(t *testing.T) {
	server, children, stateClosed := bindCleanupTestServer(t, 1)
	child := children[0]
	const concurrentCalls = 10
	var wg sync.WaitGroup
	for range concurrentCalls {
		wg.Go(func() {
			future := child.channel.Deregister().Sync()
			assert.NoError(t, future.Error())
		})
	}
	wg.Wait()
	require.True(t, child.channel.CloseFuture().IsDone(), "close must finish child cleanup")

	assert.Equal(t, int32(concurrentCalls), child.deregisters.Load())
	assert.Equal(t, int32(1), child.inactives.Load())
	assert.Equal(t, int32(1), child.unregistered.Load())
	assert.False(t, child.channel.IsActive())
	_, err := child.peer.Read(make([]byte, 1))
	assert.ErrorIs(t, err, io.EOF)
	stateClosed(child.channel.(channel.NetChannel).Conn().Conn(), http.StateClosed)
	server.chMap.Range(func(key, value any) bool {
		t.Error("StateClosed retained a deregistered child connection")
		return true
	})
}
