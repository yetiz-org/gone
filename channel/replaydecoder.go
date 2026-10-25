package channel

import (
	"errors"
	"sync"
	"time"

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

// Account for each retained chunk's composite component and allocation overhead.
const replayInputOverhead = 256

type ReplayDecoder struct {
	ByteToMessageDecoder
	// MaxBufferedBytes bounds the input left unread after decoding; non-positive values use the default.
	MaxBufferedBytes int
	// Datagram rejects incomplete input instead of replaying across reads.
	Datagram         bool
	in               buf.CompositeByteBuf
	state            ReplayState
	initialState     ReplayState
	failed           bool
	op               sync.Mutex
	decoding         bool
	budget           *InboundBudget
	reserved         int64
	assemblyTimer    *time.Timer
	assemblyDeadline time.Time
	assemblyTimeout  time.Duration
	assemblyContext  HandlerContext
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
}

func (h *ReplayDecoder) Added(ctx HandlerContext) {
	h.op.Lock()
	defer h.op.Unlock()
	h.releaseInput()
	h.failed = false
	h.budget = nil
}

func (h *ReplayDecoder) Read(ctx HandlerContext, obj any) {
	input := obj.(buf.ByteBuf)
	inputSize := int64(input.ReadableBytes())
	h.op.Lock()
	if h.failed || h.decoding {
		h.op.Unlock()
		return
	}
	if h.Decode == nil {
		h.op.Unlock()
		kklogger.WarnJ("channel:ReplayDecoder.Read#decode!no_decoder", "no decoder")
		return
	}
	h.decoding = true
	h.op.Unlock()
	var readAt time.Time
	if !h.Datagram {
		readAt = time.Now()
		if h.budget == nil {
			ch := ctx.Channel()
			h.budget = InboundBudgetFor(ch)
			timeout := GetParamInt64Default(ch, ParamFrameAssemblyTimeout, DefaultFrameAssemblyTimeout)
			if timeout <= 0 {
				timeout = DefaultFrameAssemblyTimeout
			}
			h.assemblyTimeout = time.Duration(timeout) * time.Millisecond
		}
	}
	out := &utils.Queue{}
	var decodeErr error
	kkpanic.Catch(func() {
		h.in.AddComponent(input)
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
	h.op.Lock()
	h.decoding = false
	if h.failed {
		h.releaseInput()
	} else if decodeErr != nil {
		h.releaseInput()
		h.failed = !h.Datagram
	} else if h.Datagram {
		h.releaseInput()
	} else if !h.compactInput(inputSize) {
		h.releaseInput()
		h.failed = true
		decodeErr = ErrInboundBudget
	} else {
		// Checkpoints may describe partial input; only an emitted message ends its assembly.
		incomplete := h.in.ReadableBytes() > 0 || h.state != h.initialState
		if !incomplete || out.Size() > 0 {
			h.assemblyDeadline = time.Time{}
			h.assemblyContext = nil
			if h.assemblyTimer != nil {
				h.assemblyTimer.Stop()
			}
		}
		if incomplete && h.assemblyDeadline.IsZero() {
			h.assemblyContext = ctx
			h.assemblyDeadline = readAt.Add(h.assemblyTimeout)
			if h.assemblyTimer == nil {
				h.assemblyTimer = time.AfterFunc(time.Until(h.assemblyDeadline), h.expire)
			} else {
				h.assemblyTimer.Reset(time.Until(h.assemblyDeadline))
			}
		}
	}
	h.op.Unlock()
	if decodeErr != nil {
		if !h.Datagram {
			ctx.Disconnect(nil)
		}
		ctx.FireErrorCaught(decodeErr)
		return
	}
	for elem, ok := out.TryPop(); ok; elem, ok = out.TryPop() {
		ctx.FireRead(elem)
	}
}

func (h *ReplayDecoder) releaseInput() {
	if h.assemblyTimer != nil {
		h.assemblyTimer.Stop()
	}
	h.assemblyDeadline = time.Time{}
	h.assemblyContext = nil
	if h.Datagram || h.in == nil {
		h.in = buf.NewCompositeByteBuf()
	} else {
		h.in.Skip(h.in.ReadableBytes())
		h.in.Compact()
	}
	h.state = h.initialState
	if h.budget != nil {
		h.budget.Release(h.reserved)
	}
	h.reserved = 0
}

func (h *ReplayDecoder) expire() {
	h.op.Lock()
	if h.assemblyDeadline.IsZero() || time.Until(h.assemblyDeadline) > 0 || h.failed {
		h.op.Unlock()
		return
	}
	ctx := h.assemblyContext
	h.failed = true
	if !h.decoding {
		h.releaseInput()
	}
	h.op.Unlock()
	ctx.Disconnect(nil)
	ctx.FireErrorCaught(ErrFrameAssemblyTimeout)
}

func (h *ReplayDecoder) Inactive(ctx HandlerContext) {
	h.op.Lock()
	h.failed = true
	if !h.decoding {
		h.releaseInput()
	}
	h.op.Unlock()
	ctx.FireInactive()
}

func (h *ReplayDecoder) Removed(ctx HandlerContext) {
	h.op.Lock()
	h.failed = true
	if !h.decoding {
		h.releaseInput()
	}
	h.op.Unlock()
}

func (h *ReplayDecoder) compactInput(inputSize int64) (withinBudget bool) {
	retained := h.reserved
	if h.in.ReaderIndex() == 0 {
		if inputSize > 0 {
			retained += inputSize + replayInputOverhead
		}
	} else if tail := h.in.ReadableBytes(); tail == 0 {
		retained = 0
		h.in.Compact()
	} else {
		// Own the unread tail so it pins neither consumed chunks nor a buffer merged by Bytes().
		owned := make([]byte, tail)
		h.in.Read(owned)
		h.in.Compact()
		h.in.AddComponent(buf.NewSharedByteBuf(owned))
		retained = int64(tail) + replayInputOverhead
	}
	delta := retained - h.reserved
	if delta > 0 && !h.budget.Reserve(delta) {
		return false
	}
	if delta < 0 {
		h.budget.Release(-delta)
	}
	h.reserved = retained
	return true
}
