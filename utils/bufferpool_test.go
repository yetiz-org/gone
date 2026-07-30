package utils

import (
	"sync"
	"sync/atomic"
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

func TestBufferPool_GetWithSize_OversizedDoesNotTouchPool(t *testing.T) {
	var created atomic.Int64
	pool := &BufferPool{
		pool: sync.Pool{
			New: func() any {
				created.Add(1)
				return make([]byte, 1024)
			},
		},
		size: 1024,
	}

	buf := pool.GetWithSize(2048)
	if len(buf) != 2048 {
		t.Fatalf("expected oversized buffer length 2048, got %d", len(buf))
	}
	if created.Load() != 0 {
		t.Fatalf("oversized request should not take a buffer from the pool, got %d pool allocations", created.Load())
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

func TestPutBufferForSize_RecyclesBucketByCapacity(t *testing.T) {
	buf := GetDirtyBufferForSize(1024)
	if len(buf) != 1024 {
		t.Fatalf("expected requested length 1024, got %d", len(buf))
	}
	if cap(buf) != 4*1024 {
		t.Fatalf("expected small bucket capacity 4KB, got %d", cap(buf))
	}

	buf[0] = 42
	PutBufferForSize(buf)

	reused := GetDirtyBufferForSize(1024)
	if cap(reused) != 4*1024 {
		t.Fatalf("expected recycled small bucket capacity 4KB, got %d", cap(reused))
	}
	PutBufferForSize(reused)
}

func TestGetDirtyBufferForSize_BucketBoundaries(t *testing.T) {
	tests := []struct {
		size    int
		wantCap int
	}{
		{0, 4 * 1024},
		{1, 4 * 1024},
		{4*1024 - 1, 4 * 1024},
		{4 * 1024, 4 * 1024},
		{4*1024 + 1, 16 * 1024},
		{16 * 1024, 16 * 1024},
		{16*1024 + 1, 64 * 1024},
		{64 * 1024, 64 * 1024},
		{64*1024 + 1, 64*1024 + 1},
	}

	for _, test := range tests {
		buf := GetDirtyBufferForSize(test.size)
		if len(buf) != test.size {
			t.Fatalf("size %d: expected len %d, got %d", test.size, test.size, len(buf))
		}
		if cap(buf) != test.wantCap {
			t.Fatalf("size %d: expected cap %d, got %d", test.size, test.wantCap, cap(buf))
		}
		PutBufferForSize(buf)
	}
}

func TestPutBufferForSize_ConcurrentSlicedBuffers(t *testing.T) {
	const goroutines = 32
	const iterations = 1000

	var wg sync.WaitGroup
	errCh := make(chan string, goroutines)
	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range iterations {
				buf := GetDirtyBufferForSize(1024)
				if len(buf) != 1024 || cap(buf) != 4*1024 {
					errCh <- "expected len=1024 cap=4096"
					return
				}
				PutBufferForSize(buf[:512])
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatal(err)
	}
}

func BenchmarkBufferPool_GetPut(b *testing.B) {
	pool := NewBufferPool(4096)

	b.ResetTimer()
	for b.Loop() {
		buf := pool.Get()
		pool.Put(buf)
	}
}

func BenchmarkBufferPool_DirectAllocation(b *testing.B) {
	b.ResetTimer()
	for b.Loop() {
		_ = make([]byte, 4096)
	}
}

func BenchmarkGlobalPools_Small(b *testing.B) {
	b.ResetTimer()
	for b.Loop() {
		buf := GetSmallBuffer()
		PutSmallBuffer(buf)
	}
}

func BenchmarkGlobalPools_Large(b *testing.B) {
	b.ResetTimer()
	for b.Loop() {
		buf := GetLargeBuffer()
		PutLargeBuffer(buf)
	}
}

func BenchmarkGetDirtyBufferForSize_1024_GetPut(b *testing.B) {
	b.ResetTimer()
	for b.Loop() {
		buf := GetDirtyBufferForSize(1024)
		PutBufferForSize(buf)
	}
}
