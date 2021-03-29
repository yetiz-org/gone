package channel

import (
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

type DefaultNetClientChannel struct {
	DefaultClientChannel
	conn           net.Conn
	parent         *DefaultNetServerChannel
	disconnectOnce sync.Once
}

func NewDefaultNetClientChannel(conn net.Conn) *DefaultNetClientChannel {
	ncc := DefaultNetClientChannel{
		DefaultClientChannel: *NewDefaultClientChannel(),
	}

	ncc.Unsafe.DisconnectFunc = ncc.disconnect
	ncc.conn = conn
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

func (c *DefaultNetClientChannel) IsActive() bool {
	return c.Param(paramActive).(bool)
}
