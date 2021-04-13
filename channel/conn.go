package channel

import (
	"io"
	"net"
	"time"

	kklogger "github.com/kklab-com/goth-kklogger"
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
	if read, err := c.conn.Read(b); err != nil {
		if c.IsActive() {
			if err != io.EOF && !err.(net.Error).Timeout() {
				kklogger.WarnJ("DefaultConn.Read", err.Error())
			}

			c.active = false
		}

		return read, err
	} else {
		return read, nil
	}
}

func (c *DefaultConn) Write(b []byte) (n int, err error) {
	if write, err := c.conn.Write(b); err != nil {
		if c.IsActive() {
			if err != io.EOF && !err.(net.Error).Timeout() {
				kklogger.WarnJ("DefaultConn.Write", err.Error())
			}

			c.active = false
		}

		return write, err
	} else {
		return write, nil
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

func WrapConn(conn net.Conn) Conn {
	if conn == nil {
		return nil
	}

	return &DefaultConn{
		conn:   conn,
		active: true,
	}
}
