package tcp

import (
	"net"
	"time"

	"github.com/kklab-com/gone/channel"
	"github.com/kklab-com/goth-kkutil/buf"
)

type ServerChildHandler struct {
	channel.DefaultHandler
}

func (h *ServerChildHandler) Registered(ctx channel.HandlerContext) {
	println("server registered")
	ctx.FireRegistered()
}

func (h *ServerChildHandler) Unregistered(ctx channel.HandlerContext) {
	println("server unregistered")
	ctx.FireUnregistered()
}

func (h *ServerChildHandler) Active(ctx channel.HandlerContext) {
	println("server active")
	ctx.FireActive()
}

func (h *ServerChildHandler) Inactive(ctx channel.HandlerContext) {
	println("server inactive")
	ctx.FireInactive()
}

func (h *ServerChildHandler) Read(ctx channel.HandlerContext, obj interface{}) {
	str := obj.(string)
	println("server read " + str)
	if str != "h:c b:cc" {
		ctx.Write(buf.NewByteBuf([]byte(str)), nil)
	} else {
		ctx.Write(buf.NewByteBuf([]byte(str)), nil)
		time.Sleep(time.Millisecond * 100)
		ctx.Channel().Disconnect()
		ctx.Channel().(channel.NetChannel).Parent().Close()
	}
}

func (h *ServerChildHandler) ReadCompleted(ctx channel.HandlerContext) {
	println("server read_completed")
}

func (h *ServerChildHandler) Write(ctx channel.HandlerContext, obj interface{}, future channel.Future) {
	println("server write")
	(ctx).Write(obj, future)
}

func (h *ServerChildHandler) Connect(ctx channel.HandlerContext, localAddr net.Addr, remoteAddr net.Addr, future channel.Future) {
	println("server connect")
	ctx.Connect(localAddr, remoteAddr, future)
}

func (h *ServerChildHandler) Disconnect(ctx channel.HandlerContext, future channel.Future) {
	println("server disconnect")
	ctx.Disconnect(future)
}
