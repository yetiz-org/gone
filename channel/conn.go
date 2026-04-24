package channel

import (
	"errors"
	"net"
	"os"
	"time"
)

type Conn interface {
	net.Conn
	Conn() net.Conn
	IsActive() bool
}

type DefaultConn struct {
	conn   net.Conn
	active bool
}

func (c *DefaultConn) Read(b []byte) (n int, err error) {
	if rl, err := c.conn.Read(b); err != nil {
		if c.IsActive() {
			if errors.Is(err, os.ErrDeadlineExceeded) {
				return rl, err
			}

			c.active = false
		}

		return rl, err
	} else {
		return rl, nil
	}
}

func (c *DefaultConn) Write(b []byte) (n int, err error) {
	if wl, err := c.conn.Write(b); err != nil {
		if c.IsActive() {
			if errors.Is(err, os.ErrDeadlineExceeded) {
				return wl, err
			}

			c.active = false
		}

		return wl, err
	} else {
		return wl, nil
	}
}

func (c *DefaultConn) Close() error {
	c.active = false
	return c.conn.Close()
}

func (c *DefaultConn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

func (c *DefaultConn) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

func (c *DefaultConn) SetDeadline(t time.Time) error {
	return c.conn.SetDeadline(t)
}

func (c *DefaultConn) SetReadDeadline(t time.Time) error {
	return c.conn.SetReadDeadline(t)
}

func (c *DefaultConn) SetWriteDeadline(t time.Time) error {
	return c.conn.SetWriteDeadline(t)
}

func (c *DefaultConn) Conn() net.Conn {
	return c.conn
}

func (c *DefaultConn) IsActive() bool {
	return c.active
}

// MarkInactive flips the active flag to false so subsequent operations treat
// the connection as closed. Callers that bypass DefaultConn.Write (for
// example, scatter-gather writev paths operating directly on the underlying
// net.Conn) use this to replicate DefaultConn.Write's error-state tracking.
func (c *DefaultConn) MarkInactive() {
	c.active = false
}

func WrapConn(conn net.Conn) Conn {
	if conn == nil {
		return nil
	}

	return &DefaultConn{
		conn:   conn,
		active: true,
	}
}
