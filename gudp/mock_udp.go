package gudp

import (
	"net"
	"github.com/stretchr/testify/mock"
	"github.com/yetiz-org/gone/channel"
)

// MockUdpChannel is a mock implementation of UDP Channel
type MockUdpChannel struct {
	mock.Mock
	channel.DefaultNetChannel
}

// NewMockUdpChannel creates a new MockUdpChannel instance
func NewMockUdpChannel() *MockUdpChannel {
	return &MockUdpChannel{}
}

// UnsafeConnect mocks the UnsafeConnect method with UDP address validation
func (m *MockUdpChannel) UnsafeConnect(localAddr net.Addr, remoteAddr net.Addr) error {
	args := m.Called(localAddr, remoteAddr)
	return args.Error(0)
}

// MockUdpServerChannel is a mock implementation of UDP ServerChannel
type MockUdpServerChannel struct {
	mock.Mock
	channel.DefaultNetServerChannel
	conn   *net.UDPConn
	active bool
}

// NewMockUdpServerChannel creates a new MockUdpServerChannel instance
func NewMockUdpServerChannel() *MockUdpServerChannel {
	return &MockUdpServerChannel{}
}

// UnsafeBind mocks the UnsafeBind method with UDP binding logic
func (m *MockUdpServerChannel) UnsafeBind(localAddr net.Addr) error {
	args := m.Called(localAddr)
	return args.Error(0)
}

// UnsafeAccept mocks the UnsafeAccept method 
func (m *MockUdpServerChannel) UnsafeAccept() (channel.Channel, channel.Future) {
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
func (m *MockUdpServerChannel) UnsafeClose() error {
	args := m.Called()
	return args.Error(0)
}

// IsActive mocks the IsActive method
func (m *MockUdpServerChannel) IsActive() bool {
	args := m.Called()
	return args.Bool(0)
}
