package gudp

import (
	"net"
	"sync/atomic"

	"github.com/yetiz-org/gone/channel"
)

type ClientHandler struct {
	channel.DefaultHandler
	regTrigCount, actTrigCount int32
}

func (h *ClientHandler) Registered(ctx channel.HandlerContext) {
	println("UDP client registered")
	atomic.AddInt32(&h.regTrigCount, 1)
	ctx.FireRegistered()
}

func (h *ClientHandler) Unregistered(ctx channel.HandlerContext) {
	println("UDP client unregistered")
	atomic.AddInt32(&h.regTrigCount, -1)
	ctx.FireUnregistered()
}

func (h *ClientHandler) Active(ctx channel.HandlerContext) {
	println("UDP client active")
	atomic.AddInt32(&h.actTrigCount, 1)
	ctx.FireActive()
}

func (h *ClientHandler) Inactive(ctx channel.HandlerContext) {
	println("UDP client inactive")
	atomic.AddInt32(&h.actTrigCount, -1)
	ctx.FireInactive()
}

func (h *ClientHandler) Read(ctx channel.HandlerContext, obj any) {
	println("UDP client read " + obj.(string))
}

func (h *ClientHandler) ReadCompleted(ctx channel.HandlerContext) {
	println("UDP client read_completed")
}

func (h *ClientHandler) Write(ctx channel.HandlerContext, obj any, future channel.Future) {
	println("UDP client write")
	ctx.Write(obj, future)
}

func (h *ClientHandler) Connect(ctx channel.HandlerContext, localAddr net.Addr, remoteAddr net.Addr, future channel.Future) {
	println("UDP client connect")
	ctx.Connect(localAddr, remoteAddr, future)
}

func (h *ClientHandler) Disconnect(ctx channel.HandlerContext, future channel.Future) {
	println("UDP client disconnect")
	ctx.Disconnect(future)
}

func (h *ClientHandler) Deregister(ctx channel.HandlerContext, future channel.Future) {
	println("UDP client deregister")
	ctx.Deregister(future)
}
