package channel

import (
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
	childHandler Handler
	childParams  Params
}

func (c *DefaultServerChannel) Init() Channel {
	c.ChannelPipeline = NewDefaultPipeline(c)
	c.Unsafe.CloseFunc = func() error {
		c.Unsafe.CloseLock.Unlock()
		return nil
	}

	c.Unsafe.CloseLock.Lock()
	return c
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

	cc.Init()
	cc.Pipeline().AddLast("", c.childHandler)
	cc.Pipeline().fireActive()
	return cc
}
