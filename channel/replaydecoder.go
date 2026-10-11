package channel

import (
	"errors"
	"sync"

	"github.com/yetiz-org/gone/utils"
	buf "github.com/yetiz-org/goth-bytebuf"
	kklogger "github.com/yetiz-org/goth-kklogger"
	kkpanic "github.com/yetiz-org/goth-panic"
	"github.com/yetiz-org/goth-util/structs"
)

type ReplayState int

const DefaultMaxBufferedBytes = 16 << 20

var ErrDecoderBufferLimit = errors.New("decoder buffer limit exceeded")
var ErrFrameTooLarge = errors.New("frame length exceeds limit")
var ErrMalformedFrame = errors.New("incomplete datagram frame")

var replayDecoderTruncateLen = 1 << 20

type ReplayDecoder struct {
	ByteToMessageDecoder
	// MaxBufferedBytes bounds the input left unread after decoding; non-positive values use the default.
	MaxBufferedBytes int
	// Datagram rejects incomplete input instead of replaying across reads.
	Datagram     bool
	in           buf.CompositeByteBuf
	state        ReplayState
	initialState ReplayState
	failed       bool
	op           sync.Mutex
}

func NewReplayDecoder(state ReplayState, decode func(ctx HandlerContext, in buf.ByteBuf, out structs.Queue)) (decoder *ReplayDecoder) {
	return &ReplayDecoder{
		Decode:       decode,
		state:        state,
		initialState: state,
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

	if h.in == nil {
		h.in = buf.NewCompositeByteBuf()
	}

	if h.in.Cap()-h.in.ReadableBytes() > replayDecoderTruncateLen {
		h.op.Lock()
		defer h.op.Unlock()
		h.in.Compact()
	}
}

func (h *ReplayDecoder) Added(ctx HandlerContext) {
	h.in = buf.NewCompositeByteBuf()
	h.state = h.initialState
	h.failed = false
}

func (h *ReplayDecoder) Read(ctx HandlerContext, obj any) {
	if h.failed {
		return
	}

	if h.Decode == nil {
		kklogger.WarnJ("channel:ReplayDecoder.Read#decode!no_decoder", "no decoder")
		return
	}

	h.in.AddComponent(obj.(buf.ByteBuf))
	out := &utils.Queue{}
	var decodeErr error
	kkpanic.Catch(func() {
		h.Decode(ctx, h.in, out)
	}, func(r kkpanic.Caught) {
		err, ok := r.Data().(error)
		if !ok {
			err = r
		}

		if !errors.Is(err, buf.ErrInsufficientSize) {
			decodeErr = err
		} else if h.Datagram {
			decodeErr = ErrMalformedFrame
		}
	})

	limit := h.MaxBufferedBytes
	if limit <= 0 {
		limit = DefaultMaxBufferedBytes
	}

	if decodeErr == nil && h.in.ReadableBytes() > limit {
		decodeErr = ErrDecoderBufferLimit
	}

	if decodeErr != nil {
		h.reject(ctx, decodeErr)
		return
	}

	for elem, ok := out.TryPop(); ok; elem, ok = out.TryPop() {
		ctx.FireRead(elem)
	}

	if h.Datagram {
		h.in = buf.NewCompositeByteBuf()
		h.state = h.initialState
		return
	}

	h.Checkpoint(h.state)
}

func (h *ReplayDecoder) reject(ctx HandlerContext, err error) {
	h.in = buf.NewCompositeByteBuf()
	h.state = h.initialState
	if !h.Datagram {
		h.failed = true
		ctx.Disconnect(nil)
	}
	ctx.FireErrorCaught(err)
}
