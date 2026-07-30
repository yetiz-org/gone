package channel

import buf "github.com/yetiz-org/goth-bytebuf"

type MessageEncoder interface {
	Encode(ctx HandlerContext, msg any, out buf.ByteBuf)
}

type MessageToByteEncoder struct {
	DefaultHandler
	Encode func(ctx HandlerContext, msg any, out buf.ByteBuf)
}

func (h *MessageToByteEncoder) Write(ctx HandlerContext, obj any, future Future) {
	if h.Encode == nil {
		b, ok := obj.(buf.ByteBuf)
		if !ok {
			if future == nil {
				future = NewFuture(ctx.Channel())
			}
			future.Completable().Fail(ErrUnknownObjectType)
			ctx.FireErrorCaught(ErrUnknownObjectType)
			return
		}

		out := buf.EmptyByteBuf()
		out.WriteBytes(b.Bytes())
		ctx.Write(out, future)
		return
	}

	out := buf.EmptyByteBuf()
	h.Encode(ctx, obj, out)
	ctx.Write(out, future)
}
