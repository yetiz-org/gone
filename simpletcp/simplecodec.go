package simpletcp

import (
	"fmt"
	"reflect"

	"github.com/yetiz-org/gone/channel"
	buf "github.com/yetiz-org/goth-bytebuf"
	kklogger "github.com/yetiz-org/goth-kklogger"
	"github.com/yetiz-org/goth-kkutil/structs"
)

type SimpleCodec struct {
	*channel.ReplayDecoder
	flag   byte
	length uint64
	out    buf.ByteBuf
}

const FLAG = channel.ReplayState(1)
const LENGTH = channel.ReplayState(2)
const BODY = channel.ReplayState(3)

func NewSimpleCodec() *SimpleCodec {
	handler := &SimpleCodec{}
	handler.ReplayDecoder = channel.NewReplayDecoder(FLAG, handler.decode)
	return handler
}

func (h *SimpleCodec) decode(ctx channel.HandlerContext, in buf.ByteBuf, out structs.Queue) {
	for true {
		switch h.State() {
		case FLAG:
			h.flag = in.ReadByte()
			if h.flag == 0 {
				continue
			}

			h.Checkpoint(LENGTH)
		case LENGTH:
			h.length = VarIntDecode(h.flag, in)
			h.Checkpoint(BODY)
		case BODY:
			h.out = in.ReadByteBuf(int(h.length))
			out.Push(h.out)
			h.Checkpoint(FLAG)
		}
	}
}

func (h *SimpleCodec) Write(ctx channel.HandlerContext, obj any, future channel.Future) {
	switch m := obj.(type) {
	case buf.ByteBuf:
		ctx.Write(VarIntEncode(uint64(m.ReadableBytes())).WriteByteBuf(m), future)
	default:
		kklogger.ErrorJ("SimpleCodec.Write", fmt.Sprintf("obj(%s) is not type of buf.ByteBuf", reflect.TypeOf(obj).String()))
	}
}
