package channel

import (
	"net"
	"sync"
)

type NetServerChannel interface {
	ServerChannel
}

type DefaultNetServerChannel struct {
	DefaultServerChannel
	child sync.Map
}

func (c *DefaultNetServerChannel) DeriveNetClientChannel(conn net.Conn) *DefaultNetClientChannel {
	if conn == nil {
		return nil
	}

	ncc := serverNewDefaultNetClientChannel(conn)
	ncc.parent = c
	ncc.Name = ncc.Conn().RemoteAddr().String()
	c.child.Store(ncc.Conn().Conn(), ncc)
	return ncc
}

func (c *DefaultNetServerChannel) Abandon(conn net.Conn) NetClientChannel {
	if load, ok := c.child.Load(conn); ok {
		c.child.Delete(conn)
		return load.(NetClientChannel)
	}

	return nil
}
