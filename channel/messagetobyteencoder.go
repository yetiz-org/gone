package channel

import (
	"github.com/kklab-com/goth-kkutil/buf"
)

type MessageEncoder interface {
	Encode(ctx HandlerContext, msg interface{}, out buf.ByteBuf)
}

type MessageToByteEncoder struct {
	DefaultHandler
	Encoder MessageEncoder
}

func (h *MessageToByteEncoder) Added(ctx HandlerContext) {
	if h.Encoder == nil {
		h.Encoder = h
	}
}

func (h *MessageToByteEncoder) Write(ctx HandlerContext, obj interface{}) {
	out := buf.EmptyByteBuf()
	h.Encoder.Encode(ctx, obj, out)
	ctx.FireWrite(out)
}

func (h *MessageToByteEncoder) Encode(ctx HandlerContext, msg interface{}, out buf.ByteBuf) {
	panic("implement me")
}
