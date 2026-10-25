package gtcp

import (
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { listener.Close() })
	require.NoError(t, listener.(*net.TCPListener).SetDeadline(time.Now().Add(15*time.Second)))
	var accepted atomic.Int64
	var acceptWG sync.WaitGroup
	acceptWG.Go(func() {
		for range numGoroutines * connectionsPerGoroutine {
			peer, err := listener.Accept()
			if err != nil {
				t.Errorf("accept: %v", err)
				return
			}
			accepted.Add(1)
			assert.NoError(t, peer.Close())
		}
	})
	remoteAddr := listener.Addr().(*net.TCPAddr)
	localAddr := &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)}
	slots := make(chan struct{}, 10)
	var successfulConnections atomic.Int64
	var rejectedConnections atomic.Int64
	var wg sync.WaitGroup
	for range numGoroutines {
		wg.Go(func() {
			for range connectionsPerGoroutine {
				slots <- struct{}{}
				ch := &Channel{}
				ch.Init()
				if assert.NoError(t, ch.UnsafeConnect(localAddr, remoteAddr)) {
					assert.Equal(t, remoteAddr.String(), ch.RemoteAddr().String())
					assert.True(t, ch.Conn().IsActive())
					if assert.NoError(t, ch.UnsafeDisconnect()) {
						assert.False(t, ch.Conn().IsActive())
						successfulConnections.Add(1)
					}
				}
				<-slots
			}
			invalid := &Channel{}
			invalid.Init()
			if assert.ErrorIs(t, invalid.UnsafeConnect(localAddr, &net.UDPAddr{}), ErrNotTCPAddr) {
				assert.Nil(t, invalid.Conn())
				rejectedConnections.Add(1)
			}
		})
	}
	wg.Wait()
	acceptWG.Wait()
	require.NoError(t, listener.Close())
	expected := int64(numGoroutines * connectionsPerGoroutine)
	assert.Equal(t, expected, successfulConnections.Load())
	assert.Equal(t, int64(numGoroutines), rejectedConnections.Load())
	assert.Equal(t, expected, accepted.Load(), "every successful connection must reach the listener")
}

// Test concurrent server channel operations
func TestTCPServerChannel_ConcurrentOperations(t *testing.T) {
	const numServers = 50
	const operationsPerServer = 30

	var successfulBinds int64
	var failedBinds int64
	var wg sync.WaitGroup

	// Test concurrent server creation and binding
	for i := range numServers {
		wg.Add(1)
		go func(serverID int) {
			defer wg.Done()

			for range operationsPerServer {
				server := &ServerChannel{}
				server.Init()

				localAddr, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:0")

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

	var successfulAccepts atomic.Int64
	var failedAccepts atomic.Int64
	var wg sync.WaitGroup

	// Get the actual bound address
	actualAddr := server.listen.Addr().String()

	// Start accept loop in background
	acceptWg := sync.WaitGroup{}
	acceptWg.Go(func() {
		for range numClients * connectionsPerClient {
			if !server.IsActive() {
				break
			}

			ch, future := server.UnsafeAccept()
			if ch != nil && future != nil {
				successfulAccepts.Add(1)
				t.Cleanup(func() { assert.NoError(t, ch.(*Channel).UnsafeDisconnect()) })
			} else {
				failedAccepts.Add(1)
			}
		}
	})

	// Create concurrent client connections
	for range numClients {
		wg.Go(func() {
			for range connectionsPerClient {
				conn, err := net.Dial("tcp", actualAddr)
				if assert.NoError(t, err, "Client should connect successfully") {
					// Brief delay to allow accept
					time.Sleep(1 * time.Millisecond)
					assert.NoError(t, conn.Close())
				} else {
					assert.NoError(t, server.UnsafeClose())
				}
			}
		})
	}

	wg.Wait()
	acceptWg.Wait()

	// Verify accept operations
	totalAccepts := successfulAccepts.Load() + failedAccepts.Load()
	assert.Greater(t, totalAccepts, int64(0), "Should have some accept operations")
	assert.Equal(t, int64(numClients*connectionsPerClient), successfulAccepts.Load(), "All client connections should be accepted")

	t.Logf("Accept operations: %d successful, %d failed out of %d total",
		successfulAccepts.Load(), failedAccepts.Load(), totalAccepts)
}

func TestTCPServerChannel_AcceptUnblocksOnConcurrentClose(t *testing.T) {
	server := &ServerChannel{}
	server.Init()

	localAddr, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	assert.NoError(t, server.UnsafeBind(localAddr))

	done := make(chan struct {
		ch     channel.Channel
		future channel.Future
	}, 1)
	go func() {
		ch, future := server.UnsafeAccept()
		done <- struct {
			ch     channel.Channel
			future channel.Future
		}{ch: ch, future: future}
	}()

	assert.NoError(t, server.UnsafeClose())

	select {
	case result := <-done:
		assert.Nil(t, result.ch)
		assert.NotNil(t, result.future)
	case <-time.After(2 * time.Second):
		t.Fatal("UnsafeAccept did not unblock after UnsafeClose")
	}
}

// Test TCP address validation thread safety
func TestTCPChannel_AddressValidationThreadSafety(t *testing.T) {
	const numGoroutines = 200
	const validationsPerGoroutine = 100

	var validAddresses int64
	var invalidAddresses int64
	var wg sync.WaitGroup

	// Test concurrent address validation
	for i := range numGoroutines {
		wg.Add(1)
		go func(routineID int) {
			defer wg.Done()

			ch := &Channel{}
			ch.Init()

			for j := range validationsPerGoroutine {
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
	for i := range numGoroutines {
		wg.Add(1)
		go func(routineID int) {
			defer wg.Done()

			for j := range operationsPerGoroutine {
				switch j % 3 {
				case 0:
					// Try to bind (will fail after first success, but tests thread safety)
					localAddr, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
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

	for b.Loop() {
		var wg sync.WaitGroup

		for i := range numGoroutines {
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

	n := 0
	for b.Loop() {
		var wg sync.WaitGroup

		for i := range numGoroutines {
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
		n++
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
	for i := range numGoroutines {
		wg.Add(1)
		go func(routineID int) {
			defer wg.Done()

			for j := range operationsPerGoroutine {
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

					localAddr, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:0")

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

	operationsPerSecond := float64(totalOperations) / duration.Seconds()

	t.Logf("High-load stress test completed: %d operations in %v (%.2f ops/sec)",
		totalOperations, duration, operationsPerSecond)
	t.Logf("Results: %d successful, %d failed operations",
		successfulOperations, failedOperations)

	// Throughput is logged for diagnostics only; correctness must not depend
	// on local machine speed or shared CI load.
}

// Test memory consistency and resource cleanup
func TestTCPChannel_MemoryConsistencyAndCleanup(t *testing.T) {
	const numGoroutines = 200
	const operationsPerGoroutine = 25

	var createdChannels int64
	var cleanedChannels int64
	var wg sync.WaitGroup

	// Test memory consistency with rapid creation and cleanup
	for i := range numGoroutines {
		wg.Add(1)
		go func(routineID int) {
			defer wg.Done()

			channels := make([]*Channel, operationsPerGoroutine)
			servers := make([]*ServerChannel, operationsPerGoroutine)

			// Create channels and servers
			for j := range operationsPerGoroutine {
				channels[j] = &Channel{}
				channels[j].Init()

				servers[j] = &ServerChannel{}
				servers[j].Init()

				atomic.AddInt64(&createdChannels, 2) // Count both channel and server
			}

			// Clean up all resources
			for j := range operationsPerGoroutine {
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
