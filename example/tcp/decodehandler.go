package tcp

import (
	"container/list"

	"github.com/kklab-com/gone/channel"
	"github.com/kklab-com/goth-kkutil/buf"
)

type DecodeHandler struct {
	*channel.ReplayDecoder
	obj string
}

const HEAD = channel.ReplayState(1)
const BODY = channel.ReplayState(2)

func NewDecodeHandler() *DecodeHandler {
	handler := &DecodeHandler{}
	handler.ReplayDecoder = channel.NewReplayDecoder(HEAD, handler.decode)
	return handler
}

func (h *DecodeHandler) decode(ctx channel.HandlerContext, in buf.ByteBuf, out *list.List) {
	for true {
		switch h.State() {
		case HEAD:
			bs := in.ReadByte()
			h.obj = "h:" + string(bs)
			h.Checkpoint(BODY)
		case BODY:
			bs := in.ReadBytes(2)
			out.PushFront(h.obj + " b:" + string(bs))
			h.obj = ""
			h.Checkpoint(HEAD)
		}
	}
}
