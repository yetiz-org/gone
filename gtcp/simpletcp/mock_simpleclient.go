package simpletcp

import (
	"net"
	"github.com/stretchr/testify/mock"
	"github.com/yetiz-org/gone/channel"
	buf "github.com/yetiz-org/goth-bytebuf"
)

// MockSimpleClient is a mock implementation of SimpleClient
type MockSimpleClient struct {
	mock.Mock
	AutoReconnect func() bool
	Handler       channel.Handler
}

// NewMockSimpleClient creates a new MockSimpleClient instance
func NewMockSimpleClient() *MockSimpleClient {
	return &MockSimpleClient{}
}

// Start mocks the Start method
func (m *MockSimpleClient) Start(remoteAddr net.Addr) channel.Channel {
	args := m.Called(remoteAddr)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(channel.Channel)
}

// Channel mocks the Channel method
func (m *MockSimpleClient) Channel() channel.Channel {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(channel.Channel)
}

// Write mocks the Write method
func (m *MockSimpleClient) Write(buffer buf.ByteBuf) channel.Future {
	args := m.Called(buffer)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(channel.Future)
}

// Disconnect mocks the Disconnect method
func (m *MockSimpleClient) Disconnect() channel.Future {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(channel.Future)
}