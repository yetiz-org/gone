package gws

import (
	"github.com/stretchr/testify/mock"
	"github.com/yetiz-org/gone/channel"
	"github.com/yetiz-org/gone/ghttp"
)

// MockHandlerTask is a mock implementation of HandlerTask interface
type MockHandlerTask struct {
	mock.Mock
}

// NewMockHandlerTask creates a new MockHandlerTask instance
func NewMockHandlerTask() *MockHandlerTask {
	return &MockHandlerTask{}
}

// WSPing mocks the WSPing method
func (m *MockHandlerTask) WSPing(ctx channel.HandlerContext, message *PingMessage, params map[string]any) {
	m.Called(ctx, message, params)
}

// WSPong mocks the WSPong method
func (m *MockHandlerTask) WSPong(ctx channel.HandlerContext, message *PongMessage, params map[string]any) {
	m.Called(ctx, message, params)
}

// WSClose mocks the WSClose method
func (m *MockHandlerTask) WSClose(ctx channel.HandlerContext, message *CloseMessage, params map[string]any) {
	m.Called(ctx, message, params)
}

// WSBinary mocks the WSBinary method
func (m *MockHandlerTask) WSBinary(ctx channel.HandlerContext, message *DefaultMessage, params map[string]any) {
	m.Called(ctx, message, params)
}

// WSText mocks the WSText method
func (m *MockHandlerTask) WSText(ctx channel.HandlerContext, message *DefaultMessage, params map[string]any) {
	m.Called(ctx, message, params)
}

// WSConnected mocks the WSConnected method
func (m *MockHandlerTask) WSConnected(ch channel.Channel, req *ghttp.Request, resp *ghttp.Response, params map[string]any) {
	m.Called(ch, req, resp, params)
}

// WSDisconnected mocks the WSDisconnected method
func (m *MockHandlerTask) WSDisconnected(ch channel.Channel, req *ghttp.Request, resp *ghttp.Response, params map[string]any) {
	m.Called(ch, req, resp, params)
}

// WSErrorCaught mocks the WSErrorCaught method
func (m *MockHandlerTask) WSErrorCaught(ctx channel.HandlerContext, req *ghttp.Request, resp *ghttp.Response, msg Message, err error) {
	m.Called(ctx, req, resp, msg, err)
}
