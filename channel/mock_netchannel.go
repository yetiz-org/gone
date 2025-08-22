package channel

import (
	"net"

	concurrent "github.com/yetiz-org/goth-concurrent"
)

// MockNetChannel is a mock implementation of NetChannel interface
// It embeds MockChannel to provide all Channel functionality plus network-specific features
type MockNetChannel struct {
	MockChannel
	conn Conn
}

// NewMockNetChannel creates a new MockNetChannel instance
func NewMockNetChannel() *MockNetChannel {
	return &MockNetChannel{
		MockChannel: *NewMockChannel(),
		conn:        NewMockConn(),
	}
}

// NewMockNetChannelWithConn creates a new MockNetChannel with specified connection
func NewMockNetChannelWithConn(conn Conn) *MockNetChannel {
	return &MockNetChannel{
		MockChannel: *NewMockChannel(),
		conn:        conn,
	}
}

// Conn returns the connection
func (m *MockNetChannel) Conn() Conn {
	args := m.Called()
	if args.Get(0) != nil {
		return args.Get(0).(Conn)
	}
	return m.conn
}

// RemoteAddr returns the remote address
func (m *MockNetChannel) RemoteAddr() net.Addr {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(net.Addr)
}

// SetConn sets the connection (public method for NetChannelSetConn interface)
func (m *MockNetChannel) SetConn(conn net.Conn) {
	m.Called(conn)
}

// UnsafeWrite writes data unsafely
func (m *MockNetChannel) UnsafeWrite(obj any) error {
	args := m.Called(obj)
	return args.Error(0)
}

// UnsafeRead reads data unsafely
func (m *MockNetChannel) UnsafeRead() (any, error) {
	args := m.Called()
	return args.Get(0), args.Error(1)
}

// UnsafeDisconnect disconnects unsafely
func (m *MockNetChannel) UnsafeDisconnect() error {
	args := m.Called()
	return args.Error(0)
}

// UnsafeConnect connects to remote address unsafely
func (m *MockNetChannel) UnsafeConnect(localAddr net.Addr, remoteAddr net.Addr) error {
	args := m.Called(localAddr, remoteAddr)
	return args.Error(0)
}

// UnsafeIsAutoRead returns whether auto-read is enabled
func (m *MockNetChannel) UnsafeIsAutoRead() bool {
	args := m.Called()
	return args.Bool(0)
}

// activeChannel activates the channel (internal method)
func (m *MockNetChannel) activeChannel() {
	m.Called()
}

// inactiveChannel deactivates the channel (internal method)
func (m *MockNetChannel) inactiveChannel() (bool, concurrent.Future) {
	args := m.Called()
	return args.Bool(0), args.Get(1).(concurrent.Future)
}

// Internal methods for MockNetChannel (required for interface compliance)
func (m *MockNetChannel) init(channel Channel) {
	m.Called(channel)
}

func (m *MockNetChannel) setConn(conn net.Conn) {
	m.Called(conn)
}

func (m *MockNetChannel) release() {
	m.Called()
}

func (m *MockNetChannel) setCloseFuture(future Future) {
	m.Called(future)
}

// MockNetChannel implements NetChannel interface