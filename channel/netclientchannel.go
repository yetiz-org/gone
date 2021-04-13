package channel

import (
	"fmt"
	"net"
	"sync"

	"github.com/kklab-com/goth-kklogger"
)

type NetClientChannel interface {
	ClientChannel
	Conn() Conn
	Parent() NetServerChannel
	RemoteAddr() net.Addr
	LocalAddr() net.Addr
}

var ErrNilObject = fmt.Errorf("nil object")
var ErrUnknownObject = fmt.Errorf("unknown object")

type DefaultNetClientChannel struct {
	DefaultClientChannel
	conn           Conn
	parent         *DefaultNetServerChannel
	disconnectOnce sync.Once
}

func serverNewDefaultNetClientChannel(conn net.Conn) *DefaultNetClientChannel {
	ncc := DefaultNetClientChannel{
		DefaultClientChannel: *NewDefaultClientChannel(),
	}

	ncc.Unsafe.DisconnectFunc = ncc.disconnect
	ncc.conn = WrapConn(conn)
	return &ncc
}

func NewDefaultNetClientChannel() *DefaultNetClientChannel {
	ncc := DefaultNetClientChannel{
		DefaultClientChannel: *NewDefaultClientChannel(),
	}

	ncc.Unsafe.ConnectFunc = ncc.connect
	ncc.Unsafe.CustomConnectFunc = ncc.customConnect
	ncc.Unsafe.DisconnectFunc = ncc.disconnect
	return &ncc
}

func (c *DefaultNetClientChannel) Conn() Conn {
	return c.conn
}

func (c *DefaultNetClientChannel) Parent() NetServerChannel {
	return c.parent
}

func (c *DefaultNetClientChannel) RemoteAddr() net.Addr {
	if c.conn != nil {
		return c.conn.RemoteAddr()
	}

	return nil
}

func (c *DefaultNetClientChannel) LocalAddr() net.Addr {
	if c.conn != nil {
		return c.conn.LocalAddr()
	}

	return nil
}

func (c *DefaultNetClientChannel) disconnect() error {
	var err error = nil
	c.disconnectOnce.Do(func() {
		if conn := c.Conn(); conn != nil {
			if err = conn.Close(); err != nil {
				kklogger.ErrorJ("DefaultNetClientChannel.disconnect", err.Error())
			}

			if c.parent != nil {
				c.parent.Abandon(c.Conn().Conn())
			} else {
				c.Pipeline().fireInactive()
			}
		}

		c.Unsafe.DisconnectLock.Unlock()
	})

	return err
}

func (c *DefaultNetClientChannel) connect(remoteAddr net.Addr) error {
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

func (c *DefaultNetClientChannel) customConnect(v interface{}) error {
	if conn, ok := v.(net.Conn); ok {
		c.conn = WrapConn(conn)
		c.Pipeline().fireActive()
		return nil
	}

	return ErrUnknownObject
}

func (c *DefaultNetClientChannel) IsActive() bool {
	return c.Conn().IsActive()
}
