package channel

import (
	"net"

	concurrent "github.com/yetiz-org/goth-concurrent"
)

// MockServerChannel is a mock implementation of ServerChannel interface
// It embeds MockChannel to provide all Channel functionality plus server-specific features
type MockServerChannel struct {
	MockChannel
}

// NewMockServerChannel creates a new MockServerChannel instance
func NewMockServerChannel() *MockServerChannel {
	return &MockServerChannel{
		MockChannel: *NewMockChannel(),
	}
}

// ChildParams returns child parameters
func (m *MockServerChannel) ChildParams() *Params {
	args := m.Called()
	return args.Get(0).(*Params)
}

// UnsafeBind binds to local address unsafely
func (m *MockServerChannel) UnsafeBind(localAddr net.Addr) error {
	args := m.Called(localAddr)
	return args.Error(0)
}

// UnsafeAccept accepts new connections unsafely
func (m *MockServerChannel) UnsafeAccept() (Channel, Future) {
	args := m.Called()
	return args.Get(0).(Channel), args.Get(1).(Future)
}

// UnsafeClose closes the server channel unsafely
func (m *MockServerChannel) UnsafeClose() error {
	args := m.Called()
	return args.Error(0)
}

// UnsafeRead reads data unsafely
func (m *MockServerChannel) UnsafeRead() (any, error) {
	args := m.Called()
	return args.Get(0), args.Error(1)
}

// UnsafeIsAutoRead returns whether auto-read is enabled
func (m *MockServerChannel) UnsafeIsAutoRead() bool {
	args := m.Called()
	return args.Bool(0)
}

// activeChannel activates the channel (internal method)
func (m *MockServerChannel) activeChannel() {
	m.Called()
}

// inactiveChannel deactivates the channel (internal method)
func (m *MockServerChannel) inactiveChannel() (bool, concurrent.Future) {
	args := m.Called()
	return args.Bool(0), args.Get(1).(concurrent.Future)
}

// Internal methods for MockServerChannel (required for interface compliance)
func (m *MockServerChannel) init(channel Channel) {
	m.Called(channel)
}

func (m *MockServerChannel) setChildHandler(handler Handler) ServerChannel {
	args := m.Called(handler)
	return args.Get(0).(ServerChannel)
}

func (m *MockServerChannel) setChildParams(key ParamKey, value any) {
	m.Called(key, value)
}

func (m *MockServerChannel) releaseChild(channel Channel) {
	m.Called(channel)
}

func (m *MockServerChannel) waitChildren() {
	m.Called()
}

func (m *MockServerChannel) release() {
	m.Called()
}

func (m *MockServerChannel) setCloseFuture(future Future) {
	m.Called(future)
}

// Ensure MockServerChannel implements ServerChannel interface
var _ ServerChannel = (*MockServerChannel)(nil)

// MockServerChannel implements ServerChannel interface
