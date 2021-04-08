package channel

import (
	"bytes"
	"fmt"
	"sync"

	kklogger "github.com/kklab-com/goth-kklogger"
	kkpanic "github.com/kklab-com/goth-panic"
)

type ReplayState int

type ReplayDecoder struct {
	ByteToMessageDecoder
	in    bytes.Buffer
	state ReplayState
	op    sync.Mutex
}

var replayDecoderSkip = fmt.Errorf("skip")
var replayDecoderTruncateLen = 1 << 20

func NewReplayDecoder(state ReplayState) *ReplayDecoder {
	return &ReplayDecoder{state: state}
}

func (h *ReplayDecoder) Skip() {
	panic(replayDecoderSkip)
}

func (h *ReplayDecoder) State() ReplayState {
	return h.state
}

func (h *ReplayDecoder) Checkpoint(state ReplayState) {
	h.state = state
	if h.in.Cap()-h.in.Len() > replayDecoderTruncateLen {
		h.op.Lock()
		defer h.op.Unlock()
		bs := h.in.Bytes()
		h.in.Reset()
		h.in.Write(bs)
	}
}

func (h *ReplayDecoder) Read(ctx HandlerContext, obj interface{}) {
	h.in.Write(obj.(*bytes.Buffer).Bytes())
	if h.Decoder != nil {
		kkpanic.Catch(func() {
			h.Decoder.Decode(ctx, &h.in, &h.outList)
		}, func(r *kkpanic.Caught) {
			if r.Message != replayDecoderSkip {
				kklogger.ErrorJ("ReplayDecoder.Read#Decode", r.String())
				return
			}
		})
	} else {
		kklogger.WarnJ("ReplayDecoder.Read#Decode", "no decoder")
	}

	for {
		if elem := h.outList.Back(); elem != nil {
			ctx.FireRead(elem.Value)
			h.outList.Remove(elem)
			continue
		}

		break
	}
}
