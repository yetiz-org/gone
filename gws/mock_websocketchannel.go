package gws

import (
	"github.com/stretchr/testify/mock"
	"github.com/yetiz-org/gone/channel"
	"github.com/yetiz-org/gone/ghttp"
	"github.com/gorilla/websocket"
)

// MockWebSocketChannel is a mock implementation of WebSocket Channel
type MockWebSocketChannel struct {
	mock.Mock
	*channel.DefaultNetChannel
	wsConn   *websocket.Conn
	Response *ghttp.Response
	Request  *ghttp.Request
}

// NewMockWebSocketChannel creates a new MockWebSocketChannel instance
func NewMockWebSocketChannel() *MockWebSocketChannel {
	return &MockWebSocketChannel{
		DefaultNetChannel: &channel.DefaultNetChannel{},
	}
}

// BootstrapPreInit mocks the BootstrapPreInit method
func (m *MockWebSocketChannel) BootstrapPreInit() {
	m.Called()
}

// Init mocks the Init method
func (m *MockWebSocketChannel) Init() channel.Channel {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(channel.Channel)
}

// UnsafeWrite mocks the UnsafeWrite method with WebSocket message handling
func (m *MockWebSocketChannel) UnsafeWrite(obj any) error {
	args := m.Called(obj)
	return args.Error(0)
}

// UnsafeRead mocks the UnsafeRead method for WebSocket message reading
func (m *MockWebSocketChannel) UnsafeRead() (any, error) {
	args := m.Called()
	return args.Get(0), args.Error(1)
}