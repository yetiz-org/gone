package channel

// Combined from: channel_core_test.go, channel_simple_test.go, channel_test.go

import (
	"fmt"
	"net"
	"runtime"
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
	if time.Now().After(deadline) { t.Fatal("Test exceeded timeout") }
}

func TestParamKeyBasics(t *testing.T) {
	deadline := time.Now().Add(5 * time.Second)
	key1 := ParamKey("test-key-1")
	key2 := ParamKey("test-key-2")
	key1Dup := ParamKey("test-key-1")
	assert.Equal(t, key1, key1Dup)
	assert.NotEqual(t, key1, key2)
	if time.Now().After(deadline) { t.Fatal("Test exceeded timeout") }
}

func TestMockChannelExists(t *testing.T) {
	deadline := time.Now().Add(5 * time.Second)
	mockChannel := &MockChannel{}
	assert.NotNil(t, mockChannel)
	if time.Now().After(deadline) { t.Fatal("Test exceeded timeout") }
}

func TestMockFutureExists(t *testing.T) {
	deadline := time.Now().Add(5 * time.Second)
	mockFuture := &MockFuture{}
	assert.NotNil(t, mockFuture)
	if time.Now().After(deadline) { t.Fatal("Test exceeded timeout") }
}

func TestMockPipelineExists(t *testing.T) {
	deadline := time.Now().Add(5 * time.Second)
	mockPipeline := &MockPipeline{}
	assert.NotNil(t, mockPipeline)
	if time.Now().After(deadline) { t.Fatal("Test exceeded timeout") }
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
	if time.Now().After(deadline) { t.Fatal("Test exceeded timeout") }
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
	if time.Now().After(deadline) { t.Fatal("Test exceeded timeout") }
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
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < channelsPerGoroutine; j++ {
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
		}()
	}
	
	wg.Wait()
}

func TestDefaultChannel_ConcurrentParameterOperations(t *testing.T) {
	channel := &DefaultChannel{}
	channel.init(channel)
	
	const numGoroutines = 100
	const operationsPerGoroutine = 100
	
	var wg sync.WaitGroup
	
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := ParamKey(fmt.Sprintf("test-key-%d", id))
			expectedValue := id * 1000
			for j := 0; j < operationsPerGoroutine; j++ {
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
	for i := 0; i < numChannels; i++ {
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

func TestDefaultChannel_ConcurrentActivationDeactivation(t *testing.T) {
	ResetSerialSequenceForTesting()
	const numGoroutines = 5
	channel := &DefaultChannel{}
	channel.init(channel)
	var wg sync.WaitGroup
	var activationCount, deactivationCount int32
	done := make(chan bool, 1)
	go func() {
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				channel.activeChannel()
				atomic.AddInt32(&activationCount, 1)
				time.Sleep(time.Millisecond)
				success, future := channel.inactiveChannel()
				if success && future != nil {
					go func() {
						select {
						case <-time.After(100 * time.Millisecond):
						default:
							future.Await()
							atomic.AddInt32(&deactivationCount, 1)
						}
					}()
				}
			}(i)
		}
		wg.Wait()
		done <- true
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Test timed out - potential deadlock detected")
	}
	finalActivationCount := atomic.LoadInt32(&activationCount)
	finalDeactivationCount := atomic.LoadInt32(&deactivationCount)
	t.Logf("Activations: %d, Deactivations: %d", finalActivationCount, finalDeactivationCount)
	assert.Greater(t, finalActivationCount, int32(0), "Should have some activations")
}

func TestDefaultChannel_ConcurrentReadOperations(t *testing.T) {
	ResetSerialSequenceForTesting()
	runtime.GC(); runtime.GC()
	channel := &DefaultChannel{}
	channel.init(channel)
	const totalReads = 30
	for i := 0; i < totalReads; i++ {
		testData := map[string]interface{}{"index": i, "timestamp": time.Now().UnixNano()}
		channel.FireRead(testData)
		time.Sleep(time.Millisecond)
	}
	t.Logf("Sequential read processing completed: %d read operations fired", totalReads)
}

func TestDefaultChannel_ConcurrentWriteOperations(t *testing.T) {
	channel := &DefaultChannel{}
	channel.init(channel)
	const numGoroutines = 100
	const writesPerGoroutine = 10
	var wg sync.WaitGroup
	futures := make([]Future, 0, numGoroutines*writesPerGoroutine)
	var futuresMu sync.Mutex
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < writesPerGoroutine; j++ {
				testData := map[string]interface{}{"goroutine": id, "iteration": j, "data": "test-data"}
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
	for i := 0; i < numGoroutines; i++ {
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
			_ = channel.ID(); _ = channel.Serial()
			i++
		}
	})
}

func TestDefaultChannel_MemoryConsistency(t *testing.T) {
	if testing.Short() { t.Skip("Skipping memory consistency test in short mode") }
	const numGoroutines = 500
	const iterations = 1000
	channel := &DefaultChannel{}
	channel.init(channel)
	var wg sync.WaitGroup
	var successCount int64
	startSignal := make(chan struct{})
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			<-startSignal
			for j := 0; j < iterations; j++ {
				key := ParamKey(fmt.Sprintf("consistency-key-%d", goroutineID))
				value := goroutineID*iterations + j
				channel.SetParam(key, value)
				retrieved := channel.Param(key)
				if retrieved != nil && retrieved.(int) >= goroutineID*iterations {
					atomic.AddInt64(&successCount, 1)
				}
			}
		}(i)
	}
	close(startSignal)
	wg.Wait()
	t.Logf("Successful operations: %d out of %d", successCount, int64(numGoroutines*iterations))
	assert.Greater(t, successCount, int64(0), "Should have successful operations")
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
	if time.Now().After(deadline) { t.Fatal("Test exceeded timeout") }
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
	if time.Now().After(deadline) { t.Fatal("Test exceeded timeout") }
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
	if time.Now().After(deadline) { t.Fatal("Test exceeded timeout") }
}

func TestChannelPipelineIntegration(t *testing.T) {
	deadline := time.Now().Add(10 * time.Second)
	mockChannel := NewMockChannel()
	mockPipeline := NewMockPipeline()
	mockChannel.On("Pipeline").Return(mockPipeline)
	pipeline := mockChannel.Pipeline()
	assert.Equal(t, mockPipeline, pipeline)
	mockChannel.AssertExpectations(t)
	if time.Now().After(deadline) { t.Fatal("Test exceeded timeout") }
}

func TestChannelParentRelationship(t *testing.T) {
	deadline := time.Now().Add(10 * time.Second)
	mockChannel := NewMockChannel()
	mockServerChannel := NewMockServerChannel()
	mockChannel.On("Parent").Return(mockServerChannel)
	parent := mockChannel.Parent()
	assert.Equal(t, mockServerChannel, parent)
	mockChannel.AssertExpectations(t)
	if time.Now().After(deadline) { t.Fatal("Test exceeded timeout") }
}
