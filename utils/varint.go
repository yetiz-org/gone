package utils

import (
	"math"

	buf "github.com/yetiz-org/goth-bytebuf"
)

// VarIntEncodeTo writes the variable-length encoding of val into dst and
// returns dst for chaining. The encoded form occupies 1, 3, 5, or 9 bytes
// depending on the magnitude of val.
func VarIntEncodeTo(dst buf.ByteBuf, val uint64) buf.ByteBuf {
	switch {
	case val < 0xfd:
		return dst.AppendByte(byte(val))
	case val <= math.MaxUint16:
		return dst.AppendByte(0xfd).WriteUInt16(uint16(val))
	case val <= math.MaxUint32:
		return dst.AppendByte(0xfe).WriteUInt32(uint32(val))
	default:
		return dst.AppendByte(0xff).WriteUInt64(val)
	}
}

// VarIntEncode returns a newly allocated ByteBuf carrying the variable-length
// encoding of val.
func VarIntEncode(val uint64) buf.ByteBuf {
	return VarIntEncodeTo(buf.EmptyByteBuf(), val)
}

// VarIntDecode reads a variable-length encoded value from bbf using flag as
// the length-class selector previously read from the stream.
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
