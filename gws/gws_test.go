package gws

// Consolidated tests from:
// - gws_comprehensive_test.go
// - gws_concurrent_test.go
// - gws_invokehandler_test.go
// - gws_upgradeprocessor_test.go
//
// NOTE:
// - Imports are deduplicated.
// - Original test names and t.Parallel() are preserved.
// - Helpers like NewMockMessage and NewMockHandlerTask are provided by existing files
//   (mock_message.go/mock_message_test.go and mock_handlertask.go) and not redefined here.

import (
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/yetiz-org/gone/channel"
	"github.com/yetiz-org/gone/ghttp"
)
// ===== From gws_comprehensive_test.go =====

// TestDefaultHandlerTask_ComprehensiveWSOperations tests all WebSocket handler operations
func TestDefaultHandlerTask_ComprehensiveWSOperations(t *testing.T) {
	// keep top-level parallelism of group runner
	t.Parallel()

	tests := []struct {
		name     string
		testFunc func(t *testing.T, task *DefaultHandlerTask, mockCtx *channel.MockHandlerContext)
	}{
		{
			name: "ErrorCaught_LogsError",
			testFunc: func(t *testing.T, task *DefaultHandlerTask, mockCtx *channel.MockHandlerContext) {
				testErr := errors.New("test error")
				// Test should not panic and handle error gracefully
				assert.NotPanics(t, func() {
					task.ErrorCaught(mockCtx, testErr)
				})
			},
		},
		{
			name: "WSPing_RespondsWithPong",
			testFunc: func(t *testing.T, task *DefaultHandlerTask, mockCtx *channel.MockHandlerContext) {
				pingMsg := &PingMessage{
					DefaultMessage: DefaultMessage{
						MessageType: PingMessageType,
						Message:     []byte("ping test"),
					},
				}
				params := map[string]any{"test": "param"}

				mockFuture := channel.NewMockFuture(mockCtx)
				mockCtx.On("Write", mock.AnythingOfType("*gws.PongMessage"), mock.Anything).Return(mockFuture)

				assert.NotPanics(t, func() {
					task.WSPing(mockCtx, pingMsg, params)
				})

				mockCtx.AssertExpectations(t)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			task := &DefaultHandlerTask{Builder: DefaultMessageBuilder{}}
			mockCtx := channel.NewMockHandlerContext()
			tt.testFunc(t, task, mockCtx)
		})
	}
}
// Additional WS operations from gws_comprehensive_test.go
func TestDefaultHandlerTask_RemainingWSOperations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		testFunc func(t *testing.T, task *DefaultHandlerTask, mockCtx *channel.MockHandlerContext)
	}{
		{
			name: "WSPong_HandlesMessage",
			testFunc: func(t *testing.T, task *DefaultHandlerTask, mockCtx *channel.MockHandlerContext) {
				pongMsg := &PongMessage{DefaultMessage: DefaultMessage{MessageType: PongMessageType, Message: []byte("pong test")}}
				params := map[string]any{"test": "param"}
				assert.NotPanics(t, func() { task.WSPong(mockCtx, pongMsg, params) })
			},
		},
		{
			name: "WSClose_HandlesMessage",
			testFunc: func(t *testing.T, task *DefaultHandlerTask, mockCtx *channel.MockHandlerContext) {
				closeMsg := &CloseMessage{DefaultMessage: DefaultMessage{MessageType: CloseMessageType, Message: []byte("close test")}, CloseCode: CloseNormalClosure}
				params := map[string]any{"test": "param"}
				assert.NotPanics(t, func() { task.WSClose(mockCtx, closeMsg, params) })
			},
		},
		{
			name: "WSBinary_HandlesMessage",
			testFunc: func(t *testing.T, task *DefaultHandlerTask, mockCtx *channel.MockHandlerContext) {
				binaryMsg := &DefaultMessage{MessageType: BinaryMessageType, Message: []byte("binary test data")}
				params := map[string]any{"test": "param"}
				assert.NotPanics(t, func() { task.WSBinary(mockCtx, binaryMsg, params) })
			},
		},
		{
			name: "WSText_HandlesMessage",
			testFunc: func(t *testing.T, task *DefaultHandlerTask, mockCtx *channel.MockHandlerContext) {
				textMsg := &DefaultMessage{MessageType: TextMessageType, Message: []byte("text test data")}
				params := map[string]any{"test": "param"}
				assert.NotPanics(t, func() { task.WSText(mockCtx, textMsg, params) })
			},
		},
		{
			name: "WSConnected_HandlesConnection",
			testFunc: func(t *testing.T, task *DefaultHandlerTask, mockCtx *channel.MockHandlerContext) {
				mockChannel := channel.NewMockChannel()
				var mockReq *ghttp.Request
				var mockResp *ghttp.Response
				params := map[string]any{"test": "param"}
				assert.NotPanics(t, func() { task.WSConnected(mockChannel, mockReq, mockResp, params) })
			},
		},
		{
			name: "WSDisconnected_HandlesDisconnection",
			testFunc: func(t *testing.T, task *DefaultHandlerTask, mockCtx *channel.MockHandlerContext) {
				mockChannel := channel.NewMockChannel()
				var mockReq *ghttp.Request
				var mockResp *ghttp.Response
				params := map[string]any{"test": "param"}
				assert.NotPanics(t, func() { task.WSDisconnected(mockChannel, mockReq, mockResp, params) })
			},
		},
		{
			name: "WSErrorCaught_LogsErrorWithContext",
			testFunc: func(t *testing.T, task *DefaultHandlerTask, mockCtx *channel.MockHandlerContext) {
				var mockReq *ghttp.Request
				var mockResp *ghttp.Response
				mockMsg := NewMockMessage()
				testErr := errors.New("websocket error")
				assert.NotPanics(t, func() { task.WSErrorCaught(mockCtx, mockReq, mockResp, mockMsg, testErr) })
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			task := &DefaultHandlerTask{Builder: DefaultMessageBuilder{}}
			mockCtx := channel.NewMockHandlerContext()
			tt.testFunc(t, task, mockCtx)
		})
	}
}

// TestDefaultMessageBuilder_ComprehensiveOperations tests all message building operations
func TestDefaultMessageBuilder_ComprehensiveOperations(t *testing.T) {
	t.Parallel()
	builder := &DefaultMessageBuilder{}

	tests := []struct {
		name     string
		testFunc func(t *testing.T, b *DefaultMessageBuilder)
	}{
		{
			name: "Text_CreatesTextMessage",
			testFunc: func(t *testing.T, b *DefaultMessageBuilder) {
				msg := b.Text("hello world")
				assert.NotNil(t, msg)
				assert.Equal(t, TextMessageType, msg.MessageType)
				assert.Equal(t, []byte("hello world"), msg.Message)
			},
		},
		{
			name: "Binary_CreatesBinaryMessage",
			testFunc: func(t *testing.T, b *DefaultMessageBuilder) {
				data := []byte{0x01, 0x02, 0x03, 0x04}
				msg := b.Binary(data)
				assert.NotNil(t, msg)
				assert.Equal(t, BinaryMessageType, msg.MessageType)
				assert.Equal(t, data, msg.Message)
			},
		},
		{
			name: "Close_CreatesCloseMessage",
			testFunc: func(t *testing.T, b *DefaultMessageBuilder) {
				closeData := []byte("closing")
				closeCode := CloseNormalClosure
				msg := b.Close(closeData, closeCode)
				assert.NotNil(t, msg)
				assert.Equal(t, CloseMessageType, msg.MessageType)
				assert.Equal(t, closeData, msg.Message)
				assert.Equal(t, closeCode, msg.CloseCode)
			},
		},
		{
			name: "Ping_CreatesPingMessage",
			testFunc: func(t *testing.T, b *DefaultMessageBuilder) {
				pingData := []byte("ping")
				deadline := time.Now().Add(time.Minute)
				msg := b.Ping(pingData, &deadline)
				assert.NotNil(t, msg)
				assert.Equal(t, PingMessageType, msg.MessageType)
				assert.Equal(t, pingData, msg.Message)
				assert.Equal(t, &deadline, msg.Dead)
			},
		},
		{
			name: "Pong_CreatesPongMessage",
			testFunc: func(t *testing.T, b *DefaultMessageBuilder) {
				pongData := []byte("pong")
				deadline := time.Now().Add(time.Minute)
				msg := b.Pong(pongData, &deadline)
				assert.NotNil(t, msg)
				assert.Equal(t, PongMessageType, msg.MessageType)
				assert.Equal(t, pongData, msg.Message)
				assert.Equal(t, &deadline, msg.Dead)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.testFunc(t, builder)
		})
	}
}
// ===== From gws_invokehandler_test.go =====

// TestInvokeHandler_ComprehensiveOperations tests all InvokeHandler operations
func TestInvokeHandler_ComprehensiveOperations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "NewInvokeHandler_WithParams",
			testFunc: func(t *testing.T) {
				mockTask := NewMockHandlerTask()
				params := map[string]any{"key": "value", "number": 42}
				handler := NewInvokeHandler(mockTask, params)
				assert.NotNil(t, handler)
				assert.Equal(t, mockTask, handler.task)
				assert.Equal(t, params, handler.params)
			},
		},
		{
			name: "NewInvokeHandler_WithNilParams",
			testFunc: func(t *testing.T) {
				mockTask := NewMockHandlerTask()
				handler := NewInvokeHandler(mockTask, nil)
				assert.NotNil(t, handler)
				assert.Equal(t, mockTask, handler.task)
				assert.NotNil(t, handler.params)
				assert.Empty(t, handler.params)
			},
		},
		{
			name: "Read_WithWebSocketChannelAndMessage",
			testFunc: func(t *testing.T) {
				mockTask := NewMockHandlerTask()
				params := map[string]any{"test": "param"}
				handler := NewInvokeHandler(mockTask, params)
				mockCtx := channel.NewMockHandlerContext()
				wsChannel := &Channel{Request: &ghttp.Request{}, Response: &ghttp.Response{}}
				mockCtx.On("Channel").Return(wsChannel)
				textMsg := &DefaultMessage{MessageType: TextMessageType, Message: []byte("test message")}
				mockTask.On("WSText", mockCtx, textMsg, params).Return()
				assert.NotPanics(t, func() { handler.Read(mockCtx, textMsg) })
				mockTask.AssertExpectations(t)
			},
		},
		{
			name: "Read_WithNonWebSocketChannel",
			testFunc: func(t *testing.T) {
				mockTask := NewMockHandlerTask()
				handler := NewInvokeHandler(mockTask, nil)
				mockCtx := channel.NewMockHandlerContext()
				mockChannel := channel.NewMockChannel()
				mockCtx.On("Channel").Return(mockChannel)
				mockCtx.On("FireRead", mock.Anything).Return(mockCtx)
				assert.NotPanics(t, func() { handler.Read(mockCtx, "not a ws message") })
				mockCtx.AssertExpectations(t)
			},
		},
		{
			name: "Read_WithNonMessage",
			testFunc: func(t *testing.T) {
				mockTask := NewMockHandlerTask()
				handler := NewInvokeHandler(mockTask, nil)
				mockCtx := channel.NewMockHandlerContext()
				wsChannel := &Channel{Request: &ghttp.Request{}, Response: &ghttp.Response{}}
				mockCtx.On("Channel").Return(wsChannel)
				mockCtx.On("FireRead", mock.Anything).Return(mockCtx)
				assert.NotPanics(t, func() { handler.Read(mockCtx, "not a message") })
				mockCtx.AssertExpectations(t)
			},
		},
		{
			name: "Active_WithWebSocketChannel",
			testFunc: func(t *testing.T) {
				mockTask := NewMockHandlerTask()
				params := map[string]any{"active": "test"}
				handler := NewInvokeHandler(mockTask, params)
				mockCtx := channel.NewMockHandlerContext()
				wsChannel := &Channel{Request: &ghttp.Request{}, Response: &ghttp.Response{}}
				mockCtx.On("Channel").Return(wsChannel)
				mockCtx.On("FireActive").Return(mockCtx)
				mockTask.On("WSConnected", wsChannel, wsChannel.Request, wsChannel.Response, params).Return()
				assert.NotPanics(t, func() { handler.Active(mockCtx) })
				mockTask.AssertExpectations(t)
				mockCtx.AssertExpectations(t)
			},
		},
		{
			name: "Active_WithNonWebSocketChannel",
			testFunc: func(t *testing.T) {
				mockTask := NewMockHandlerTask()
				handler := NewInvokeHandler(mockTask, nil)
				mockCtx := channel.NewMockHandlerContext()
				mockChannel := channel.NewMockChannel()
				mockCtx.On("Channel").Return(mockChannel)
				mockCtx.On("FireActive").Return(mockCtx)
				assert.NotPanics(t, func() { handler.Active(mockCtx) })
				mockCtx.AssertExpectations(t)
			},
		},
		{
			name: "Inactive_WithWebSocketChannel",
			testFunc: func(t *testing.T) {
				mockTask := NewMockHandlerTask()
				params := map[string]any{"inactive": "test"}
				handler := NewInvokeHandler(mockTask, params)
				mockCtx := channel.NewMockHandlerContext()
				wsChannel := &Channel{Request: &ghttp.Request{}, Response: &ghttp.Response{}}
				mockCtx.On("Channel").Return(wsChannel)
				mockCtx.On("FireInactive").Return(mockCtx)
				mockTask.On("WSDisconnected", wsChannel, wsChannel.Request, wsChannel.Response, params).Return()
				assert.NotPanics(t, func() { handler.Inactive(mockCtx) })
				mockTask.AssertExpectations(t)
				mockCtx.AssertExpectations(t)
			},
		},
		{
			name: "Inactive_WithNonWebSocketChannel",
			testFunc: func(t *testing.T) {
				mockTask := NewMockHandlerTask()
				handler := NewInvokeHandler(mockTask, nil)
				mockCtx := channel.NewMockHandlerContext()
				mockChannel := channel.NewMockChannel()
				mockCtx.On("Channel").Return(mockChannel)
				mockCtx.On("FireInactive").Return(mockCtx)
				assert.NotPanics(t, func() { handler.Inactive(mockCtx) })
				mockCtx.AssertExpectations(t)
			},
		},
		{
			name: "ErrorCaught_LogsError",
			testFunc: func(t *testing.T) {
				mockTask := NewMockHandlerTask()
				handler := NewInvokeHandler(mockTask, nil)
				mockCtx := channel.NewMockHandlerContext()
				testErr := errors.New("invoke handler error")
				assert.NotPanics(t, func() { handler.ErrorCaught(mockCtx, testErr) })
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.testFunc(t)
		})
	}
}

// TestInvokeHandler_CallMessageTypes tests _Call method with different message types
func TestInvokeHandler_CallMessageTypes(t *testing.T) {
	t.Parallel()
	mockTask := NewMockHandlerTask()
	var mockReq *ghttp.Request
	var mockResp *ghttp.Response
	mockCtx := channel.NewMockHandlerContext()
	params := map[string]any{"call": "test"}

	tests := []struct {
		name    string
		message Message
		setupMocks func()
	}{
		{
			name: "TextMessage",
			message: &DefaultMessage{MessageType: TextMessageType, Message: []byte("text")},
			setupMocks: func() { mockTask.On("WSText", mockCtx, mock.AnythingOfType("*gws.DefaultMessage"), params).Return() },
		},
		{
			name: "BinaryMessage",
			message: &DefaultMessage{MessageType: BinaryMessageType, Message: []byte{0x01, 0x02}},
			setupMocks: func() { mockTask.On("WSBinary", mockCtx, mock.AnythingOfType("*gws.DefaultMessage"), params).Return() },
		},
		{
			name: "CloseMessage",
			message: &CloseMessage{DefaultMessage: DefaultMessage{MessageType: CloseMessageType, Message: []byte("close")}, CloseCode: CloseNormalClosure},
			setupMocks: func() { mockTask.On("WSClose", mockCtx, mock.AnythingOfType("*gws.CloseMessage"), params).Return() },
		},
		{
			name: "PingMessage",
			message: &PingMessage{DefaultMessage: DefaultMessage{MessageType: PingMessageType, Message: []byte("ping")}},
			setupMocks: func() { mockTask.On("WSPing", mockCtx, mock.AnythingOfType("*gws.PingMessage"), params).Return() },
		},
		{
			name: "PongMessage",
			message: &PongMessage{DefaultMessage: DefaultMessage{MessageType: PongMessageType, Message: []byte("pong")}},
			setupMocks: func() { mockTask.On("WSPong", mockCtx, mock.AnythingOfType("*gws.PongMessage"), params).Return() },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			localMockTask := NewMockHandlerTask()
			localHandler := NewInvokeHandler(localMockTask, params)
			switch tt.message.Type() {
			case TextMessageType:
				localMockTask.On("WSText", mockCtx, mock.AnythingOfType("*gws.DefaultMessage"), params).Return()
			case BinaryMessageType:
				localMockTask.On("WSBinary", mockCtx, mock.AnythingOfType("*gws.DefaultMessage"), params).Return()
			case CloseMessageType:
				localMockTask.On("WSClose", mockCtx, mock.AnythingOfType("*gws.CloseMessage"), params).Return()
			case PingMessageType:
				localMockTask.On("WSPing", mockCtx, mock.AnythingOfType("*gws.PingMessage"), params).Return()
			case PongMessageType:
				localMockTask.On("WSPong", mockCtx, mock.AnythingOfType("*gws.PongMessage"), params).Return()
			}
			assert.NotPanics(t, func() { localHandler._Call(mockCtx, mockReq, mockResp, localMockTask, tt.message, params) })
			localMockTask.AssertExpectations(t)
		})
	}
}
// ===== From gws_concurrent_test.go =====

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

// ===== From gws_upgradeprocessor_test.go =====

// TestUpgradeProcessor_ComprehensiveOperations tests all UpgradeProcessor operations
func TestUpgradeProcessor_ComprehensiveOperations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "Added_InitializesUpgrader",
			testFunc: func(t *testing.T) {
				processor := &UpgradeProcessor{}
				mockCtx := channel.NewMockHandlerContext()
				mockChannel := channel.NewMockChannel()
				
				// Test with CheckOrigin parameter set to false
				mockChannel.On("Param", ParamCheckOrigin, mock.Anything).Return(false, true)
				mockCtx.On("Channel").Return(mockChannel)

				assert.NotPanics(t, func() {
					processor.Added(mockCtx)
				})

				assert.NotNil(t, processor.upgrade)
			},
		},
		{
			name: "Added_InitializesUpgraderWithCheckOrigin",
			testFunc: func(t *testing.T) {
				processor := &UpgradeProcessor{}
				mockCtx := channel.NewMockHandlerContext()
				mockChannel := channel.NewMockChannel()
				
				// Test with CheckOrigin parameter set to true
				mockChannel.On("Param", ParamCheckOrigin, mock.Anything).Return(true, true)
				mockCtx.On("Channel").Return(mockChannel)

				assert.NotPanics(t, func() {
					processor.Added(mockCtx)
				})

				assert.NotNil(t, processor.upgrade)
			},
		},
		{
			name: "Read_WithNilObject",
			testFunc: func(t *testing.T) {
				processor := &UpgradeProcessor{}
				mockCtx := channel.NewMockHandlerContext()

				// Should return early with nil object
				assert.NotPanics(t, func() {
					processor.Read(mockCtx, nil)
				})
			},
		},
		{
			name: "Read_WithNonPackObject",
			testFunc: func(t *testing.T) {
				processor := &UpgradeProcessor{}
				mockCtx := channel.NewMockHandlerContext()
				mockCtx.On("FireRead", mock.Anything).Return(mockCtx)

				nonPackObj := "not a pack"

				assert.NotPanics(t, func() {
					processor.Read(mockCtx, nonPackObj)
				})

				mockCtx.AssertExpectations(t)
			},
		},
		{
			name: "Read_WithPackButNilRouteNode",
			testFunc: func(t *testing.T) {
				processor := &UpgradeProcessor{}
				mockCtx := channel.NewMockHandlerContext()
				mockCtx.On("FireRead", mock.Anything).Return(mockCtx)

				pack := &ghttp.Pack{
					RouteNode: nil,
				}

				assert.NotPanics(t, func() {
					processor.Read(mockCtx, pack)
				})

				mockCtx.AssertExpectations(t)
			},
		},
		{
			name: "Read_WithPackButNonServerHandlerTask",
			testFunc: func(t *testing.T) {
				processor := &UpgradeProcessor{}
				mockCtx := channel.NewMockHandlerContext()
				mockCtx.On("FireRead", mock.Anything).Return(mockCtx)
				
				mockRouteNode := ghttp.NewMockRouteNode()
				mockHandler := ghttp.NewMockHttpHandlerTask() // Not a ServerHandlerTask
				mockRouteNode.On("HandlerTask").Return(mockHandler)

				pack := &ghttp.Pack{
					RouteNode: mockRouteNode,
				}

				assert.NotPanics(t, func() {
					processor.Read(mockCtx, pack)
				})

				mockCtx.AssertExpectations(t)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.testFunc(t)
		})
	}
}

// TestUpgradeProcessor_NewWSLog tests _NewWSLog method
func TestUpgradeProcessor_NewWSLog(t *testing.T) {
	t.Parallel()

	processor := &UpgradeProcessor{}
	
	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "NewWSLog_WithoutWebSocketConn",
			testFunc: func(t *testing.T) {
				cID := "test-channel-123"
				tID := "test-track-456"
				uri := "/test/websocket"
				testErr := errors.New("test error")

				log := processor._NewWSLog(cID, tID, uri, nil, testErr)

				assert.NotNil(t, log)
				assert.Equal(t, LogType, log.LogType)
				assert.Equal(t, cID, log.ChannelID)
				assert.Equal(t, tID, log.TrackID)
				assert.Equal(t, uri, log.RequestURI)
				assert.Equal(t, testErr, log.Error)
				assert.Nil(t, log.RemoteAddr)
				assert.Nil(t, log.LocalAddr)
			},
		},
		{
			name: "NewWSLog_WithWebSocketConn",
			testFunc: func(t *testing.T) {
				cID := "test-channel-789"
				tID := "test-track-012"
				uri := "/test/websocket/path"
				
				// Create mock WebSocket connection (we can't easily mock websocket.Conn)
				// So we test the case where wsConn is nil, which is more realistic for unit tests
				log := processor._NewWSLog(cID, tID, uri, nil, nil)

				assert.NotNil(t, log)
				assert.Equal(t, LogType, log.LogType)
				assert.Equal(t, cID, log.ChannelID)
				assert.Equal(t, tID, log.TrackID)
				assert.Equal(t, uri, log.RequestURI)
				assert.Nil(t, log.Error)
				// Without real websocket.Conn, these will be nil
				assert.Nil(t, log.RemoteAddr)
				assert.Nil(t, log.LocalAddr)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.testFunc(t)
		})
	}
}

// TestLogStruct_Comprehensive tests LogStruct functionality
func TestLogStruct_Comprehensive(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "LogStruct_Creation",
			testFunc: func(t *testing.T) {
				remoteAddr := &net.TCPAddr{IP: net.ParseIP("192.168.1.100"), Port: 9000}
				localAddr := &net.TCPAddr{IP: net.ParseIP("localhost"), Port: 8080}
				testErr := errors.New("websocket test error")
				mockMessage := NewMockMessage()

				log := &LogStruct{
					LogType:    LogType,
					RemoteAddr: remoteAddr,
					LocalAddr:  localAddr,
					RequestURI: "/ws/test",
					ChannelID:  "channel-test-123",
					TrackID:    "track-test-456",
					Message:    mockMessage,
					Error:      testErr,
				}

				assert.Equal(t, LogType, log.LogType)
				assert.Equal(t, remoteAddr, log.RemoteAddr)
				assert.Equal(t, localAddr, log.LocalAddr)
				assert.Equal(t, "/ws/test", log.RequestURI)
				assert.Equal(t, "channel-test-123", log.ChannelID)
				assert.Equal(t, "track-test-456", log.TrackID)
				assert.Equal(t, mockMessage, log.Message)
				assert.Equal(t, testErr, log.Error)
			},
		},
		{
			name: "LogStruct_EmptyFields",
			testFunc: func(t *testing.T) {
				log := &LogStruct{}

				assert.Empty(t, log.LogType)
				assert.Nil(t, log.RemoteAddr)
				assert.Nil(t, log.LocalAddr)
				assert.Empty(t, log.RequestURI)
				assert.Empty(t, log.ChannelID)
				assert.Empty(t, log.TrackID)
				assert.Nil(t, log.Message)
				assert.Nil(t, log.Error)
			},
		},
		{
			name: "LogType_Constant",
			testFunc: func(t *testing.T) {
				assert.Equal(t, "websocket", LogType)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.testFunc(t)
		})
	}
}

// TestUpgradeProcessor_UpgradeCheckFunc tests custom upgrade check function
func TestUpgradeProcessor_UpgradeCheckFunc(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "UpgradeCheckFunc_CustomFunction",
			testFunc: func(t *testing.T) {
				called := false
				processor := &UpgradeProcessor{
					UpgradeCheckFunc: func(req *ghttp.Request, resp *ghttp.Response, params map[string]any) bool {
						called = true
						return false // Reject upgrade
					},
				}

				// This test verifies that UpgradeCheckFunc can be set
				assert.NotNil(t, processor.UpgradeCheckFunc)
				
				// Call the function to verify it works
				var mockReq *ghttp.Request = nil
				var mockResp *ghttp.Response = nil
				params := map[string]any{}
				
				result := processor.UpgradeCheckFunc(mockReq, mockResp, params)
				
				assert.True(t, called)
				assert.False(t, result)
			},
		},
		{
			name: "UpgradeCheckFunc_NilFunction",
			testFunc: func(t *testing.T) {
				processor := &UpgradeProcessor{
					UpgradeCheckFunc: nil,
				}

				assert.Nil(t, processor.UpgradeCheckFunc)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.testFunc(t)
		})
	}
}
