package channel

import (
	"container/list"

	"github.com/kklab-com/goth-kkutil/buf"
)

type MessageDecoder interface {
	Decode(ctx HandlerContext, in buf.ByteBuf, out *list.List)
}

type ByteToMessageDecoder struct {
	DefaultHandler
	Decode func(ctx HandlerContext, in buf.ByteBuf, out *list.List)
}

func (h *ByteToMessageDecoder) Added(ctx HandlerContext) {
	if h.Decode == nil {
		h.Decode = h.decode
	}
}

func (h *ByteToMessageDecoder) Read(ctx HandlerContext, obj interface{}) {
	out := &list.List{}
	h.Decode(ctx, obj.(buf.ByteBuf), out)
	for elem := out.Back(); elem != nil; func() {
		out.Remove(elem)
		elem = out.Back()
	}() {
		ctx.FireRead(elem.Value)
	}

	ctx.FireReadCompleted()
}

func (h *ByteToMessageDecoder) decode(ctx HandlerContext, in buf.ByteBuf, out *list.List) {
	out.PushFront(in)
}
