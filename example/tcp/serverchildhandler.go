package tcp

import (
	"github.com/kklab-com/gone/channel"
	"github.com/kklab-com/goth-kkutil/buf"
)

type ServerChildHandler struct {
	channel.DefaultHandler
}

func (h *ServerChildHandler) Read(ctx channel.HandlerContext, obj interface{}) {
	str := obj.(string)
	if str != "h:c b:cc" {
		ctx.FireWrite(buf.NewByteBuf([]byte(str)))
	} else {
		ctx.Channel().Disconnect()
	}
}

func (h *ServerChildHandler) ReadCompleted(ctx channel.HandlerContext) {
	println("server read_completed")
}

func (h *ServerChildHandler) Disconnect(ctx channel.HandlerContext) {
	println("server disconnect")
	ctx.Disconnect()
}

func (h *ServerChildHandler) Active(ctx channel.HandlerContext) {
	println("server active")
	ctx.FireActive()
}

func (h *ServerChildHandler) Inactive(ctx channel.HandlerContext) {
	println("server inactive")
	ctx.FireInactive()
}
