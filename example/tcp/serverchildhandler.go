package tcp

import (
	"bytes"

	"github.com/kklab-com/gone/channel"
)

type ServerChildHandler struct {
	channel.DefaultHandler
}

func (h *ServerChildHandler) Read(ctx channel.HandlerContext, obj interface{}) {
	println(obj.(string))
	ctx.FireWrite(bytes.NewBufferString(obj.(string)))
}

func (h *ServerChildHandler) ReadCompleted(ctx channel.HandlerContext) {
	println("server read_completed")
}

func (h *ServerChildHandler) Disconnect(ctx channel.HandlerContext) {
	println("server disconnect")
	ctx.Disconnect()
}
