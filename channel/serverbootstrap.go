package channel

import (
	"net"
	"reflect"
)

type ServerBootstrap interface {
	Bootstrap
	ChildHandler(handler Handler) ServerBootstrap
	Bind(localAddr net.Addr) Future
}

type DefaultServerBootstrap struct {
	DefaultBootstrap
	childHandler Handler
}

func (d *DefaultServerBootstrap) ChildHandler(handler Handler) ServerBootstrap {
	d.childHandler = handler
	return d
}

func (d *DefaultServerBootstrap) Bind(localAddr net.Addr) Future {
	var serverChannel = reflect.New(d.channelType).Interface().(ServerChannel)
	serverChannel.Init()
	if d.handler != nil {
		serverChannel.Pipeline().AddLast("ROOT", d.handler)
	}

	if d.childHandler != nil {
		serverChannel.SetChildHandler(d.childHandler)
	}

	d.Params().Range(func(k ParamKey, v interface{}) bool {
		serverChannel.SetParam(k, v)
		return true
	})

	future := NewChannelFuture(serverChannel, func() interface{} {
		serverChannel.Bind(localAddr)
		return serverChannel
	})

	return future
}

func NewServerBootstrap() ServerBootstrap {
	bootstrap := DefaultServerBootstrap{}
	return &bootstrap
}
