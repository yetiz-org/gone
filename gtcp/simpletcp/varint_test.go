package simpletcp

import (
	"math"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVarInt(t *testing.T) {
	maxArr := []int64{math.MaxInt8, math.MaxInt16, math.MaxInt32, math.MaxInt64}
	for _, max := range maxArr {
		for i := 0; i < 256; i++ {
			v := uint64(rand.Intn(int(max)))
			varint := VarIntEncode(v)
			assert.Equal(t, v, VarIntDecode(varint.ReadByte(), varint))
		}
	}
}
