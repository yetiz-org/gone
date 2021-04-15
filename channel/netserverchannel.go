package channel

import (
	"net"
	"reflect"
)

type NetServerChannel interface {
	NetChannel
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

func (c *DefaultServerChannel) DeriveNetChildChannel(typ reflect.Type, conn net.Conn) Channel {
	if conn == nil {
		return nil
	}

	dc := NewDefaultChannel()
	dc.parent = c

	vcc := reflect.New(typ)
	cc := vcc.Interface().(NetChannel)
	if icc := vcc.Elem().FieldByName("DefaultNetChannel"); icc.IsValid() && icc.CanSet() {
		icc.Set(reflect.ValueOf(dc))
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
