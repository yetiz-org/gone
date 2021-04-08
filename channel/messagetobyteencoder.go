package channel

import "bytes"

type MessageEncoder interface {
	Encode(ctx HandlerContext, msg interface{}, out *bytes.Buffer)
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
	out := &bytes.Buffer{}
	h.Encoder.Encode(ctx, obj, out)
	ctx.FireWrite(out)
}

func (h *MessageToByteEncoder) Encode(ctx HandlerContext, msg interface{}, out *bytes.Buffer) {
	panic("implement me")
}
