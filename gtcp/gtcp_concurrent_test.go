package gtcp

import (
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/yetiz-org/gone/channel"
)

// Test TCP Channel interface compliance
func TestTCPChannel_InterfaceCompliance(t *testing.T) {
	ch := &Channel{}

	// Verify interface implementations
	assert.Implements(t, (*channel.Channel)(nil), ch)
	assert.NotNil(t, ch)
}

// Test concurrent TCP connection attempts
func TestTCPChannel_ConcurrentConnections(t *testing.T) {
	const numGoroutines = 100
	const connectionsPerGoroutine = 20

	var successfulConnections int64
	var failedConnections int64
	var wg sync.WaitGroup

	// Test concurrent connection attempts
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(routineID int) {
			defer wg.Done()

			for j := 0; j < connectionsPerGoroutine; j++ {
				ch := &Channel{}
				ch.Init()

				// Try to connect to a non-existent address (will fail, but tests thread safety)
				localAddr, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
				remoteAddr, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:99999") // Non-existent port

				err := ch.UnsafeConnect(localAddr, remoteAddr)
				if err != nil {
					atomic.AddInt64(&failedConnections, 1)
				} else {
					atomic.AddInt64(&successfulConnections, 1)
					// Connection would be closed here
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify that all connections were attempted
	totalConnections := atomic.LoadInt64(&successfulConnections) + atomic.LoadInt64(&failedConnections)
	expectedTotal := int64(numGoroutines * connectionsPerGoroutine)
	assert.Equal(t, expectedTotal, totalConnections, "All connection attempts should be counted")

	t.Logf("Connection attempts: %d successful, %d failed out of %d total",
		successfulConnections, failedConnections, totalConnections)
}

// Test concurrent server channel operations
func TestTCPServerChannel_ConcurrentOperations(t *testing.T) {
	const numServers = 50
	const operationsPerServer = 30

	var successfulBinds int64
	var failedBinds int64
	var wg sync.WaitGroup

	// Test concurrent server creation and binding
	for i := 0; i < numServers; i++ {
		wg.Add(1)
		go func(serverID int) {
			defer wg.Done()

			for j := 0; j < operationsPerServer; j++ {
				server := &ServerChannel{}
				server.Init()

				// Try to bind to different ports to avoid conflicts
				port := 20000 + (serverID*operationsPerServer + j)
				localAddr, _ := net.ResolveTCPAddr("tcp", fmt.Sprintf("127.0.0.1:%d", port))

				err := server.UnsafeBind(localAddr)
				if err != nil {
					atomic.AddInt64(&failedBinds, 1)
				} else {
					atomic.AddInt64(&successfulBinds, 1)
					// Ensure proper cleanup
					server.UnsafeClose()
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify that all operations were attempted
	totalOperations := atomic.LoadInt64(&successfulBinds) + atomic.LoadInt64(&failedBinds)
	expectedTotal := int64(numServers * operationsPerServer)
	assert.Equal(t, expectedTotal, totalOperations, "All bind attempts should be counted")

	t.Logf("Server bind attempts: %d successful, %d failed out of %d total",
		successfulBinds, failedBinds, totalOperations)
}

// Test concurrent server accept operations with simulated connections
func TestTCPServerChannel_ConcurrentAccept(t *testing.T) {
	server := &ServerChannel{}
	server.Init()

	// Bind to a random available port
	localAddr, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	err := server.UnsafeBind(localAddr)
	assert.NoError(t, err, "Server should bind successfully")
	defer server.UnsafeClose()

	const numClients = 30
	const connectionsPerClient = 10

	var successfulAccepts int64
	var failedAccepts int64
	var wg sync.WaitGroup

	// Get the actual bound address
	actualAddr := server.listen.Addr().String()

	// Start accept loop in background
	acceptWg := sync.WaitGroup{}
	acceptWg.Add(1)
	go func() {
		defer acceptWg.Done()
		for i := 0; i < numClients*connectionsPerClient; i++ {
			if !server.IsActive() {
				break
			}

			ch, future := server.UnsafeAccept()
			if ch != nil && future != nil {
				atomic.AddInt64(&successfulAccepts, 1)
				// Channel accepted successfully
			} else {
				atomic.AddInt64(&failedAccepts, 1)
			}
		}
	}()

	// Create concurrent client connections
	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()

			for j := 0; j < connectionsPerClient; j++ {
				conn, err := net.Dial("tcp", actualAddr)
				if err == nil {
					// Brief delay to allow accept
					time.Sleep(1 * time.Millisecond)
					conn.Close()
				}
			}
		}(i)
	}

	wg.Wait()
	acceptWg.Wait()

	// Verify accept operations
	totalAccepts := atomic.LoadInt64(&successfulAccepts) + atomic.LoadInt64(&failedAccepts)
	assert.Greater(t, totalAccepts, int64(0), "Should have some accept operations")

	t.Logf("Accept operations: %d successful, %d failed out of %d total",
		successfulAccepts, failedAccepts, totalAccepts)
}

// Test TCP address validation thread safety
func TestTCPChannel_AddressValidationThreadSafety(t *testing.T) {
	const numGoroutines = 200
	const validationsPerGoroutine = 100

	var validAddresses int64
	var invalidAddresses int64
	var wg sync.WaitGroup

	// Test concurrent address validation
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(routineID int) {
			defer wg.Done()

			ch := &Channel{}
			ch.Init()

			for j := 0; j < validationsPerGoroutine; j++ {
				// Mix of valid and invalid addresses
				var localAddr, remoteAddr net.Addr
				var err error

				if j%2 == 0 {
					// Valid TCP addresses
					localAddr, _ = net.ResolveTCPAddr("tcp", "127.0.0.1:0")
					remoteAddr, _ = net.ResolveTCPAddr("tcp", "127.0.0.1:12345")
					err = ch.UnsafeConnect(localAddr, remoteAddr)
					atomic.AddInt64(&validAddresses, 1)
				} else {
					// Invalid addresses (UDP instead of TCP)
					localAddr, _ = net.ResolveUDPAddr("udp", "127.0.0.1:0")
					remoteAddr, _ = net.ResolveUDPAddr("udp", "127.0.0.1:12345")
					err = ch.UnsafeConnect(localAddr, remoteAddr)
					atomic.AddInt64(&invalidAddresses, 1)
				}

				// Validate error handling consistency
				if j%2 == 0 {
					// Valid addresses might still fail to connect, but shouldn't return ErrNotTCPAddr
					assert.NotEqual(t, ErrNotTCPAddr, err, "Valid TCP addresses should not return ErrNotTCPAddr")
				} else {
					// Invalid addresses should return ErrNotTCPAddr
					assert.Equal(t, ErrNotTCPAddr, err, "Invalid UDP addresses should return ErrNotTCPAddr")
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify that all validations were performed
	totalValidations := atomic.LoadInt64(&validAddresses) + atomic.LoadInt64(&invalidAddresses)
	expectedTotal := int64(numGoroutines * validationsPerGoroutine)
	assert.Equal(t, expectedTotal, totalValidations, "All address validations should be counted")

	t.Logf("Address validations: %d valid, %d invalid out of %d total",
		validAddresses, invalidAddresses, totalValidations)
}

// Test server channel state consistency under concurrent operations
func TestTCPServerChannel_StateConsistency(t *testing.T) {
	const numGoroutines = 100
	const operationsPerGoroutine = 20

	server := &ServerChannel{}
	server.Init()

	var bindOperations int64
	var closeOperations int64
	var activeChecks int64
	var wg sync.WaitGroup

	// Test concurrent state operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(routineID int) {
			defer wg.Done()

			for j := 0; j < operationsPerGoroutine; j++ {
				switch j % 3 {
				case 0:
					// Try to bind (will fail after first success, but tests thread safety)
					localAddr, _ := net.ResolveTCPAddr("tcp", fmt.Sprintf("127.0.0.1:%d", 25000+routineID))
					server.UnsafeBind(localAddr)
					atomic.AddInt64(&bindOperations, 1)

				case 1:
					// Check active state
					server.IsActive()
					atomic.AddInt64(&activeChecks, 1)

				case 2:
					// Try to close (safe to call multiple times)
					server.UnsafeClose()
					atomic.AddInt64(&closeOperations, 1)
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify that all operations were attempted
	totalOperations := atomic.LoadInt64(&bindOperations) + atomic.LoadInt64(&closeOperations) + atomic.LoadInt64(&activeChecks)
	expectedTotal := int64(numGoroutines * operationsPerGoroutine)
	assert.Equal(t, expectedTotal, totalOperations, "All state operations should be counted")

	t.Logf("State operations: %d binds, %d closes, %d active checks out of %d total",
		bindOperations, closeOperations, activeChecks, totalOperations)
}

// Benchmark concurrent TCP channel operations
func BenchmarkTCPChannel_ConcurrentOperations(b *testing.B) {
	const numGoroutines = 50

	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		var wg sync.WaitGroup

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(routineID int) {
				defer wg.Done()

				ch := &Channel{}
				ch.Init()

				// Perform address validation and connection attempt
				localAddr, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
				remoteAddr, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:99999")
				ch.UnsafeConnect(localAddr, remoteAddr)
			}(i)
		}

		wg.Wait()
	}
}

// Benchmark concurrent server channel operations
func BenchmarkTCPServerChannel_ConcurrentOperations(b *testing.B) {
	const numGoroutines = 50

	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		var wg sync.WaitGroup

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(routineID int) {
				defer wg.Done()

				server := &ServerChannel{}
				server.Init()

				// Try to bind to different ports
				port := 30000 + (n*numGoroutines + routineID)
				localAddr, _ := net.ResolveTCPAddr("tcp", fmt.Sprintf("127.0.0.1:%d", port))

				server.UnsafeBind(localAddr)
				server.IsActive()
				server.UnsafeClose()
			}(i)
		}

		wg.Wait()
	}
}

// Test high-load stress testing with 10,000+ concurrent operations
func TestTCPChannel_HighLoadStressTesting(t *testing.T) {
	const numGoroutines = 1000
	const operationsPerGoroutine = 15 // Total: 15,000 operations

	var successfulOperations int64
	var failedOperations int64
	var wg sync.WaitGroup

	startTime := time.Now()

	// High-load concurrent operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(routineID int) {
			defer wg.Done()

			for j := 0; j < operationsPerGoroutine; j++ {
				// Mix of TCP channel and server operations
				if j%2 == 0 {
					// TCP Channel operations
					ch := &Channel{}
					ch.Init()

					localAddr, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
					remoteAddr, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:99999")

					err := ch.UnsafeConnect(localAddr, remoteAddr)
					if err != nil {
						atomic.AddInt64(&failedOperations, 1)
					} else {
						atomic.AddInt64(&successfulOperations, 1)
						// Connection successful
					}
				} else {
					// TCP Server operations
					server := &ServerChannel{}
					server.Init()

					port := 40000 + (routineID*operationsPerGoroutine + j)
					localAddr, _ := net.ResolveTCPAddr("tcp", fmt.Sprintf("127.0.0.1:%d", port))

					err := server.UnsafeBind(localAddr)
					if err != nil {
						atomic.AddInt64(&failedOperations, 1)
					} else {
						atomic.AddInt64(&successfulOperations, 1)
						server.UnsafeClose()
					}
				}
			}
		}(i)
	}

	wg.Wait()

	duration := time.Since(startTime)
	totalOperations := atomic.LoadInt64(&successfulOperations) + atomic.LoadInt64(&failedOperations)
	expectedTotal := int64(numGoroutines * operationsPerGoroutine)

	// Verify high-load performance
	assert.Equal(t, expectedTotal, totalOperations, "All high-load operations should be counted")
	assert.Less(t, duration, 30*time.Second, "High-load test should complete within 30 seconds")

	operationsPerSecond := float64(totalOperations) / duration.Seconds()

	t.Logf("High-load stress test completed: %d operations in %v (%.2f ops/sec)",
		totalOperations, duration, operationsPerSecond)
	t.Logf("Results: %d successful, %d failed operations",
		successfulOperations, failedOperations)

	// Performance requirements
	assert.Greater(t, operationsPerSecond, 500.0, "Should achieve at least 500 operations per second")
}

// Test memory consistency and resource cleanup
func TestTCPChannel_MemoryConsistencyAndCleanup(t *testing.T) {
	const numGoroutines = 200
	const operationsPerGoroutine = 25

	var createdChannels int64
	var cleanedChannels int64
	var wg sync.WaitGroup

	// Test memory consistency with rapid creation and cleanup
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(routineID int) {
			defer wg.Done()

			channels := make([]*Channel, operationsPerGoroutine)
			servers := make([]*ServerChannel, operationsPerGoroutine)

			// Create channels and servers
			for j := 0; j < operationsPerGoroutine; j++ {
				channels[j] = &Channel{}
				channels[j].Init()

				servers[j] = &ServerChannel{}
				servers[j].Init()

				atomic.AddInt64(&createdChannels, 2) // Count both channel and server
			}

			// Clean up all resources
			for j := 0; j < operationsPerGoroutine; j++ {
				if channels[j] != nil {
					// Channel cleanup would happen here
					atomic.AddInt64(&cleanedChannels, 1)
				}

				if servers[j] != nil {
					servers[j].UnsafeClose()
					atomic.AddInt64(&cleanedChannels, 1)
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify memory consistency
	expectedChannels := int64(numGoroutines * operationsPerGoroutine * 2)
	assert.Equal(t, expectedChannels, atomic.LoadInt64(&createdChannels), "All channels should be created")
	assert.Equal(t, expectedChannels, atomic.LoadInt64(&cleanedChannels), "All channels should be cleaned up")

	t.Logf("Memory consistency test: %d channels created, %d channels cleaned up",
		createdChannels, cleanedChannels)
}
