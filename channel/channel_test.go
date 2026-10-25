package channel

// Combined from: channel_core_test.go, channel_simple_test.go, channel_test.go

import (
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// MockAddr implements net.Addr for testing
// (from channel_test.go)
type MockAddr struct {
	network string
	address string
}

func (m *MockAddr) Network() string { return m.network }
func (m *MockAddr) String() string  { return m.address }

// --- From channel_simple_test.go ---
func TestChannelErrorConstants(t *testing.T) {
	deadline := time.Now().Add(5 * time.Second)
	assert.NotNil(t, ErrNotActive)
	assert.NotNil(t, ErrNilObject)
	assert.NotNil(t, ErrUnknownObjectType)
	assert.Contains(t, ErrNotActive.Error(), "not active")
	assert.Contains(t, ErrNilObject.Error(), "nil object")
	assert.Contains(t, ErrUnknownObjectType.Error(), "unknown object type")
	if time.Now().After(deadline) {
		t.Fatal("Test exceeded timeout")
	}
}

func TestParamKeyBasics(t *testing.T) {
	deadline := time.Now().Add(5 * time.Second)
	key1 := ParamKey("test-key-1")
	key2 := ParamKey("test-key-2")
	key1Dup := ParamKey("test-key-1")
	assert.Equal(t, key1, key1Dup)
	assert.NotEqual(t, key1, key2)
	if time.Now().After(deadline) {
		t.Fatal("Test exceeded timeout")
	}
}

func TestMockChannelExists(t *testing.T) {
	deadline := time.Now().Add(5 * time.Second)
	mockChannel := &MockChannel{}
	assert.NotNil(t, mockChannel)
	if time.Now().After(deadline) {
		t.Fatal("Test exceeded timeout")
	}
}

func TestMockFutureExists(t *testing.T) {
	deadline := time.Now().Add(5 * time.Second)
	mockFuture := &MockFuture{}
	assert.NotNil(t, mockFuture)
	if time.Now().After(deadline) {
		t.Fatal("Test exceeded timeout")
	}
}

func TestMockPipelineExists(t *testing.T) {
	deadline := time.Now().Add(5 * time.Second)
	mockPipeline := &MockPipeline{}
	assert.NotNil(t, mockPipeline)
	if time.Now().After(deadline) {
		t.Fatal("Test exceeded timeout")
	}
}

// --- From channel_core_test.go ---
func TestChannelErrorValues(t *testing.T) {
	deadline := time.Now().Add(5 * time.Second)
	assert.NotNil(t, ErrNotActive)
	assert.NotNil(t, ErrNilObject)
	assert.NotNil(t, ErrUnknownObjectType)
	assert.Contains(t, ErrNotActive.Error(), "not active")
	assert.Contains(t, ErrNilObject.Error(), "nil object")
	assert.Contains(t, ErrUnknownObjectType.Error(), "unknown object type")
	if time.Now().After(deadline) {
		t.Fatal("Test exceeded timeout")
	}
}

func TestParamKeyValues(t *testing.T) {
	deadline := time.Now().Add(5 * time.Second)
	key1 := ParamKey("test-key-1")
	key2 := ParamKey("test-key-2")
	key1Dup := ParamKey("test-key-1")
	assert.Equal(t, key1, key1Dup)
	assert.NotEqual(t, key1, key2)
	assert.NotEqual(t, key1, key2)
	assert.Equal(t, ParamKey("test-key-1"), key1)
	if time.Now().After(deadline) {
		t.Fatal("Test exceeded timeout")
	}
}

// --- From channel_test.go ---
// Test Channel interface compliance and basic functionality
func TestDefaultChannel_InterfaceCompliance(t *testing.T) {
	var _ Channel = (*DefaultChannel)(nil)

	channel := &DefaultChannel{}
	channel.init(channel)

	// Test basic properties
	assert.NotEmpty(t, channel.ID(), "Channel ID should not be empty")
	assert.Greater(t, channel.Serial(), uint64(0), "Channel serial should be greater than 0")
	assert.NotNil(t, channel.Pipeline(), "Pipeline should not be nil")
	assert.NotNil(t, channel.CloseFuture(), "CloseFuture should not be nil")
	assert.NotNil(t, channel.Params(), "Params should not be nil")
}

// Test concurrent channel ID generation for uniqueness
func TestDefaultChannel_ConcurrentIDGeneration(t *testing.T) {
	const numGoroutines = 1000
	const channelsPerGoroutine = 10

	var wg sync.WaitGroup
	idSet := sync.Map{}
	serialSet := sync.Map{}

	// Create channels concurrently
	for range numGoroutines {
		wg.Go(func() {
			for range channelsPerGoroutine {
				channel := &DefaultChannel{}
				channel.init(channel)

				// Check ID uniqueness
				id := channel.ID()
				if _, exists := idSet.LoadOrStore(id, true); exists {
					t.Errorf("Duplicate channel ID found: %s", id)
				}

				// Check serial uniqueness
				serial := channel.Serial()
				if _, exists := serialSet.LoadOrStore(serial, true); exists {
					t.Errorf("Duplicate channel serial found: %d", serial)
				}
			}
		})
	}

	wg.Wait()
}

func TestDefaultChannel_ConcurrentParameterOperations(t *testing.T) {
	channel := &DefaultChannel{}
	channel.init(channel)

	const numGoroutines = 100
	const operationsPerGoroutine = 100

	var wg sync.WaitGroup

	for i := range numGoroutines {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := ParamKey(fmt.Sprintf("test-key-%d", id))
			expectedValue := id * 1000
			for j := range operationsPerGoroutine {
				channel.SetParam(key, expectedValue+j)
				value := channel.Param(key)
				if value != nil {
					assert.IsType(t, 0, value, "Parameter value should be int")
				}
			}
		}(i)
	}
	wg.Wait()
}

func TestDefaultChannel_ConcurrentLifecycleOperations(t *testing.T) {
	const numChannels = 100
	var wg sync.WaitGroup
	channels := make([]*DefaultChannel, numChannels)
	for i := range numChannels {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			channel := &DefaultChannel{}
			channel.init(channel)
			channels[idx] = channel
			pipeline := channel.Pipeline()
			assert.NotNil(t, pipeline, "Pipeline should not be nil")
			testKey := ParamKey("test-key")
			channel.SetParam(testKey, idx)
			value := channel.Param(testKey)
			assert.Equal(t, idx, value, "Parameter value should match")
		}(i)
	}
	wg.Wait()
	for i, channel := range channels {
		assert.NotNil(t, channel, "Channel %d should not be nil", i)
		assert.NotEmpty(t, channel.ID(), "Channel %d ID should not be empty", i)
		assert.Greater(t, channel.Serial(), uint64(0), "Channel %d serial should be greater than 0", i)
	}
}

type lifecycleTestChannel struct {
	DefaultChannel
}

func (c *lifecycleTestChannel) UnsafeIsAutoRead() (autoRead bool) {
	return false
}

func TestDefaultChannel_ConcurrentActivationDeactivation(t *testing.T) {
	const numGoroutines = 5
	channel := &lifecycleTestChannel{}
	channel.init(channel)
	var wg sync.WaitGroup
	var activationCount, deactivationCount atomic.Int32
	start := make(chan struct{})
	for range numGoroutines {
		wg.Go(func() {
			<-start
			channel.activeChannel()
			activationCount.Add(1)
			success, future := channel.inactiveChannel()
			select {
			case <-future.Done():
				assert.NoError(t, future.Error())
				assert.Same(t, &channel.DefaultChannel, future.Get())
			case <-time.After(time.Second):
				t.Error("inactive transition did not settle")
			}
			if success {
				deactivationCount.Add(1)
			}
		})
	}
	close(start)
	joined := make(chan struct{})
	go func() {
		wg.Wait()
		close(joined)
	}()
	select {
	case <-joined:
	case <-time.After(10 * time.Second):
		t.Fatal("activation/deactivation workload did not finish")
	}
	assert.Equal(t, int32(numGoroutines), activationCount.Load())
	assert.Greater(t, deactivationCount.Load(), int32(0))
	assert.LessOrEqual(t, deactivationCount.Load(), int32(numGoroutines))
	assert.False(t, channel.IsActive())
	requireDoneFuture(t, channel.CloseFuture())
	assert.NoError(t, channel.CloseFuture().Error())
}

func TestDefaultChannel_ConcurrentReadOperations(t *testing.T) {
	channel := &DefaultChannel{}
	channel.init(channel)
	const totalReads = 30
	received := make(chan any, totalReads)
	channel.Pipeline().AddLast("capture", NewRWHandler(func(ctx HandlerContext, obj any) {
		select {
		case received <- obj:
		default:
			t.Error("more read deliveries than requested")
		}
	}, nil))
	payloads := make([]*int, totalReads)
	var wg sync.WaitGroup
	for i := range totalReads {
		payloads[i] = new(i)
		wg.Go(func() {
			assert.Same(t, channel, channel.FireRead(payloads[i]))
		})
	}
	wg.Wait()
	close(received)
	seen := make(map[*int]int, totalReads)
	for obj := range received {
		payload, ok := obj.(*int)
		if assert.True(t, ok, "read payload type must be preserved") {
			seen[payload]++
		}
	}
	assert.Len(t, seen, totalReads)
	for i, payload := range payloads {
		assert.Equal(t, 1, seen[payload], "payload %d delivered exactly once", i)
	}
}

func TestDefaultChannel_ConcurrentWriteOperations(t *testing.T) {
	channel := &DefaultChannel{}
	channel.init(channel)
	const numGoroutines = 100
	const writesPerGoroutine = 10
	var wg sync.WaitGroup
	futures := make([]Future, 0, numGoroutines*writesPerGoroutine)
	var futuresMu sync.Mutex
	for i := range numGoroutines {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := range writesPerGoroutine {
				testData := map[string]any{"goroutine": id, "iteration": j, "data": "test-data"}
				future := channel.Write(testData)
				futuresMu.Lock()
				futures = append(futures, future)
				futuresMu.Unlock()
			}
		}(i)
	}
	wg.Wait()
	expectedWrites := numGoroutines * writesPerGoroutine
	actualWrites := len(futures)
	t.Logf("Concurrent write operations completed: %d write operations created (expected: %d)", actualWrites, expectedWrites)
	assert.Equal(t, expectedWrites, actualWrites, "Should create expected number of write futures")
}

func TestDefaultChannel_ConcurrentLocalAddrOperations(t *testing.T) {
	channel := &DefaultChannel{}
	channel.init(channel)
	const numGoroutines = 100
	var wg sync.WaitGroup
	for i := range numGoroutines {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			addr := &MockAddr{network: "tcp", address: "127.0.0.1:" + string(rune(8000+id))}
			channel.setLocalAddr(addr)
			retrievedAddr := channel.LocalAddr()
			if retrievedAddr != nil {
				assert.Implements(t, (*net.Addr)(nil), retrievedAddr, "Should implement net.Addr")
			}
		}(i)
	}
	wg.Wait()
	finalAddr := channel.LocalAddr()
	if finalAddr != nil {
		assert.Implements(t, (*net.Addr)(nil), finalAddr, "Final address should implement net.Addr")
	}
}

func BenchmarkDefaultChannel_ConcurrentOperations(b *testing.B) {
	channel := &DefaultChannel{}
	channel.init(channel)
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := ParamKey(fmt.Sprintf("bench-key-%d", i))
			channel.SetParam(key, i)
			_ = channel.Param(key)
			_ = channel.ID()
			_ = channel.Serial()
			i++
		}
	})
}

func TestDefaultChannel_MemoryConsistency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory consistency test in short mode")
	}
	const numGoroutines = 500
	const iterations = 1000
	channel := &DefaultChannel{}
	channel.init(channel)
	var wg sync.WaitGroup
	var successCount atomic.Int64
	startSignal := make(chan struct{})
	for i := range numGoroutines {
		wg.Go(func() {
			<-startSignal
			key := ParamKey(fmt.Sprintf("consistency-key-%d", i))
			for j := range iterations {
				value := i*iterations + j
				channel.SetParam(key, value)
				retrieved := channel.Param(key)
				if retrieved == value {
					successCount.Add(1)
				} else {
					t.Errorf("worker %d iteration %d: got %v, want %d", i, j, retrieved, value)
				}
			}
		})
	}
	close(startSignal)
	wg.Wait()
	assert.Equal(t, int64(numGoroutines*iterations), successCount.Load())
}

// --- Additional mocks-based tests merged from channel_core_test.go ---
func TestChannelLifecycleOperations(t *testing.T) {
	deadline := time.Now().Add(10 * time.Second)
	mockChannel := NewMockChannel()
	mockFuture := NewMockFuture(mockChannel)
	mockChannel.On("CloseFuture").Return(mockFuture)
	closeFuture := mockChannel.CloseFuture()
	assert.Equal(t, mockFuture, closeFuture)
	mockChannel.On("Close").Return(mockFuture)
	closeResult := mockChannel.Close()
	assert.Equal(t, mockFuture, closeResult)
	mockChannel.On("Deregister").Return(mockFuture)
	deregisterResult := mockChannel.Deregister()
	assert.Equal(t, mockFuture, deregisterResult)
	mockChannel.AssertExpectations(t)
	if time.Now().After(deadline) {
		t.Fatal("Test exceeded timeout")
	}
}

func TestChannelNetworkOperations(t *testing.T) {
	deadline := time.Now().Add(10 * time.Second)
	mockChannel := NewMockChannel()
	mockFuture := NewMockFuture(mockChannel)
	localAddr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080}
	remoteAddr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 9090}
	mockChannel.On("Bind", localAddr).Return(mockFuture)
	bindResult := mockChannel.Bind(localAddr)
	assert.Equal(t, mockFuture, bindResult)
	mockChannel.On("Connect", localAddr, remoteAddr).Return(mockFuture)
	connectResult := mockChannel.Connect(localAddr, remoteAddr)
	assert.Equal(t, mockFuture, connectResult)
	mockChannel.On("Disconnect").Return(mockFuture)
	disconnectResult := mockChannel.Disconnect()
	assert.Equal(t, mockFuture, disconnectResult)
	mockChannel.On("LocalAddr").Return(localAddr)
	actualLocalAddr := mockChannel.LocalAddr()
	assert.Equal(t, localAddr, actualLocalAddr)
	mockChannel.AssertExpectations(t)
	if time.Now().After(deadline) {
		t.Fatal("Test exceeded timeout")
	}
}

func TestChannelReadWriteOperations(t *testing.T) {
	deadline := time.Now().Add(10 * time.Second)
	mockChannel := NewMockChannel()
	mockFuture := NewMockFuture(mockChannel)
	mockChannel.On("Read").Return(mockChannel)
	readResult := mockChannel.Read()
	assert.Equal(t, mockChannel, readResult)
	testObject := "test-data"
	mockChannel.On("FireRead", testObject).Return(mockChannel)
	fireReadResult := mockChannel.FireRead(testObject)
	assert.Equal(t, mockChannel, fireReadResult)
	mockChannel.On("FireReadCompleted").Return(mockChannel)
	fireReadCompletedResult := mockChannel.FireReadCompleted()
	assert.Equal(t, mockChannel, fireReadCompletedResult)
	mockChannel.On("Write", testObject).Return(mockFuture)
	writeResult := mockChannel.Write(testObject)
	assert.Equal(t, mockFuture, writeResult)
	mockChannel.AssertExpectations(t)
	if time.Now().After(deadline) {
		t.Fatal("Test exceeded timeout")
	}
}

func TestChannelPipelineIntegration(t *testing.T) {
	deadline := time.Now().Add(10 * time.Second)
	mockChannel := NewMockChannel()
	mockPipeline := NewMockPipeline()
	mockChannel.On("Pipeline").Return(mockPipeline)
	pipeline := mockChannel.Pipeline()
	assert.Equal(t, mockPipeline, pipeline)
	mockChannel.AssertExpectations(t)
	if time.Now().After(deadline) {
		t.Fatal("Test exceeded timeout")
	}
}

func TestChannelParentRelationship(t *testing.T) {
	deadline := time.Now().Add(10 * time.Second)
	mockChannel := NewMockChannel()
	mockServerChannel := NewMockServerChannel()
	mockChannel.On("Parent").Return(mockServerChannel)
	parent := mockChannel.Parent()
	assert.Equal(t, mockServerChannel, parent)
	mockChannel.AssertExpectations(t)
	if time.Now().After(deadline) {
		t.Fatal("Test exceeded timeout")
	}
}
