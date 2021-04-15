package channel

import (
	"fmt"
	"net"
	"sync"

	"github.com/kklab-com/goth-kklogger"
)

type NetChannel interface {
	Channel
	Conn() Conn
	RemoteAddr() net.Addr
}

type NetChannelPostActive interface {
	PostActive(conn net.Conn)
}

var ErrNilObject = fmt.Errorf("nil object")
var ErrUnknownObject = fmt.Errorf("unknown object")

type DefaultNetChannel struct {
	DefaultChannel
	conn           Conn
	disconnectOnce sync.Once
	WriteLock      sync.Mutex
}

func serverNewChildChannel(conn net.Conn) *DefaultNetChannel {
	ncc := DefaultNetChannel{
		DefaultChannel: *NewDefaultChannel(),
	}

	ncc.Unsafe.DisconnectFunc = ncc.disconnect
	ncc.conn = WrapConn(conn)
	return &ncc
}

func NewDefaultNetClientChannel() *DefaultNetChannel {
	ncc := DefaultNetChannel{
		DefaultChannel: *NewDefaultChannel(),
	}

	ncc.Unsafe.ConnectFunc = ncc.connect
	ncc.Unsafe.DisconnectFunc = ncc.disconnect
	return &ncc
}

func (c *DefaultNetChannel) Conn() Conn {
	return c.conn
}

func (c *DefaultNetChannel) RemoteAddr() net.Addr {
	if c.conn != nil {
		return c.conn.RemoteAddr()
	}

	return nil
}

func (c *DefaultNetChannel) LocalAddr() net.Addr {
	if c.localAddr == nil {
		if c.conn != nil {
			c.localAddr = c.conn.LocalAddr()
			return c.conn.LocalAddr()
		}
	}

	return nil
}

func (c *DefaultNetChannel) disconnect() error {
	var err error = nil
	c.disconnectOnce.Do(func() {
		if conn := c.conn; conn != nil {
			if err = conn.Close(); err != nil {
				kklogger.ErrorJ("DefaultNetChannel.disconnect", err.Error())
			}

			if c.parent != nil {
				c.parent.Abandon(c.conn.Conn())
			} else {
				c.Pipeline().fireInactive()
			}
		}

		c.Unsafe.DisconnectLock.Unlock()
	})

	return err
}

func (c *DefaultNetChannel) connect(remoteAddr net.Addr) error {
	if remoteAddr == nil {
		return ErrNilObject
	}

	if conn, err := net.Dial(remoteAddr.Network(), remoteAddr.String()); err != nil {
		return err
	} else {
		c.conn = WrapConn(conn)
	}

	c.Pipeline().fireActive()
	return nil
}

func (c *DefaultNetChannel) IsActive() bool {
	return c.conn.IsActive()
}

func (c *DefaultNetChannel) PostActive(conn net.Conn) {
	c.conn = WrapConn(conn)
	c.Pipeline().fireActive()
}
