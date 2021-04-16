package tcp

import "github.com/kklab-com/gone/channel"

type ClientHandler struct {
	channel.DefaultHandler
}

func (h *ClientHandler) Read(ctx channel.HandlerContext, obj interface{}) {
	println(obj.(string))
}

func (h *ClientHandler) ReadCompleted(ctx channel.HandlerContext) {
	println("client read_completed")
}

func (h *ClientHandler) Disconnect(ctx channel.HandlerContext, future channel.Future) {
	println("client disconnect")
	ctx.Disconnect(future)
}

func (h *ClientHandler) Active(ctx channel.HandlerContext) {
	println("client active")
	ctx.FireActive()
}

func (h *ClientHandler) Inactive(ctx channel.HandlerContext) {
	println("client inactive")
	ctx.FireInactive()
}
