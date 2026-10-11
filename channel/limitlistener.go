package channel

import (
	"net"
	"sync"
)

// LimitListener returns a Listener that accepts at most n connections at once. Accept waits while n are
// open, and closing a connection frees its slot. A nonpositive n returns l unchanged.
func LimitListener(l net.Listener, n int) (limited net.Listener) {
	if n <= 0 {
		return l
	}

	return &limitListener{Listener: l, slots: make(chan struct{}, n), done: make(chan struct{})}
}

type limitListener struct {
	net.Listener
	slots     chan struct{}
	done      chan struct{}
	closeOnce sync.Once
}

func (l *limitListener) Accept() (conn net.Conn, rtErr error) {
	select {
	case l.slots <- struct{}{}:
	case <-l.done:
		return nil, net.ErrClosed
	}

	if conn, rtErr = l.Listener.Accept(); rtErr != nil {
		<-l.slots
		return nil, rtErr
	}

	return &limitConn{Conn: conn, release: func() { <-l.slots }}, nil
}

func (l *limitListener) Close() (rtErr error) {
	l.closeOnce.Do(func() { close(l.done) })
	return l.Listener.Close()
}

type limitConn struct {
	net.Conn
	releaseOnce sync.Once
	release     func()
}

func (c *limitConn) Close() (rtErr error) {
	rtErr = c.Conn.Close()
	c.releaseOnce.Do(c.release)
	return rtErr
}

// CloseWrite lets net/http half-close a connection after rejecting a request body, so the client reads the
// response instead of a reset.
func (c *limitConn) CloseWrite() (rtErr error) {
	if cw, ok := c.Conn.(interface{ CloseWrite() error }); ok {
		return cw.CloseWrite()
	}

	return nil
}
