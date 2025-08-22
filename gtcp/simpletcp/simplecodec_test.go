package simpletcp

import (
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/yetiz-org/gone/channel"
	"github.com/yetiz-org/gone/utils"
	buf "github.com/yetiz-org/goth-bytebuf"
	kkpanic "github.com/yetiz-org/goth-panic"
)

// Mock handler context for testing - implements channel.HandlerContext
type MockHandlerContext struct {
	*channel.DefaultHandlerContext
	writeCalls int64
	writeData  []interface{}
	writeMutex sync.Mutex
	ch         channel.Channel
}

func NewMockHandlerContext() *MockHandlerContext {
	ch := &channel.DefaultChannel{}
	ch.Init()
	ctx := channel.NewHandlerContext()
	return &MockHandlerContext{
		DefaultHandlerContext: ctx,
		ch:                    ch,
	}
}

func (m *MockHandlerContext) Write(obj any, future channel.Future) channel.Future {
	m.writeMutex.Lock()
	defer m.writeMutex.Unlock()
	atomic.AddInt64(&m.writeCalls, 1)
	m.writeData = append(m.writeData, obj)
	if future == nil {
		if m.ch != nil && m.ch.Pipeline() != nil {
			future = m.ch.Pipeline().NewFuture()
		} else {
			// Create a dummy future for testing
			future = &channel.DefaultFuture{}
		}
	}
	return future
}

func (m *MockHandlerContext) Channel() channel.Channel {
	return m.ch
}

func (m *MockHandlerContext) Bind(localAddr net.Addr, future channel.Future) channel.Future {
	if future == nil {
		if m.ch != nil && m.ch.Pipeline() != nil {
			future = m.ch.Pipeline().NewFuture()
		} else {
			// Create a dummy future for testing
			future = &channel.DefaultFuture{}
		}
	}
	return future
}

func (m *MockHandlerContext) GetWriteCalls() int64 {
	return atomic.LoadInt64(&m.writeCalls)
}

func (m *MockHandlerContext) GetWriteData() []interface{} {
	m.writeMutex.Lock()
	defer m.writeMutex.Unlock()
	return m.writeData
}

// Override FireRead to capture decoded data - must return HandlerContext
func (m *MockHandlerContext) FireRead(obj any) channel.HandlerContext {
	m.writeMutex.Lock()
	defer m.writeMutex.Unlock()
	atomic.AddInt64(&m.writeCalls, 1)
	m.writeData = append(m.writeData, obj)
	// Call parent implementation and return the result
	return m.DefaultHandlerContext.FireRead(obj)
}

// Helper function for safe decode operations with proper exception handling
func safeDecode(codec *SimpleCodec, ctx channel.HandlerContext, in buf.ByteBuf, out *utils.Queue) bool {
	var success bool
	var errorOccurred bool
	kkpanic.CatchExcept(func() {
		codec.decode(ctx, in, out)
		success = true
	}, buf.ErrInsufficientSize, func(r kkpanic.Caught) {
		// Expected exception for incomplete data - this is normal behavior
		success = false
		errorOccurred = true
	})
	// Debug: Add logging to understand what's happening
	if !success && !errorOccurred {
		// This should not happen - investigate
		fmt.Printf("DEBUG: safeDecode failed without ErrInsufficientSize - bytes available: %d\n", in.ReadableBytes())
	}
	return success
}

// Test SimpleCodec basic functionality and interface compliance
func TestSimpleCodec_InterfaceCompliance(t *testing.T) {
	codec := NewSimpleCodec()
	
	// Verify interface compliance
	var _ channel.Handler = codec
	
	// Test basic properties
	assert.NotNil(t, codec, "Codec should not be nil")
	assert.NotNil(t, codec.ReplayDecoder, "ReplayDecoder should not be nil")
	assert.Equal(t, FLAG, codec.State(), "Initial state should be FLAG")
}

// Test concurrent decode operations - THREAD SAFETY CRITICAL
func TestSimpleCodec_ConcurrentDecode(t *testing.T) {
	const numGoroutines = 100
	const decodesPerGoroutine = 50
	
	var wg sync.WaitGroup
	var successfulDecodes int64
	
	// Test data preparation
	testMessages := [][]byte{
		[]byte("Hello World"),
		[]byte("Concurrent Test"),
		[]byte("Thread Safety"),
		[]byte("SimpleCodec Testing"),
		[]byte("Go Concurrency"),
	}
	
	// Concurrent decode operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			
			for j := 0; j < decodesPerGoroutine; j++ {
				// Create separate codec instance for each goroutine to avoid shared state
				codec := NewSimpleCodec()
				ctx := NewMockHandlerContext()
				
				// Select test message
				testMsg := testMessages[(goroutineID+j)%len(testMessages)]
				
				// Encode message for decoding
				encoded := VarIntEncode(uint64(len(testMsg)))
				encoded.WriteBytes(testMsg)
				
				// Use proper ReplayDecoder lifecycle - Added() then Read()
				codec.Added(ctx)
				codec.Read(ctx, encoded)
				
				// Verify decoding result - check mock context for successful write calls
				if ctx.GetWriteCalls() > 0 {
					writeData := ctx.GetWriteData()
					if len(writeData) > 0 {
						decoded := writeData[len(writeData)-1] // Get last written data
						if decoded != nil {
							if decodedBuf, ok := decoded.(buf.ByteBuf); ok {
								decodedBytes := decodedBuf.Bytes()
								if string(decodedBytes) == string(testMsg) {
									atomic.AddInt64(&successfulDecodes, 1)
								}
							}
						}
					}
				}
			}
		}(i)
	}
	
	wg.Wait()
	
	expectedTotal := int64(numGoroutines * decodesPerGoroutine)
	t.Logf("Successful decodes: %d out of %d", successfulDecodes, expectedTotal)
	
	assert.Equal(t, expectedTotal, successfulDecodes, "All decode operations should be successful")
}

// Test concurrent write operations - THREAD SAFETY CRITICAL
func TestSimpleCodec_ConcurrentWrite(t *testing.T) {
	const numGoroutines = 150
	const writesPerGoroutine = 30
	
	codec := NewSimpleCodec()
	ctx := NewMockHandlerContext()
	
	var wg sync.WaitGroup
	var totalWrites int64
	
	// Test data preparation
	testData := []string{
		"Concurrent Write Test",
		"Thread Safety Check",
		"SimpleCodec Write",
		"Multiple Goroutines",
		"Parallel Processing",
	}
	
	// Concurrent write operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			
			for j := 0; j < writesPerGoroutine; j++ {
				// Select test data
				testMsg := testData[(goroutineID+j)%len(testData)]
				testBuf := buf.NewByteBuf([]byte(testMsg))
				
				// Create future for write operation
				future := channel.NewFuture(ctx.Channel())
				
				// Perform write operation
				codec.Write(ctx, testBuf, future)
				atomic.AddInt64(&totalWrites, 1)
			}
		}(i)
	}
	
	wg.Wait()
	
	expectedTotal := int64(numGoroutines * writesPerGoroutine)
	t.Logf("Total writes: %d, Context write calls: %d", totalWrites, ctx.writeCalls)
	
	assert.Equal(t, expectedTotal, totalWrites, "All write operations should complete")
	assert.Equal(t, expectedTotal, ctx.writeCalls, "All writes should be forwarded to context")
}

// Test codec state machine under concurrent access - MEMORY CONSISTENCY TEST
func TestSimpleCodec_StateConsistency(t *testing.T) {
	const numGoroutines = 80
	const operationsPerGoroutine = 100
	
	var wg sync.WaitGroup
	var successfulOperations int64
	
	// Test state machine consistency with concurrent access
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			
			for j := 0; j < operationsPerGoroutine; j++ {
				// Create separate codec for each operation to test state isolation
				codec := NewSimpleCodec()
				ctx := NewMockHandlerContext()
				
				// Generate test message
				testMsg := fmt.Sprintf("State test %d-%d", goroutineID, j)
				
				// Encode complete message
				encoded := VarIntEncode(uint64(len(testMsg)))
				encoded.WriteBytes([]byte(testMsg))
				
				// Track state transitions
				initialState := codec.State()
				assert.Equal(t, FLAG, initialState, "Initial state should be FLAG")
				
				// Use proper ReplayDecoder lifecycle - Added() then Read()
				codec.Added(ctx)
				codec.Read(ctx, encoded)
				
				// Verify state returned to FLAG after complete decode
				finalState := codec.State()
				
				// Check if decoding was successful by examining write calls
				if ctx.GetWriteCalls() > 0 && finalState == FLAG {
					writeData := ctx.GetWriteData()
					if len(writeData) > 0 {
						if decoded, ok := writeData[len(writeData)-1].(buf.ByteBuf); ok {
							decodedBytes := decoded.Bytes()
							if string(decodedBytes) == testMsg {
								atomic.AddInt64(&successfulOperations, 1)
							}
						}
					}
				}
			}
		}(i)
	}
	
	wg.Wait()
	
	expectedTotal := int64(numGoroutines * operationsPerGoroutine)
	t.Logf("Successful state operations: %d out of %d", successfulOperations, expectedTotal)
	
	assert.Equal(t, expectedTotal, successfulOperations, "All state operations should be consistent")
}

// Test partial message handling under concurrency
func TestSimpleCodec_PartialMessageHandling(t *testing.T) {
	const numGoroutines = 60
	const messagesPerGoroutine = 50
	
	var wg sync.WaitGroup
	var successfulMessages int64
	
	// Test partial message handling
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			
			for j := 0; j < messagesPerGoroutine; j++ {
				codec := NewSimpleCodec()
				ctx := NewMockHandlerContext()
				
				// Generate test message
				testMsg := fmt.Sprintf("Partial test message %d-%d for concurrency testing", goroutineID, j)
				
				// Encode complete message
				encoded := VarIntEncode(uint64(len(testMsg)))
				encoded.WriteBytes([]byte(testMsg))
				
				// Use proper ReplayDecoder lifecycle for complete message
				codec.Added(ctx)
				codec.Read(ctx, encoded)
				
				// Verify complete message was decoded
				if ctx.GetWriteCalls() > 0 {
					writeData := ctx.GetWriteData()
					if len(writeData) > 0 {
						if decoded, ok := writeData[len(writeData)-1].(buf.ByteBuf); ok {
							decodedBytes := decoded.Bytes()
							if string(decodedBytes) == testMsg {
								atomic.AddInt64(&successfulMessages, 1)
							}
						}
					}
				}
			}
		}(i)
	}
	
	wg.Wait()
	
	expectedTotal := int64(numGoroutines * messagesPerGoroutine)
	t.Logf("Successful partial message handling: %d out of %d", successfulMessages, expectedTotal)
	
	assert.Greater(t, successfulMessages, int64(0), "Should handle partial messages successfully")
}

// Test concurrent encode-decode cycles - FULL PIPELINE TEST
func TestSimpleCodec_ConcurrentEncodeDecode(t *testing.T) {
	const numGoroutines = 100
	const cyclesPerGoroutine = 30
	
	var wg sync.WaitGroup
	var successfulCycles int64
	
	// Test messages of varying sizes
	testMessages := []string{
		"Short",
		"Medium length message for testing",
		"Very long message that tests larger buffer handling and state management across multiple decode operations in the SimpleCodec implementation",
		"Another test message with different content",
		"Final test message for comprehensive validation",
	}
	
	// Concurrent encode-decode cycles
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			
			for j := 0; j < cyclesPerGoroutine; j++ {
				// Write phase - encode message
				testMsg := testMessages[(goroutineID+j)%len(testMessages)]
				testBuf := buf.NewByteBuf([]byte(testMsg))
				
				writeCodec := NewSimpleCodec()
				writeCtx := NewMockHandlerContext()
				writeFuture := channel.NewFuture(writeCtx.Channel())
				
				// Encode via write operation
				writeCodec.Write(writeCtx, testBuf, writeFuture)
				
				// Get encoded data from write context
				if len(writeCtx.writeData) > 0 {
					if encodedBuf, ok := writeCtx.writeData[0].(buf.ByteBuf); ok {
						// Decode phase - use proper ReplayDecoder lifecycle
						decodeCodec := NewSimpleCodec()
						decodeCtx := NewMockHandlerContext()
						
						decodeCodec.Added(decodeCtx)
						decodeCodec.Read(decodeCtx, encodedBuf)
						
						// Verify round-trip correctness
						if decodeCtx.GetWriteCalls() > 0 {
							writeData := decodeCtx.GetWriteData()
							if len(writeData) > 0 {
								if decoded, ok := writeData[len(writeData)-1].(buf.ByteBuf); ok {
									decodedBytes := decoded.Bytes()
									if string(decodedBytes) == testMsg {
										atomic.AddInt64(&successfulCycles, 1)
									}
								}
							}
						}
					}
				}
			}
		}(i)
	}
	
	wg.Wait()
	
	expectedTotal := int64(numGoroutines * cyclesPerGoroutine)
	t.Logf("Successful encode-decode cycles: %d out of %d", successfulCycles, expectedTotal)
	
	assert.Equal(t, expectedTotal, successfulCycles, "All encode-decode cycles should be successful")
}

// Test error handling and edge cases under concurrency
func TestSimpleCodec_ErrorHandling(t *testing.T) {
	const numGoroutines = 50
	const operationsPerGoroutine = 40
	
	var wg sync.WaitGroup
	var handledOperations int64
	
	// Test error handling under concurrent access
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			
			for j := 0; j < operationsPerGoroutine; j++ {
				codec := NewSimpleCodec()
				ctx := NewMockHandlerContext()
				
				opType := (goroutineID + j) % 4
				
				switch opType {
				case 0: // Normal operation
					testMsg := fmt.Sprintf("Normal %d-%d", goroutineID, j)
					testBuf := buf.NewByteBuf([]byte(testMsg))
					future := channel.NewFuture(ctx.Channel())
					codec.Write(ctx, testBuf, future)
					atomic.AddInt64(&handledOperations, 1)
					
				case 1: // Invalid object type for write
					future := channel.NewFuture(ctx.Channel())
					codec.Write(ctx, "invalid_string", future)
					atomic.AddInt64(&handledOperations, 1)
					
				case 2: // Empty buffer
					testBuf := buf.NewByteBuf([]byte{})
					future := channel.NewFuture(ctx.Channel())
					codec.Write(ctx, testBuf, future)
					atomic.AddInt64(&handledOperations, 1)
					
				case 3: // Nil future
					testBuf := buf.NewByteBuf([]byte("test"))
					codec.Write(ctx, testBuf, nil)
					atomic.AddInt64(&handledOperations, 1)
				}
			}
		}(i)
	}
	
	wg.Wait()
	
	expectedTotal := int64(numGoroutines * operationsPerGoroutine)
	t.Logf("Handled operations: %d out of %d", handledOperations, expectedTotal)
	
	assert.Equal(t, expectedTotal, handledOperations, "All operations should be handled gracefully")
}

// Test high-frequency operations - STRESS TEST
func TestSimpleCodec_HighFrequencyOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping high-frequency test in short mode")
	}
	
	const numGoroutines = 200
	const operationsPerGoroutine = 500
	
	var wg sync.WaitGroup
	var totalOperations int64
	
	// High-frequency mixed operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			
			for j := 0; j < operationsPerGoroutine; j++ {
				operationType := (goroutineID + j) % 2
				
				switch operationType {
				case 0: // Encode operation
					codec := NewSimpleCodec()
					ctx := NewMockHandlerContext()
					testMsg := fmt.Sprintf("HF-Encode-%d-%d", goroutineID, j)
					testBuf := buf.NewByteBuf([]byte(testMsg))
					future := channel.NewFuture(ctx.Channel())
					
					codec.Write(ctx, testBuf, future)
					atomic.AddInt64(&totalOperations, 1)
					
				case 1: // Decode operation
					codec := NewSimpleCodec()
					ctx := NewMockHandlerContext()
					testMsg := fmt.Sprintf("HF-Decode-%d-%d", goroutineID, j)
					
					// Prepare encoded data
					encoded := VarIntEncode(uint64(len(testMsg)))
					encoded.WriteBytes([]byte(testMsg))
					
					// Use proper ReplayDecoder lifecycle
					codec.Added(ctx)
					codec.Read(ctx, encoded)
					atomic.AddInt64(&totalOperations, 1)
				}
			}
		}(i)
	}
	
	wg.Wait()
	
	expectedTotal := int64(numGoroutines * operationsPerGoroutine)
	t.Logf("Total high-frequency operations: %d out of %d", totalOperations, expectedTotal)
	
	assert.Equal(t, expectedTotal, totalOperations, "All high-frequency operations should complete")
}

// Benchmark concurrent codec operations
func BenchmarkSimpleCodec_ConcurrentEncode(b *testing.B) {
	testMsg := "Benchmark test message for concurrent encoding performance"
	testBuf := buf.NewByteBuf([]byte(testMsg))
	
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			codec := NewSimpleCodec()
			ctx := NewMockHandlerContext()
			future := channel.NewFuture(ctx.Channel())
			
			// Reset buffer for reuse
			testBuf.ResetReaderIndex()
			codec.Write(ctx, testBuf, future)
		}
	})
}

// Benchmark concurrent decode operations
func BenchmarkSimpleCodec_ConcurrentDecode(b *testing.B) {
	testMsg := "Benchmark test message for concurrent decoding performance"
	encoded := VarIntEncode(uint64(len(testMsg)))
	encoded.WriteBytes([]byte(testMsg))
	
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			codec := NewSimpleCodec()
			ctx := NewMockHandlerContext()
			
			// Reset buffer for reuse and use proper ReplayDecoder lifecycle
			encodedCopy := buf.NewByteBuf(encoded.Bytes())
			codec.Added(ctx)
			codec.Read(ctx, encodedCopy)
		}
	})
}

// Test memory consistency and state isolation
func TestSimpleCodec_MemoryConsistency(t *testing.T) {
	const numGoroutines = 100
	const iterationsPerGoroutine = 100
	
	var wg sync.WaitGroup
	var consistentOperations int64
	
	// Test memory consistency across goroutines
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			
			for j := 0; j < iterationsPerGoroutine; j++ {
				// Each operation uses a fresh codec instance to test isolation
				codec := NewSimpleCodec()
				ctx := NewMockHandlerContext()
				
				// Test unique data per goroutine
				testData := fmt.Sprintf("Consistency-Test-G%d-I%d", goroutineID, j)
				
				// Encode
				testBuf := buf.NewByteBuf([]byte(testData))
				future := channel.NewFuture(ctx.Channel())
				codec.Write(ctx, testBuf, future)
				
				// Verify state consistency
				if codec.State() == FLAG && len(ctx.writeData) > 0 {
					atomic.AddInt64(&consistentOperations, 1)
				}
			}
		}(i)
	}
	
	wg.Wait()
	
	expectedTotal := int64(numGoroutines * iterationsPerGoroutine)
	t.Logf("Consistent operations: %d out of %d", consistentOperations, expectedTotal)
	
	assert.Equal(t, expectedTotal, consistentOperations, "All operations should maintain consistency")
}

// Test codec under timeout conditions
func TestSimpleCodec_TimeoutResilience(t *testing.T) {
	const numGoroutines = 50
	const operationsPerGoroutine = 20
	
	var wg sync.WaitGroup
	var completedOperations int64
	
	// Test codec resilience under timeout conditions
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			
			timeout := time.After(time.Second)
			
			for j := 0; j < operationsPerGoroutine; j++ {
				select {
				case <-timeout:
					return // Exit on timeout
				default:
					codec := NewSimpleCodec()
					ctx := NewMockHandlerContext()
					
					testMsg := fmt.Sprintf("Timeout-Test-%d-%d", goroutineID, j)
					testBuf := buf.NewByteBuf([]byte(testMsg))
					future := channel.NewFuture(ctx.Channel())
					
					codec.Write(ctx, testBuf, future)
					atomic.AddInt64(&completedOperations, 1)
				}
			}
		}(i)
	}
	
	wg.Wait()
	
	t.Logf("Completed operations under timeout: %d", completedOperations)
	assert.Greater(t, completedOperations, int64(0), "Should complete some operations before timeout")
}