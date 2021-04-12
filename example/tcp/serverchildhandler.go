package tcp

import (
	"github.com/kklab-com/gone/channel"
	"github.com/kklab-com/goth-kkutil/buf"
)

type ServerChildHandler struct {
	channel.DefaultHandler
}

func (h *ServerChildHandler) Read(ctx channel.HandlerContext, obj interface{}) {
	println(obj.(string))
	ctx.FireWrite(buf.NewByteBuf([]byte(obj.(string))))
}

func (h *ServerChildHandler) ReadCompleted(ctx channel.HandlerContext) {
	println("server read_completed")
}

func (h *ServerChildHandler) Disconnect(ctx channel.HandlerContext) {
	println("server disconnect")
	ctx.Disconnect()
}
