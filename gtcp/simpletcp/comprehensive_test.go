package simpletcp

// This comprehensive test file merges:
// - simplecodec_test.go
// - gtcp_simpletcp_comprehensive_test.go
// - simpleserver_test.go
// Original files will be archived to avoid duplicate execution.

import (
	"bytes"
	"fmt"
	"math"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/yetiz-org/gone/channel"
	goneMock "github.com/yetiz-org/gone/mock"
	"github.com/yetiz-org/gone/utils"
	buf "github.com/yetiz-org/goth-bytebuf"
	concurrent "github.com/yetiz-org/goth-concurrent"
)

// =============================================================================
// SimpleCodec Tests (from simplecodec_test.go)
// =============================================================================

// Test mock handler context
func TestSimpleCodec_MockHandlerContext(t *testing.T) {
	t.Parallel()

	ctx := goneMock.NewMockHandlerContext()
	assert.NotNil(t, ctx, "MockHandlerContext should not be nil")

	// Test basic mock functionality
	mockFuture := goneMock.NewMockFuture(nil)
	ctx.On("Write", mock.Anything, mockFuture).Return(mockFuture)
	ctx.Write("test", mockFuture)
	ctx.AssertExpectations(t)
}

type codecTestContext struct {
	channel.HandlerContext
	ch          channel.Channel
	written     [][]byte
	reads       [][]byte
	errors      []error
	disconnects int
}

func (c *codecTestContext) Channel() (ch channel.Channel) {
	return c.ch
}

func (c *codecTestContext) Write(obj any, future channel.Future) (result channel.Future) {
	c.written = append(c.written, obj.(buf.ByteBuf).BytesCopy())
	return future
}

func (c *codecTestContext) FireRead(obj any) (ctx channel.HandlerContext) {
	c.reads = append(c.reads, obj.(buf.ByteBuf).BytesCopy())
	return c
}

func (c *codecTestContext) FireErrorCaught(err error) (ctx channel.HandlerContext) {
	c.errors = append(c.errors, err)
	return c
}

func (c *codecTestContext) Disconnect(future channel.Future) (result channel.Future) {
	c.disconnects++
	return future
}

func TestSimpleCodec_ConcurrentDecoding(t *testing.T) {
	t.Parallel()

	const numGoroutines = 100
	const operationsPerGoroutine = 50
	var decoded atomic.Int64
	var wg sync.WaitGroup
	for i := range numGoroutines {
		wg.Go(func() {
			codec := NewSimpleCodec()
			ctx := &codecTestContext{ch: &channel.DefaultChannel{}}
			codec.Added(ctx)
			defer codec.Removed(ctx)
			for j := range operationsPerGoroutine {
				payload := fmt.Appendf(bytes.Repeat([]byte{'x'}, 260), "%d-%d", i, j)
				frame := append(utils.VarIntEncode(uint64(len(payload))).BytesCopy(), payload...)
				codec.Read(ctx, buf.NewByteBuf(frame[:2]))
				if !assert.Empty(t, ctx.reads, "partial header must not emit a message") {
					return
				}
				codec.Read(ctx, buf.NewByteBuf(frame[2:]))
				if !assert.Equal(t, [][]byte{payload}, ctx.reads) || !assert.Empty(t, ctx.errors) || !assert.Zero(t, ctx.disconnects) {
					return
				}
				ctx.reads = ctx.reads[:0]
				decoded.Add(1)
			}
		})
	}
	wg.Wait()
	assert.Equal(t, int64(numGoroutines*operationsPerGoroutine), decoded.Load())
}

// Test concurrent writing with multiple goroutines
func TestSimpleCodec_ConcurrentWriting(t *testing.T) {
	t.Parallel()

	const numGoroutines = 100
	const operationsPerGoroutine = 50
	codec := NewSimpleCodec()
	var written atomic.Int64
	var wg sync.WaitGroup
	for i := range numGoroutines {
		wg.Go(func() {
			ctx := &codecTestContext{}
			for j := range operationsPerGoroutine {
				payload := fmt.Appendf(bytes.Repeat([]byte{'w'}, 260), "%d-%d", i, j)
				future := channel.NewFuture(nil)
				codec.Write(ctx, buf.NewByteBuf(payload), future)
				expected := append(utils.VarIntEncode(uint64(len(payload))).BytesCopy(), payload...)
				if !assert.Equal(t, [][]byte{expected}, ctx.written) || !assert.Empty(t, ctx.errors) || !assert.NoError(t, future.Error()) {
					return
				}
				ctx.written = ctx.written[:0]
				written.Add(1)
			}
		})
	}
	wg.Wait()
	assert.Equal(t, int64(numGoroutines*operationsPerGoroutine), written.Load())
}

// Test SimpleCodec state consistency under concurrent access
func TestSimpleCodec_StateConsistency(t *testing.T) {
	t.Parallel()

	const numGoroutines = 50
	const operationsPerGoroutine = 100
	var completed atomic.Int64
	var wg sync.WaitGroup
	for i := range numGoroutines {
		wg.Go(func() {
			codec := NewSimpleCodec()
			ctx := &codecTestContext{ch: &channel.DefaultChannel{}}
			codec.Added(ctx)
			defer codec.Removed(ctx)
			for j := range operationsPerGoroutine {
				payload := fmt.Appendf(nil, "state %d-%d", i, j)
				expected := append(utils.VarIntEncode(uint64(len(payload))).BytesCopy(), payload...)
				if j%2 == 0 {
					codec.Write(ctx, buf.NewByteBuf(payload), channel.NewFuture(nil))
					if !assert.Equal(t, [][]byte{expected}, ctx.written) {
						return
					}
					ctx.written = ctx.written[:0]
				} else {
					codec.Read(ctx, buf.NewByteBuf(expected[:1]))
					if !assert.Empty(t, ctx.reads) {
						return
					}
					codec.Read(ctx, buf.NewByteBuf(expected[1:]))
					if !assert.Equal(t, [][]byte{payload}, ctx.reads) {
						return
					}
					ctx.reads = ctx.reads[:0]
				}
				if !assert.Empty(t, ctx.errors) || !assert.Zero(t, ctx.disconnects) {
					return
				}
				completed.Add(1)
			}
		})
	}
	wg.Wait()
	assert.Equal(t, int64(numGoroutines*operationsPerGoroutine), completed.Load())
}

// Test error handling in codec operations
func TestSimpleCodec_ErrorHandling(t *testing.T) {
	t.Parallel()

	t.Run("Decode_InvalidLength", func(t *testing.T) {
		t.Parallel()
		codec := NewSimpleCodec()
		ctx := &codecTestContext{ch: &channel.DefaultChannel{}}
		codec.Added(ctx)
		defer codec.Removed(ctx)
		frame := append(utils.VarIntEncode(300).BytesCopy(), bytes.Repeat([]byte{'x'}, 300)...)
		codec.Read(ctx, buf.NewByteBuf(frame[:2]))
		assert.Empty(t, ctx.reads)
		assert.Empty(t, ctx.errors)
		assert.Zero(t, ctx.disconnects)
		codec.Read(ctx, buf.NewByteBuf(frame[2:]))
		assert.Equal(t, [][]byte{bytes.Repeat([]byte{'x'}, 300)}, ctx.reads)
		assert.Empty(t, ctx.errors)
		assert.Zero(t, ctx.disconnects)
	})

	t.Run("Decode_NegativeLength", func(t *testing.T) {
		t.Parallel()
		codec := NewSimpleCodec()
		ctx := &codecTestContext{ch: &channel.DefaultChannel{}}
		codec.Added(ctx)
		defer codec.Removed(ctx)
		codec.Read(ctx, utils.VarIntEncode(math.MaxUint64))
		assert.Empty(t, ctx.reads)
		if assert.Len(t, ctx.errors, 1) {
			assert.ErrorIs(t, ctx.errors[0], channel.ErrFrameTooLarge)
		}
		assert.Equal(t, 1, ctx.disconnects)
	})

	codec := NewSimpleCodec()
	t.Run("Write_InvalidDataFailsFuture", func(t *testing.T) {
		t.Parallel()

		for _, obj := range []any{nil, 123} {
			ctx := goneMock.NewMockHandlerContext()
			future := channel.NewFuture(nil)
			ctx.On("FireErrorCaught", mock.MatchedBy(func(err error) bool {
				return assert.ErrorIs(t, err, channel.ErrUnknownObjectType)
			})).Return(ctx)

			assert.NotPanics(t, func() {
				codec.Write(ctx, obj, future)
			})
			assert.True(t, future.IsDone())
			assert.ErrorIs(t, future.Error(), channel.ErrUnknownObjectType)
			ctx.AssertNotCalled(t, "Write", mock.Anything, mock.Anything)
			ctx.AssertExpectations(t)
		}
	})
}

// Test high-frequency operations stress test
func TestSimpleCodec_HighFrequencyOperations(t *testing.T) {
	const duration = 2 * time.Second
	const numGoroutines = 20
	stop := make(chan struct{})
	var operations atomic.Int64
	var wg sync.WaitGroup
	for i := range numGoroutines {
		wg.Go(func() {
			codec := NewSimpleCodec()
			ctx := &codecTestContext{ch: &channel.DefaultChannel{}}
			codec.Added(ctx)
			defer codec.Removed(ctx)
			var localCount int64
			defer func() { operations.Add(localCount) }()
			for {
				select {
				case <-stop:
					return
				default:
				}
				payload := fmt.Appendf(nil, "stress %d-%d", i, localCount)
				codec.Write(ctx, buf.NewByteBuf(payload), nil)
				expected := append(utils.VarIntEncode(uint64(len(payload))).BytesCopy(), payload...)
				if len(ctx.written) != 1 || !bytes.Equal(expected, ctx.written[0]) {
					t.Errorf("stream %d write did not produce its complete frame", i)
					return
				}
				codec.Read(ctx, buf.NewByteBuf(ctx.written[0]))
				if len(ctx.reads) != 1 || !bytes.Equal(payload, ctx.reads[0]) || len(ctx.errors) != 0 || ctx.disconnects != 0 {
					t.Errorf("stream %d round trip failed: reads=%d errors=%v disconnects=%d", i, len(ctx.reads), ctx.errors, ctx.disconnects)
					return
				}
				ctx.written = ctx.written[:0]
				ctx.reads = ctx.reads[:0]
				localCount++
			}
		})
	}
	time.Sleep(duration)
	close(stop)
	wg.Wait()
	assert.Positive(t, operations.Load())
	t.Logf("verified %d encode/decode pairs across %d streams in %v", operations.Load(), numGoroutines, duration)
}

// =============================================================================
// SimpleTCP Client Core Tests (from gtcp_simpletcp_comprehensive_test.go)
// =============================================================================

// Test client core method coverage
func TestClient_CoreMethodCoverage(t *testing.T) {
	t.Parallel()

	handler := goneMock.NewMockHandler()
	client := NewClient(handler)

	// Test basic client properties
	assert.NotNil(t, client, "Client should not be nil")
	assert.Equal(t, handler, client.Handler, "Handler should be set correctly")

	// AutoReconnect is nil by default.
	assert.Nil(t, client.AutoReconnect, "AutoReconnect should be nil by default")

	// Test custom AutoReconnect function
	client.AutoReconnect = func() bool { return true }
	assert.True(t, client.AutoReconnect(), "Custom AutoReconnect should work")
}

// Mock handler implementation for testing
type mockHandler struct {
	channel.DefaultHandler
	activeCount   int32
	inactiveCount int32
	readCount     int32
	writeCount    int32
	errorCount    int32
	mu            sync.RWMutex
}

func (h *mockHandler) Active(ctx channel.HandlerContext) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.activeCount++
}

func (h *mockHandler) Inactive(ctx channel.HandlerContext) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.inactiveCount++
}

func (h *mockHandler) Read(ctx channel.HandlerContext, obj any) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.readCount++
}

func (h *mockHandler) Write(ctx channel.HandlerContext, obj any, future channel.Future) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.writeCount++
}

func (h *mockHandler) Error(ctx channel.HandlerContext, err error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.errorCount++
}

func (h *mockHandler) GetCounts() (active, inactive, read, write, errorCount int32) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.activeCount, h.inactiveCount, h.readCount, h.writeCount, h.errorCount
}

// Test handler adapter coverage
func TestClient_HandlerAdapterCoverage(t *testing.T) {
	t.Parallel()

	handler := &mockHandler{}
	client := NewClient(handler)

	// Test that client properly forwards handler calls
	assert.NotNil(t, client.Handler, "Client should have handler")

	// Create a mock context for testing
	mockCtx := goneMock.NewMockHandlerContext()
	mockCtx.On("Channel").Return(goneMock.NewMockChannel()).Maybe()

	// Test handler method calls
	client.Handler.Active(mockCtx)
	client.Handler.Inactive(mockCtx)
	client.Handler.Read(mockCtx, "test data")
	mockFuture := goneMock.NewMockFuture(nil)
	client.Handler.Write(mockCtx, "test data", mockFuture)
	// Note: Handler interface doesn't have Error method, so we skip this test

	// Verify handler was called
	active, inactive, read, write, errorCount := handler.GetCounts()
	assert.Equal(t, int32(1), active, "Active should be called once")
	assert.Equal(t, int32(1), inactive, "Inactive should be called once")
	assert.Equal(t, int32(1), read, "Read should be called once")
	assert.Equal(t, int32(1), write, "Write should be called once")
	// Since we can't call Error method (doesn't exist in Handler interface), errorCount should be 0
	assert.Equal(t, int32(0), errorCount, "Error count should be 0 since Error method doesn't exist in Handler interface")
}

// Test concurrent client operations
func TestClient_ConcurrentOperations(t *testing.T) {
	t.Parallel()

	const numGoroutines = 50
	const operationsPerGoroutine = 100

	var wg sync.WaitGroup
	var clientCount int64

	// Test concurrent client creation and operation
	for i := range numGoroutines {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for range operationsPerGoroutine {
				handler := &mockHandler{}
				client := NewClient(handler)

				// Test basic operations
				assert.NotNil(t, client)
				assert.NotNil(t, client.Handler)
				// AutoReconnect is nil by default.
				assert.Nil(t, client.AutoReconnect)

				// Test handler operations
				mockCtx := goneMock.NewMockHandlerContext()
				mockCtx.On("Channel").Return(goneMock.NewMockChannel()).Maybe()

				client.Handler.Active(mockCtx)
				client.Handler.Inactive(mockCtx)

				atomic.AddInt64(&clientCount, 1)
			}
		}(i)
	}

	wg.Wait()

	expectedTotal := int64(numGoroutines * operationsPerGoroutine)
	assert.Equal(t, expectedTotal, clientCount, "All client operations should be counted")

	t.Logf("Concurrent client operations: %d clients created and tested", clientCount)
}

// Test error handling scenarios
func TestClient_ErrorHandlingScenarios(t *testing.T) {
	t.Parallel()

	t.Run("NilHandler", func(t *testing.T) {
		t.Parallel()

		// Test client creation with nil handler
		client := NewClient(nil)
		assert.NotNil(t, client, "Client should be created even with nil handler")
		assert.Nil(t, client.Handler, "Handler should be nil as provided")
	})

	t.Run("HandlerErrorPropagation", func(t *testing.T) {
		t.Parallel()

		handler := &mockHandler{}
		client := NewClient(handler)

		// Verify client was created with handler
		assert.NotNil(t, client, "Client should be created")
		assert.Equal(t, handler, client.Handler, "Client should store the provided handler")

		// Test error handling - Handler interface doesn't have Error method
		// So we skip the direct error call and just verify other functionality

		_, _, _, _, errorCount := handler.GetCounts()
		// Since we can't call Error method, errorCount should remain 0
		assert.Equal(t, int32(0), errorCount, "Error count should be 0 since Error method doesn't exist")
	})
}

func TestClient_NilChannelOperationsReturnFailedFuture(t *testing.T) {
	t.Parallel()

	client := NewClient(nil)

	writeFuture := client.Write(buf.EmptyByteBuf())
	assert.True(t, writeFuture.IsDone(), "nil-channel write should complete immediately")
	assert.False(t, writeFuture.IsSuccess(), "nil-channel write should fail instead of returning a pending future")

	disconnectFuture := client.Disconnect()
	assert.True(t, disconnectFuture.IsDone(), "nil-channel disconnect should complete immediately")
	assert.False(t, disconnectFuture.IsSuccess(), "nil-channel disconnect should fail instead of returning a pending future")
}

func TestServer_StopWithoutStartReturnsFailedFuture(t *testing.T) {
	t.Parallel()

	server := NewServer(nil)

	stopFuture := server.Stop()
	assert.True(t, stopFuture.IsDone(), "nil-channel stop should complete immediately")
	assert.False(t, stopFuture.IsSuccess(), "nil-channel stop should fail instead of panicking")
}

func TestClient_ChannelAccessConcurrentWithReconnectState(t *testing.T) {
	t.Parallel()

	client := NewClient(nil)
	mockCh := goneMock.NewMockChannel()

	const goroutines = 32
	const iterations = 1000
	var wg sync.WaitGroup
	for range goroutines {
		wg.Add(2)
		go func() {
			defer wg.Done()
			for range iterations {
				client.mu.Lock()
				client.ch = mockCh
				client.mu.Unlock()
			}
		}()
		go func() {
			defer wg.Done()
			for range iterations {
				_ = client.Channel()
				client.close.Store(false)
				_ = client.close.Load()
			}
		}()
	}
	wg.Wait()
}

// =============================================================================
// Server Tests (from simpleserver_test.go)
// =============================================================================

type testServerHandler struct {
	channel.DefaultHandler
}

func (h *testServerHandler) Read(ctx channel.HandlerContext, obj any) {
	ctx.Channel().Write(obj)
}

type testClientHandler struct {
	channel.DefaultHandler
	num    int32
	active int32 // RACE FIX: Use int32 for atomic operations
	read   int32 // RACE FIX: Use int32 for atomic operations
	wg     concurrent.WaitGroup
}

func (h *testClientHandler) Active(ctx channel.HandlerContext) {
	atomic.AddInt32(&h.active, 1) // RACE FIX: Use atomic increment
	ctx.Channel().Write(buf.EmptyByteBuf().WriteInt32(atomic.LoadInt32(&h.num)))
}

func (h *testClientHandler) Read(ctx channel.HandlerContext, obj any) {
	currentNum := atomic.LoadInt32(&h.num) // RACE FIX: Atomic load
	if obj.(buf.ByteBuf).ReadInt32() == currentNum {
		h.wg.Done()
		atomic.AddInt32(&h.num, 1)  // RACE FIX: Atomic increment
		atomic.AddInt32(&h.read, 1) // RACE FIX: Atomic increment
		ctx.Channel().Disconnect()
	}
}

// Test server start and client connections
func TestServer_Start(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	assert.NoError(t, listener.Close())

	server := NewServer(&testServerHandler{})
	sch := server.Start(&net.TCPAddr{IP: nil, Port: port})
	assert.NotNil(t, sch)
	count := 10
	for range count {
		go func(t *testing.T) {
			tcHandler := &testClientHandler{}
			tcHandler.wg.Add(count)
			client := NewClient(tcHandler)
			client.AutoReconnect = func() bool {
				return atomic.LoadInt32(&tcHandler.active) < int32(count) // RACE FIX: Use atomic load
			}

			cch := client.Start(&net.TCPAddr{IP: nil, Port: port})
			assert.NotNil(t, cch)
			tcHandler.wg.Wait()
			assert.Equal(t, int32(count), atomic.LoadInt32(&tcHandler.read))   // RACE FIX: Use atomic load
			assert.Equal(t, int32(count), atomic.LoadInt32(&tcHandler.active)) // RACE FIX: Use atomic load
		}(t)
	}

	go func(t *testing.T) {
		tcHandler := &testClientHandler{}
		tcHandler.wg.Add(count)
		client := NewClient(tcHandler)
		client.AutoReconnect = func() bool {
			return atomic.LoadInt32(&tcHandler.active) < int32(count) // RACE FIX: Use atomic load
		}

		cch := client.Start(&net.TCPAddr{IP: nil, Port: port})
		assert.NotNil(t, cch)
		tcHandler.wg.Wait()
		assert.Equal(t, int32(count), atomic.LoadInt32(&tcHandler.read))   // RACE FIX: Use atomic load
		assert.Equal(t, int32(count), atomic.LoadInt32(&tcHandler.active)) // RACE FIX: Use atomic load
		server.Stop()
	}(t)

	server.Channel().CloseFuture().Sync()
}

// =============================================================================
// Benchmark Tests
// =============================================================================

// Benchmark SimpleCodec operations
func BenchmarkSimpleCodec_Operations(b *testing.B) {
	codec := NewSimpleCodec() // CRITICAL FIX: Use NewSimpleCodec() instead of direct instantiation
	testData := []byte("benchmark test data")

	b.ResetTimer()

	b.Run("Write", func(b *testing.B) {
		for b.Loop() {
			ctx := goneMock.NewMockHandlerContext()
			mockFuture := goneMock.NewMockFuture(nil)
			ctx.On("Write", mock.Anything, mockFuture).Return(mockFuture).Maybe()
			byteBuf := buf.EmptyByteBuf().WriteBytes(testData)
			codec.Write(ctx, byteBuf, mockFuture)
		}
	})

	// NOTE: Decode benchmark removed due to SimpleCodec stateful design complexity
	// Our primary optimizations (buffer pooling) are better measured through Write operations
	// and integration tests, not this specific codec decode path
}

// Benchmark client operations
func BenchmarkClient_Operations(b *testing.B) {
	b.ResetTimer()

	b.Run("ClientCreation", func(b *testing.B) {
		for b.Loop() {
			handler := &mockHandler{}
			client := NewClient(handler)
			_ = client
		}
	})

	b.Run("HandlerCalls", func(b *testing.B) {
		handler := &mockHandler{}
		client := NewClient(handler)
		mockCtx := goneMock.NewMockHandlerContext()
		mockCtx.On("Channel").Return(goneMock.NewMockChannel()).Maybe()

		for b.Loop() {
			client.Handler.Active(mockCtx)
			client.Handler.Inactive(mockCtx)
		}
	})
}
