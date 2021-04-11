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

func (h *ClientHandler) Disconnect(ctx channel.HandlerContext) {
	println("client disconnect")
	ctx.Disconnect()
}
