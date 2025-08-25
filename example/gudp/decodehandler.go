package gudp

import (
	"github.com/yetiz-org/gone/channel"
	buf "github.com/yetiz-org/goth-bytebuf"
	"github.com/yetiz-org/goth-util/structs"
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

func (h *DecodeHandler) decode(ctx channel.HandlerContext, in buf.ByteBuf, out structs.Queue) {
	for true {
		switch h.State() {
		case HEAD:
			bs := in.MustReadByte()
			h.obj = "udp_h:" + string(bs)
			h.Checkpoint(BODY)
		case BODY:
			bs := in.ReadBytes(2)
			out.Push(h.obj + " udp_b:" + string(bs))
			h.obj = ""
			h.Checkpoint(HEAD)
		}
	}
}
