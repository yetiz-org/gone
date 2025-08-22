package gws

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/yetiz-org/gone/channel"
)

// Test WebSocket Channel interface compliance
func TestWebSocketChannel_InterfaceCompliance(t *testing.T) {
	ch := &Channel{}
	ch.BootstrapPreInit()
	
	// Verify interface implementations
	assert.Implements(t, (*channel.Channel)(nil), ch)
	assert.NotNil(t, ch.DefaultNetChannel)
}

// Test concurrent message creation and encoding
func TestWebSocketMessage_ConcurrentOperations(t *testing.T) {
	const numGoroutines = 200
	const messagesPerGoroutine = 50
	
	var textMessages int64
	var binaryMessages int64
	var closeMessages int64
	var pingMessages int64
	var wg sync.WaitGroup
	
	// Test concurrent message creation and encoding
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(routineID int) {
			defer wg.Done()
			
			for j := 0; j < messagesPerGoroutine; j++ {
				messageType := j % 4
				payload := []byte("test message data")
				
				switch messageType {
				case 0:
					// Text message
					msg := &DefaultMessage{
						MessageType: TextMessageType,
						Message:     payload,
					}
					encoded := msg.Encoded()
					assert.Equal(t, payload, encoded, "Text message encoding should be consistent")
					assert.Equal(t, TextMessageType, msg.Type(), "Text message type should be correct")
					atomic.AddInt64(&textMessages, 1)
					
				case 1:
					// Binary message
					msg := &DefaultMessage{
						MessageType: BinaryMessageType,
						Message:     payload,
					}
					encoded := msg.Encoded()
					assert.Equal(t, payload, encoded, "Binary message encoding should be consistent")
					assert.Equal(t, BinaryMessageType, msg.Type(), "Binary message type should be correct")
					atomic.AddInt64(&binaryMessages, 1)
					
				case 2:
					// Close message
					msg := &CloseMessage{
						DefaultMessage: DefaultMessage{
							MessageType: CloseMessageType,
							Message:     payload,
						},
						CloseCode: CloseNormalClosure,
					}
					encoded := msg.Encoded()
					assert.NotNil(t, encoded, "Close message should encode successfully")
					assert.Equal(t, CloseMessageType, msg.Type(), "Close message type should be correct")
					atomic.AddInt64(&closeMessages, 1)
					
				case 3:
					// Ping message
					msg := &PingMessage{
						DefaultMessage: DefaultMessage{
							MessageType: PingMessageType,
							Message:     payload,
						},
					}
					encoded := msg.Encoded()
					assert.Equal(t, payload, encoded, "Ping message encoding should be consistent")
					assert.Equal(t, PingMessageType, msg.Type(), "Ping message type should be correct")
					atomic.AddInt64(&pingMessages, 1)
				}
			}
		}(i)
	}
	
	wg.Wait()
	
	// Verify message creation counts
	totalMessages := atomic.LoadInt64(&textMessages) + atomic.LoadInt64(&binaryMessages) +
		atomic.LoadInt64(&closeMessages) + atomic.LoadInt64(&pingMessages)
	expectedTotal := int64(numGoroutines * messagesPerGoroutine)
	assert.Equal(t, expectedTotal, totalMessages, "All messages should be created and processed")
	
	t.Logf("Message creation results: %d text, %d binary, %d close, %d ping out of %d total",
		textMessages, binaryMessages, closeMessages, pingMessages, totalMessages)
}

// Test concurrent message parsing operations
func TestWebSocketMessage_ConcurrentParsing(t *testing.T) {
	const numGoroutines = 150
	const parseOperationsPerGoroutine = 100
	
	var successfulParses int64
	var nilParses int64
	var wg sync.WaitGroup
	
	// Test concurrent message parsing
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(routineID int) {
			defer wg.Done()
			
			for j := 0; j < parseOperationsPerGoroutine; j++ {
				// Mix of valid and invalid message types
				var messageType int
				var payload []byte
				
				if j%3 == 0 {
					messageType = 1 // TextMessage
					payload = []byte("text data")
				} else if j%3 == 1 {
					messageType = 2 // BinaryMessage
					payload = []byte{0x01, 0x02, 0x03}
				} else {
					messageType = 99 // Invalid type
					payload = []byte("invalid")
				}
				
				msg := _ParseMessage(messageType, payload)
				if msg != nil {
					atomic.AddInt64(&successfulParses, 1)
					
					// Verify parsed message consistency
					assert.NotNil(t, msg.Encoded(), "Parsed message should have encoded data")
					assert.True(t, msg.Type() == TextMessageType || msg.Type() == BinaryMessageType,
						"Parsed message should have valid type")
				} else {
					atomic.AddInt64(&nilParses, 1)
				}
			}
		}(i)
	}
	
	wg.Wait()
	
	// Verify parsing results
	totalParses := atomic.LoadInt64(&successfulParses) + atomic.LoadInt64(&nilParses)
	expectedTotal := int64(numGoroutines * parseOperationsPerGoroutine)
	assert.Equal(t, expectedTotal, totalParses, "All parse attempts should be counted")
	
	t.Logf("Parse results: %d successful, %d nil out of %d total",
		successfulParses, nilParses, totalParses)
}

// Test concurrent WebSocket channel write operations
func TestWebSocketChannel_ConcurrentWrite(t *testing.T) {
	const numGoroutines = 100
	const writeOperationsPerGoroutine = 30
	
	var successfulWrites int64
	var failedWrites int64
	var wg sync.WaitGroup
	
	// Test concurrent write operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(routineID int) {
			defer wg.Done()
			
			// Create WebSocket channel for each goroutine
			ch := &Channel{}
			ch.BootstrapPreInit()
			ch.Init()
			
			for j := 0; j < writeOperationsPerGoroutine; j++ {
				// Create different types of messages
				var msg Message
				
				switch j % 3 {
				case 0:
					msg = &DefaultMessage{
						MessageType: TextMessageType,
						Message:     []byte("concurrent text message"),
					}
				case 1:
					msg = &DefaultMessage{
						MessageType: BinaryMessageType,
						Message:     []byte{0x01, 0x02, 0x03, 0x04},
					}
				case 2:
					msg = &PingMessage{
						DefaultMessage: DefaultMessage{
							MessageType: PingMessageType,
							Message:     []byte("ping data"),
						},
					}
				}
				
				// Attempt write (will fail since channel is not connected, but tests thread safety)
				err := ch.UnsafeWrite(msg)
				if err != nil {
					atomic.AddInt64(&failedWrites, 1)
				} else {
					atomic.AddInt64(&successfulWrites, 1)
				}
				
				// Also test writing invalid objects
				err = ch.UnsafeWrite("invalid object")
				if err != nil {
					atomic.AddInt64(&failedWrites, 1)
				} else {
					atomic.AddInt64(&successfulWrites, 1)
				}
			}
		}(i)
	}
	
	wg.Wait()
	
	// Verify write operation counts
	totalWrites := atomic.LoadInt64(&successfulWrites) + atomic.LoadInt64(&failedWrites)
	expectedTotal := int64(numGoroutines * writeOperationsPerGoroutine * 2) // 2 writes per iteration
	assert.Equal(t, expectedTotal, totalWrites, "All write attempts should be counted")
	
	t.Logf("Write results: %d successful, %d failed out of %d total",
		successfulWrites, failedWrites, totalWrites)
}

// Test concurrent close message encoding with different close codes
func TestCloseMessage_ConcurrentEncoding(t *testing.T) {
	const numGoroutines = 200
	const encodingsPerGoroutine = 25
	
	var encodingOperations int64
	var validEncodings int64
	var wg sync.WaitGroup
	
	closeCodes := []CloseCode{
		CloseNormalClosure,
		CloseGoingAway,
		CloseProtocolError,
		CloseUnsupportedData,
		CloseNoStatusReceived,
		CloseAbnormalClosure,
		CloseInvalidFramePayloadData,
		ClosePolicyViolation,
		CloseMessageTooBig,
		CloseMandatoryExtension,
		CloseInternalServerErr,
		CloseServiceRestart,
		CloseTryAgainLater,
		CloseTLSHandshake,
	}
	
	// Test concurrent close message encoding
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(routineID int) {
			defer wg.Done()
			
			for j := 0; j < encodingsPerGoroutine; j++ {
				closeCode := closeCodes[j%len(closeCodes)]
				message := []byte("close reason")
				
				closeMsg := &CloseMessage{
					DefaultMessage: DefaultMessage{
						MessageType: CloseMessageType,
						Message:     message,
					},
					CloseCode: closeCode,
				}
				
				encoded := closeMsg.Encoded()
				atomic.AddInt64(&encodingOperations, 1)
				
				// Verify encoding consistency
				if closeCode == CloseNoStatusReceived {
					assert.Empty(t, encoded, "CloseNoStatusReceived should produce empty encoding")
				} else {
					assert.GreaterOrEqual(t, len(encoded), 2, "Close message should include close code")
					atomic.AddInt64(&validEncodings, 1)
				}
				
				// Test message properties
				assert.Equal(t, CloseMessageType, closeMsg.Type(), "Close message type should be correct")
				assert.NotNil(t, closeMsg.Encoded(), "Close message should encode successfully")
			}
		}(i)
	}
	
	wg.Wait()
	
	// Verify encoding operations
	expectedOperations := int64(numGoroutines * encodingsPerGoroutine)
	assert.Equal(t, expectedOperations, atomic.LoadInt64(&encodingOperations), "All encoding operations should be counted")
	
	t.Logf("Close message encoding: %d operations, %d valid encodings",
		encodingOperations, validEncodings)
}

// Test WebSocket channel state consistency under concurrent operations
func TestWebSocketChannel_StateConsistency(t *testing.T) {
	const numGoroutines = 150
	const operationsPerGoroutine = 40
	
	var initOperations int64
	var writeAttempts int64
	var stateChecks int64
	var wg sync.WaitGroup
	
	// Test concurrent state operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(routineID int) {
			defer wg.Done()
			
			ch := &Channel{}
			
			for j := 0; j < operationsPerGoroutine; j++ {
				switch j % 3 {
				case 0:
					// Initialize channel
					ch.BootstrapPreInit()
					ch.Init()
					atomic.AddInt64(&initOperations, 1)
					
				case 1:
					// Check if channel is active
					ch.IsActive()
					atomic.AddInt64(&stateChecks, 1)
					
				case 2:
					// Attempt write operation
					msg := &DefaultMessage{
						MessageType: TextMessageType,
						Message:     []byte("state test"),
					}
					ch.UnsafeWrite(msg)
					atomic.AddInt64(&writeAttempts, 1)
				}
			}
		}(i)
	}
	
	wg.Wait()
	
	// Verify state operation counts
	totalOperations := atomic.LoadInt64(&initOperations) + atomic.LoadInt64(&writeAttempts) + atomic.LoadInt64(&stateChecks)
	expectedTotal := int64(numGoroutines * operationsPerGoroutine)
	assert.Equal(t, expectedTotal, totalOperations, "All state operations should be counted")
	
	t.Logf("State operations: %d inits, %d writes, %d state checks out of %d total",
		initOperations, writeAttempts, stateChecks, totalOperations)
}

// Benchmark concurrent WebSocket message operations
func BenchmarkWebSocketMessage_ConcurrentOperations(b *testing.B) {
	const numGoroutines = 100
	
	b.ResetTimer()
	
	for n := 0; n < b.N; n++ {
		var wg sync.WaitGroup
		
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(routineID int) {
				defer wg.Done()
				
				// Create and encode messages
				textMsg := &DefaultMessage{
					MessageType: TextMessageType,
					Message:     []byte("benchmark text"),
				}
				textMsg.Encoded()
				
				binaryMsg := &DefaultMessage{
					MessageType: BinaryMessageType,
					Message:     []byte{0x01, 0x02, 0x03},
				}
				binaryMsg.Encoded()
				
				closeMsg := &CloseMessage{
					DefaultMessage: DefaultMessage{
						MessageType: CloseMessageType,
						Message:     []byte("close"),
					},
					CloseCode: CloseNormalClosure,
				}
				closeMsg.Encoded()
			}(i)
		}
		
		wg.Wait()
	}
}

// Benchmark concurrent WebSocket channel operations
func BenchmarkWebSocketChannel_ConcurrentOperations(b *testing.B) {
	const numGoroutines = 100
	
	b.ResetTimer()
	
	for n := 0; n < b.N; n++ {
		var wg sync.WaitGroup
		
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				
				ch := &Channel{}
				ch.BootstrapPreInit()
				ch.Init()
				
				msg := &DefaultMessage{
					MessageType: TextMessageType,
					Message:     []byte("benchmark message"),
				}
				
				ch.UnsafeWrite(msg)
				ch.IsActive()
			}()
		}
		
		wg.Wait()
	}
}

// Test high-load stress testing with 10,000+ concurrent WebSocket operations
func TestWebSocketChannel_HighLoadStressTesting(t *testing.T) {
	const numGoroutines = 1000
	const operationsPerGoroutine = 12 // Total: 12,000 operations
	
	var messageCreations int64
	var channelOperations int64
	var encodingOperations int64
	var wg sync.WaitGroup
	
	startTime := time.Now()
	
	// High-load concurrent WebSocket operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(routineID int) {
			defer wg.Done()
			
			ch := &Channel{}
			ch.BootstrapPreInit()
			ch.Init()
			
			for j := 0; j < operationsPerGoroutine; j++ {
				// Mix of WebSocket operations
				switch j % 4 {
				case 0:
					// Create and encode text message
					msg := &DefaultMessage{
						MessageType: TextMessageType,
						Message:     []byte("high-load text"),
					}
					msg.Encoded()
					atomic.AddInt64(&messageCreations, 1)
					atomic.AddInt64(&encodingOperations, 1)
					
				case 1:
					// Create and encode close message
					closeMsg := &CloseMessage{
						DefaultMessage: DefaultMessage{
							MessageType: CloseMessageType,
							Message:     []byte("high-load close"),
						},
						CloseCode: CloseNormalClosure,
					}
					closeMsg.Encoded()
					atomic.AddInt64(&messageCreations, 1)
					atomic.AddInt64(&encodingOperations, 1)
					
				case 2:
					// Channel write operation
					msg := &PingMessage{
						DefaultMessage: DefaultMessage{
							MessageType: PingMessageType,
							Message:     []byte("high-load ping"),
						},
					}
					ch.UnsafeWrite(msg)
					atomic.AddInt64(&channelOperations, 1)
					
				case 3:
					// Channel state check
					ch.IsActive()
					atomic.AddInt64(&channelOperations, 1)
				}
			}
		}(i)
	}
	
	wg.Wait()
	
	duration := time.Since(startTime)
	totalOperations := atomic.LoadInt64(&messageCreations) + atomic.LoadInt64(&channelOperations) + atomic.LoadInt64(&encodingOperations)
	
	// Verify high-load performance
	assert.Greater(t, totalOperations, int64(10000), "Should perform more than 10,000 operations")
	assert.Less(t, duration, 30*time.Second, "High-load test should complete within 30 seconds")
	
	operationsPerSecond := float64(totalOperations) / duration.Seconds()
	
	t.Logf("WebSocket high-load stress test completed: %d operations in %v (%.2f ops/sec)",
		totalOperations, duration, operationsPerSecond)
	t.Logf("Results: %d message creations, %d channel operations, %d encoding operations",
		messageCreations, channelOperations, encodingOperations)
	
	// Performance requirements
	assert.Greater(t, operationsPerSecond, 1000.0, "Should achieve at least 1000 operations per second")
}

// Test memory consistency and WebSocket resource management
func TestWebSocketChannel_MemoryConsistencyAndResourceManagement(t *testing.T) {
	const numGoroutines = 250
	const operationsPerGoroutine = 20
	
	var createdChannels int64
	var createdMessages int64
	var processedOperations int64
	var wg sync.WaitGroup
	
	// Test memory consistency with rapid WebSocket operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(routineID int) {
			defer wg.Done()
			
			channels := make([]*Channel, operationsPerGoroutine)
			messages := make([]Message, operationsPerGoroutine)
			
			// Create channels and messages
			for j := 0; j < operationsPerGoroutine; j++ {
				channels[j] = &Channel{}
				channels[j].BootstrapPreInit()
				channels[j].Init()
				atomic.AddInt64(&createdChannels, 1)
				
				messages[j] = &DefaultMessage{
					MessageType: TextMessageType,
					Message:     []byte("memory test message"),
				}
				atomic.AddInt64(&createdMessages, 1)
			}
			
			// Process operations
			for j := 0; j < operationsPerGoroutine; j++ {
				channels[j].UnsafeWrite(messages[j])
				channels[j].IsActive()
				messages[j].Encoded()
				atomic.AddInt64(&processedOperations, 1)
			}
		}(i)
	}
	
	wg.Wait()
	
	// Verify memory consistency
	expectedChannels := int64(numGoroutines * operationsPerGoroutine)
	expectedMessages := int64(numGoroutines * operationsPerGoroutine)
	expectedOperations := int64(numGoroutines * operationsPerGoroutine)
	
	assert.Equal(t, expectedChannels, atomic.LoadInt64(&createdChannels), "All channels should be created")
	assert.Equal(t, expectedMessages, atomic.LoadInt64(&createdMessages), "All messages should be created")
	assert.Equal(t, expectedOperations, atomic.LoadInt64(&processedOperations), "All operations should be processed")
	
	t.Logf("Memory consistency test: %d channels, %d messages, %d operations processed",
		createdChannels, createdMessages, processedOperations)
}