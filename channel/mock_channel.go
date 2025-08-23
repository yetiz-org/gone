package channel

import (
	"net"

	"github.com/stretchr/testify/mock"
	concurrent "github.com/yetiz-org/goth-concurrent"
)

// MockChannel is a mock implementation of Channel interface
// It provides complete testify/mock integration for testing channel behaviors
type MockChannel struct {
	mock.Mock
	id     string
	serial uint64
}

// NewMockChannel creates a new MockChannel instance with default configuration
func NewMockChannel() *MockChannel {
	return &MockChannel{
		id:     "mock-channel-id",
		serial: 1,
	}
}

// NewMockChannelWithID creates a new MockChannel with specified ID and serial
func NewMockChannelWithID(id string, serial uint64) *MockChannel {
	return &MockChannel{
		id:     id,
		serial: serial,
	}
}

// Serial returns the channel serial number
func (m *MockChannel) Serial() uint64 {
	args := m.Called()
	if args.Get(0) != nil {
		return args.Get(0).(uint64)
	}
	return m.serial
}

// ID returns the channel ID
func (m *MockChannel) ID() string {
	args := m.Called()
	if args.Get(0) != nil {
		return args.Get(0).(string)
	}
	return m.id
}

// Init initializes the channel and returns itself
func (m *MockChannel) Init() Channel {
	args := m.Called()
	return args.Get(0).(Channel)
}

// Pipeline returns the channel pipeline
func (m *MockChannel) Pipeline() Pipeline {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(Pipeline)
}

// CloseFuture returns the close future
func (m *MockChannel) CloseFuture() Future {
	args := m.Called()
	return args.Get(0).(Future)
}

// Bind binds to local address
func (m *MockChannel) Bind(localAddr net.Addr) Future {
	args := m.Called(localAddr)
	return args.Get(0).(Future)
}

// Close closes the channel
func (m *MockChannel) Close() Future {
	args := m.Called()
	return args.Get(0).(Future)
}

// Connect connects to remote address
func (m *MockChannel) Connect(localAddr net.Addr, remoteAddr net.Addr) Future {
	args := m.Called(localAddr, remoteAddr)
	return args.Get(0).(Future)
}

// Disconnect disconnects the channel
func (m *MockChannel) Disconnect() Future {
	args := m.Called()
	return args.Get(0).(Future)
}

// Deregister deregisters the channel
func (m *MockChannel) Deregister() Future {
	args := m.Called()
	return args.Get(0).(Future)
}

// Read reads from the channel
func (m *MockChannel) Read() Channel {
	args := m.Called()
	return args.Get(0).(Channel)
}

// FireRead fires a read event
func (m *MockChannel) FireRead(obj any) Channel {
	args := m.Called(obj)
	return args.Get(0).(Channel)
}

// FireReadCompleted fires a read completed event
func (m *MockChannel) FireReadCompleted() Channel {
	args := m.Called()
	return args.Get(0).(Channel)
}

// Write writes to the channel
func (m *MockChannel) Write(obj any) Future {
	args := m.Called(obj)
	return args.Get(0).(Future)
}

// IsActive returns whether the channel is active
func (m *MockChannel) IsActive() bool {
	args := m.Called()
	return args.Bool(0)
}

// SetParam sets a parameter
func (m *MockChannel) SetParam(key ParamKey, value any) {
	m.Called(key, value)
}

// Param gets a parameter
func (m *MockChannel) Param(key ParamKey) any {
	args := m.Called(key)
	return args.Get(0)
}

// Params returns all parameters
func (m *MockChannel) Params() *Params {
	args := m.Called()
	return args.Get(0).(*Params)
}

// Parent returns the parent server channel
func (m *MockChannel) Parent() ServerChannel {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(ServerChannel)
}

// LocalAddr returns the local address
func (m *MockChannel) LocalAddr() net.Addr {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(net.Addr)
}

// Internal methods for MockChannel (required for interface compliance)
func (m *MockChannel) activeChannel() {
	m.Called()
}

func (m *MockChannel) init(channel Channel) {
	m.Called(channel)
}

func (m *MockChannel) unsafe() Unsafe {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(Unsafe)
}

func (m *MockChannel) setLocalAddr(addr net.Addr) {
	m.Called(addr)
}

func (m *MockChannel) setPipeline(pipeline Pipeline) {
	m.Called(pipeline)
}

func (m *MockChannel) setUnsafe(unsafe Unsafe) {
	m.Called(unsafe)
}

func (m *MockChannel) setParent(channel ServerChannel) {
	m.Called(channel)
}

func (m *MockChannel) release() {
	m.Called()
}

func (m *MockChannel) inactiveChannel() (bool, concurrent.Future) {
	args := m.Called()
	return args.Bool(0), args.Get(1).(concurrent.Future)
}

func (m *MockChannel) setCloseFuture(future Future) {
	m.Called(future)
}

// Ensure MockChannel implements Channel interface
var _ Channel = (*MockChannel)(nil)