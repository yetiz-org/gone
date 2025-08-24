package gws

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/yetiz-org/gone/channel"
	"github.com/yetiz-org/gone/ghttp"
)

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
				wsChannel := &Channel{
					Request:  &ghttp.Request{},
					Response: &ghttp.Response{},
				}
				mockCtx.On("Channel").Return(wsChannel)
				
				textMsg := &DefaultMessage{
					MessageType: TextMessageType,
					Message:     []byte("test message"),
				}
				
				mockTask.On("WSText", mockCtx, textMsg, params).Return()
				
				assert.NotPanics(t, func() {
					handler.Read(mockCtx, textMsg)
				})
				
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
				
				testObj := "not a ws message"
				
				assert.NotPanics(t, func() {
					handler.Read(mockCtx, testObj)
				})
				
				mockCtx.AssertExpectations(t)
			},
		},
		{
			name: "Read_WithNonMessage",
			testFunc: func(t *testing.T) {
				mockTask := NewMockHandlerTask()
				handler := NewInvokeHandler(mockTask, nil)
				
				mockCtx := channel.NewMockHandlerContext()
				wsChannel := &Channel{
					Request:  &ghttp.Request{},
					Response: &ghttp.Response{},
				}
				mockCtx.On("Channel").Return(wsChannel)
				mockCtx.On("FireRead", mock.Anything).Return(mockCtx)
				
				notMessage := "not a message"
				
				assert.NotPanics(t, func() {
					handler.Read(mockCtx, notMessage)
				})
				
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
				wsChannel := &Channel{
					Request:  &ghttp.Request{},
					Response: &ghttp.Response{},
				}
				mockCtx.On("Channel").Return(wsChannel)
				mockCtx.On("FireActive").Return(mockCtx)
				
				mockTask.On("WSConnected", wsChannel, wsChannel.Request, wsChannel.Response, params).Return()
				
				assert.NotPanics(t, func() {
					handler.Active(mockCtx)
				})
				
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
				
				assert.NotPanics(t, func() {
					handler.Active(mockCtx)
				})
				
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
				wsChannel := &Channel{
					Request:  &ghttp.Request{},
					Response: &ghttp.Response{},
				}
				mockCtx.On("Channel").Return(wsChannel)
				mockCtx.On("FireInactive").Return(mockCtx)
				
				mockTask.On("WSDisconnected", wsChannel, wsChannel.Request, wsChannel.Response, params).Return()
				
				assert.NotPanics(t, func() {
					handler.Inactive(mockCtx)
				})
				
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
				
				assert.NotPanics(t, func() {
					handler.Inactive(mockCtx)
				})
				
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
				
				assert.NotPanics(t, func() {
					handler.ErrorCaught(mockCtx, testErr)
				})
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
	var mockReq *ghttp.Request = nil
	var mockResp *ghttp.Response = nil
	mockCtx := channel.NewMockHandlerContext()
	params := map[string]any{"call": "test"}

	tests := []struct {
		name        string
		message     Message
		setupMocks  func()
	}{
		{
			name: "TextMessage",
			message: &DefaultMessage{
				MessageType: TextMessageType,
				Message:     []byte("text"),
			},
			setupMocks: func() {
				mockTask.On("WSText", mockCtx, mock.AnythingOfType("*gws.DefaultMessage"), params).Return()
			},
		},
		{
			name: "BinaryMessage",
			message: &DefaultMessage{
				MessageType: BinaryMessageType,
				Message:     []byte{0x01, 0x02},
			},
			setupMocks: func() {
				mockTask.On("WSBinary", mockCtx, mock.AnythingOfType("*gws.DefaultMessage"), params).Return()
			},
		},
		{
			name: "CloseMessage",
			message: &CloseMessage{
				DefaultMessage: DefaultMessage{
					MessageType: CloseMessageType,
					Message:     []byte("close"),
				},
				CloseCode: CloseNormalClosure,
			},
			setupMocks: func() {
				mockTask.On("WSClose", mockCtx, mock.AnythingOfType("*gws.CloseMessage"), params).Return()
			},
		},
		{
			name: "PingMessage",
			message: &PingMessage{
				DefaultMessage: DefaultMessage{
					MessageType: PingMessageType,
					Message:     []byte("ping"),
				},
			},
			setupMocks: func() {
				mockTask.On("WSPing", mockCtx, mock.AnythingOfType("*gws.PingMessage"), params).Return()
			},
		},
		{
			name: "PongMessage",
			message: &PongMessage{
				DefaultMessage: DefaultMessage{
					MessageType: PongMessageType,
					Message:     []byte("pong"),
				},
			},
			setupMocks: func() {
				mockTask.On("WSPong", mockCtx, mock.AnythingOfType("*gws.PongMessage"), params).Return()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			
			// Reset mocks for each test
			localMockTask := NewMockHandlerTask()
			localHandler := NewInvokeHandler(localMockTask, params)
			tt.setupMocks = func() {
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
			}
			tt.setupMocks()
			
			assert.NotPanics(t, func() {
				localHandler._Call(mockCtx, mockReq, mockResp, localMockTask, tt.message, params)
			})
			
			localMockTask.AssertExpectations(t)
		})
	}
}