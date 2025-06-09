package example

import (
	"sync/atomic"

	"github.com/yetiz-org/gone/channel"
)

type ServerChildCountHandler struct {
	channel.DefaultHandler
	regTrigCount, actTrigCount int32
}

func (h *ServerChildCountHandler) Registered(ctx channel.HandlerContext) {
	atomic.AddInt32(&h.regTrigCount, 1)
	ctx.FireRegistered()
}

func (h *ServerChildCountHandler) Unregistered(ctx channel.HandlerContext) {
	atomic.AddInt32(&h.regTrigCount, -1)
	ctx.FireUnregistered()
}

func (h *ServerChildCountHandler) Active(ctx channel.HandlerContext) {
	atomic.AddInt32(&h.actTrigCount, 1)
	ctx.FireActive()
}

func (h *ServerChildCountHandler) Inactive(ctx channel.HandlerContext) {
	atomic.AddInt32(&h.actTrigCount, -1)
	ctx.FireInactive()
}

func (h *ServerChildCountHandler) Write(ctx channel.HandlerContext, obj any, future channel.Future) {
	ctx.Write(obj, future)
}

func (h *ServerChildCountHandler) Disconnect(ctx channel.HandlerContext, future channel.Future) {
	ctx.Disconnect(future)
}

func (h *ServerChildCountHandler) Deregister(ctx channel.HandlerContext, future channel.Future) {
	ctx.Deregister(future)
}
