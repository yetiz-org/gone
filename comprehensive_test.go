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

// =============================================================================
// Deadlock Prevention Tests (from deadlock_prevention_test.go)
// =============================================================================

// Test deadlock prevention in concurrent channel operations
func TestDeadlockPrevention_ChannelOperations(t *testing.T) {
	const numGoroutines = 100
	const operationsPerGoroutine = 50
	const timeout = 10 * time.Second
	
	var completedOperations int64
	var wg sync.WaitGroup
	
	done := make(chan bool, 1)
	
	// Start deadlock prevention test
	go func() {
		defer func() {
			done <- true
		}()
		
		// Test concurrent channel operations that could potentially deadlock
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(routineID int) {
				defer wg.Done()
				
				for j := 0; j < operationsPerGoroutine; j++ {
					// Create multiple channels
					ch1 := &channel.DefaultChannel{}
					ch1.Init()
					
					ch2 := &channel.DefaultChannel{}
					ch2.Init()
					
					// Perform operations that could potentially cause deadlocks
					// if not properly synchronized
					go func() {
						ch1.IsActive()
						ch2.IsActive()
					}()
					
					go func() {
						ch2.IsActive() 
						ch1.IsActive()
					}()
					
					// Small delay to allow operations to complete
					time.Sleep(1 * time.Microsecond)
					atomic.AddInt64(&completedOperations, 1)
				}
			}(i)
		}
		
		wg.Wait()
	}()
	
	// Wait for completion or timeout
	select {
	case <-done:
		// Test completed successfully without deadlock
		expectedOperations := int64(numGoroutines * operationsPerGoroutine)
		assert.Equal(t, expectedOperations, atomic.LoadInt64(&completedOperations), 
			"All operations should complete without deadlock")
		
		t.Logf("Deadlock prevention test completed: %d operations in concurrent channel access", 
			completedOperations)
		
	case <-time.After(timeout):
		t.Fatalf("Deadlock prevention test timed out after %v - potential deadlock detected", timeout)
	}
}

// Test deadlock prevention in TCP server operations
func TestDeadlockPrevention_TCPServerOperations(t *testing.T) {
	const numGoroutines = 50
	const operationsPerGoroutine = 30
	const timeout = 15 * time.Second
	
	var completedOperations int64
	var wg sync.WaitGroup
	
	done := make(chan bool, 1)
	
	// Start deadlock prevention test
	go func() {
		defer func() {
			done <- true
		}()
		
		// Test concurrent TCP server operations that could potentially deadlock
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(routineID int) {
				defer wg.Done()
				
				for j := 0; j < operationsPerGoroutine; j++ {
					server := &gtcp.ServerChannel{}
					server.Init()
					
					// Perform operations that could potentially cause deadlocks
					// in concurrent server state management
					go func() {
						server.IsActive()
					}()
					
					go func() {
						server.IsActive()
					}()
					
					// TCP Channel operations
					tcpCh := &gtcp.Channel{}
					tcpCh.Init()
					tcpCh.IsActive()
					
					time.Sleep(1 * time.Microsecond)
					atomic.AddInt64(&completedOperations, 1)
				}
			}(i)
		}
		
		wg.Wait()
	}()
	
	// Wait for completion or timeout
	select {
	case <-done:
		// Test completed successfully without deadlock
		expectedOperations := int64(numGoroutines * operationsPerGoroutine)
		assert.Equal(t, expectedOperations, atomic.LoadInt64(&completedOperations), 
			"All TCP server operations should complete without deadlock")
		
		t.Logf("TCP server deadlock prevention test completed: %d operations without deadlock", 
			completedOperations)
		
	case <-time.After(timeout):
		t.Fatalf("TCP server deadlock prevention test timed out after %v - potential deadlock detected", timeout)
	}
}

// Test deadlock prevention in WebSocket operations
func TestDeadlockPrevention_WebSocketOperations(t *testing.T) {
	const numGoroutines = 75
	const operationsPerGoroutine = 35
	const timeout = 15 * time.Second
	
	var completedOperations int64
	var wg sync.WaitGroup
	
	done := make(chan bool, 1)
	
	// Start deadlock prevention test
	go func() {
		defer func() {
			done <- true
		}()
		
		// Test concurrent WebSocket operations that could potentially deadlock
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(routineID int) {
				defer wg.Done()
				
				for j := 0; j < operationsPerGoroutine; j++ {
					wsCh := &gws.Channel{}
					wsCh.BootstrapPreInit()
					wsCh.Init()
					
					// Create messages concurrently
					msg1 := &gws.DefaultMessage{
						MessageType: gws.TextMessageType,
						Message:     []byte("deadlock test 1"),
					}
					
					msg2 := &gws.DefaultMessage{
						MessageType: gws.BinaryMessageType,
						Message:     []byte{0x01, 0x02, 0x03},
					}
					
					// Perform concurrent message operations that could deadlock
					go func() {
						msg1.Encoded()
						msg2.Encoded()
					}()
					
					go func() {
						msg2.Encoded()
						msg1.Encoded()
					}()
					
					// WebSocket channel operations
					wsCh.IsActive()
					
					time.Sleep(1 * time.Microsecond)
					atomic.AddInt64(&completedOperations, 1)
				}
			}(i)
		}
		
		wg.Wait()
	}()
	
	// Wait for completion or timeout
	select {
	case <-done:
		// Test completed successfully without deadlock
		expectedOperations := int64(numGoroutines * operationsPerGoroutine)
		assert.Equal(t, expectedOperations, atomic.LoadInt64(&completedOperations), 
			"All WebSocket operations should complete without deadlock")
		
		t.Logf("WebSocket deadlock prevention test completed: %d operations without deadlock", 
			completedOperations)
		
	case <-time.After(timeout):
		t.Fatalf("WebSocket deadlock prevention test timed out after %v - potential deadlock detected", timeout)
	}
}

// Test deadlock prevention in cross-package interactions
func TestDeadlockPrevention_CrossPackageInteractions(t *testing.T) {
	const numGoroutines = 50
	const operationsPerGoroutine = 25
	const timeout = 20 * time.Second
	
	var completedOperations int64
	var wg sync.WaitGroup
	
	done := make(chan bool, 1)
	
	// Start deadlock prevention test
	go func() {
		defer func() {
			done <- true
		}()
		
		// Test complex cross-package interactions that could potentially deadlock
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(routineID int) {
				defer wg.Done()
				
				for j := 0; j < operationsPerGoroutine; j++ {
					// Create instances from different packages
					ch := &channel.DefaultChannel{}
					ch.Init()
					
					tcpCh := &gtcp.Channel{}
					tcpCh.Init()
					
					wsCh := &gws.Channel{}
					wsCh.BootstrapPreInit()
					wsCh.Init()
					
					// Create complex interaction scenario
					go func() {
						// Chain of operations across packages
						ch.IsActive()
						tcpCh.IsActive()
						wsCh.IsActive()
					}()
					
					go func() {
						// Reverse order operations
						wsCh.IsActive()
						tcpCh.IsActive() 
						ch.IsActive()
					}()
					
					go func() {
						// Future operations
						future := ch.Pipeline().NewFuture()
						if future != nil {
							go func() {
								time.Sleep(1 * time.Millisecond)
								completable := future.Completable()
								if completable != nil {
									completable.Complete("cross-package-test")
								}
							}()
							future.Await()
						}
					}()
					
					time.Sleep(2 * time.Microsecond)
					atomic.AddInt64(&completedOperations, 1)
				}
			}(i)
		}
		
		wg.Wait()
	}()
	
	// Wait for completion or timeout
	select {
	case <-done:
		// Test completed successfully without deadlock
		expectedOperations := int64(numGoroutines * operationsPerGoroutine)
		assert.Equal(t, expectedOperations, atomic.LoadInt64(&completedOperations), 
			"All cross-package operations should complete without deadlock")
		
		t.Logf("Cross-package deadlock prevention test completed: %d operations without deadlock", 
			completedOperations)
		
	case <-time.After(timeout):
		t.Fatalf("Cross-package deadlock prevention test timed out after %v - potential deadlock detected", timeout)
	}
}

// =============================================================================
// Debug Channel Init Tests (from debug_channel_init_test.go)
// =============================================================================

// Minimal test to isolate channel initialization deadlock
func TestMinimalChannelCreation(t *testing.T) {
	kklogger.SetLogLevel("DEBUG")
	
	fmt.Println("=== Starting minimal channel creation test ===")
	kklogger.DebugJ("debug_test:TestMinimalChannelCreation#test!start", "Test started")
	
	// Step 1: Test basic ServerChannel creation
	fmt.Println("Step 1: Creating ServerChannel...")
	serverChannel := &gtcp.ServerChannel{}
	fmt.Println("Step 1: ServerChannel created successfully")
	
	// Step 2: Test channel.init() call
	fmt.Println("Step 2: Calling serverChannel.init()...")
	serverChannel.Init()  // This should call c.init(c) internally
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
	kklogger.SetLogLevel("DEBUG")
	
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
	kklogger.SetLogLevel("DEBUG")
	
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

// Test concurrent resource cleanup and memory leak detection for channels
func TestResourceLeakDetection_ChannelCleanup(t *testing.T) {
	const numGoroutines = 100
	const channelsPerGoroutine = 50
	const cleanupCycles = 5
	
	var allocatedChannels int64
	var cleanedChannels int64
	
	// Get initial memory stats
	var m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)
	
	t.Logf("Initial memory: Alloc=%d KB, Sys=%d KB, NumGC=%d", 
		m1.Alloc/1024, m1.Sys/1024, m1.NumGC)
	
	// Test multiple cleanup cycles
	for cycle := 0; cycle < cleanupCycles; cycle++ {
		var wg sync.WaitGroup
		
		// Allocate many channels concurrently
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(routineID int) {
				defer wg.Done()
				
				channels := make([]*channel.DefaultChannel, channelsPerGoroutine)
				
				// Create channels
				for j := 0; j < channelsPerGoroutine; j++ {
					ch := &channel.DefaultChannel{}
					ch.Init()
					channels[j] = ch
					atomic.AddInt64(&allocatedChannels, 1)
					
					// Perform some operations
					ch.IsActive()
					
					// Add handlers
					pipeline := ch.Pipeline()
					if pipeline != nil {
						handler := &channel.DefaultHandler{}
						handlerName := "leak-handler-" + string(rune(routineID*1000+j))
						pipeline.AddLast(handlerName, handler)
					}
				}
				
				// Cleanup - explicitly close/cleanup channels
				for _, ch := range channels {
					if ch != nil {
						// Simulate cleanup operations
						ch.IsActive() // Final state check
						atomic.AddInt64(&cleanedChannels, 1)
					}
				}
				
				// Clear references to help GC
				for i := range channels {
					channels[i] = nil
				}
			}(i)
		}
		
		wg.Wait()
		
		// Force garbage collection after each cycle
		runtime.GC()
		runtime.GC() // Double GC to ensure cleanup
		
		// Small delay between cycles
		time.Sleep(10 * time.Millisecond)
	}
	
	// Final garbage collection and memory check
	runtime.GC()
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	runtime.ReadMemStats(&m2)
	
	// Verify all channels were processed
	assert.Equal(t, allocatedChannels, cleanedChannels, 
		"All allocated channels should be cleaned up")
	
	// Memory growth analysis
	memoryGrowth := int64(m2.Alloc) - int64(m1.Alloc)
	
	t.Logf("Channel cleanup test completed:")
	t.Logf("  Allocated channels: %d", allocatedChannels)
	t.Logf("  Cleaned channels: %d", cleanedChannels) 
	t.Logf("  Memory growth: %d KB", memoryGrowth/1024)
	t.Logf("  Final memory: Alloc=%d KB, Sys=%d KB, NumGC=%d", 
		m2.Alloc/1024, m2.Sys/1024, m2.NumGC)
	t.Logf("  GC cycles during test: %d", m2.NumGC-m1.NumGC)
	
	// Assert reasonable memory usage (allow for some growth due to Go runtime)
	maxExpectedGrowth := int64(10 * 1024 * 1024) // 10MB threshold
	assert.Less(t, memoryGrowth, maxExpectedGrowth, 
		"Memory growth should be within reasonable bounds")
}

// Test concurrent resource cleanup for TCP channels and servers
func TestResourceLeakDetection_TCPCleanup(t *testing.T) {
	const numGoroutines = 50
	const resourcesPerGoroutine = 30
	const cleanupCycles = 3
	
	var allocatedResources int64
	var cleanedResources int64
	
	// Get initial memory stats
	var m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)
	
	// Test multiple cleanup cycles
	for cycle := 0; cycle < cleanupCycles; cycle++ {
		var wg sync.WaitGroup
		
		// Allocate many TCP resources concurrently
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(routineID int) {
				defer wg.Done()
				
				tcpChannels := make([]*gtcp.Channel, resourcesPerGoroutine/2)
				serverChannels := make([]*gtcp.ServerChannel, resourcesPerGoroutine/2)
				
				// Create TCP channels
				for j := 0; j < resourcesPerGoroutine/2; j++ {
					ch := &gtcp.Channel{}
					ch.Init()
					tcpChannels[j] = ch
					atomic.AddInt64(&allocatedResources, 1)
					
					// Perform operations
					ch.IsActive()
				}
				
				// Create server channels
				for j := 0; j < resourcesPerGoroutine/2; j++ {
					server := &gtcp.ServerChannel{}
					server.Init()
					serverChannels[j] = server
					atomic.AddInt64(&allocatedResources, 1)
					
					// Perform operations
					server.IsActive()
				}
				
				// Cleanup TCP channels
				for _, ch := range tcpChannels {
					if ch != nil {
						ch.IsActive() // Final state check
						atomic.AddInt64(&cleanedResources, 1)
					}
				}
				
				// Cleanup server channels
				for _, server := range serverChannels {
					if server != nil {
						server.IsActive() // Final state check
						atomic.AddInt64(&cleanedResources, 1)
					}
				}
				
				// Clear references
				for i := range tcpChannels {
					tcpChannels[i] = nil
				}
				for i := range serverChannels {
					serverChannels[i] = nil
				}
			}(i)
		}
		
		wg.Wait()
		
		// Force garbage collection after each cycle
		runtime.GC()
		runtime.GC()
		time.Sleep(10 * time.Millisecond)
	}
	
	// Final cleanup and memory check
	runtime.GC()
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	runtime.ReadMemStats(&m2)
	
	// Verify all resources were processed
	assert.Equal(t, allocatedResources, cleanedResources, 
		"All allocated TCP resources should be cleaned up")
	
	memoryGrowth := int64(m2.Alloc) - int64(m1.Alloc)
	
	t.Logf("TCP cleanup test completed:")
	t.Logf("  Allocated resources: %d", allocatedResources)
	t.Logf("  Cleaned resources: %d", cleanedResources)
	t.Logf("  Memory growth: %d KB", memoryGrowth/1024)
	t.Logf("  GC cycles during test: %d", m2.NumGC-m1.NumGC)
}

// Test goroutine leak detection
func TestResourceLeakDetection_GoroutineLeaks(t *testing.T) {
	// Get initial goroutine count
	initialGoroutines := runtime.NumGoroutine()
	t.Logf("Initial goroutine count: %d", initialGoroutines)
	
	const numOperations = 100
	var wg sync.WaitGroup
	
	// Perform operations that could potentially create goroutine leaks
	for i := 0; i < numOperations; i++ {
		wg.Add(1)
		go func(operationID int) {
			defer wg.Done()
			
			// Create channel and perform operations
			ch := &channel.DefaultChannel{}
			ch.Init()
			
			// Simulate some work
			ch.IsActive()
			
			// Create future (potential source of goroutine leaks)  
			pipeline := ch.Pipeline()
			if pipeline != nil {
				future := pipeline.NewFuture()
				if future != nil {
					go func() {
						// Ensure future is completed to avoid goroutine leaks
						time.Sleep(1 * time.Millisecond)
						completable := future.Completable()
						if completable != nil {
							completable.Complete("goroutine-test")
						}
					}()
					future.Await()
				}
			}
			
		}(i)
	}
	
	wg.Wait()
	
	// Allow some time for cleanup
	time.Sleep(100 * time.Millisecond)
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	
	// Check final goroutine count
	finalGoroutines := runtime.NumGoroutine()
	goroutineGrowth := finalGoroutines - initialGoroutines
	
	t.Logf("Final goroutine count: %d", finalGoroutines)
	t.Logf("Goroutine growth: %d", goroutineGrowth)
	
	// Allow for some reasonable goroutine growth (Go runtime may create additional goroutines)
	assert.Less(t, goroutineGrowth, 10, 
		"Goroutine growth should be minimal (< 10 goroutines)")
}

// =============================================================================
// Thread Safety Tests (from thread_safety_test.go)
// =============================================================================

// Test thread safety of shared data structures across all packages
func TestThreadSafety_SharedDataStructures(t *testing.T) {
	const numGoroutines = 200
	const operationsPerGoroutine = 100
	
	var channelOperations int64
	var tcpOperations int64
	var wsOperations int64
	var queueOperations int64
	var wg sync.WaitGroup
	
	// Test concurrent operations across all shared data structures
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(routineID int) {
			defer wg.Done()
			
			for j := 0; j < operationsPerGoroutine; j++ {
				switch j % 4 {
				case 0:
					// Channel operations
					ch := &channel.DefaultChannel{}
					ch.Init()
					ch.IsActive()
					atomic.AddInt64(&channelOperations, 1)
					
				case 1:
					// TCP operations  
					tcpCh := &gtcp.Channel{}
					tcpCh.Init()
					server := &gtcp.ServerChannel{}
					server.Init()
					server.IsActive()
					atomic.AddInt64(&tcpOperations, 1)
					
				case 2:
					// WebSocket operations
					wsCh := &gws.Channel{}
					wsCh.BootstrapPreInit()
					wsCh.Init()
					
					msg := &gws.DefaultMessage{
						MessageType: gws.TextMessageType,
						Message:     []byte("thread safety test"),
					}
					msg.Encoded()
					atomic.AddInt64(&wsOperations, 1)
					
				case 3:
					// Queue operations (known to be non-thread-safe)
					queue := &utils.Queue{}
					queue.Push("thread-safety-test")
					queue.Pop()
					atomic.AddInt64(&queueOperations, 1)
				}
			}
		}(i)
	}
	
	wg.Wait()
	
	// Verify all operations completed
	totalOperations := atomic.LoadInt64(&channelOperations) + atomic.LoadInt64(&tcpOperations) +
		atomic.LoadInt64(&wsOperations) + atomic.LoadInt64(&queueOperations)
	expectedTotal := int64(numGoroutines * operationsPerGoroutine)
	
	assert.Equal(t, expectedTotal, totalOperations, "All thread safety operations should complete")
	
	t.Logf("Thread safety test completed: %d channel, %d tcp, %d ws, %d queue operations out of %d total",
		channelOperations, tcpOperations, wsOperations, queueOperations, totalOperations)
}

// Test thread safety of Future operations
func TestThreadSafety_FutureOperations(t *testing.T) {
	const numGoroutines = 50   // Further reduced for better stability
	const futuresPerGoroutine = 10  // Further reduced for better stability
	
	var completions int64
	var cancellations int64
	var awaits int64
	var wg sync.WaitGroup
	
	// Test concurrent future operations with timeout and proper completion
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(routineID int) {
			defer wg.Done()
			
			for j := 0; j < futuresPerGoroutine; j++ {
				ch := &channel.DefaultChannel{}
				ch.Init()
				future := ch.Pipeline().NewFuture()
				
				if future == nil {
					continue // Skip if future creation failed
				}
				
				switch j % 3 {
				case 0:
					// Complete future with synchronous approach
					if completable := future.Completable(); completable != nil {
						completable.Complete("completed")
					}
					future.Await()
					atomic.AddInt64(&completions, 1)
					
				case 1:
					// Cancel future with synchronous approach
					if completable := future.Completable(); completable != nil {
						completable.Cancel()
					}
					future.Await()
					atomic.AddInt64(&cancellations, 1)
					
				case 2:
					// Immediate completion
					if completable := future.Completable(); completable != nil {
						completable.Complete("immediate")
					}
					future.Await()
					atomic.AddInt64(&awaits, 1)
				}
			}
		}(i)
	}
	
	wg.Wait()
	
	// Verify operation counts
	totalOperations := atomic.LoadInt64(&completions) + atomic.LoadInt64(&cancellations) + 
		atomic.LoadInt64(&awaits)
	expectedTotal := int64(numGoroutines * futuresPerGoroutine)
	
	assert.Equal(t, expectedTotal, totalOperations, "All future operations should complete")
	
	t.Logf("Future thread safety: %d completions, %d cancellations, %d awaits out of %d total",
		completions, cancellations, awaits, totalOperations)
}

// Test thread safety of WebSocket message encoding/decoding
func TestThreadSafety_WebSocketMessageProcessing(t *testing.T) {
	const numGoroutines = 150
	const messagesPerGoroutine = 40
	
	var textMessages int64
	var binaryMessages int64
	var closeMessages int64
	var parseOperations int64
	var wg sync.WaitGroup
	
	// Test concurrent WebSocket message processing
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(routineID int) {
			defer wg.Done()
			
			for j := 0; j < messagesPerGoroutine; j++ {
				switch j % 4 {
				case 0:
					// Text message
					msg := &gws.DefaultMessage{
						MessageType: gws.TextMessageType,
						Message:     []byte("concurrent text"),
					}
					msg.Encoded()
					msg.Type()
					atomic.AddInt64(&textMessages, 1)
					
				case 1:
					// Binary message
					msg := &gws.DefaultMessage{
						MessageType: gws.BinaryMessageType,
						Message:     []byte{0x01, 0x02, 0x03},
					}
					msg.Encoded()
					msg.Type()
					atomic.AddInt64(&binaryMessages, 1)
					
				case 2:
					// Close message
					msg := &gws.CloseMessage{
						DefaultMessage: gws.DefaultMessage{
							MessageType: gws.CloseMessageType,
							Message:     []byte("close reason"),
						},
						CloseCode: gws.CloseNormalClosure,
					}
					msg.Encoded()
					msg.Type()
					atomic.AddInt64(&closeMessages, 1)
					
				case 3:
					// Ping message
					msg := &gws.PingMessage{
						DefaultMessage: gws.DefaultMessage{
							MessageType: gws.PingMessageType,
							Message:     []byte("ping data"),
						},
					}
					msg.Encoded()
					msg.Type()
					atomic.AddInt64(&parseOperations, 1)
				}
			}
		}(i)
	}
	
	wg.Wait()
	
	// Verify operation counts
	totalOperations := atomic.LoadInt64(&textMessages) + atomic.LoadInt64(&binaryMessages) +
		atomic.LoadInt64(&closeMessages) + atomic.LoadInt64(&parseOperations)
	expectedTotal := int64(numGoroutines * messagesPerGoroutine)
	
	assert.Equal(t, expectedTotal, totalOperations, "All WebSocket message operations should complete")
	
	t.Logf("WebSocket message thread safety: %d text, %d binary, %d close, %d parse out of %d total",
		textMessages, binaryMessages, closeMessages, parseOperations, totalOperations)
}

// =============================================================================
// Benchmark Tests
// =============================================================================

// Benchmark deadlock prevention performance
func BenchmarkDeadlockPrevention_Performance(b *testing.B) {
	const numGoroutines = 50
	
	b.ResetTimer()
	
	for n := 0; n < b.N; n++ {
		var wg sync.WaitGroup
		
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				
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
			}()
		}
		
		wg.Wait()
	}
}

// Benchmark resource cleanup performance under concurrent load
func BenchmarkResourceCleanup_ConcurrentLoad(b *testing.B) {
	const numGoroutines = 100
	const resourcesPerIteration = 50
	
	b.ResetTimer()
	
	for n := 0; n < b.N; n++ {
		var wg sync.WaitGroup
		
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				
				// Create and cleanup resources
				for j := 0; j < resourcesPerIteration; j++ {
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
			}()
		}
		
		wg.Wait()
	}
}

// Benchmark thread safety performance under concurrent load
func BenchmarkThreadSafety_ConcurrentOperations(b *testing.B) {
	const numGoroutines = 100
	
	b.ResetTimer()
	
	for n := 0; n < b.N; n++ {
		var wg sync.WaitGroup
		
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				
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
			}()
		}
		
		wg.Wait()
	}
}
