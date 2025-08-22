package gudp

import (
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/yetiz-org/gone/channel"
)

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
