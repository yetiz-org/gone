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

const FLAG = channel.ReplayState(1)
const LENGTH = channel.ReplayState(2)
const BODY = channel.ReplayState(3)

// MaxFrameLength leaves space for a three-byte prefix in a 65,507-byte datagram.
const MaxFrameLength = 65504

// SimpleCodec handles encoding and decoding for UDP messages
type SimpleCodec struct {
	*channel.ReplayDecoder
	flag   byte
	length uint64
	out    buf.ByteBuf
}

// NewSimpleCodec creates a new SimpleCodec instance for UDP message handling
func NewSimpleCodec() (codec *SimpleCodec) {
	handler := &SimpleCodec{}
	handler.ReplayDecoder = channel.NewReplayDecoder(FLAG, handler.decode)
	handler.Datagram = true
	return handler
}

// decode handles the decoding of incoming UDP messages
func (h *SimpleCodec) decode(ctx channel.HandlerContext, in buf.ByteBuf, out structs.Queue) {
	for in.ReadableBytes() > 0 || h.State() != FLAG {
		switch h.State() {
		case FLAG:
			h.flag = in.MustReadByte()
			if h.flag == 0 {
				continue
			}

			h.Checkpoint(LENGTH)
		case LENGTH:
			h.length = utils.VarIntDecode(h.flag, in)
			if h.length > MaxFrameLength {
				panic(channel.ErrFrameTooLarge)
			}
			h.Checkpoint(BODY)
		case BODY:
			h.out = in.ReadByteBuf(int(h.length))
			out.Push(h.out)
			h.out = nil
			h.Checkpoint(FLAG)
		}
	}
}

// Write handles encoding and writing UDP messages
func (h *SimpleCodec) Write(ctx channel.HandlerContext, obj any, future channel.Future) {
	switch m := obj.(type) {
	case buf.ByteBuf:
		if m.ReadableBytes() > MaxFrameLength {
			if future != nil {
				future.Completable().Fail(channel.ErrFrameTooLarge)
			} else {
				kklogger.ErrorJ("gudp:SimpleCodec.Write#write!frame_too_large", fmt.Sprintf("drop %d-byte payload over MaxFrameLength", m.ReadableBytes()))
			}

			return
		}
		header := buf.EmptyByteBuf()
		utils.VarIntEncodeTo(header, uint64(m.ReadableBytes()))
		ctx.Write(buf.NewCompositeByteBuf(header, m), future)
	default:
		if obj == nil {
			kklogger.ErrorJ("gudp:SimpleCodec.Write#write!type_error", "obj is nil, not type of buf.ByteBuf")
		} else {
			kklogger.ErrorJ("gudp:SimpleCodec.Write#write!type_error", fmt.Sprintf("obj(%s) is not type of buf.ByteBuf", reflect.TypeOf(obj).String()))
		}
		if future != nil {
			future.Completable().Fail(channel.ErrUnknownObjectType)
		}
		ctx.FireErrorCaught(channel.ErrUnknownObjectType)
	}
}
