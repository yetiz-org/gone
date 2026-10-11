package channel

import (
	"io"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLimitListener(t *testing.T) {
	inner, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	assert.Same(t, inner, LimitListener(inner, 0), "a nonpositive limit leaves the listener unchanged")

	l := LimitListener(inner, 1)
	defer l.Close()
	for range 2 {
		conn, err := net.Dial("tcp", inner.Addr().String())
		require.NoError(t, err)
		defer conn.Close()
	}

	first, err := l.Accept()
	require.NoError(t, err)
	accepted := make(chan net.Conn, 1)
	go func() {
		if conn, err := l.Accept(); err == nil {
			accepted <- conn
		}
	}()

	select {
	case <-accepted:
		t.Fatal("a second connection was accepted while the only slot was held")
	case <-time.After(100 * time.Millisecond):
	}

	require.NoError(t, first.Close())
	select {
	case conn := <-accepted:
		conn.Close()
	case <-time.After(time.Second):
		t.Fatal("closing a connection did not free its slot")
	}
}

func TestLimitListener_ConnCanCloseWrite(t *testing.T) {
	inner, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	l := LimitListener(inner, 1)
	defer l.Close()
	client, err := net.Dial("tcp", inner.Addr().String())
	require.NoError(t, err)
	defer client.Close()
	server, err := l.Accept()
	require.NoError(t, err)
	defer server.Close()

	closeWriter, ok := server.(interface{ CloseWrite() error })
	require.True(t, ok, "net/http half-closes a connection through CloseWrite after rejecting a body")
	require.NoError(t, closeWriter.CloseWrite())
	client.SetReadDeadline(time.Now().Add(time.Second))
	_, err = client.Read(make([]byte, 1))
	assert.ErrorIs(t, err, io.EOF)
}

func TestLimitListener_CloseUnblocksWaitingAccept(t *testing.T) {
	inner, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	l := LimitListener(inner, 1)
	conn, err := net.Dial("tcp", inner.Addr().String())
	require.NoError(t, err)
	defer conn.Close()
	held, err := l.Accept()
	require.NoError(t, err)
	defer held.Close()

	result := make(chan error, 1)
	go func() {
		_, err := l.Accept()
		result <- err
	}()

	time.Sleep(50 * time.Millisecond)
	require.NoError(t, l.Close())
	select {
	case err := <-result:
		assert.ErrorIs(t, err, net.ErrClosed)
	case <-time.After(time.Second):
		t.Fatal("Close did not unblock an Accept waiting for a slot")
	}
}

func TestDefaultNetChannel_UnsafeDisconnectClosesInactiveConn(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()
	local, err := net.Dial("tcp", ln.Addr().String())
	require.NoError(t, err)
	remote, err := ln.Accept()
	require.NoError(t, err)
	defer remote.Close()

	c := &DefaultNetChannel{}
	c.SetConn(local)
	c.Conn().(*DefaultConn).markInactive() // what a failed read or write does
	require.NoError(t, c.UnsafeDisconnect())

	remote.SetReadDeadline(time.Now().Add(time.Second))
	_, err = remote.Read(make([]byte, 1))
	assert.ErrorIs(t, err, io.EOF, "the peer must see the connection closed")
	assert.NoError(t, c.UnsafeDisconnect(), "disconnecting twice is not an error")
}
