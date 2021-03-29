package channel

import (
	"reflect"
	"sync"
)

type Bootstrap interface {
	Handler(handler Handler) Bootstrap
	ChannelType(typ reflect.Type) Bootstrap
	Connect(host string, port int) Bootstrap
	SetParams(key ParamKey, value interface{})
	Params() map[ParamKey]interface{}
}

type DefaultBootstrap struct {
	handler     Handler
	channelType reflect.Type
	atomicLock  sync.Mutex
	params      map[ParamKey]interface{}
}

func (d *DefaultBootstrap) SetParams(key ParamKey, value interface{}) {
	d._initParams()
	d.params[key] = value
}

func (d *DefaultBootstrap) Params() map[ParamKey]interface{} {
	d._initParams()
	return d.params
}

func (d *DefaultBootstrap) _initParams() {
	if d.params == nil {
		d.atomicLock.Lock()
		defer d.atomicLock.Unlock()
		if d.params == nil {
			d.params = map[ParamKey]interface{}{}
		}
	}
}
func NewBootstrap() Bootstrap {
	bootstrap := DefaultBootstrap{}
	return &bootstrap
}

func (d *DefaultBootstrap) Handler(handler Handler) Bootstrap {
	d.handler = handler
	return d
}

func (d *DefaultBootstrap) ChannelType(typ reflect.Type) Bootstrap {
	d.channelType = typ
	return d
}

func (d *DefaultBootstrap) Connect(host string, port int) Bootstrap {
	panic("implement me")
	return d
}
