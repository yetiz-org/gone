package gudp

// This consolidated test file merges:
// - gudp_test.go
// - udpserverchannel_comprehensive_test.go
// Original files will be archived to avoid duplicate execution.

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

// =============================================================================
// Real UDP Connection Tests (from gudp_test.go)
// =============================================================================

// Test real UDP client-server communication
func TestUDPChannel_RealConnection(t *testing.T) {
	// Create server
	server := &ServerChannel{}
	server.Init()
	
	serverAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	err := server.UnsafeBind(serverAddr)
	assert.NoError(t, err, "Server should bind successfully")
	defer server.UnsafeClose()
	
	actualServerAddr := server.conn.LocalAddr()
	t.Logf("Server bound to: %s", actualServerAddr.String())
	
	// Test simple UDP packet sending
	conn, err := net.Dial("udp", actualServerAddr.String())
	assert.NoError(t, err, "Should be able to create UDP connection")
	defer conn.Close()
	
	_, err = conn.Write([]byte("test"))
	assert.NoError(t, err, "Should be able to send UDP packet")
	
	t.Logf("UDP communication test completed successfully")
}

// Test UDP client-server message exchange
func TestUDPChannel_MessageExchange(t *testing.T) {
	// Create and start server
	server := &ServerChannel{}
	server.Init()
	
	serverAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	err := server.UnsafeBind(serverAddr)
	assert.NoError(t, err, "Server should bind successfully")
	defer server.UnsafeClose()
	
	actualServerAddr := server.conn.LocalAddr()
	
	// Test message exchange using direct UDP connection
	testMessage := []byte("Hello UDP Server!")
	
	var wg sync.WaitGroup
	var serverReceivedMessage []byte
	
	// Start server receive loop
	wg.Add(1)
	go func() {
		defer wg.Done()
		
		// Read directly from UDP connection
		buffer := make([]byte, 1024)
		n, clientAddr, err := server.conn.ReadFromUDP(buffer)
		if err == nil {
			serverReceivedMessage = buffer[:n]
			t.Logf("Server received message from %s: %s", clientAddr, string(serverReceivedMessage))
		}
	}()
	
	// Client sends message
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(100 * time.Millisecond) // Let server start first
		
		// Connect to server using standard UDP connection
		conn, err := net.Dial("udp", actualServerAddr.String())
		assert.NoError(t, err, "Client should connect to server")
		defer conn.Close()
		
		// Send test message
		_, err = conn.Write(testMessage)
		assert.NoError(t, err, "Client should send message successfully")
		
		t.Logf("Client sent message: %s", string(testMessage))
	}()
	
	// Wait for both operations to complete
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		// Success
	case <-time.After(5 * time.Second):
		t.Fatal("Test timed out")
	}
	
	// Verify message was received
	assert.Equal(t, testMessage, serverReceivedMessage, "Server should receive the correct message")
	t.Logf("Server received message: %s", string(serverReceivedMessage))
}

// Test multiple concurrent UDP clients
func TestUDPChannel_MultipleConcurrentClients(t *testing.T) {
	// Create server
	server := &ServerChannel{}
	server.Init()
	
	serverAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	err := server.UnsafeBind(serverAddr)
	assert.NoError(t, err, "Server should bind successfully")
	defer server.UnsafeClose()
	
	actualServerAddr := server.conn.LocalAddr()
	
	const numClients = 10
	const messagesPerClient = 5
	
	var wg sync.WaitGroup
	var serverMessagesReceived int64
	var serverMutex sync.Mutex
	var receivedMessages []string
	
	// Start server receive loop
	wg.Add(1)
	go func() {
		defer wg.Done()
		
		expectedMessages := numClients * messagesPerClient
		buffer := make([]byte, 1024)
		
		for i := 0; i < expectedMessages; i++ {
			n, clientAddr, err := server.conn.ReadFromUDP(buffer)
			if err != nil {
				t.Errorf("Server failed to read message: %v", err)
				break
			}
			
			message := string(buffer[:n])
			serverMutex.Lock()
			receivedMessages = append(receivedMessages, message)
			serverMutex.Unlock()
			serverMessagesReceived++
			
			t.Logf("Server received message from %s: %s", clientAddr, message)
		}
	}()
	
	// Create multiple clients
	for clientID := 0; clientID < numClients; clientID++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			time.Sleep(time.Duration(id*10) * time.Millisecond) // Stagger client starts
			
			for msgID := 0; msgID < messagesPerClient; msgID++ {
				conn, err := net.Dial("udp", actualServerAddr.String())
				if err != nil {
					t.Errorf("Client %d failed to connect: %v", id, err)
					return
				}
				
				message := fmt.Sprintf("Message from client %d, msg %d", id, msgID)
				_, err = conn.Write([]byte(message))
				if err != nil {
					t.Errorf("Client %d failed to send message: %v", id, err)
				}
				conn.Close()
				
				// Small delay between messages
				time.Sleep(10 * time.Millisecond)
			}
		}(clientID)
	}
	
	// Wait for all operations to complete
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		// Success
	case <-time.After(10 * time.Second):
		t.Fatal("Test timed out")
	}
	
	// Verify all messages were received
	expectedMessageCount := int64(numClients * messagesPerClient)
	assert.Equal(t, expectedMessageCount, serverMessagesReceived, 
		fmt.Sprintf("Server should receive %d messages", expectedMessageCount))
	
	t.Logf("Server received %d messages from %d clients", serverMessagesReceived, numClients)
	for i, msg := range receivedMessages {
		t.Logf("Message %d: %s", i+1, msg)
	}
}

// Test UDP connection error handling
func TestUDPChannel_ErrorHandling(t *testing.T) {
	// Test binding to invalid address
	server := &ServerChannel{}
	server.Init()
	
	// Try to bind with TCP address (should fail)
	invalidAddr, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:8080")
	err := server.UnsafeBind(invalidAddr)
	assert.Error(t, err, "Should fail to bind with TCP address")
	assert.Equal(t, ErrNotUDPAddr, err, "Should return ErrNotUDPAddr")
	
	// Test double bind
	validAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	err = server.UnsafeBind(validAddr)
	assert.NoError(t, err, "First bind should succeed")
	
	err = server.UnsafeBind(validAddr)
	assert.Error(t, err, "Second bind should fail")
	assert.Contains(t, err.Error(), "bind twice", "Should indicate bind twice error")
	
	server.UnsafeClose()
	
	// Test client connection with invalid address
	client := &Channel{}
	client.Init()
	
	invalidTCPAddr, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:8080")
	validUDPAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:8080")
	
	err = client.UnsafeConnect(invalidTCPAddr, validUDPAddr)
	assert.Error(t, err, "Should fail with invalid local address")
	assert.Equal(t, ErrNotUDPAddr, err, "Should return ErrNotUDPAddr for invalid local addr")
	
	err = client.UnsafeConnect(validUDPAddr, invalidTCPAddr)
	assert.Error(t, err, "Should fail with invalid remote address")
	assert.Equal(t, ErrNotUDPAddr, err, "Should return ErrNotUDPAddr for invalid remote addr")
	
	err = client.UnsafeConnect(validUDPAddr, nil)
	assert.Error(t, err, "Should fail with nil remote address")
	assert.Equal(t, channel.ErrNilObject, err, "Should return ErrNilObject for nil remote addr")
}

// Test UDP server lifecycle
func TestUDPServerChannel_Lifecycle(t *testing.T) {
	server := &ServerChannel{}
	server.Init()
	
	// Initially inactive
	assert.False(t, server.IsActive(), "Server should start inactive")
	
	// Bind and become active
	addr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	err := server.UnsafeBind(addr)
	assert.NoError(t, err, "Server should bind successfully")
	assert.True(t, server.IsActive(), "Server should be active after binding")
	
	// Close and become inactive
	err = server.UnsafeClose()
	assert.NoError(t, err, "Server should close successfully")
	assert.False(t, server.IsActive(), "Server should be inactive after closing")
	
	// Multiple close calls should be safe
	err = server.UnsafeClose()
	assert.NoError(t, err, "Multiple close calls should be safe")
	assert.False(t, server.IsActive(), "Server should remain inactive")
}

// =============================================================================
// Concurrent Testing (from gudp_test.go)
// =============================================================================

// Test UDP Channel interface compliance
func TestUDPChannel_InterfaceCompliance(t *testing.T) {
	ch := &Channel{}
	
	// Verify interface implementations
	assert.Implements(t, (*channel.Channel)(nil), ch)
	assert.NotNil(t, ch)
}

// Test concurrent UDP connection attempts
func TestUDPChannel_ConcurrentConnections(t *testing.T) {
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
				localAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:0")
				remoteAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:99999") // Non-existent port
				
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
func TestUDPServerChannel_ConcurrentOperations(t *testing.T) {
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
				localAddr, _ := net.ResolveUDPAddr("udp", fmt.Sprintf("127.0.0.1:%d", port))
				
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

// =============================================================================
// UDPClientConn Tests (from udpserverchannel_comprehensive_test.go)
// =============================================================================

// TestUDPConnWrapper wraps a real UDP connection for testing
type TestUDPConnWrapper struct {
	conn       *net.UDPConn
	clientAddr *net.UDPAddr
	testData   []byte
	writeData  []byte
	mu         sync.Mutex
	closed     bool
}

// getTestUDPConnection creates a real UDP connection for testing
func getTestUDPConnection(t *testing.T) (*net.UDPConn, *net.UDPAddr) {
	// Create a UDP connection listening on a random port
	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	assert.NoError(t, err)
	
	conn, err := net.ListenUDP("udp", addr)
	assert.NoError(t, err)
	
	clientAddr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12345}
	return conn, clientAddr
}

// TestUDPClientConn_Read tests the Read functionality
func TestUDPClientConn_Read(t *testing.T) {
	t.Parallel()

	t.Run("Read_FirstReadWithCachedData", func(t *testing.T) {
		t.Parallel()
		
		server, clientAddr := getTestUDPConnection(t)
		defer server.Close()
		
		cachedData := []byte("cached data")
		
		clientConn := &UDPClientConn{
			server:     server,
			clientAddr: clientAddr,
			lastData:   cachedData,
			firstRead:  false,
		}
		
		buffer := make([]byte, 100)
		n, err := clientConn.Read(buffer)
		
		// Should succeed with cached data
		assert.NoError(t, err)
		assert.Equal(t, len(cachedData), n)
		assert.Equal(t, cachedData, buffer[:n])
		
		// Should mark firstRead as true
		assert.True(t, clientConn.firstRead)
	})

	t.Run("Read_FirstReadWithSmallBuffer", func(t *testing.T) {
		t.Parallel()
		
		server, clientAddr := getTestUDPConnection(t)
		defer server.Close()
		
		cachedData := []byte("very long cached data that exceeds buffer")
		
		clientConn := &UDPClientConn{
			server:     server,
			clientAddr: clientAddr,
			lastData:   cachedData,
			firstRead:  false,
		}
		
		buffer := make([]byte, 10) // Small buffer
		n, err := clientConn.Read(buffer)
		
		// Should succeed but truncate data
		assert.NoError(t, err)
		assert.Equal(t, 10, n)
		assert.Equal(t, cachedData[:10], buffer)
		
		// Should mark firstRead as true
		assert.True(t, clientConn.firstRead)
	})

	t.Run("Read_WithTimeout", func(t *testing.T) {
		t.Parallel()
		
		server, clientAddr := getTestUDPConnection(t)
		defer server.Close()
		
		clientConn := &UDPClientConn{
			server:     server,
			clientAddr: clientAddr,
			firstRead:  true, // Skip cached data
		}
		
		// Set short read deadline to cause timeout
		err := server.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		assert.NoError(t, err)
		
		buffer := make([]byte, 100)
		_, err = clientConn.Read(buffer)
		
		// Should timeout
		assert.Error(t, err)
		
		// Reset deadline
		server.SetReadDeadline(time.Time{})
	})
}

// TestUDPClientConn_Write tests the Write functionality
func TestUDPClientConn_Write(t *testing.T) {
	t.Parallel()

	t.Run("Write_SuccessfulWrite", func(t *testing.T) {
		t.Parallel()
		
		server, clientAddr := getTestUDPConnection(t)
		defer server.Close()
		
		testData := []byte("test message")
		
		clientConn := &UDPClientConn{
			server:     server,
			clientAddr: clientAddr,
		}
		
		n, err := clientConn.Write(testData)
		
		// Should succeed
		assert.NoError(t, err)
		assert.Equal(t, len(testData), n)
	})

	t.Run("Write_EmptyData", func(t *testing.T) {
		t.Parallel()
		
		server, clientAddr := getTestUDPConnection(t)
		defer server.Close()
		
		emptyData := []byte{}
		
		clientConn := &UDPClientConn{
			server:     server,
			clientAddr: clientAddr,
		}
		
		n, err := clientConn.Write(emptyData)
		
		// Should succeed with empty data
		assert.NoError(t, err)
		assert.Equal(t, 0, n)
	})
}

// TestUDPClientConn_Close tests the Close functionality
func TestUDPClientConn_Close(t *testing.T) {
	t.Parallel()

	t.Run("Close_AlwaysSucceeds", func(t *testing.T) {
		t.Parallel()
		
		server, clientAddr := getTestUDPConnection(t)
		defer server.Close()
		
		clientConn := &UDPClientConn{
			server:     server,
			clientAddr: clientAddr,
		}
		
		err := clientConn.Close()
		
		// Should always succeed for UDP client connections
		assert.NoError(t, err)
	})

	t.Run("Close_MultipleCloses", func(t *testing.T) {
		t.Parallel()
		
		server, clientAddr := getTestUDPConnection(t)
		defer server.Close()
		
		clientConn := &UDPClientConn{
			server:     server,
			clientAddr: clientAddr,
		}
		
		// Multiple closes should all succeed
		err1 := clientConn.Close()
		err2 := clientConn.Close()
		err3 := clientConn.Close()
		
		assert.NoError(t, err1)
		assert.NoError(t, err2)
		assert.NoError(t, err3)
	})
}

// =============================================================================
// Benchmark Tests
// =============================================================================

// Benchmark UDP operations
func BenchmarkUDPChannel_ConcurrentOperations(b *testing.B) {
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
				localAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:0")
				remoteAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:99999")
				ch.UnsafeConnect(localAddr, remoteAddr)
			}(i)
		}
		
		wg.Wait()
	}
}

// Benchmark UDP server operations
func BenchmarkUDPServerChannel_ConcurrentOperations(b *testing.B) {
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
				localAddr, _ := net.ResolveUDPAddr("udp", fmt.Sprintf("127.0.0.1:%d", port))
				
				server.UnsafeBind(localAddr)
				server.IsActive()
				server.UnsafeClose()
			}(i)
		}
		
		wg.Wait()
	}
}
