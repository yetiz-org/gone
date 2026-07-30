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
			New: func() any {
				return make([]byte, size)
			},
		},
		size: size,
	}
}

// Get retrieves a buffer from the pool and clears it to prevent dirty data
// Returns a clean byte slice of the pool's configured size
func (bp *BufferPool) Get() []byte {
	buf := bp.pool.Get().([]byte)
	// Clear the buffer to prevent dirty data leakage
	bp.clearBuffer(buf)
	return buf
}

// GetDirty retrieves a buffer without clearing it. Callers must treat the
// returned slice as uninitialized and only access indices they themselves
// populate (e.g. via Read). Use Get when the buffer is consumed in full.
func (bp *BufferPool) GetDirty() []byte {
	return bp.pool.Get().([]byte)
}

// Put returns a buffer to the pool for reuse
// The buffer should be of the same size as the pool's configured size
func (bp *BufferPool) Put(buf []byte) {
	// Only return buffers of the expected size to maintain pool consistency
	if cap(buf) == bp.size {
		bp.pool.Put(buf[:bp.size])
	}
}

// clearBuffer zeroes the buffer so pooled reuse cannot leak prior contents.
// The builtin clear is lowered to memclr by the compiler.
func (bp *BufferPool) clearBuffer(buf []byte) {
	clear(buf)
}

// GetWithSize retrieves a buffer and resizes it if needed
// If the requested size is larger than the pool buffer, it creates a new buffer
// This provides flexibility while still benefiting from pooling for common sizes
func (bp *BufferPool) GetWithSize(size int) []byte {
	if size > bp.size {
		return make([]byte, size)
	}

	buf := bp.Get() // This already clears the buffer
	// Return slice of the requested size
	return buf[:size]
}

// Size returns the configured buffer size for this pool
func (bp *BufferPool) Size() int {
	return bp.size
}

// Convenience functions for global pools

// GetSmallBuffer gets a clean 4KB buffer from the small buffer pool
func GetSmallBuffer() []byte {
	return SmallBufferPool.Get()
}

// PutSmallBuffer returns a 4KB buffer to the small buffer pool
func PutSmallBuffer(buf []byte) {
	SmallBufferPool.Put(buf)
}

// GetMediumBuffer gets a clean 16KB buffer from the medium buffer pool
func GetMediumBuffer() []byte {
	return MediumBufferPool.Get()
}

// PutMediumBuffer returns a 16KB buffer to the medium buffer pool
func PutMediumBuffer(buf []byte) {
	MediumBufferPool.Put(buf)
}

// GetLargeBuffer gets a clean 64KB buffer from the large buffer pool
func GetLargeBuffer() []byte {
	return LargeBufferPool.Get()
}

// PutLargeBuffer returns a 64KB buffer to the large buffer pool
func PutLargeBuffer(buf []byte) {
	LargeBufferPool.Put(buf)
}

// GetBufferForSize returns the most appropriate clean buffer for the given size
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

// GetDirtyBufferForSize returns an uncleared buffer of the requested size.
// The caller must fully overwrite indices it reads back. Sizes above the
// largest pool fall back to a plain make, which is zero-initialized.
func GetDirtyBufferForSize(size int) []byte {
	switch {
	case size <= 4*1024:
		buf := SmallBufferPool.GetDirty()
		return buf[:size]
	case size <= 16*1024:
		buf := MediumBufferPool.GetDirty()
		return buf[:size]
	case size <= 64*1024:
		buf := LargeBufferPool.GetDirty()
		return buf[:size]
	default:
		return make([]byte, size)
	}
}

// PutBufferForSize returns a buffer to the appropriate pool based on its size
func PutBufferForSize(buf []byte) {
	size := cap(buf)
	switch {
	case size == 4*1024:
		SmallBufferPool.Put(buf)
	case size == 16*1024:
		MediumBufferPool.Put(buf)
	case size == 64*1024:
		LargeBufferPool.Put(buf)
		// For other sizes, just let it be garbage collected
	}
}
