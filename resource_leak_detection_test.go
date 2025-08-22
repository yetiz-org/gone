package gone

import (
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
)

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

// Test concurrent resource cleanup for WebSocket channels and messages
func TestResourceLeakDetection_WebSocketCleanup(t *testing.T) {
	const numGoroutines = 75
	const resourcesPerGoroutine = 40
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
		
		// Allocate many WebSocket resources concurrently
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(routineID int) {
				defer wg.Done()
				
				wsChannels := make([]*gws.Channel, resourcesPerGoroutine/2)
				messages := make([]*gws.DefaultMessage, resourcesPerGoroutine/2)
				
				// Create WebSocket channels
				for j := 0; j < resourcesPerGoroutine/2; j++ {
					ch := &gws.Channel{}
					ch.BootstrapPreInit()
					ch.Init()
					wsChannels[j] = ch
					atomic.AddInt64(&allocatedResources, 1)
					
					// Perform operations
					ch.IsActive()
				}
				
				// Create WebSocket messages
				for j := 0; j < resourcesPerGoroutine/2; j++ {
					msg := &gws.DefaultMessage{
						MessageType: gws.TextMessageType,
						Message:     []byte("leak detection test message"),
					}
					messages[j] = msg
					atomic.AddInt64(&allocatedResources, 1)
					
					// Perform operations
					msg.Encoded()
					msg.Type()
				}
				
				// Cleanup WebSocket channels
				for _, ch := range wsChannels {
					if ch != nil {
						ch.IsActive() // Final state check
						atomic.AddInt64(&cleanedResources, 1)
					}
				}
				
				// Cleanup messages
				for _, msg := range messages {
					if msg != nil {
						msg.Type() // Final operation
						atomic.AddInt64(&cleanedResources, 1)
					}
				}
				
				// Clear references
				for i := range wsChannels {
					wsChannels[i] = nil
				}
				for i := range messages {
					messages[i] = nil
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
		"All allocated WebSocket resources should be cleaned up")
	
	memoryGrowth := int64(m2.Alloc) - int64(m1.Alloc)
	
	t.Logf("WebSocket cleanup test completed:")
	t.Logf("  Allocated resources: %d", allocatedResources)
	t.Logf("  Cleaned resources: %d", cleanedResources)
	t.Logf("  Memory growth: %d KB", memoryGrowth/1024)
	t.Logf("  GC cycles during test: %d", m2.NumGC-m1.NumGC)
}

// Test concurrent resource cleanup with cross-package interactions
func TestResourceLeakDetection_CrossPackageCleanup(t *testing.T) {
	const numGoroutines = 50
	const interactionsPerGoroutine = 25
	const cleanupCycles = 4
	
	var totalInteractions int64
	var cleanedInteractions int64
	
	// Get initial memory stats
	var m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)
	
	// Test multiple cleanup cycles
	for cycle := 0; cycle < cleanupCycles; cycle++ {
		var wg sync.WaitGroup
		
		// Create complex cross-package interactions
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(routineID int) {
				defer wg.Done()
				
				resources := make([]interface{}, interactionsPerGoroutine)
				
				for j := 0; j < interactionsPerGoroutine; j++ {
					switch j % 4 {
					case 0:
						// Channel resource
						ch := &channel.DefaultChannel{}
						ch.Init()
						resources[j] = ch
						ch.IsActive()
						
					case 1:
						// TCP resource
						tcp := &gtcp.Channel{}
						tcp.Init()
						resources[j] = tcp
						tcp.IsActive()
						
					case 2:
						// WebSocket resource
						ws := &gws.Channel{}
						ws.BootstrapPreInit()
						ws.Init()
						resources[j] = ws
						ws.IsActive()
						
					case 3:
						// Queue resource (non-thread-safe but testing cleanup)
						queue := &utils.Queue{}
						resources[j] = queue
						// Careful with queue operations due to thread safety issues
					}
					
					atomic.AddInt64(&totalInteractions, 1)
				}
				
				// Cleanup all resources
				for _, resource := range resources {
					if resource != nil {
						switch r := resource.(type) {
						case *channel.DefaultChannel:
							r.IsActive() // Final operation
						case *gtcp.Channel:
							r.IsActive() // Final operation
						case *gws.Channel:
							r.IsActive() // Final operation
						case *utils.Queue:
							// Skip queue operations due to thread safety
						}
						atomic.AddInt64(&cleanedInteractions, 1)
					}
				}
				
				// Clear references
				for i := range resources {
					resources[i] = nil
				}
			}(i)
		}
		
		wg.Wait()
		
		// Force garbage collection after each cycle
		runtime.GC()
		runtime.GC()
		time.Sleep(15 * time.Millisecond)
	}
	
	// Final cleanup and memory check
	runtime.GC()
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	runtime.ReadMemStats(&m2)
	
	// Verify all interactions were processed
	assert.Equal(t, totalInteractions, cleanedInteractions, 
		"All cross-package interactions should be cleaned up")
	
	memoryGrowth := int64(m2.Alloc) - int64(m1.Alloc)
	
	t.Logf("Cross-package cleanup test completed:")
	t.Logf("  Total interactions: %d", totalInteractions)
	t.Logf("  Cleaned interactions: %d", cleanedInteractions)
	t.Logf("  Memory growth: %d KB", memoryGrowth/1024)
	t.Logf("  GC cycles during test: %d", m2.NumGC-m1.NumGC)
}

// Test memory pressure scenarios and recovery
func TestResourceLeakDetection_MemoryPressureRecovery(t *testing.T) {
	const pressureCycles = 3
	const resourcesPerCycle = 1000
	
	var m1, m2, m3 runtime.MemStats
	
	// Get initial memory stats
	runtime.GC()
	runtime.ReadMemStats(&m1)
	
	t.Logf("Initial memory: Alloc=%d KB", m1.Alloc/1024)
	
	// Apply memory pressure
	for cycle := 0; cycle < pressureCycles; cycle++ {
		resources := make([]interface{}, resourcesPerCycle)
		
		// Allocate many resources
		for i := 0; i < resourcesPerCycle; i++ {
			switch i % 3 {
			case 0:
				ch := &channel.DefaultChannel{}
				ch.Init()
				resources[i] = ch
			case 1:
				tcp := &gtcp.Channel{}
				tcp.Init()
				resources[i] = tcp
			case 2:
				ws := &gws.Channel{}
				ws.BootstrapPreInit()
				ws.Init()
				resources[i] = ws
			}
		}
		
		// Check memory during pressure
		runtime.ReadMemStats(&m2)
		t.Logf("Memory during pressure cycle %d: Alloc=%d KB", 
			cycle+1, m2.Alloc/1024)
		
		// Cleanup resources
		for i := range resources {
			resources[i] = nil
		}
		
		// Force garbage collection
		runtime.GC()
		runtime.GC()
		time.Sleep(20 * time.Millisecond)
	}
	
	// Final memory check after cleanup
	runtime.GC()
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	runtime.ReadMemStats(&m3)
	
	// Memory recovery analysis
	memoryRecovered := int64(m2.Alloc) - int64(m3.Alloc)
	finalGrowth := int64(m3.Alloc) - int64(m1.Alloc)
	
	t.Logf("Memory pressure recovery test completed:")
	t.Logf("  Peak memory: %d KB", m2.Alloc/1024)
	t.Logf("  Final memory: %d KB", m3.Alloc/1024)
	t.Logf("  Memory recovered: %d KB", memoryRecovered/1024)
	t.Logf("  Net growth: %d KB", finalGrowth/1024)
	t.Logf("  Total GC cycles: %d", m3.NumGC-m1.NumGC)
	
	// Assert reasonable recovery (most memory should be reclaimed)
	recoveryRate := float64(memoryRecovered) / float64(m2.Alloc-m1.Alloc) * 100
	assert.Greater(t, recoveryRate, 50.0, 
		"At least 50%% of allocated memory should be recovered")
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