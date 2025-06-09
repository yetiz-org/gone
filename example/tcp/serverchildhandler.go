package tcp

import (
	"net"
	"sync/atomic"
	"time"

	buf "github.com/kklab-com/goth-bytebuf"
	"github.com/yetiz-org/gone/channel"
)

type ServerChildHandler struct {
	channel.DefaultHandler
	regTrigCount, actTrigCount int32
}

func (h *ServerChildHandler) Registered(ctx channel.HandlerContext) {
	println("server registered")
	atomic.AddInt32(&h.regTrigCount, 1)
	ctx.FireRegistered()
}

func (h *ServerChildHandler) Unregistered(ctx channel.HandlerContext) {
	println("server unregistered")
	atomic.AddInt32(&h.regTrigCount, -1)
	ctx.FireUnregistered()
}

func (h *ServerChildHandler) Active(ctx channel.HandlerContext) {
	println("server active")
	atomic.AddInt32(&h.actTrigCount, 1)
	ctx.FireActive()
}

func (h *ServerChildHandler) Inactive(ctx channel.HandlerContext) {
	println("server inactive")
	atomic.AddInt32(&h.actTrigCount, -1)
	ctx.FireInactive()
}

func (h *ServerChildHandler) Read(ctx channel.HandlerContext, obj any) {
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

func (h *ServerChildHandler) Write(ctx channel.HandlerContext, obj any, future channel.Future) {
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

func (h *ServerChildHandler) Deregister(ctx channel.HandlerContext, future channel.Future) {
	println("server deregister")
	ctx.Disconnect(future)
}
