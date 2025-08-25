package utils

import (
	"sync"
)

// BufferPool provides a thread-safe pool for byte buffers of different sizes
// This helps reduce memory allocations and GC pressure for frequently used buffers
type BufferPool struct {
	pool sync.Pool
	size int
}

// Global buffer pools for common sizes
var (
	// SmallBufferPool for small buffers (4KB) - typical for network I/O, headers, small messages
	SmallBufferPool = NewBufferPool(4 * 1024)

	// MediumBufferPool for medium buffers (16KB) - good for most application data
	MediumBufferPool = NewBufferPool(16 * 1024)

	// LargeBufferPool for large buffers (64KB) - for UDP max packet size, large transfers
	LargeBufferPool = NewBufferPool(64 * 1024)
)

// NewBufferPool creates a new buffer pool with the specified buffer size
func NewBufferPool(size int) *BufferPool {
	return &BufferPool{
		pool: sync.Pool{
			New: func() interface{} {
				return make([]byte, size)
			},
		},
		size: size,
	}
}

// Get retrieves a buffer from the pool
// Returns a byte slice of the pool's configured size
func (bp *BufferPool) Get() []byte {
	return bp.pool.Get().([]byte)
}

// Put returns a buffer to the pool for reuse
// The buffer should be of the same size as the pool's configured size
func (bp *BufferPool) Put(buf []byte) {
	// Only return buffers of the expected size to maintain pool consistency
	if len(buf) == bp.size {
		bp.pool.Put(buf)
	}
}

// GetWithSize retrieves a buffer and resizes it if needed
// If the requested size is larger than the pool buffer, it creates a new buffer
// This provides flexibility while still benefiting from pooling for common sizes
func (bp *BufferPool) GetWithSize(size int) []byte {
	buf := bp.Get()
	if len(buf) < size {
		// If requested size is larger, create a new buffer
		// Don't return the pool buffer since we can't use it
		return make([]byte, size)
	}
	// Return slice of the requested size
	return buf[:size]
}

// Size returns the configured buffer size for this pool
func (bp *BufferPool) Size() int {
	return bp.size
}

// Convenience functions for global pools

// GetSmallBuffer gets a 4KB buffer from the small buffer pool
func GetSmallBuffer() []byte {
	return SmallBufferPool.Get()
}

// PutSmallBuffer returns a 4KB buffer to the small buffer pool
func PutSmallBuffer(buf []byte) {
	SmallBufferPool.Put(buf)
}

// GetMediumBuffer gets a 16KB buffer from the medium buffer pool
func GetMediumBuffer() []byte {
	return MediumBufferPool.Get()
}

// PutMediumBuffer returns a 16KB buffer to the medium buffer pool
func PutMediumBuffer(buf []byte) {
	MediumBufferPool.Put(buf)
}

// GetLargeBuffer gets a 64KB buffer from the large buffer pool
func GetLargeBuffer() []byte {
	return LargeBufferPool.Get()
}

// PutLargeBuffer returns a 64KB buffer to the large buffer pool
func PutLargeBuffer(buf []byte) {
	LargeBufferPool.Put(buf)
}

// GetBufferForSize returns the most appropriate buffer for the given size
// This helps choose the right pool automatically based on size requirements
func GetBufferForSize(size int) []byte {
	switch {
	case size <= 4*1024:
		return SmallBufferPool.GetWithSize(size)
	case size <= 16*1024:
		return MediumBufferPool.GetWithSize(size)
	case size <= 64*1024:
		return LargeBufferPool.GetWithSize(size)
	default:
		// For very large sizes, just allocate directly
		return make([]byte, size)
	}
}

// PutBufferForSize returns a buffer to the appropriate pool based on its size
func PutBufferForSize(buf []byte) {
	size := len(buf)
	switch size {
	case 4 * 1024:
		SmallBufferPool.Put(buf)
	case 16 * 1024:
		MediumBufferPool.Put(buf)
	case 64 * 1024:
		LargeBufferPool.Put(buf)
		// For other sizes, just let it be garbage collected
	}
}
