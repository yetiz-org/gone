package channel

import (
	"fmt"
	"net"

	"github.com/kklab-com/goth-kklogger"
)

type HandlerContext interface {
	Name() string
	Channel() Channel
	FireRead(obj interface{}) HandlerContext
	FireReadCompleted() HandlerContext
	FireWrite(obj interface{}) HandlerContext
	FireErrorCaught(err error) HandlerContext
	Bind(localAddr net.Addr) HandlerContext
	Close() HandlerContext
	Connect(remoteAddr net.Addr, localAddr net.Addr) HandlerContext
	Disconnect() HandlerContext
	prev() HandlerContext
	setPrev(prev HandlerContext) HandlerContext
	next() HandlerContext
	setNext(prev HandlerContext) HandlerContext
	setChannel(channel Channel) HandlerContext
	handler() Handler
}

type DefaultHandlerContext struct {
	name     string
	channel  Channel
	_handler Handler
	nextCtx  HandlerContext
	prevCtx  HandlerContext
}

func (d *DefaultHandlerContext) setPrev(prev HandlerContext) HandlerContext {
	d.prevCtx = prev
	return d
}

func (d *DefaultHandlerContext) next() HandlerContext {
	return d.nextCtx
}

func (d *DefaultHandlerContext) setNext(next HandlerContext) HandlerContext {
	d.nextCtx = next
	return d
}

func (d *DefaultHandlerContext) setChannel(channel Channel) HandlerContext {
	d.channel = channel
	return d
}

func (d *DefaultHandlerContext) handler() Handler {
	return d._handler
}

func NewHandlerContext() *DefaultHandlerContext {
	context := new(DefaultHandlerContext)
	return context
}

func (d *DefaultHandlerContext) Name() string {
	return d.name
}

func (d *DefaultHandlerContext) Channel() Channel {
	return d.channel
}

func (d *DefaultHandlerContext) FireRead(obj interface{}) HandlerContext {
	if d.next() != nil {
		if c, ok := d.next().(*DefaultHandlerContext); ok {
			defer c.deferErrorCaught()
		}

		d.next().handler().Read(d.next(), obj)
	}

	return d
}

func (d *DefaultHandlerContext) FireReadCompleted() HandlerContext {
	if d.next() != nil {
		if c, ok := d.next().(*DefaultHandlerContext); ok {
			defer c.deferErrorCaught()
		}

		d.next().handler().ReadCompleted(d.next())
	}

	return d
}

func (d *DefaultHandlerContext) FireWrite(obj interface{}) HandlerContext {
	if d.prev() != nil {
		if c, ok := d.prev().(*DefaultHandlerContext); ok {
			defer c.deferErrorCaught()
		}

		d.prev().handler().Write(d.prev(), obj)
	}

	return d
}

func (d *DefaultHandlerContext) FireErrorCaught(err error) HandlerContext {
	if d.prev() != nil {
		if c, ok := d.prev().(*DefaultHandlerContext); ok {
			defer c.deferErrorCaught()
		}

		d.prev().handler().ErrorCaught(d.prev(), err)
	}

	return d
}

func (d *DefaultHandlerContext) Bind(localAddr net.Addr) HandlerContext {
	if d.prev() != nil {
		if c, ok := d.prev().(*DefaultHandlerContext); ok {
			defer c.deferErrorCaught()
		}

		d.prev().handler().Bind(d.prev(), localAddr)
	}

	return d
}

func (d *DefaultHandlerContext) Close() HandlerContext {
	if d.prev() != nil {
		if c, ok := d.prev().(*DefaultHandlerContext); ok {
			defer c.deferErrorCaught()
		}

		d.prev().handler().Close(d.prev())
	}

	return d
}

func (d *DefaultHandlerContext) Connect(remoteAddr net.Addr, localAddr net.Addr) HandlerContext {
	if d.prev() != nil {
		if c, ok := d.prev().(*DefaultHandlerContext); ok {
			defer c.deferErrorCaught()
		}

		d.prev().handler().Connect(d.prev(), remoteAddr, localAddr)
	}

	return d
}

func (d *DefaultHandlerContext) Disconnect() HandlerContext {
	if d.prev() != nil {
		if c, ok := d.prev().(*DefaultHandlerContext); ok {
			defer c.deferErrorCaught()
		}

		d.prev().handler().Disconnect(d.prev())
	}

	return d
}

func (d *DefaultHandlerContext) prev() HandlerContext {
	return d.prevCtx
}

func (d *DefaultHandlerContext) deferErrorCaught() {
	if err := recover(); err != nil {
		switch e := err.(type) {
		case error:
			kklogger.ErrorJ("HandlerContext.ErrorCaught", e.Error())
			d.handler().ErrorCaught(d, e)
		default:
			kklogger.ErrorJ("HandlerContext.ErrorCaught", fmt.Sprintf("%v", e))
			d.handler().ErrorCaught(d, fmt.Errorf("%v", e))
		}
	}
}

type LogStruct struct {
	Action  string
	Handler string
}
