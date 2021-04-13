package channel

import (
	"bytes"
	"fmt"
	"net"
	"runtime/debug"
	"runtime/pprof"

	"github.com/kklab-com/goth-kklogger"
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
	fireActive() Pipeline
	fireInactive() Pipeline
	fireRead(obj interface{}) Pipeline
	fireReadCompleted() Pipeline
	fireErrorCaught(err error) Pipeline
	Write(obj interface{}) Pipeline
	Bind(localAddr net.Addr) Pipeline
	Close() Pipeline
	Connect(remoteAddr net.Addr) Pipeline
	Disconnect() Pipeline
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

func NewDefaultPipeline(channel Channel) Pipeline {
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
	context.channel = channel
	return context
}

func (p *DefaultPipeline) _NewTailHandlerContext(channel Channel) HandlerContext {
	context := new(DefaultHandlerContext)
	context.name = PipelineTailHandlerContextName
	context._handler = &tailHandler{}
	context.channel = channel
	return context
}

type headHandler struct {
	DefaultHandler
}

func (h *headHandler) Write(ctx HandlerContext, obj interface{}) {
	if channel, ok := ctx.Channel().(ClientChannel); ok {
		if err := channel.unsafe().WriteFunc(obj); err != nil {
			kklogger.ErrorJ("HeadHandler.Write", err.Error())
		}
	}
}

func (h *headHandler) Bind(ctx HandlerContext, localAddr net.Addr) {
	if channel, ok := ctx.Channel().(ServerChannel); ok {
		if err := channel.unsafe().BindFunc(localAddr); err != nil {
			kklogger.ErrorJ("HeadHandler.Bind", err.Error())
		}
	}
}

func (h *headHandler) Close(ctx HandlerContext) {
	if channel, ok := ctx.Channel().(ServerChannel); ok {
		if err := channel.unsafe().CloseFunc(); err != nil {
			kklogger.ErrorJ("HeadHandler.Close", err.Error())
		}
	}
}

func (h *headHandler) Connect(ctx HandlerContext, remoteAddr net.Addr) {
	if channel, ok := ctx.Channel().(ClientChannel); ok {
		if err := channel.unsafe().ConnectFunc(remoteAddr); err != nil {
			kklogger.ErrorJ("HeadHandler.Connect", err.Error())
		}
	}
}

func (h *headHandler) Disconnect(ctx HandlerContext) {
	if channel, ok := ctx.Channel().(ClientChannel); ok {
		if err := channel.unsafe().DisconnectFunc(); err != nil {
			kklogger.ErrorJ("HeadHandler.Disconnect", err.Error())
		}
	}
}

func (h *headHandler) ErrorCaught(ctx HandlerContext, err error) {
	var ce *HandlerCaughtError
	if e, ok := err.(*HandlerCaughtError); ok {
		ce = e
	} else {
		buffer := &bytes.Buffer{}
		pprof.Lookup("goroutine").WriteTo(buffer, 1)
		ce = &HandlerCaughtError{
			PanicCallStack:  string(debug.Stack()),
			GoRoutineStacks: buffer.String(),
		}

		if err != nil {
			ce.Err = err.Error()
		}
	}

	kklogger.ErrorJ("HeadHandler.ErrorCaught", ce)
}

type tailHandler struct {
	DefaultHandler
}

func (h *tailHandler) Read(ctx HandlerContext, obj interface{}) {
	ctx.FireErrorCaught(fmt.Errorf("message doesn't be catched"))
}

func (p *DefaultPipeline) AddLast(name string, elem Handler) Pipeline {
	final := p.tail
	ctx := NewHandlerContext()
	ctx.setChannel(p.channel)
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
	p.head.setNext(nil)
	p.tail.setPrev(nil)
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

func (p *DefaultPipeline) Write(obj interface{}) Pipeline {
	p.tail.FireWrite(obj)
	return p
}

func (p *DefaultPipeline) Bind(localAddr net.Addr) Pipeline {
	p.tail.Bind(localAddr)
	return p
}

func (p *DefaultPipeline) Close() Pipeline {
	p.tail.Close()
	return p
}

func (p *DefaultPipeline) Connect(remoteAddr net.Addr) Pipeline {
	p.tail.Connect(remoteAddr)
	return p
}

func (p *DefaultPipeline) Disconnect() Pipeline {
	p.tail.Disconnect()
	return p
}
