package channel

import (
	"bytes"
	"container/list"
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
	out := &list.List{}
	if h.Decoder != nil {
		kkpanic.Catch(func() {
			h.Decoder.Decode(ctx, &h.in, out)
		}, func(r *kkpanic.Caught) {
			if r.Message != replayDecoderSkip {
				kklogger.ErrorJ("ReplayDecoder.Read#Decode", r.String())
				return
			}
		})
	} else {
		kklogger.WarnJ("ReplayDecoder.Read#Decode", "no decoder")
	}

	for elem := out.Back(); elem != nil; func() {
		out.Remove(elem)
		elem = out.Back()
	}() {
		ctx.FireRead(elem.Value)
	}
}
