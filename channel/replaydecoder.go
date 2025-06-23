package channel

import (
	"sync"

	"github.com/yetiz-org/gone/utils"
	buf "github.com/yetiz-org/goth-bytebuf"
	kklogger "github.com/yetiz-org/goth-kklogger"
	kkpanic "github.com/yetiz-org/goth-panic"
	"github.com/yetiz-org/goth-util/structs"
)

type ReplayState int

type ReplayDecoder struct {
	ByteToMessageDecoder
	in    buf.ByteBuf
	state ReplayState
	op    sync.Mutex
}

var replayDecoderTruncateLen = 1 << 20

func NewReplayDecoder(state ReplayState, decode func(ctx HandlerContext, in buf.ByteBuf, out structs.Queue)) *ReplayDecoder {
	return &ReplayDecoder{
		ByteToMessageDecoder: ByteToMessageDecoder{
			Decode: decode,
		},
		state: state,
	}
}

func (h *ReplayDecoder) Skip() {
	panic(buf.ErrInsufficientSize)
}

func (h *ReplayDecoder) State() ReplayState {
	return h.state
}

func (h *ReplayDecoder) Checkpoint(state ReplayState) {
	h.state = state
	if h.in.Cap()-h.in.ReadableBytes() > replayDecoderTruncateLen {
		h.op.Lock()
		defer h.op.Unlock()
		bs := h.in.Bytes()
		h.in.Reset()
		h.in.Write(bs)
	}
}

func (h *ReplayDecoder) Added(ctx HandlerContext) {
	h.in = buf.EmptyByteBuf()
}

func (h *ReplayDecoder) Read(ctx HandlerContext, obj any) {
	if h.Decode != nil {
		h.in.Write(obj.(buf.ByteBuf).Bytes())
		out := &utils.Queue{}
		kkpanic.CatchExcept(func() {
			h.Decode(ctx, h.in, out)
		}, buf.ErrInsufficientSize, func(r kkpanic.Caught) {
			kklogger.ErrorJ("ReplayDecoder.Read#Decode", r.String())
		})

		for elem := out.Pop(); elem != nil; elem = out.Pop() {
			ctx.FireRead(elem)
		}
	} else {
		kklogger.WarnJ("ReplayDecoder.Read#Decode", "no decoder")
	}
}
