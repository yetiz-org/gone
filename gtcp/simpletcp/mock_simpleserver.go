package simpletcp

import (
	"net"
	"github.com/stretchr/testify/mock"
	"github.com/yetiz-org/gone/channel"
)

// MockSimpleServer is a mock implementation of SimpleServer
type MockSimpleServer struct {
	mock.Mock
	Handler channel.Handler
}

// NewMockSimpleServer creates a new MockSimpleServer instance
func NewMockSimpleServer() *MockSimpleServer {
	return &MockSimpleServer{}
}

// Start mocks the Start method
func (m *MockSimpleServer) Start(localAddr net.Addr) channel.Channel {
	args := m.Called(localAddr)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(channel.Channel)
}

// Channel mocks the Channel method
func (m *MockSimpleServer) Channel() channel.Channel {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(channel.Channel)
}

// Stop mocks the Stop method
func (m *MockSimpleServer) Stop() channel.Future {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(channel.Future)
}