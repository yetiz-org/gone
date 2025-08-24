package gudp

import (
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

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

	t.Run("Read_EmptyCachedData", func(t *testing.T) {
		t.Parallel()
		
		server, clientAddr := getTestUDPConnection(t)
		defer server.Close()
		
		clientConn := &UDPClientConn{
			server:     server,
			clientAddr: clientAddr,
			lastData:   nil, // No cached data
			firstRead:  false,
		}
		
		// Set short read deadline to cause timeout (since no data available)
		err := server.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		assert.NoError(t, err)
		
		buffer := make([]byte, 100)
		_, err = clientConn.Read(buffer)
		
		// Should timeout or error
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

	t.Run("Write_LargeData", func(t *testing.T) {
		t.Parallel()
		
		server, clientAddr := getTestUDPConnection(t)
		defer server.Close()
		
		// Create large data (but within UDP limits)
		largeData := make([]byte, 1024)
		for i := range largeData {
			largeData[i] = byte(i % 256)
		}
		
		clientConn := &UDPClientConn{
			server:     server,
			clientAddr: clientAddr,
		}
		
		n, err := clientConn.Write(largeData)
		
		// Should succeed
		assert.NoError(t, err)
		assert.Equal(t, len(largeData), n)
	})

	t.Run("Write_ToNilAddress", func(t *testing.T) {
		t.Parallel()
		
		server, _ := getTestUDPConnection(t)
		defer server.Close()
		
		testData := []byte("test message")
		
		clientConn := &UDPClientConn{
			server:     server,
			clientAddr: nil, // Nil address
		}
		
		// Should handle nil address (might error or succeed depending on implementation)
		// We just verify it doesn't panic
		assert.NotPanics(t, func() {
			clientConn.Write(testData)
		})
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

	t.Run("Close_AfterOperations", func(t *testing.T) {
		t.Parallel()
		
		server, clientAddr := getTestUDPConnection(t)
		defer server.Close()
		
		clientConn := &UDPClientConn{
			server:     server,
			clientAddr: clientAddr,
			lastData:   []byte("test data"),
		}
		
		// Do some operations first
		buffer := make([]byte, 100)
		clientConn.Read(buffer)
		clientConn.Write([]byte("test"))
		
		// Then close
		err := clientConn.Close()
		assert.NoError(t, err)
		
		// Operations after close should still work (since UDP is connectionless)
		err = clientConn.Close()
		assert.NoError(t, err)
	})
}

// TestUDPClientConn_AddressMethods tests address getter methods
func TestUDPClientConn_AddressMethods(t *testing.T) {
	t.Parallel()

	t.Run("LocalAddr_ReturnsServerLocalAddr", func(t *testing.T) {
		t.Parallel()
		
		server, clientAddr := getTestUDPConnection(t)
		defer server.Close()
		
		clientConn := &UDPClientConn{
			server:     server,
			clientAddr: clientAddr,
		}
		
		addr := clientConn.LocalAddr()
		
		// Should return the server's local address
		assert.NotNil(t, addr)
		assert.Equal(t, server.LocalAddr(), addr)
	})

	t.Run("RemoteAddr_ReturnsClientAddr", func(t *testing.T) {
		t.Parallel()
		
		server, clientAddr := getTestUDPConnection(t)
		defer server.Close()
		
		clientConn := &UDPClientConn{
			server:     server,
			clientAddr: clientAddr,
		}
		
		addr := clientConn.RemoteAddr()
		
		// Should return the client address
		assert.Equal(t, clientAddr, addr)
	})

	t.Run("RemoteAddr_WithNilClientAddr", func(t *testing.T) {
		t.Parallel()
		
		server, _ := getTestUDPConnection(t)
		defer server.Close()
		
		clientConn := &UDPClientConn{
			server:     server,
			clientAddr: nil,
		}
		
		addr := clientConn.RemoteAddr()
		
		// Should return nil client address
		assert.Nil(t, addr)
	})

	t.Run("AddressMethods_Consistency", func(t *testing.T) {
		t.Parallel()
		
		server, clientAddr := getTestUDPConnection(t)
		defer server.Close()
		
		clientConn := &UDPClientConn{
			server:     server,
			clientAddr: clientAddr,
		}
		
		// Multiple calls should return consistent results
		localAddr1 := clientConn.LocalAddr()
		localAddr2 := clientConn.LocalAddr()
		remoteAddr1 := clientConn.RemoteAddr()
		remoteAddr2 := clientConn.RemoteAddr()
		
		assert.Equal(t, localAddr1, localAddr2)
		assert.Equal(t, remoteAddr1, remoteAddr2)
		assert.Equal(t, clientAddr, remoteAddr1)
	})
}

// TestUDPClientConn_DeadlineMethods tests deadline setting methods
func TestUDPClientConn_DeadlineMethods(t *testing.T) {
	t.Parallel()

	t.Run("SetDeadline_Success", func(t *testing.T) {
		t.Parallel()
		
		server, clientAddr := getTestUDPConnection(t)
		defer server.Close()
		
		deadline := time.Now().Add(5 * time.Second)
		
		clientConn := &UDPClientConn{
			server:     server,
			clientAddr: clientAddr,
		}
		
		err := clientConn.SetDeadline(deadline)
		
		assert.NoError(t, err)
	})

	t.Run("SetReadDeadline_Success", func(t *testing.T) {
		t.Parallel()
		
		server, clientAddr := getTestUDPConnection(t)
		defer server.Close()
		
		deadline := time.Now().Add(3 * time.Second)
		
		clientConn := &UDPClientConn{
			server:     server,
			clientAddr: clientAddr,
		}
		
		err := clientConn.SetReadDeadline(deadline)
		
		assert.NoError(t, err)
	})

	t.Run("SetWriteDeadline_Success", func(t *testing.T) {
		t.Parallel()
		
		server, clientAddr := getTestUDPConnection(t)
		defer server.Close()
		
		deadline := time.Now().Add(7 * time.Second)
		
		clientConn := &UDPClientConn{
			server:     server,
			clientAddr: clientAddr,
		}
		
		err := clientConn.SetWriteDeadline(deadline)
		
		assert.NoError(t, err)
	})

	t.Run("SetDeadline_ZeroTime", func(t *testing.T) {
		t.Parallel()
		
		server, clientAddr := getTestUDPConnection(t)
		defer server.Close()
		
		clientConn := &UDPClientConn{
			server:     server,
			clientAddr: clientAddr,
		}
		
		// Zero time should disable deadline
		err := clientConn.SetDeadline(time.Time{})
		assert.NoError(t, err)
		
		err = clientConn.SetReadDeadline(time.Time{})
		assert.NoError(t, err)
		
		err = clientConn.SetWriteDeadline(time.Time{})
		assert.NoError(t, err)
	})

	t.Run("SetDeadline_PastTime", func(t *testing.T) {
		t.Parallel()
		
		server, clientAddr := getTestUDPConnection(t)
		defer server.Close()
		
		pastDeadline := time.Now().Add(-1 * time.Hour)
		
		clientConn := &UDPClientConn{
			server:     server,
			clientAddr: clientAddr,
		}
		
		// Past deadline should be allowed (implementation dependent)
		// Some implementations may return error, others may accept it
		assert.NotPanics(t, func() {
			clientConn.SetDeadline(pastDeadline)
		})
	})

	t.Run("SetDeadline_SequentialCalls", func(t *testing.T) {
		t.Parallel()
		
		server, clientAddr := getTestUDPConnection(t)
		defer server.Close()
		
		clientConn := &UDPClientConn{
			server:     server,
			clientAddr: clientAddr,
		}
		
		// Sequential deadline setting should work
		deadline1 := time.Now().Add(1 * time.Second)
		deadline2 := time.Now().Add(2 * time.Second)
		deadline3 := time.Now().Add(3 * time.Second)
		
		err1 := clientConn.SetDeadline(deadline1)
		err2 := clientConn.SetReadDeadline(deadline2)
		err3 := clientConn.SetWriteDeadline(deadline3)
		
		assert.NoError(t, err1)
		assert.NoError(t, err2)
		assert.NoError(t, err3)
	})
}

// TestUDPClientConn_IntegrationScenarios tests complete UDP client connection workflows
func TestUDPClientConn_IntegrationScenarios(t *testing.T) {
	t.Parallel()

	t.Run("UDPClientConn_CompleteLifecycle", func(t *testing.T) {
		t.Parallel()
		
		server, clientAddr := getTestUDPConnection(t)
		defer server.Close()
		
		cachedData := []byte("initial data")
		testData := []byte("subsequent data")
		
		clientConn := &UDPClientConn{
			server:     server,
			clientAddr: clientAddr,
			lastData:   cachedData,
			firstRead:  false,
		}
		
		// Test address methods
		localAddr := clientConn.LocalAddr()
		assert.NotNil(t, localAddr)
		assert.Equal(t, server.LocalAddr(), localAddr)
		
		remoteAddr := clientConn.RemoteAddr()
		assert.Equal(t, clientAddr, remoteAddr)
		
		// Test first read (cached data)
		buffer := make([]byte, 100)
		n, err := clientConn.Read(buffer)
		assert.NoError(t, err)
		assert.Equal(t, len(cachedData), n)
		assert.Equal(t, cachedData, buffer[:n])
		assert.True(t, clientConn.firstRead)
		
		// Test write
		n, err = clientConn.Write(testData)
		assert.NoError(t, err)
		assert.Equal(t, len(testData), n)
		
		// Test deadline operations
		deadline := time.Now().Add(5 * time.Second)
		err = clientConn.SetDeadline(deadline)
		assert.NoError(t, err)
		
		err = clientConn.SetReadDeadline(deadline)
		assert.NoError(t, err)
		
		err = clientConn.SetWriteDeadline(deadline)
		assert.NoError(t, err)
		
		// Test close
		err = clientConn.Close()
		assert.NoError(t, err)
	})

	t.Run("UDPClientConn_ConcurrentOperations", func(t *testing.T) {
		t.Parallel()
		
		server, clientAddr := getTestUDPConnection(t)
		defer server.Close()
		
		testData := []byte("concurrent test")
		
		clientConn := &UDPClientConn{
			server:     server,
			clientAddr: clientAddr,
			firstRead:  true, // Skip cached data
		}
		
		// Run concurrent operations
		done := make(chan bool, 15)
		
		// Concurrent writes (safe for UDP)
		for i := 0; i < 5; i++ {
			go func() {
				defer func() { done <- true }()
				n, err := clientConn.Write(testData)
				assert.NoError(t, err)
				assert.Equal(t, len(testData), n)
			}()
		}
		
		// Concurrent address operations (should be safe)
		for i := 0; i < 5; i++ {
			go func() {
				defer func() { done <- true }()
				localAddr := clientConn.LocalAddr()
				assert.NotNil(t, localAddr)
				
				remoteAddr := clientConn.RemoteAddr()
				assert.Equal(t, clientAddr, remoteAddr)
			}()
		}
		
		// Concurrent deadline operations
		for i := 0; i < 5; i++ {
			go func(idx int) {
				defer func() { done <- true }()
				deadline := time.Now().Add(time.Duration(idx+1) * time.Second)
				err := clientConn.SetDeadline(deadline)
				assert.NoError(t, err)
			}(i)
		}
		
		// Wait for all operations to complete
		for i := 0; i < 15; i++ {
			<-done
		}
		
		// Final close
		err := clientConn.Close()
		assert.NoError(t, err)
	})

	t.Run("UDPClientConn_EdgeCases", func(t *testing.T) {
		t.Parallel()
		
		server, clientAddr := getTestUDPConnection(t)
		defer server.Close()
		
		// Test with various edge case scenarios
		testCases := []struct {
			name         string
			cachedData   []byte
			firstRead    bool
			clientAddr   *net.UDPAddr
			expectPanic  bool
		}{
			{
				name:        "NormalCase",
				cachedData:  []byte("normal data"),
				firstRead:   false,
				clientAddr:  clientAddr,
				expectPanic: false,
			},
			{
				name:        "EmptyCachedData",
				cachedData:  []byte{},
				firstRead:   false,
				clientAddr:  clientAddr,
				expectPanic: false,
			},
			{
				name:        "NilCachedData",
				cachedData:  nil,
				firstRead:   false,
				clientAddr:  clientAddr,
				expectPanic: false,
			},
			{
				name:        "AlreadyFirstRead",
				cachedData:  []byte("ignored data"),
				firstRead:   true,
				clientAddr:  clientAddr,
				expectPanic: false,
			},
			{
				name:        "NilClientAddr",
				cachedData:  []byte("test data"),
				firstRead:   false,
				clientAddr:  nil,
				expectPanic: false,
			},
		}
		
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				clientConn := &UDPClientConn{
					server:     server,
					clientAddr: tc.clientAddr,
					lastData:   tc.cachedData,
					firstRead:  tc.firstRead,
				}
				
				if tc.expectPanic {
					assert.Panics(t, func() {
						clientConn.LocalAddr()
					})
				} else {
					assert.NotPanics(t, func() {
						// Test all methods don't panic
						clientConn.LocalAddr()
						clientConn.RemoteAddr()
						clientConn.Close()
						clientConn.SetDeadline(time.Now().Add(time.Second))
						clientConn.SetReadDeadline(time.Now().Add(time.Second))
						clientConn.SetWriteDeadline(time.Now().Add(time.Second))
						
						// Write and Read may error but shouldn't panic
						clientConn.Write([]byte("test"))
						buffer := make([]byte, 100)
						clientConn.Read(buffer)
					})
				}
			})
		}
	})

	t.Run("UDPClientConn_StressTest", func(t *testing.T) {
		t.Parallel()
		
		server, clientAddr := getTestUDPConnection(t)
		defer server.Close()
		
		clientConn := &UDPClientConn{
			server:     server,
			clientAddr: clientAddr,
			firstRead:  true, // Skip cached data for cleaner test
		}
		
		// Perform many operations to test stability
		for i := 0; i < 100; i++ {
			// Address operations
			localAddr := clientConn.LocalAddr()
			assert.NotNil(t, localAddr)
			
			remoteAddr := clientConn.RemoteAddr()
			assert.Equal(t, clientAddr, remoteAddr)
			
			// Write operation
			testData := []byte(fmt.Sprintf("test data %d", i))
			n, err := clientConn.Write(testData)
			assert.NoError(t, err)
			assert.Equal(t, len(testData), n)
			
			// Deadline operations
			deadline := time.Now().Add(time.Duration(i%10+1) * time.Second)
			err = clientConn.SetDeadline(deadline)
			assert.NoError(t, err)
			
			// Close operation (should be idempotent)
			err = clientConn.Close()
			assert.NoError(t, err)
		}
	})
}
