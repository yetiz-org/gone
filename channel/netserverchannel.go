package channel

import (
	"net"
)

type NetServerChannel interface {
	ServerChannel
}

type DefaultNetServerChannel struct {
	DefaultServerChannel
}

func (c *DefaultNetServerChannel) Conn() Conn {
	return nil
}

func (c *DefaultNetServerChannel) RemoteAddr() net.Addr {
	return nil
}

func (c *DefaultNetServerChannel) LocalAddr() net.Addr {
	return nil
}

func (c *DefaultServerChannel) DeriveNetChildChannel(child NetChannel, parent NetServerChannel, conn net.Conn) Channel {
	if conn == nil {
		return nil
	}

	child.setConn(conn)
	c.DeriveChildChannel(child, parent)
	return child
}

func (c *DefaultNetServerChannel) UnsafeBind(localAddr net.Addr) error {
	return nil
}

func (c *DefaultNetServerChannel) UnsafeAccept() Channel {
	return nil
}

func (c *DefaultNetServerChannel) UnsafeClose() error {
	c.closeChildNotify()
	return nil
}
