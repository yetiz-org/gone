package channel

import (
	"errors"
	"sync/atomic"
)

var ErrInboundBudget = errors.New("shared incomplete input budget exceeded")
var ErrFrameAssemblyTimeout = errors.New("frame assembly timeout")

const inboundBudgetParam = ParamKey("[gone]inbound_budget")

// InboundBudget accounts for input retained while a frame, message or request is incomplete.
type InboundBudget struct {
	limit int64
	used  atomic.Int64
}

// InboundBudgetFor returns the accepted connection's server-owned budget, or a standalone channel's budget.
func InboundBudgetFor(ch Channel) (budget *InboundBudget) {
	owner := ch
	if parent := ch.Parent(); parent != nil {
		owner = parent
	}
	if value, ok := owner.Params().Load(inboundBudgetParam); ok {
		return value.(*InboundBudget)
	}
	limit := GetParamInt64Default(owner, ParamMaxInProgressBytes, DefaultMaxInProgressBytes)
	if limit <= 0 {
		limit = DefaultMaxInProgressBytes
	}
	value, _ := owner.Params().LoadOrStore(inboundBudgetParam, &InboundBudget{limit: limit})
	return value.(*InboundBudget)
}

func (b *InboundBudget) Reserve(size int64) (reserved bool) {
	if size == 0 {
		return true
	}
	if size < 0 {
		return false
	}
	for {
		used := b.used.Load()
		if size > b.limit-used {
			return false
		}
		if b.used.CompareAndSwap(used, used+size) {
			return true
		}
	}
}

func (b *InboundBudget) Release(size int64) {
	if size == 0 {
		return
	}
	for {
		used := b.used.Load()
		if size < 0 || size > used {
			panic("incomplete input budget underflow")
		}
		if b.used.CompareAndSwap(used, used-size) {
			return
		}
	}
}
