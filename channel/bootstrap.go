package channel

import (
	"net"
	"reflect"

	"github.com/kklab-com/gone/concurrent"
)

type Bootstrap interface {
	Handler(handler Handler) Bootstrap
	ChannelType(ch Channel) Bootstrap
	Connect(remoteAddr net.Addr) Future
	SetParams(key ParamKey, value interface{})
	Params() *Params
}

type DefaultBootstrap struct {
	handler     Handler
	channelType reflect.Type
	params      Params
}

func (d *DefaultBootstrap) SetParams(key ParamKey, value interface{}) {
	d.params.Store(key, value)
}

func (d *DefaultBootstrap) Params() *Params {
	return &d.params
}

func NewBootstrap() Bootstrap {
	bootstrap := DefaultBootstrap{}
	return &bootstrap
}

func (d *DefaultBootstrap) Handler(handler Handler) Bootstrap {
	d.handler = handler
	return d
}

func (d *DefaultBootstrap) ChannelType(ch Channel) Bootstrap {
	d.channelType = reflect.ValueOf(ch).Elem().Type()
	return d
}

func (d *DefaultBootstrap) Connect(remoteAddr net.Addr) Future {
	var channel = reflect.New(d.channelType).Interface().(Channel)
	channel.Init()
	if d.handler != nil {
		channel.Pipeline().AddLast("ROOT", d.handler)
	}

	d.Params().Range(func(k ParamKey, v interface{}) bool {
		channel.SetParam(k, v)
		return true
	})

	future := NewChannelFuture(channel, func(f concurrent.Future) interface{} {
		channel.Connect(remoteAddr)
		return channel
	})

	return future
}
