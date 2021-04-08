package channel

import (
	"bytes"
	"container/list"
)

type MessageDecoder interface {
	Decode(ctx HandlerContext, in *bytes.Buffer, out *list.List)
}

type ByteToMessageDecoder struct {
	DefaultHandler
	outList list.List
	Decoder MessageDecoder
}

func (h *ByteToMessageDecoder) Added(ctx HandlerContext) {
	if h.Decoder == nil {
		h.Decoder = h
	}
}

func (h *ByteToMessageDecoder) Read(ctx HandlerContext, obj interface{}) {
	h.Decoder.Decode(ctx, obj.(*bytes.Buffer), &h.outList)
	if elem := h.outList.Back(); elem != nil {
		ctx.FireRead(elem.Value)
	}

	ctx.FireReadCompleted()
}

func (h *ByteToMessageDecoder) Decode(ctx HandlerContext, in *bytes.Buffer, out *list.List) {
	out.PushFront(in)
}
