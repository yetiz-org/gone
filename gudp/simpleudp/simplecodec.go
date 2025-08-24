package simpleudp

import (
	"fmt"
	"reflect"

	"github.com/yetiz-org/gone/channel"
	"github.com/yetiz-org/gone/utils"
	buf "github.com/yetiz-org/goth-bytebuf"
	kklogger "github.com/yetiz-org/goth-kklogger"
	"github.com/yetiz-org/goth-util/structs"
)

// SimpleCodec handles encoding and decoding for UDP messages
type SimpleCodec struct {
	*channel.ReplayDecoder
	flag   byte
	length uint64
	out    buf.ByteBuf
}

const FLAG = channel.ReplayState(1)
const LENGTH = channel.ReplayState(2)
const BODY = channel.ReplayState(3)

// NewSimpleCodec creates a new SimpleCodec instance for UDP message handling
func NewSimpleCodec() *SimpleCodec {
	handler := &SimpleCodec{}
	handler.ReplayDecoder = channel.NewReplayDecoder(FLAG, handler.decode)
	return handler
}

// decode handles the decoding of incoming UDP messages
func (h *SimpleCodec) decode(ctx channel.HandlerContext, in buf.ByteBuf, out structs.Queue) {
	for true {
		switch h.State() {
		case FLAG:
			// Check if buffer has enough bytes to read - use Skip() to trigger proper exception handling
			if in.ReadableBytes() < 1 {
				h.Skip() // This will panic with buf.ErrInsufficientSize - should be caught by caller
			}
			h.flag = in.MustReadByte()
			if h.flag == 0 {
				continue
			}

			h.Checkpoint(LENGTH)
		case LENGTH:
			h.length = utils.VarIntDecode(h.flag, in)
			h.Checkpoint(BODY)
		case BODY:
			h.out = in.ReadByteBuf(int(h.length))
			out.Push(h.out)
			h.Checkpoint(FLAG)
		}
	}
}

// Write handles encoding and writing UDP messages
func (h *SimpleCodec) Write(ctx channel.HandlerContext, obj any, future channel.Future) {
	switch m := obj.(type) {
	case buf.ByteBuf:
		ctx.Write(utils.VarIntEncode(uint64(m.ReadableBytes())).WriteByteBuf(m), future)
	default:
		if obj == nil {
			kklogger.ErrorJ("gudp:SimpleCodec.Write#write!type_error", "obj is nil, not type of buf.ByteBuf")
		} else {
			kklogger.ErrorJ("gudp:SimpleCodec.Write#write!type_error", fmt.Sprintf("obj(%s) is not type of buf.ByteBuf", reflect.TypeOf(obj).String()))
		}
	}
}
