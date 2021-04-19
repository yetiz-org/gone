package tcp

import "github.com/kklab-com/gone/channel"

type ClientHandler struct {
	channel.DefaultHandler
}

func (h *ClientHandler) Registered(ctx channel.HandlerContext) {
	println("client registered")
	ctx.FireRegistered()
}

func (h *ClientHandler) Unregistered(ctx channel.HandlerContext) {
	println("client unregistered")
	ctx.FireUnregistered()
}

func (h *ClientHandler) Active(ctx channel.HandlerContext) {
	println("client active")
	ctx.FireActive()
}

func (h *ClientHandler) Inactive(ctx channel.HandlerContext) {
	println("client inactive")
	ctx.FireInactive()
}

func (h *ClientHandler) Read(ctx channel.HandlerContext, obj interface{}) {
	println("client read " + obj.(string))
}

func (h *ClientHandler) ReadCompleted(ctx channel.HandlerContext) {
	println("client read_completed")
}

func (h *ClientHandler) Disconnect(ctx channel.HandlerContext, future channel.Future) {
	println("client disconnect")
	ctx.Disconnect(future)
}
