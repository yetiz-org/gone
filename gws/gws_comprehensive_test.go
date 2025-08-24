package gws

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/yetiz-org/gone/channel"
	"github.com/yetiz-org/gone/ghttp"
)

// TestDefaultHandlerTask_ComprehensiveWSOperations tests all WebSocket handler operations
func TestDefaultHandlerTask_ComprehensiveWSOperations(t *testing.T) {
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
		{
			name: "WSPong_HandlesMessage",
			testFunc: func(t *testing.T, task *DefaultHandlerTask, mockCtx *channel.MockHandlerContext) {
				pongMsg := &PongMessage{
					DefaultMessage: DefaultMessage{
						MessageType: PongMessageType,
						Message:     []byte("pong test"),
					},
				}
				params := map[string]any{"test": "param"}

				// WSPong is a no-op method, should not panic
				assert.NotPanics(t, func() {
					task.WSPong(mockCtx, pongMsg, params)
				})
			},
		},
		{
			name: "WSClose_HandlesMessage",
			testFunc: func(t *testing.T, task *DefaultHandlerTask, mockCtx *channel.MockHandlerContext) {
				closeMsg := &CloseMessage{
					DefaultMessage: DefaultMessage{
						MessageType: CloseMessageType,
						Message:     []byte("close test"),
					},
					CloseCode: CloseNormalClosure,
				}
				params := map[string]any{"test": "param"}

				// WSClose is a no-op method, should not panic
				assert.NotPanics(t, func() {
					task.WSClose(mockCtx, closeMsg, params)
				})
			},
		},
		{
			name: "WSBinary_HandlesMessage",
			testFunc: func(t *testing.T, task *DefaultHandlerTask, mockCtx *channel.MockHandlerContext) {
				binaryMsg := &DefaultMessage{
					MessageType: BinaryMessageType,
					Message:     []byte("binary test data"),
				}
				params := map[string]any{"test": "param"}

				// WSBinary is a no-op method, should not panic
				assert.NotPanics(t, func() {
					task.WSBinary(mockCtx, binaryMsg, params)
				})
			},
		},
		{
			name: "WSText_HandlesMessage",
			testFunc: func(t *testing.T, task *DefaultHandlerTask, mockCtx *channel.MockHandlerContext) {
				textMsg := &DefaultMessage{
					MessageType: TextMessageType,
					Message:     []byte("text test data"),
				}
				params := map[string]any{"test": "param"}

				// WSText is a no-op method, should not panic
				assert.NotPanics(t, func() {
					task.WSText(mockCtx, textMsg, params)
				})
			},
		},
		{
			name: "WSConnected_HandlesConnection",
			testFunc: func(t *testing.T, task *DefaultHandlerTask, mockCtx *channel.MockHandlerContext) {
				mockChannel := channel.NewMockChannel()
				var mockReq *ghttp.Request = nil
				var mockResp *ghttp.Response = nil
				params := map[string]any{"test": "param"}

				// WSConnected is a no-op method, should not panic
				assert.NotPanics(t, func() {
					task.WSConnected(mockChannel, mockReq, mockResp, params)
				})
			},
		},
		{
			name: "WSDisconnected_HandlesDisconnection",
			testFunc: func(t *testing.T, task *DefaultHandlerTask, mockCtx *channel.MockHandlerContext) {
				mockChannel := channel.NewMockChannel()
				var mockReq *ghttp.Request = nil
				var mockResp *ghttp.Response = nil
				params := map[string]any{"test": "param"}

				// WSDisconnected is a no-op method, should not panic
				assert.NotPanics(t, func() {
					task.WSDisconnected(mockChannel, mockReq, mockResp, params)
				})
			},
		},
		{
			name: "WSErrorCaught_LogsErrorWithContext",
			testFunc: func(t *testing.T, task *DefaultHandlerTask, mockCtx *channel.MockHandlerContext) {
				var mockReq *ghttp.Request = nil
				var mockResp *ghttp.Response = nil
				mockMsg := NewMockMessage()
				testErr := errors.New("websocket error")

				// WSErrorCaught should handle error gracefully
				assert.NotPanics(t, func() {
					task.WSErrorCaught(mockCtx, mockReq, mockResp, mockMsg, testErr)
				})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			
			task := &DefaultHandlerTask{
				Builder: DefaultMessageBuilder{},
			}
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