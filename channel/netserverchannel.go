package channel

import (
	"net"
	"reflect"
	"sync"
)

type NetServerChannel interface {
	ServerChannel
}

type DefaultNetServerChannel struct {
	DefaultServerChannel
	child sync.Map
}

func (c *DefaultNetServerChannel) DeriveClientChannel(typ reflect.Type, conn net.Conn) NetClientChannel {
	if conn == nil {
		return nil
	}

	ncc := serverNewDefaultNetClientChannel(conn)
	ncc.parent = c
	ncc.Name = ncc.Conn().RemoteAddr().String()
	vcc := reflect.New(typ)
	cc := vcc.Interface().(NetClientChannel)
	c.child.Store(ncc.Conn().Conn(), cc)
	if icc := vcc.Elem().FieldByName("DefaultNetClientChannel"); icc.IsValid() && icc.CanSet() {
		icc.Set(reflect.ValueOf(ncc))
	} else {
		return nil
	}

	c.childParams.Range(func(k ParamKey, v interface{}) bool {
		cc.SetParam(k, v)
		return true
	})

	cc.Init()
	cc.Pipeline().AddLast("", c.childHandler)
	cc.Pipeline().fireActive()
	return cc
}

func (c *DefaultNetServerChannel) Abandon(conn net.Conn) NetClientChannel {
	if load, ok := c.child.Load(conn); ok {
		ncc := load.(NetClientChannel)
		ncc.Pipeline().fireInactive()
		c.child.Delete(conn)
		return ncc
	}

	return nil
}

func (c *DefaultNetServerChannel) Child(conn net.Conn) NetClientChannel {
	if load, ok := c.child.Load(conn); ok {
		ncc := load.(NetClientChannel)
		return ncc
	}

	return nil
}
