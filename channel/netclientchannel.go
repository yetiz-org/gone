package channel

import (
	"fmt"
	"net"
	"sync"

	"github.com/kklab-com/goth-kklogger"
)

type NetClientChannel interface {
	ClientChannel
	Conn() net.Conn
	Parent() NetServerChannel
	RemoteAddr() net.Addr
	LocalAddr() net.Addr
}

var ErrNilObject = fmt.Errorf("nil object")

type DefaultNetClientChannel struct {
	DefaultClientChannel
	conn           net.Conn
	parent         *DefaultNetServerChannel
	disconnectOnce sync.Once
}

func serverNewDefaultNetClientChannel(conn net.Conn) *DefaultNetClientChannel {
	ncc := DefaultNetClientChannel{
		DefaultClientChannel: *NewDefaultClientChannel(),
	}

	ncc.Unsafe.DisconnectFunc = ncc.disconnect
	ncc.conn = conn
	return &ncc
}

func NewDefaultNetClientChannel() *DefaultNetClientChannel {
	ncc := DefaultNetClientChannel{
		DefaultClientChannel: *NewDefaultClientChannel(),
	}

	ncc.Unsafe.ConnectFunc = ncc.connect
	ncc.Unsafe.DisconnectFunc = ncc.disconnect
	return &ncc
}

func (c *DefaultNetClientChannel) Conn() net.Conn {
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
		c.SetParam(paramActive, false)
		if conn := c.Conn(); conn != nil {
			if c.parent != nil {
				c.parent.Abandon(c.Conn())
			}

			if err = conn.Close(); err != nil {
				kklogger.ErrorJ("DefaultNetClientChannel.disconnect", err.Error())
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
		c.conn = conn
		c.SetParam(paramActive, true)
	}

	return nil
}

func (c *DefaultNetClientChannel) IsActive() bool {
	return c.Param(paramActive).(bool)
}
