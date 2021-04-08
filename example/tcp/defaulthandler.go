package tcp

import "github.com/kklab-com/gone/channel"

type DefaultHandler struct {
	channel.DefaultHandler
}

func (h *DefaultHandler) Read(ctx channel.HandlerContext, obj interface{}) {
	println(obj.(string))
}

func (h *DefaultHandler) ReadCompleted(ctx channel.HandlerContext) {
	println("read_completed")
}

func (h *DefaultHandler) Disconnect(ctx channel.HandlerContext) {
	println("disconnect")
	ctx.Disconnect()
}
