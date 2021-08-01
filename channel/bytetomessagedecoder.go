package channel

import (
	"github.com/kklab-com/gone/utils"
	"github.com/kklab-com/goth-kkutil/buf"
)

type MessageDecoder interface {
	Decode(ctx HandlerContext, in buf.ByteBuf, out *utils.Queue)
}

type ByteToMessageDecoder struct {
	DefaultHandler
	Decode func(ctx HandlerContext, in buf.ByteBuf, out *utils.Queue)
}

func (h *ByteToMessageDecoder) Added(ctx HandlerContext) {
	if h.Decode == nil {
		h.Decode = h.decode
	}
}

func (h *ByteToMessageDecoder) Read(ctx HandlerContext, obj interface{}) {
	out := &utils.Queue{}
	h.Decode(ctx, obj.(buf.ByteBuf), out)
	for elem := out.Pop(); elem != nil; elem = out.Pop() {
		ctx.FireRead(elem)
	}

	ctx.FireReadCompleted()
}

func (h *ByteToMessageDecoder) decode(ctx HandlerContext, in buf.ByteBuf, out *utils.Queue) {
	out.Push(in)
}
