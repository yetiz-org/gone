package simpletcp

import (
	"math"

	buf "github.com/kklab-com/goth-bytebuf"
)

func VarIntEncode(val uint64) buf.ByteBuf {
	if val < 0xfd {
		return buf.EmptyByteBuf().WriteByte(byte(val))
	} else if val <= math.MaxUint16 {
		return buf.NewByteBuf([]byte{0xfd}).WriteUInt16(uint16(val))
	} else if val <= math.MaxUint32 {
		return buf.NewByteBuf([]byte{0xfe}).WriteUInt32(uint32(val))
	} else {
		return buf.NewByteBuf([]byte{0xff}).WriteUInt64(val)
	}
}

func VarIntDecode(flag byte, bbf buf.ByteBuf) uint64 {
	switch flag {
	case 0xfd:
		return uint64(bbf.ReadUInt16())
	case 0xfe:
		return uint64(bbf.ReadUInt32())
	case 0xff:
		return bbf.ReadUInt64()
	default:
		return uint64(flag)
	}
}
