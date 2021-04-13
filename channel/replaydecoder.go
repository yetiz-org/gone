package channel

import (
	"container/list"
	"sync"

	kklogger "github.com/kklab-com/goth-kklogger"
	"github.com/kklab-com/goth-kkutil/buf"
	kkpanic "github.com/kklab-com/goth-panic"
)

type ReplayState int

type ReplayDecoder struct {
	ByteToMessageDecoder
	in    buf.ByteBuf
	state ReplayState
	op    sync.Mutex
}

var replayDecoderTruncateLen = 1 << 20

func NewReplayDecoder(state ReplayState, decode func(ctx HandlerContext, in buf.ByteBuf, out *list.List)) *ReplayDecoder {
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

func (h *ReplayDecoder) Read(ctx HandlerContext, obj interface{}) {
	if h.Decode != nil {
		h.in.Write(obj.(buf.ByteBuf).Bytes())
		out := &list.List{}
		kkpanic.Catch(func() {
			h.Decode(ctx, h.in, out)
		}, func(r *kkpanic.Caught) {
			if r.Message != buf.ErrInsufficientSize {
				kklogger.ErrorJ("ReplayDecoder.Read#Decode", r.String())
				return
			}
		})

		for elem := out.Back(); elem != nil; func() {
			out.Remove(elem)
			elem = out.Back()
		}() {
			ctx.FireRead(elem.Value)
		}
	} else {
		kklogger.WarnJ("ReplayDecoder.Read#Decode", "no decoder")
	}
}
