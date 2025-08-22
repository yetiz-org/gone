package gws

import (
	"github.com/stretchr/testify/mock"
	"github.com/yetiz-org/gone/channel"
	"github.com/yetiz-org/gone/ghttp"
)

// MockServerHandlerTask is a mock implementation of ServerHandlerTask interface
type MockServerHandlerTask struct {
	mock.Mock
}

// NewMockServerHandlerTask creates a new MockServerHandlerTask instance
func NewMockServerHandlerTask() *MockServerHandlerTask {
	return &MockServerHandlerTask{}
}

// WSUpgrade mocks the WSUpgrade method specific to ServerHandlerTask
func (m *MockServerHandlerTask) WSUpgrade(req *ghttp.Request, resp *ghttp.Response, params map[string]any) bool {
	args := m.Called(req, resp, params)
	return args.Bool(0)
}

// All HandlerTask methods are embedded and need to be implemented
func (m *MockServerHandlerTask) WSPing(ctx channel.HandlerContext, message *PingMessage, params map[string]any) {
	m.Called(ctx, message, params)
}

func (m *MockServerHandlerTask) WSPong(ctx channel.HandlerContext, message *PongMessage, params map[string]any) {
	m.Called(ctx, message, params)
}

func (m *MockServerHandlerTask) WSClose(ctx channel.HandlerContext, message *CloseMessage, params map[string]any) {
	m.Called(ctx, message, params)
}

func (m *MockServerHandlerTask) WSBinary(ctx channel.HandlerContext, message *DefaultMessage, params map[string]any) {
	m.Called(ctx, message, params)
}

func (m *MockServerHandlerTask) WSText(ctx channel.HandlerContext, message *DefaultMessage, params map[string]any) {
	m.Called(ctx, message, params)
}

func (m *MockServerHandlerTask) WSConnected(ch channel.Channel, req *ghttp.Request, resp *ghttp.Response, params map[string]any) {
	m.Called(ch, req, resp, params)
}

func (m *MockServerHandlerTask) WSDisconnected(ch channel.Channel, req *ghttp.Request, resp *ghttp.Response, params map[string]any) {
	m.Called(ch, req, resp, params)
}

func (m *MockServerHandlerTask) WSErrorCaught(ctx channel.HandlerContext, req *ghttp.Request, resp *ghttp.Response, msg Message, err error) {
	m.Called(ctx, req, resp, msg, err)
}