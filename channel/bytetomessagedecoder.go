package channel

import (
	buf "github.com/kklab-com/goth-bytebuf"
	"github.com/kklab-com/goth-kkutil/structs"
	"github.com/yetiz-org/gone/utils"
)

type MessageDecoder interface {
	Decode(ctx HandlerContext, in buf.ByteBuf, out structs.Queue)
}

type ByteToMessageDecoder struct {
	DefaultHandler
	Decode func(ctx HandlerContext, in buf.ByteBuf, out structs.Queue)
}

func (h *ByteToMessageDecoder) Added(ctx HandlerContext) {
	if h.Decode == nil {
		h.Decode = h.decode
	}
}

func (h *ByteToMessageDecoder) Read(ctx HandlerContext, obj any) {
	out := &utils.Queue{}
	h.Decode(ctx, obj.(buf.ByteBuf), out)
	for elem := out.Pop(); elem != nil; elem = out.Pop() {
		ctx.FireRead(elem)
	}

	ctx.FireReadCompleted()
}

func (h *ByteToMessageDecoder) decode(ctx HandlerContext, in buf.ByteBuf, out structs.Queue) {
	out.Push(in)
}
