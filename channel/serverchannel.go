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

func (c *DefaultServerChannel) DeriveChildChannel(typ reflect.Type, parent ServerChannel) Channel {
	channelType := reflect.New(typ)
	var channel = channelType.Interface().(Channel)
	ValueSetFieldVal(&channelType, "pipeline", _NewDefaultPipeline(channel))
	ValueSetFieldVal(&channelType, "parent", parent)
	channel.Init()

	c.ChildParams().Range(func(k ParamKey, v interface{}) bool {
		channel.SetParam(k, v)
		return true
	})

	if c.childHandler != nil {
		channel.Pipeline().AddLast("ROOT", c.childHandler)
	}

	ValueSetFieldVal(&channelType, "closeFuture", channel.Pipeline().newFuture())
	//channel.Pipeline().fireRegistered()
	return channel
}

func (c *DefaultServerChannel) UnsafeBind(localAddr net.Addr) error {
	return nil
}

func (c *DefaultServerChannel) UnsafeAccept() error {
	return nil
}

func (c *DefaultServerChannel) UnsafeClose() error {
	c.closeChildNotify()
	return nil
}
