package channel

import (
	"fmt"
	"net"

	"github.com/kklab-com/gone/concurrent"
	"github.com/kklab-com/goth-kklogger"
	kkpanic "github.com/kklab-com/goth-panic"
)

type Pipeline interface {
	AddLast(name string, elem Handler) Pipeline
	RemoveFirst() Pipeline
	Remove(elem Handler) Pipeline
	RemoveByName(name string) Pipeline
	Clear() Pipeline
	Channel() Channel
	Param(key ParamKey) interface{}
	SetParam(key ParamKey, value interface{}) Pipeline
	Params() *Params
	fireRegistered() Pipeline
	fireUnregistered() Pipeline
	fireActive() Pipeline
	fireInactive() Pipeline
	fireRead(obj interface{}) Pipeline
	fireReadCompleted() Pipeline
	fireErrorCaught(err error) Pipeline
	Read() Pipeline
	Write(obj interface{}) Future
	Bind(localAddr net.Addr) Future
	Close() Future
	Connect(localAddr net.Addr, remoteAddr net.Addr) Future
	Disconnect() Future
	Deregister() Future
	newFuture() Future
}

type PipelineSetChannel interface {
	SetChannel(channel Channel)
}

const PipelineHeadHandlerContextName = "DEFAULT_HEAD_HANDLER_CONTEXT"
const PipelineTailHandlerContextName = "DEFAULT_TAIL_HANDLER_CONTEXT"

type DefaultPipeline struct {
	head    HandlerContext
	tail    HandlerContext
	carrier Params
	channel Channel
}

func (p *DefaultPipeline) Channel() Channel {
	return p.channel
}

func (p *DefaultPipeline) RemoveFirst() Pipeline {
	final := p.head
	if final.next() == nil {
		return p
	}

	next := final.next()
	if next.next() != nil {
		next.next().setPrev(final)
		final.setNext(next.next())
	}

	next.setNext(nil)
	next.setPrev(nil)
	return p
}

func _NewDefaultPipeline(channel Channel) Pipeline {
	pipeline := new(DefaultPipeline)
	pipeline.head = pipeline._NewHeadHandlerContext(channel)
	pipeline.tail = pipeline._NewTailHandlerContext(channel)
	pipeline.head.setNext(pipeline.tail)
	pipeline.tail.setPrev(pipeline.head)
	pipeline.channel = channel
	return pipeline
}

func (p *DefaultPipeline) _NewHeadHandlerContext(channel Channel) HandlerContext {
	context := new(DefaultHandlerContext)
	context.name = PipelineHeadHandlerContextName
	context._handler = &headHandler{}
	context.pipeline = p
	return context
}

func (p *DefaultPipeline) _NewTailHandlerContext(channel Channel) HandlerContext {
	context := new(DefaultHandlerContext)
	context.name = PipelineTailHandlerContextName
	context._handler = &tailHandler{}
	context.pipeline = p
	return context
}

type headHandler struct {
	DefaultHandler
}

func (h *headHandler) read(ctx HandlerContext) {
	if channel, ok := ctx.Channel().(UnsafeRead); ok && ctx.Channel().IsActive() {
		if err := channel.UnsafeRead(); err != nil {
			ctx.Channel().inactiveChannel()
		}
	}
}

func (h *headHandler) Write(ctx HandlerContext, obj interface{}, future Future) {
	if channel, ok := ctx.Channel().(UnsafeWrite); ok && ctx.Channel().IsActive() {
		if err := channel.UnsafeWrite(obj); err != nil {
			ctx.Channel().inactiveChannel()
			h.futureCancel(future)
		} else {
			h.futureSuccess(future)
		}
	}
}

func (h *headHandler) Bind(ctx HandlerContext, localAddr net.Addr, future Future) {
	if channel, ok := ctx.Channel().(UnsafeBind); ok && !ctx.Channel().CloseFuture().IsDone() {
		if err := channel.UnsafeBind(localAddr); err != nil {
			kklogger.WarnJ("HeadHandler.Bind", err.Error())
			ctx.Channel().inactiveChannel()
			h.futureCancel(future)
		} else {
			ctx.Channel().activeChannel()
			if channel, ok := ctx.Channel().(UnsafeAccept); ok {
				go func() {
					for ctx.Channel().IsActive() {
						if child := channel.UnsafeAccept(); child == nil {
							if ctx.Channel().IsActive() {
								kklogger.WarnJ("HeadHandler.UnsafeAccept", "nil child")
							}

							return
						} else {
							child.Pipeline().fireRegistered()
							child.activeChannel()
						}
					}
				}()
			}

			h.futureSuccess(future)
		}
	}
}

func (h *headHandler) Close(ctx HandlerContext, future Future) {
	if channel, ok := ctx.Channel().(UnsafeClose); ok && !ctx.Channel().CloseFuture().IsDone() {
		ctx.Channel().inactiveChannel()
		err := channel.UnsafeClose()
		if err != nil {
			kklogger.ErrorJ("HeadHandler.Close", err.Error())
		}

		if err != nil {
			h.futureCancel(future)
		} else {
			h.futureSuccess(future)
		}
	}
}

func (h *headHandler) Connect(ctx HandlerContext, localAddr net.Addr, remoteAddr net.Addr, future Future) {
	if channel, ok := ctx.Channel().(UnsafeConnect); ok && !ctx.Channel().CloseFuture().IsDone() {
		if err := channel.UnsafeConnect(localAddr, remoteAddr); err != nil {
			kklogger.ErrorJ("HeadHandler.Connect", err.Error())
			ctx.Channel().inactiveChannel()
			h.futureCancel(future)
		} else {
			ctx.Channel().activeChannel()
			h.futureSuccess(future)
		}
	}
}

func (h *headHandler) Disconnect(ctx HandlerContext, future Future) {
	if channel, ok := ctx.Channel().(UnsafeDisconnect); ok && !ctx.Channel().CloseFuture().IsDone() {
		ctx.Channel().inactiveChannel()
		err := channel.UnsafeDisconnect()
		//if err != nil {
		//	kklogger.ErrorJ("HeadHandler.Disconnect", err.Error())
		//}

		if err != nil {
			h.futureCancel(future)
		} else {
			h.futureSuccess(future)
		}
	}
}

func (h *headHandler) futureCancel(future Future) {
	future.(concurrent.ManualFuture).Cancel()
}

func (h *headHandler) futureSuccess(future Future) {
	future.(concurrent.ManualFuture).Success()
}

func (h *headHandler) ErrorCaught(ctx HandlerContext, err error) {
	var ce kkpanic.Caught
	if e, ok := err.(*kkpanic.CaughtImpl); ok {
		ce = e
	} else {
		ce = kkpanic.Convert(e)
	}

	kklogger.ErrorJ("HeadHandler.ErrorCaught", ce)
}

type tailHandler struct {
	DefaultHandler
}

func (h *tailHandler) Read(ctx HandlerContext, obj interface{}) {
	ctx.FireErrorCaught(fmt.Errorf("message doesn't be catched"))
}

func (h *tailHandler) Deregister(ctx HandlerContext, future Future) {
	ctx.Channel().inactiveChannel()
	future.(concurrent.ManualFuture).Success()
}

func (p *DefaultPipeline) AddLast(name string, elem Handler) Pipeline {
	final := p.tail
	ctx := NewHandlerContext()
	ctx.pipeline = p
	ctx.name = name
	ctx.setNext(final)
	ctx.setPrev(final.prev())
	ctx.next().setPrev(ctx)
	ctx.prev().setNext(ctx)
	ctx._handler = elem
	ctx._handler.Added(p.head)

	return p
}

func (p *DefaultPipeline) Remove(elem Handler) Pipeline {
	final := p.head.next()
	for final != nil && final != p.tail {
		if final.handler() == elem {
			final.next().setPrev(final.prev())
			final.prev().setNext(final.next())
			final.handler().Removed(final)
			break
		}

		final = final.next()
	}

	return p
}

func (p *DefaultPipeline) RemoveByName(name string) Pipeline {
	final := p.head.next()
	for final != nil {
		if final.Name() == name &&
			name != PipelineHeadHandlerContextName &&
			name != PipelineTailHandlerContextName {
			final.next().setPrev(final.prev())
			final.prev().setNext(final.next())
			final.handler().Removed(final)
			break
		}

		final = final.next()
	}

	return p
}

func (p *DefaultPipeline) Clear() Pipeline {
	if next := p.head.next(); next != nil {
		next.setPrev(nil)
	}

	if prev := p.tail.prev(); prev != nil {
		prev.setNext(nil)
	}

	p.head.setNext(p.tail)
	p.tail.setPrev(p.head)
	return p
}

func (p *DefaultPipeline) Param(key ParamKey) interface{} {
	if v, f := p.carrier.Load(key); f {
		return v
	}

	return nil
}

func (p *DefaultPipeline) SetParam(key ParamKey, value interface{}) Pipeline {
	p.carrier.Store(key, value)
	return p
}

func (p *DefaultPipeline) Params() *Params {
	return &p.carrier
}

func (p *DefaultPipeline) fireRegistered() Pipeline {
	p.head.FireRegistered()
	return p
}

func (p *DefaultPipeline) fireUnregistered() Pipeline {
	p.head.FireUnregistered()
	return p
}

func (p *DefaultPipeline) fireActive() Pipeline {
	p.head.FireActive()
	return p
}

func (p *DefaultPipeline) fireInactive() Pipeline {
	p.head.FireInactive()
	return p
}

func (p *DefaultPipeline) fireRead(obj interface{}) Pipeline {
	p.head.FireRead(obj)
	return p
}

func (p *DefaultPipeline) fireReadCompleted() Pipeline {
	p.head.FireReadCompleted()
	return p
}

func (p *DefaultPipeline) fireErrorCaught(err error) Pipeline {
	p.head.FireErrorCaught(err)
	return p
}

func (p *DefaultPipeline) Read() Pipeline {
	p.tail.read()
	return p
}

func (p *DefaultPipeline) Write(obj interface{}) Future {
	return p.tail.Write(obj, p.newFuture())
}

func (p *DefaultPipeline) Bind(localAddr net.Addr) Future {
	return p.tail.Bind(localAddr, p.newFuture())
}

func (p *DefaultPipeline) Close() Future {
	return p.tail.Close(p.newFuture())
}

func (p *DefaultPipeline) Connect(localAddr net.Addr, remoteAddr net.Addr) Future {
	return p.tail.Connect(localAddr, remoteAddr, p.newFuture())
}

func (p *DefaultPipeline) Disconnect() Future {
	return p.tail.Disconnect(p.newFuture())
}

func (p *DefaultPipeline) Deregister() Future {
	return p.head.Deregister(p.newFuture())
}

func (p *DefaultPipeline) newFuture() Future {
	return NewFuture(p.Channel())
}

func (p *DefaultPipeline) SetChannel(channel Channel) {
	p.channel = channel
}
