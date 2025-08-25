package utils

import (
	"testing"
)

func TestBufferPool_BasicOperations(t *testing.T) {
	pool := NewBufferPool(1024)

	// Test Get
	buf := pool.Get()
	if len(buf) != 1024 {
		t.Errorf("Expected buffer size 1024, got %d", len(buf))
	}

	// Test Put
	pool.Put(buf)

	// Test Get again - should reuse the buffer
	buf2 := pool.Get()
	if len(buf2) != 1024 {
		t.Errorf("Expected buffer size 1024, got %d", len(buf2))
	}
}

func TestBufferPool_GetWithSize(t *testing.T) {
	pool := NewBufferPool(1024)

	// Test getting smaller size
	buf := pool.GetWithSize(512)
	if len(buf) != 512 {
		t.Errorf("Expected buffer size 512, got %d", len(buf))
	}

	// Test getting larger size
	buf2 := pool.GetWithSize(2048)
	if len(buf2) != 2048 {
		t.Errorf("Expected buffer size 2048, got %d", len(buf2))
	}
}

func TestGlobalBufferPools(t *testing.T) {
	// Test small buffer pool
	smallBuf := GetSmallBuffer()
	if len(smallBuf) != 4*1024 {
		t.Errorf("Expected small buffer size 4KB, got %d", len(smallBuf))
	}
	PutSmallBuffer(smallBuf)

	// Test medium buffer pool
	mediumBuf := GetMediumBuffer()
	if len(mediumBuf) != 16*1024 {
		t.Errorf("Expected medium buffer size 16KB, got %d", len(mediumBuf))
	}
	PutMediumBuffer(mediumBuf)

	// Test large buffer pool
	largeBuf := GetLargeBuffer()
	if len(largeBuf) != 64*1024 {
		t.Errorf("Expected large buffer size 64KB, got %d", len(largeBuf))
	}
	PutLargeBuffer(largeBuf)
}

func TestGetBufferForSize(t *testing.T) {
	tests := []struct {
		size     int
		expected int
	}{
		{1024, 1024},             // Should use small pool
		{8 * 1024, 8 * 1024},     // Should use medium pool
		{32 * 1024, 32 * 1024},   // Should use large pool
		{128 * 1024, 128 * 1024}, // Should allocate directly
	}

	for _, test := range tests {
		buf := GetBufferForSize(test.size)
		if len(buf) != test.expected {
			t.Errorf("For size %d, expected %d, got %d", test.size, test.expected, len(buf))
		}
	}
}

func TestPutBufferForSize(t *testing.T) {
	// Test putting buffers of different sizes
	buf4k := make([]byte, 4*1024)
	buf16k := make([]byte, 16*1024)
	buf64k := make([]byte, 64*1024)
	buf128k := make([]byte, 128*1024)

	// These should not panic
	PutBufferForSize(buf4k)
	PutBufferForSize(buf16k)
	PutBufferForSize(buf64k)
	PutBufferForSize(buf128k) // Should be ignored
}

func BenchmarkBufferPool_GetPut(b *testing.B) {
	pool := NewBufferPool(4096)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := pool.Get()
		pool.Put(buf)
	}
}

func BenchmarkBufferPool_DirectAllocation(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = make([]byte, 4096)
	}
}

func BenchmarkGlobalPools_Small(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := GetSmallBuffer()
		PutSmallBuffer(buf)
	}
}

func BenchmarkGlobalPools_Large(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := GetLargeBuffer()
		PutLargeBuffer(buf)
	}
}
