package channel

import (
	"net"
	"reflect"
)

type ServerBootstrap interface {
	Bootstrap
	ChildHandler(handler Handler) ServerBootstrap
	SetChildParams(key ParamKey, value interface{})
	ChildParams() *Params
	Bind(localAddr net.Addr) Future
}

type DefaultServerBootstrap struct {
	DefaultBootstrap
	childHandler Handler
	childParams  Params
}

func (d *DefaultServerBootstrap) ChildHandler(handler Handler) ServerBootstrap {
	d.childHandler = handler
	return d
}

func (d *DefaultServerBootstrap) SetChildParams(key ParamKey, value interface{}) {
	d.childParams.Store(key, value)
}

func (d *DefaultServerBootstrap) ChildParams() *Params {
	return &d.childParams
}

func (d *DefaultServerBootstrap) Bind(localAddr net.Addr) Future {
	serverChannelType := reflect.New(d.channelType)
	var serverChannel = serverChannelType.Interface().(ServerChannel)
	ValueSetFieldVal(&serverChannelType, "pipeline", _NewDefaultPipeline(serverChannel))
	serverChannel.Init()
	d.Params().Range(func(k ParamKey, v interface{}) bool {
		serverChannel.SetParam(k, v)
		return true
	})

	if d.handler != nil {
		serverChannel.Pipeline().AddLast("ROOT", d.handler)
	}

	if d.childHandler != nil {
		serverChannel.setChildHandler(d.childHandler)
	}

	serverChannel.setLocalAddr(localAddr)
	ValueSetFieldVal(&serverChannelType, "closeFuture", serverChannel.Pipeline().newFuture())
	serverChannel.Pipeline().fireRegistered()
	return serverChannel.Bind(localAddr)
}

func NewServerBootstrap() ServerBootstrap {
	bootstrap := DefaultServerBootstrap{}
	return &bootstrap
}
