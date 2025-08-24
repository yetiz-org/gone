package utils

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	buf "github.com/yetiz-org/goth-bytebuf"
)

// Test VarIntEncode with comprehensive table-driven tests
func TestVarIntEncode(t *testing.T) {
	tests := []struct {
		name    string
		input   uint64
		wantErr bool
	}{
		{"small_value_0", 0, false},
		{"small_value_1", 1, false},
		{"small_value_252", 252, false},
		{"boundary_253", 253, false},
		{"uint16_max", math.MaxUint16, false},
		{"uint16_plus_1", math.MaxUint16 + 1, false},
		{"uint32_max", math.MaxUint32, false},
		{"uint32_plus_1", math.MaxUint32 + 1, false},
		{"uint64_max", math.MaxUint64, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := VarIntEncode(tt.input)

			// Verify buffer is not nil
			assert.NotNil(t, result, "TestCase: %s should return non-nil ByteBuf", tt.name)

			// Verify result has readable content
			assert.True(t, result.ReadableBytes() > 0,
				"TestCase: %s should have readable bytes", tt.name)
		})
	}
}

// Test VarIntDecode with comprehensive table-driven tests
func TestVarIntDecode(t *testing.T) {
	tests := []struct {
		name     string
		flag     byte
		data     []byte
		expected uint64
		wantErr  bool
	}{
		{"default_flag_0", 0x00, []byte{}, 0, false},
		{"default_flag_1", 0x01, []byte{}, 1, false},
		{"default_flag_252", 0xfc, []byte{}, 252, false},
		{"flag_0xfd_uint16", 0xfd, []byte{0x00, 0xfd}, 253, false},
		{"flag_0xfd_max_uint16", 0xfd, []byte{0xff, 0xff}, math.MaxUint16, false},
		{"flag_0xfe_uint32", 0xfe, []byte{0x00, 0x01, 0x00, 0x00}, math.MaxUint16 + 1, false},
		{"flag_0xfe_max_uint32", 0xfe, []byte{0xff, 0xff, 0xff, 0xff}, math.MaxUint32, false},
		{"flag_0xff_uint64", 0xff, []byte{0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00}, math.MaxUint32 + 1, false},
		{"flag_0xff_max_uint64", 0xff, []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}, math.MaxUint64, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create ByteBuf with test data
			byteBuf := buf.NewByteBuf(tt.data)

			result := VarIntDecode(tt.flag, byteBuf)

			assert.Equal(t, tt.expected, result,
				"TestCase: %s, Flag: 0x%02x, Data: %v, Got: %d, Want: %d",
				tt.name, tt.flag, tt.data, result, tt.expected)
		})
	}
}

// Test VarIntEncode and VarIntDecode round-trip compatibility
func TestVarIntRoundTrip(t *testing.T) {
	testValues := []uint64{
		0, 1, 252, 253, 254, 255,
		math.MaxUint16 - 1, math.MaxUint16, math.MaxUint16 + 1,
		math.MaxUint32 - 1, math.MaxUint32, math.MaxUint32 + 1,
		math.MaxUint64 - 1, math.MaxUint64,
		42, 1337, 65535, 16777216, 4294967296,
	}

	for _, val := range testValues {
		t.Run("round_trip", func(t *testing.T) {
			t.Parallel()

			// Encode
			encoded := VarIntEncode(val)

			// Read first byte as flag
			flag, err := encoded.ReadByte()
			assert.NoError(t, err, "Should read first byte without error")

			// Decode using remaining buffer
			decoded := VarIntDecode(flag, encoded)

			assert.Equal(t, val, decoded,
				"Round-trip failed for value %d: decoded to %d",
				val, decoded)
		})
	}
}

// Test VarIntEncode boundary conditions
func TestVarIntEncode_BoundaryConditions(t *testing.T) {
	t.Parallel()

	// Test exactly at boundaries
	boundaryTests := []struct {
		name        string
		value       uint64
		expectBytes int
	}{
		{"boundary_252", 252, 1}, // Should use single byte
		{"boundary_253", 253, 3}, // Should use 0xfd prefix
		{"boundary_uint16_max", math.MaxUint16, 3},
		{"boundary_uint16_plus_1", math.MaxUint16 + 1, 5},
		{"boundary_uint32_max", math.MaxUint32, 5},
		{"boundary_uint32_plus_1", math.MaxUint32 + 1, 9},
	}

	for _, tt := range boundaryTests {
		t.Run(tt.name, func(t *testing.T) {
			result := VarIntEncode(tt.value)

			assert.Equal(t, tt.expectBytes, result.ReadableBytes(),
				"TestCase: %s, Expected bytes: %d, Got: %d",
				tt.name, tt.expectBytes, result.ReadableBytes())
		})
	}
}

// Test VarIntDecode with all flag values including edge cases
func TestVarIntDecode_AllFlags(t *testing.T) {
	t.Parallel()

	// Test all possible default flag values (0x00 to 0xfc)
	for i := byte(0); i <= 0xfc; i++ {
		t.Run("default_flag", func(t *testing.T) {
			result := VarIntDecode(i, buf.EmptyByteBuf())
			expected := uint64(i)

			assert.Equal(t, expected, result,
				"Flag 0x%02x should decode to %d, got %d",
				i, expected, result)
		})
	}
}

// Performance benchmark for VarIntEncode
func BenchmarkVarIntEncode(b *testing.B) {
	testValues := []uint64{42, 253, math.MaxUint16, math.MaxUint32, math.MaxUint64}

	for _, val := range testValues {
		b.Run("encode", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = VarIntEncode(val)
			}
		})
	}
}

// Performance benchmark for VarIntDecode
func BenchmarkVarIntDecode(b *testing.B) {
	testCases := []struct {
		name string
		flag byte
		data []byte
	}{
		{"small", 0x42, []byte{}},
		{"uint16", 0xfd, []byte{0xff, 0xff}},
		{"uint32", 0xfe, []byte{0xff, 0xff, 0xff, 0xff}},
		{"uint64", 0xff, []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			byteBuf := buf.NewByteBuf(tc.data)
			for i := 0; i < b.N; i++ {
				_ = VarIntDecode(tc.flag, byteBuf)
			}
		})
	}
}
