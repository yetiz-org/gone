package tcp

import (
	"time"

	"github.com/kklab-com/gone/channel"
	"github.com/kklab-com/goth-kkutil/buf"
)

type ServerChildHandler struct {
	channel.DefaultHandler
}

func (h *ServerChildHandler) Read(ctx channel.HandlerContext, obj interface{}) {
	str := obj.(string)
	if str != "h:c b:cc" {
		ctx.Write(buf.NewByteBuf([]byte(str)), nil)
	} else {
		ctx.Write(buf.NewByteBuf([]byte(str)), nil)
		ctx.Channel().Disconnect()
		time.Sleep(time.Millisecond * 100)
		ctx.Channel().(channel.NetChannel).Parent().Close()
	}
}

func (h *ServerChildHandler) ReadCompleted(ctx channel.HandlerContext) {
	println("server read_completed")
}

func (h *ServerChildHandler) Disconnect(ctx channel.HandlerContext, future channel.Future) {
	println("server disconnect")
	ctx.Disconnect(future)
}

func (h *ServerChildHandler) Active(ctx channel.HandlerContext) {
	println("server active")
	ctx.FireActive()
}

func (h *ServerChildHandler) Inactive(ctx channel.HandlerContext) {
	println("server inactive")
	ctx.FireInactive()
}
