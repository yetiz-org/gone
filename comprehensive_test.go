package gone

// This comprehensive test file merges:
// - deadlock_prevention_test.go
// - debug_channel_init_test.go
// - resource_leak_detection_test.go
// - thread_safety_test.go
// Original files will be archived to avoid duplicate execution.

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/yetiz-org/gone/channel"
	"github.com/yetiz-org/gone/gtcp"
	"github.com/yetiz-org/gone/gws"
	"github.com/yetiz-org/gone/utils"
	kklogger "github.com/yetiz-org/goth-kklogger"
)

func init() {
	kklogger.SetLogLevel("DEBUG")
}

// =============================================================================
// Deadlock Prevention Tests (from deadlock_prevention_test.go)
// =============================================================================

// Test deadlock prevention in concurrent channel operations
func TestDeadlockPrevention_ChannelOperations(t *testing.T) {
	const numGoroutines = 100
	const operationsPerGoroutine = 50
	const timeout = 10 * time.Second

	var completedOperations atomic.Int64
	var wg sync.WaitGroup
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range numGoroutines {
			wg.Go(func() {
				for range operationsPerGoroutine {
					ch1 := &channel.DefaultChannel{}
					ch1.Init()
					ch2 := &channel.DefaultChannel{}
					ch2.Init()
					var operations sync.WaitGroup
					operations.Go(func() {
						assert.False(t, ch1.IsActive())
						assert.False(t, ch2.IsActive())
					})
					operations.Go(func() {
						assert.False(t, ch2.IsActive())
						assert.False(t, ch1.IsActive())
					})
					operations.Wait()
					completedOperations.Add(1)
				}
			})
		}
		wg.Wait()
	}()

	select {
	case <-done:
		assert.Equal(t, int64(numGoroutines*operationsPerGoroutine), completedOperations.Load(), "All operations should complete without deadlock")
	case <-time.After(timeout):
		t.Fatalf("Deadlock prevention test timed out after %v - potential deadlock detected", timeout)
	}
}

// Test deadlock prevention in TCP server operations
func TestDeadlockPrevention_TCPServerOperations(t *testing.T) {
	const numGoroutines = 50
	const operationsPerGoroutine = 30
	const timeout = 15 * time.Second

	var completedOperations atomic.Int64
	var wg sync.WaitGroup
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range numGoroutines {
			wg.Go(func() {
				for range operationsPerGoroutine {
					server := &gtcp.ServerChannel{}
					server.Init()
					tcpCh := &gtcp.Channel{}
					tcpCh.Init()
					assert.False(t, tcpCh.IsActive())
					var operations sync.WaitGroup
					operations.Go(func() {
						assert.False(t, server.IsActive())
					})
					operations.Go(func() {
						assert.False(t, server.IsActive())
					})
					operations.Wait()
					completedOperations.Add(1)
				}
			})
		}
		wg.Wait()
	}()

	select {
	case <-done:
		assert.Equal(t, int64(numGoroutines*operationsPerGoroutine), completedOperations.Load(), "All operations should complete without deadlock")
	case <-time.After(timeout):
		t.Fatalf("Deadlock prevention test timed out after %v - potential deadlock detected", timeout)
	}
}

// Test deadlock prevention in WebSocket operations
func TestDeadlockPrevention_WebSocketOperations(t *testing.T) {
	const numGoroutines = 75
	const operationsPerGoroutine = 35
	const timeout = 15 * time.Second

	var completedOperations atomic.Int64
	var wg sync.WaitGroup
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range numGoroutines {
			wg.Go(func() {
				for range operationsPerGoroutine {
					wsCh := &gws.Channel{}
					wsCh.BootstrapPreInit()
					wsCh.Init()
					msg1 := &gws.DefaultMessage{MessageType: gws.TextMessageType, Message: []byte("deadlock test 1")}
					msg2 := &gws.DefaultMessage{MessageType: gws.BinaryMessageType, Message: []byte{0x01, 0x02, 0x03}}
					assert.False(t, wsCh.IsActive())
					var operations sync.WaitGroup
					operations.Go(func() {
						assert.Equal(t, []byte("deadlock test 1"), msg1.Encoded())
						assert.Equal(t, []byte{0x01, 0x02, 0x03}, msg2.Encoded())
					})
					operations.Go(func() {
						assert.Equal(t, []byte{0x01, 0x02, 0x03}, msg2.Encoded())
						assert.Equal(t, []byte("deadlock test 1"), msg1.Encoded())
					})
					operations.Wait()
					completedOperations.Add(1)
				}
			})
		}
		wg.Wait()
	}()

	select {
	case <-done:
		assert.Equal(t, int64(numGoroutines*operationsPerGoroutine), completedOperations.Load(), "All operations should complete without deadlock")
	case <-time.After(timeout):
		t.Fatalf("Deadlock prevention test timed out after %v - potential deadlock detected", timeout)
	}
}

// Test deadlock prevention in cross-package interactions
func TestDeadlockPrevention_CrossPackageInteractions(t *testing.T) {
	const numGoroutines = 50
	const operationsPerGoroutine = 25
	const timeout = 20 * time.Second

	var completedOperations atomic.Int64
	var wg sync.WaitGroup
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range numGoroutines {
			wg.Go(func() {
				for range operationsPerGoroutine {
					ch := &channel.DefaultChannel{}
					ch.Init()
					tcpCh := &gtcp.Channel{}
					tcpCh.Init()
					wsCh := &gws.Channel{}
					wsCh.BootstrapPreInit()
					wsCh.Init()
					var operations sync.WaitGroup
					operations.Go(func() {
						assert.False(t, ch.IsActive())
						assert.False(t, tcpCh.IsActive())
						assert.False(t, wsCh.IsActive())
					})
					operations.Go(func() {
						assert.False(t, wsCh.IsActive())
						assert.False(t, tcpCh.IsActive())
						assert.False(t, ch.IsActive())
					})
					operations.Go(func() {
						future := ch.Pipeline().NewFuture()
						var completer sync.WaitGroup
						completer.Go(func() {
							assert.True(t, future.Completable().Complete("cross-package-test"))
						})
						future.Await()
						completer.Wait()
						assert.True(t, future.IsSuccess())
						assert.Equal(t, "cross-package-test", future.GetNow())
					})
					operations.Wait()
					completedOperations.Add(1)
				}
			})
		}
		wg.Wait()
	}()

	select {
	case <-done:
		assert.Equal(t, int64(numGoroutines*operationsPerGoroutine), completedOperations.Load(), "All operations should complete without deadlock")
	case <-time.After(timeout):
		t.Fatalf("Deadlock prevention test timed out after %v - potential deadlock detected", timeout)
	}
}

// =============================================================================
// Debug Channel Init Tests (from debug_channel_init_test.go)
// =============================================================================

// Minimal test to isolate channel initialization deadlock
func TestMinimalChannelCreation(t *testing.T) {
	fmt.Println("=== Starting minimal channel creation test ===")
	kklogger.DebugJ("debug_test:TestMinimalChannelCreation#test!start", "Test started")

	// Step 1: Test basic ServerChannel creation
	fmt.Println("Step 1: Creating ServerChannel...")
	serverChannel := &gtcp.ServerChannel{}
	fmt.Println("Step 1: ServerChannel created successfully")

	// Step 2: Test channel.init() call
	fmt.Println("Step 2: Calling serverChannel.init()...")
	serverChannel.Init() // This should call c.init(c) internally
	fmt.Println("Step 2: serverChannel.Init() completed")

	// Step 3: Test pipeline access
	fmt.Println("Step 3: Accessing Pipeline...")
	pipeline := serverChannel.Pipeline()
	if pipeline == nil {
		fmt.Println("ERROR: Pipeline is nil!")
		t.Fatal("Pipeline is nil after Init()")
	}
	fmt.Println("Step 3: Pipeline access successful")

	// Step 4: Test basic pipeline operations
	fmt.Println("Step 4: Testing pipeline operations...")
	future := pipeline.NewFuture()
	if future == nil {
		fmt.Println("ERROR: NewFuture returned nil!")
		t.Fatal("NewFuture returned nil")
	}
	fmt.Println("Step 4: Pipeline NewFuture successful")

	fmt.Println("=== All steps completed successfully ===")
	kklogger.DebugJ("debug_test:TestMinimalChannelCreation#test!end", "Test completed successfully")
}

// Test ServerBootstrap creation without binding
func TestMinimalBootstrapCreation(t *testing.T) {
	fmt.Println("=== Starting minimal bootstrap creation test ===")

	// Step 1: Create bootstrap
	fmt.Println("Step 1: Creating ServerBootstrap...")
	bootstrap := channel.NewServerBootstrap()
	fmt.Println("Step 1: ServerBootstrap created")

	// Step 2: Set channel type
	fmt.Println("Step 2: Setting ChannelType...")
	bootstrap.ChannelType(&gtcp.ServerChannel{})
	fmt.Println("Step 2: ChannelType set")

	// Step 3: Set child handler
	fmt.Println("Step 3: Setting ChildHandler...")
	bootstrap.ChildHandler(channel.NewInitializer(func(ch channel.Channel) {
		fmt.Println("Initializer callback called")
	}))
	fmt.Println("Step 3: ChildHandler set")

	fmt.Println("=== Bootstrap creation completed successfully ===")
}

// Test manual Future completion to verify Sync() mechanism
func TestManualFutureCompletion(t *testing.T) {
	fmt.Println("=== Starting manual future completion test ===")

	// Create a simple channel and get a future from its pipeline
	serverChannel := &gtcp.ServerChannel{}
	serverChannel.Init()

	fmt.Println("Creating future from pipeline...")
	future := serverChannel.Pipeline().NewFuture()

	fmt.Println("Testing future completion manually...")

	// Complete the future in a goroutine
	go func() {
		fmt.Println("Goroutine: Completing future manually...")
		// Use the correct completion method via Completable interface
		fmt.Println("Goroutine: Calling future.Completable().Complete()")
		completed := future.Completable().Complete(serverChannel)
		fmt.Printf("Goroutine: Complete() returned: %v\n", completed)
		fmt.Println("Goroutine: Manual completion done")
	}()

	fmt.Println("Calling future.Sync() with timeout...")

	// Create a timeout to prevent infinite hang
	done := make(chan bool, 1)
	go func() {
		result := future.Sync()
		fmt.Printf("Sync completed! Result: %T\n", result)
		done <- true
	}()

	// Wait for either completion or timeout
	select {
	case <-done:
		fmt.Println("SUCCESS: Manual future completion and Sync() worked!")
	case <-time.After(5 * time.Second):
		fmt.Println("TIMEOUT: Manual future completion failed - Sync() still hangs")
		t.Fatal("Manual future completion test timed out")
	}

	fmt.Println("=== Manual future completion test completed ===")
}

// =============================================================================
// Resource Leak Detection Tests (from resource_leak_detection_test.go)
// =============================================================================

type cleanupLifecycleHandler struct {
	channel.DefaultHandler
	unregistered atomic.Int64
	removed      atomic.Int64
}

func (h *cleanupLifecycleHandler) Removed(ctx channel.HandlerContext) {
	h.removed.Add(1)
}

func (h *cleanupLifecycleHandler) Unregistered(ctx channel.HandlerContext) {
	h.unregistered.Add(1)
	ctx.FireUnregistered()
}

// Test concurrent resource cleanup and memory leak detection for channels
func TestResourceLeakDetection_ChannelCleanup(t *testing.T) {
	const numGoroutines = 100
	const channelsPerGoroutine = 50
	const cleanupCycles = 5

	var allocatedChannels, cleanedChannels atomic.Int64
	var m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)

	for range cleanupCycles {
		var wg sync.WaitGroup
		for i := range numGoroutines {
			wg.Go(func() {
				channels := make([]channel.Channel, channelsPerGoroutine)
				observers := make([]*cleanupLifecycleHandler, channelsPerGoroutine)
				for j := range channelsPerGoroutine {
					observer := &cleanupLifecycleHandler{}
					ch := &channel.DefaultChannel{}
					ch.Init()
					ch.Pipeline().AddLast("lifecycle", observer)
					channels[j], observers[j] = ch, observer
					allocatedChannels.Add(1)
					assert.False(t, ch.CloseFuture().IsDone())
					assert.Zero(t, observer.unregistered.Load())
					assert.False(t, ch.IsActive())
					ch.Pipeline().AddLast(fmt.Sprintf("leak-handler-%d-%d", i, j), &channel.DefaultHandler{})
				}

				for j, ch := range channels {
					future := ch.Deregister().Sync()
					if assert.True(t, future.IsSuccess()) && assert.Equal(t, int64(1), observers[j].unregistered.Load()) && assert.False(t, ch.IsActive()) {
						cleanedChannels.Add(1)
					}

					// Never-active channels deregister without completing the active-lifetime CloseFuture.
					assert.False(t, ch.CloseFuture().IsDone())
					ch.Pipeline().Clear()
					assert.Equal(t, int64(1), observers[j].removed.Load())
				}
				clear(channels)
				clear(observers)
			})
		}
		wg.Wait()
		runtime.GC()
	}

	runtime.GC()
	runtime.ReadMemStats(&m2)
	assert.Equal(t, int64(numGoroutines*channelsPerGoroutine*cleanupCycles), allocatedChannels.Load())
	assert.Equal(t, allocatedChannels.Load(), cleanedChannels.Load(), "All allocated channels should be cleaned up")
	memoryGrowth := int64(m2.Alloc) - int64(m1.Alloc)
	t.Logf("Channel cleanup: allocated=%d cleaned=%d memory_growth=%d KB GC_cycles=%d", allocatedChannels.Load(), cleanedChannels.Load(), memoryGrowth/1024, m2.NumGC-m1.NumGC)
	maxExpectedGrowth := int64(10 * 1024 * 1024)
	assert.Less(t, memoryGrowth, maxExpectedGrowth, "Memory growth should be within reasonable bounds")
}

// Test concurrent resource cleanup for TCP channels and servers
func TestResourceLeakDetection_TCPCleanup(t *testing.T) {
	const numGoroutines = 50
	const resourcesPerGoroutine = 30
	const cleanupCycles = 3

	var allocatedResources, cleanedResources atomic.Int64
	var m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)
	for range cleanupCycles {
		var wg sync.WaitGroup
		for range numGoroutines {
			wg.Go(func() {
				tcpChannels := make([]channel.Channel, resourcesPerGoroutine/2)
				observers := make([]*cleanupLifecycleHandler, resourcesPerGoroutine/2)
				serverChannels := make([]*gtcp.ServerChannel, resourcesPerGoroutine/2)
				for j := range resourcesPerGoroutine / 2 {
					observer := &cleanupLifecycleHandler{}
					ch := &gtcp.Channel{}
					ch.DefaultChannel.Init()
					ch.Pipeline().(channel.PipelineSetChannel).SetChannel(ch)
					ch.Init()
					ch.Pipeline().AddLast("lifecycle", observer)
					tcpChannels[j], observers[j] = ch, observer
					allocatedResources.Add(1)
					assert.False(t, ch.CloseFuture().IsDone())
					assert.False(t, ch.IsActive())
				}

				for j := range resourcesPerGoroutine / 2 {
					server := &gtcp.ServerChannel{}
					server.Init()
					server.Pipeline().(channel.PipelineSetChannel).SetChannel(server)
					serverChannels[j] = server
					allocatedResources.Add(1)
					assert.False(t, server.IsActive())
				}

				for j, ch := range tcpChannels {
					future := ch.Deregister().Sync()
					if assert.True(t, future.IsSuccess()) && assert.Equal(t, int64(1), observers[j].unregistered.Load()) && assert.False(t, ch.IsActive()) {
						cleanedResources.Add(1)
					}
					assert.False(t, ch.CloseFuture().IsDone())
					ch.Pipeline().Clear()
					assert.Equal(t, int64(1), observers[j].removed.Load())
				}

				for _, server := range serverChannels {
					future := server.Close().Sync()
					if assert.True(t, future.IsSuccess()) && assert.True(t, server.CloseFuture().IsSuccess()) && assert.False(t, server.IsActive()) {
						cleanedResources.Add(1)
					}
				}
				clear(tcpChannels)
				clear(observers)
				clear(serverChannels)
			})
		}
		wg.Wait()
		runtime.GC()
	}

	runtime.GC()
	runtime.ReadMemStats(&m2)
	assert.Equal(t, int64(numGoroutines*resourcesPerGoroutine*cleanupCycles), allocatedResources.Load())
	assert.Equal(t, allocatedResources.Load(), cleanedResources.Load(), "All allocated TCP resources should be cleaned up")
	t.Logf("TCP cleanup: allocated=%d cleaned=%d memory_growth=%d KB GC_cycles=%d", allocatedResources.Load(), cleanedResources.Load(), (int64(m2.Alloc)-int64(m1.Alloc))/1024, m2.NumGC-m1.NumGC)
}

// Test goroutine leak detection
func TestResourceLeakDetection_GoroutineLeaks(t *testing.T) {
	initialGoroutines := runtime.NumGoroutine()
	const numOperations = 100
	var wg sync.WaitGroup
	for range numOperations {
		wg.Go(func() {
			ch := &channel.DefaultChannel{}
			ch.Init()
			assert.False(t, ch.IsActive())
			future := ch.Pipeline().NewFuture()
			var completer sync.WaitGroup
			completer.Go(func() {
				assert.True(t, future.Completable().Complete("goroutine-test"))
			})
			future.Await()
			completer.Wait()
			assert.True(t, future.IsSuccess())
			assert.Equal(t, "goroutine-test", future.GetNow())
			assert.True(t, ch.Deregister().Sync().IsSuccess())
		})
	}
	wg.Wait()
	runtime.GC()
	goroutineGrowth := runtime.NumGoroutine() - initialGoroutines
	t.Logf("Goroutine growth after joined operations: %d", goroutineGrowth)
	assert.Less(t, goroutineGrowth, 10, "Goroutine growth should be minimal (< 10 goroutines)")
}

// =============================================================================
// Thread Safety Tests (from thread_safety_test.go)
// =============================================================================

// Test thread safety of shared data structures across all packages
func TestThreadSafety_SharedDataStructures(t *testing.T) {
	const numGoroutines = 200
	const operationsPerGoroutine = 100

	var channelOperations, tcpOperations, wsOperations, queueOperations atomic.Int64
	var wg sync.WaitGroup
	ch := &channel.DefaultChannel{}
	ch.Init()
	tcpCh := &gtcp.Channel{}
	tcpCh.Init()
	server := &gtcp.ServerChannel{}
	server.Init()
	wsCh := &gws.Channel{}
	wsCh.BootstrapPreInit()
	wsCh.Init()
	msg := &gws.DefaultMessage{MessageType: gws.TextMessageType, Message: []byte("thread safety test")}
	queue := &utils.Queue{}
	seen := make([]atomic.Bool, numGoroutines*operationsPerGoroutine)
	for i := range numGoroutines {
		wg.Go(func() {
			for j := range operationsPerGoroutine {
				switch j % 4 {
				case 0:
					assert.False(t, ch.IsActive())
					channelOperations.Add(1)
				case 1:
					assert.False(t, tcpCh.IsActive())
					assert.False(t, server.IsActive())
					tcpOperations.Add(1)
				case 2:
					assert.False(t, wsCh.IsActive())
					assert.Equal(t, []byte("thread safety test"), msg.Encoded())
					assert.Equal(t, gws.TextMessageType, msg.Type())
					wsOperations.Add(1)
				case 3:
					queue.Push(i*operationsPerGoroutine + j)
					value, ok := queue.Pop().(int)
					if assert.True(t, ok, "each push must supply a queued identity") && assert.GreaterOrEqual(t, value, 0) && assert.Less(t, value, len(seen)) {
						assert.Equal(t, 3, value%4)
						assert.True(t, seen[value].CompareAndSwap(false, true), "queue returned an identity twice: %d", value)
					}
					queueOperations.Add(1)
				}
			}
		})
	}
	wg.Wait()
	for i := range numGoroutines {
		for j := 3; j < operationsPerGoroutine; j += 4 {
			assert.True(t, seen[i*operationsPerGoroutine+j].Load(), "queue lost identity %d", i*operationsPerGoroutine+j)
		}
	}
	assert.Zero(t, queue.Size())
	totalOperations := channelOperations.Load() + tcpOperations.Load() + wsOperations.Load() + queueOperations.Load()
	assert.Equal(t, int64(numGoroutines*operationsPerGoroutine), totalOperations, "All thread safety operations should complete")
	t.Logf("Shared operations: channel=%d tcp=%d ws=%d queue=%d total=%d", channelOperations.Load(), tcpOperations.Load(), wsOperations.Load(), queueOperations.Load(), totalOperations)
}

// Test thread safety of Future operations
func TestThreadSafety_FutureOperations(t *testing.T) {
	const numGoroutines = 50
	const futuresPerGoroutine = 10

	var completions, cancellations, awaits atomic.Int64
	var wg sync.WaitGroup
	ch := &channel.DefaultChannel{}
	ch.Init()
	for range numGoroutines {
		wg.Go(func() {
			for j := range futuresPerGoroutine {
				future := ch.Pipeline().NewFuture()
				if !assert.NotNil(t, future) {
					return
				}
				switch j % 3 {
				case 0:
					assert.True(t, future.Completable().Complete("completed"))
					future.Await()
					assert.True(t, future.IsDone())
					assert.True(t, future.IsSuccess())
					assert.False(t, future.IsCancelled())
					assert.NoError(t, future.Error())
					assert.Equal(t, "completed", future.GetNow())
					assert.Same(t, ch, future.Channel())
					assert.False(t, future.Completable().Cancel())
					completions.Add(1)
				case 1:
					assert.True(t, future.Completable().Cancel())
					future.Await()
					assert.True(t, future.IsDone())
					assert.True(t, future.IsCancelled())
					assert.False(t, future.IsSuccess())
					assert.Nil(t, future.GetNow())
					assert.Nil(t, future.Channel())
					assert.False(t, future.Completable().Complete("late"))
					cancellations.Add(1)
				case 2:
					assert.True(t, future.Completable().Complete("immediate"))
					future.Await()
					assert.True(t, future.IsDone())
					assert.True(t, future.IsSuccess())
					assert.False(t, future.IsCancelled())
					assert.NoError(t, future.Error())
					assert.Equal(t, "immediate", future.GetNow())
					assert.Same(t, ch, future.Channel())
					assert.False(t, future.Completable().Complete("late"))
					awaits.Add(1)
				}
			}
		})
	}
	wg.Wait()
	assert.Equal(t, int64(200), completions.Load())
	assert.Equal(t, int64(150), cancellations.Load())
	assert.Equal(t, int64(150), awaits.Load())
	totalOperations := completions.Load() + cancellations.Load() + awaits.Load()
	assert.Equal(t, int64(numGoroutines*futuresPerGoroutine), totalOperations, "All future operations should complete")
}

// Test thread safety of WebSocket message encoding/decoding
func TestThreadSafety_WebSocketMessageProcessing(t *testing.T) {
	const numGoroutines = 150
	const messagesPerGoroutine = 40

	var textMessages, binaryMessages, closeMessages, parseOperations atomic.Int64
	var wg sync.WaitGroup

	for range numGoroutines {
		wg.Go(func() {

			for j := range messagesPerGoroutine {
				switch j % 4 {
				case 0:
					msg := &gws.DefaultMessage{
						MessageType: gws.TextMessageType,
						Message:     []byte("concurrent text"),
					}
					assert.Equal(t, []byte("concurrent text"), msg.Encoded())
					assert.Equal(t, gws.TextMessageType, msg.Type())
					textMessages.Add(1)

				case 1:
					msg := &gws.DefaultMessage{
						MessageType: gws.BinaryMessageType,
						Message:     []byte{0x01, 0x02, 0x03},
					}
					assert.Equal(t, []byte{0x01, 0x02, 0x03}, msg.Encoded())
					assert.Equal(t, gws.BinaryMessageType, msg.Type())
					binaryMessages.Add(1)

				case 2:
					msg := &gws.CloseMessage{
						MessageType: gws.CloseMessageType,
						Message:     []byte("close reason"),
						CloseCode:   gws.CloseNormalClosure,
					}
					assert.Equal(t, append([]byte{0x03, 0xE8}, []byte("close reason")...), msg.Encoded())
					assert.Equal(t, gws.CloseMessageType, msg.Type())
					closeMessages.Add(1)

				case 3:
					msg := &gws.PingMessage{
						MessageType: gws.PingMessageType,
						Message:     []byte("ping data"),
					}
					assert.Equal(t, []byte("ping data"), msg.Encoded())
					assert.Equal(t, gws.PingMessageType, msg.Type())
					parseOperations.Add(1)
				}
			}
		})
	}

	wg.Wait()

	totalOperations := textMessages.Load() + binaryMessages.Load() +
		closeMessages.Load() + parseOperations.Load()
	expectedTotal := int64(numGoroutines * messagesPerGoroutine)

	assert.Equal(t, expectedTotal, totalOperations, "All WebSocket message operations should complete")

	t.Logf("WebSocket message thread safety: %d text, %d binary, %d close, %d parse out of %d total",
		textMessages.Load(), binaryMessages.Load(), closeMessages.Load(), parseOperations.Load(), totalOperations)
}

// =============================================================================
// Benchmark Tests
// =============================================================================

// Benchmark deadlock prevention performance
func BenchmarkDeadlockPrevention_Performance(b *testing.B) {
	const numGoroutines = 50

	b.ResetTimer()

	for b.Loop() {
		var wg sync.WaitGroup

		for range numGoroutines {
			wg.Go(func() {

				// Simulate deadlock prevention scenarios
				ch := &channel.DefaultChannel{}
				ch.Init()

				future := ch.Pipeline().NewFuture()
				if future != nil {
					go func() {
						completable := future.Completable()
						if completable != nil {
							completable.Complete("benchmark")
						}
					}()

					future.Await()
				}
				ch.IsActive()
			})
		}

		wg.Wait()
	}
}

// Benchmark resource cleanup performance under concurrent load
func BenchmarkResourceCleanup_ConcurrentLoad(b *testing.B) {
	const numGoroutines = 100
	const resourcesPerIteration = 50

	b.ResetTimer()

	for b.Loop() {
		var wg sync.WaitGroup

		for range numGoroutines {
			wg.Go(func() {

				// Create and cleanup resources
				for j := range resourcesPerIteration {
					switch j % 3 {
					case 0:
						ch := &channel.DefaultChannel{}
						ch.Init()
						ch.IsActive()
						ch = nil
					case 1:
						tcp := &gtcp.Channel{}
						tcp.Init()
						tcp.IsActive()
						tcp = nil
					case 2:
						ws := &gws.Channel{}
						ws.BootstrapPreInit()
						ws.Init()
						ws.IsActive()
						ws = nil
					}
				}
			})
		}

		wg.Wait()
	}
}

// Benchmark thread safety performance under concurrent load
func BenchmarkThreadSafety_ConcurrentOperations(b *testing.B) {
	const numGoroutines = 100

	b.ResetTimer()

	for b.Loop() {
		var wg sync.WaitGroup

		for range numGoroutines {
			wg.Go(func() {

				// Channel operations
				ch := &channel.DefaultChannel{}
				ch.Init()
				ch.IsActive()

				// TCP operations
				tcpCh := &gtcp.Channel{}
				tcpCh.Init()
				tcpCh.IsActive()

				// WebSocket operations
				msg := &gws.DefaultMessage{
					MessageType: gws.TextMessageType,
					Message:     []byte("benchmark"),
				}
				msg.Encoded()
				msg.Type()
			})
		}

		wg.Wait()
	}
}
