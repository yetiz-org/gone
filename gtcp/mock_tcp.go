package gtcp

import (
	"net"
	"github.com/yetiz-org/gone/channel"
)

// MockTcpChannel is a mock implementation of TCP Channel
// It embeds MockNetChannel to provide proper mock inheritance
type MockTcpChannel struct {
	channel.MockNetChannel
}

// NewMockTcpChannel creates a new MockTcpChannel instance
func NewMockTcpChannel() *MockTcpChannel {
	return &MockTcpChannel{
		MockNetChannel: *channel.NewMockNetChannel(),
	}
}

// UnsafeConnect mocks the UnsafeConnect method with TCP address validation
func (m *MockTcpChannel) UnsafeConnect(localAddr net.Addr, remoteAddr net.Addr) error {
	args := m.Called(localAddr, remoteAddr)
	return args.Error(0)
}

// MockTcpServerChannel is a mock implementation of TCP ServerChannel
// It embeds MockServerChannel to provide proper mock inheritance
type MockTcpServerChannel struct {
	channel.MockServerChannel
	listen net.Listener
	active bool
}

// NewMockTcpServerChannel creates a new MockTcpServerChannel instance
func NewMockTcpServerChannel() *MockTcpServerChannel {
	return &MockTcpServerChannel{
		MockServerChannel: *channel.NewMockServerChannel(),
	}
}

// UnsafeBind mocks the UnsafeBind method with TCP binding logic
func (m *MockTcpServerChannel) UnsafeBind(localAddr net.Addr) error {
	args := m.Called(localAddr)
	return args.Error(0)
}

// UnsafeAccept mocks the UnsafeAccept method 
func (m *MockTcpServerChannel) UnsafeAccept() (channel.Channel, channel.Future) {
	args := m.Called()
	var ch channel.Channel
	if args.Get(0) != nil {
		ch = args.Get(0).(channel.Channel)
	}
	var future channel.Future
	if args.Get(1) != nil {
		future = args.Get(1).(channel.Future)
	}
	return ch, future
}

// UnsafeClose mocks the UnsafeClose method
func (m *MockTcpServerChannel) UnsafeClose() error {
	args := m.Called()
	return args.Error(0)
}

// IsActive mocks the IsActive method
func (m *MockTcpServerChannel) IsActive() bool {
	args := m.Called()
	return args.Bool(0)
}

// Ensure interface compliance
var _ channel.NetChannel = (*MockTcpChannel)(nil)
var _ channel.ServerChannel = (*MockTcpServerChannel)(nil)