package channel

import (
	"context"
	"net"
	"reflect"
)

type ServerChannel interface {
	Channel
	setChildHandler(handler Handler) ServerChannel
	setChildParams(key ParamKey, value interface{})
	ChildParams() *Params
}

type DefaultServerChannel struct {
	DefaultChannel
	childHandler     Handler
	childParams      Params
	closeChildNotify context.CancelFunc
}

func (c *DefaultServerChannel) setChildHandler(handler Handler) ServerChannel {
	c.childHandler = handler
	return c
}

func (c *DefaultServerChannel) setChildParams(key ParamKey, value interface{}) {
	c.childParams.Store(key, value)
}

func (c *DefaultServerChannel) ChildParams() *Params {
	return &c.childParams
}

func (c *DefaultServerChannel) DeriveChildChannel(typ reflect.Type) Channel {
	dc := NewDefaultChannel()
	dc.parent = c

	vcc := reflect.New(typ)
	cc := vcc.Interface().(Channel)
	c.childParams.Range(func(k ParamKey, v interface{}) bool {
		cc.SetParam(k, v)
		return true
	})

	cc.Pipeline().AddLast("", c.childHandler)
	cc.Pipeline().fireActive()
	return cc
}

func (c *DefaultServerChannel) UnsafeBind(localAddr net.Addr) error {
	return nil
}

func (c *DefaultServerChannel) UnsafeClose() error {
	c.closeChildNotify()
	return nil
}
