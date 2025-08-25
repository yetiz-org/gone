package channel

import (
	"net"
	"time"

	"github.com/stretchr/testify/mock"
)

// MockConn is a mock implementation of Conn interface
// It provides complete testify/mock integration for testing connection behaviors
type MockConn struct {
	mock.Mock
}

// NewMockConn creates a new MockConn instance
func NewMockConn() *MockConn {
	return &MockConn{}
}

// Read reads data from the connection
func (m *MockConn) Read(b []byte) (n int, err error) {
	args := m.Called(b)
	return args.Int(0), args.Error(1)
}

// Write writes data to the connection
func (m *MockConn) Write(b []byte) (n int, err error) {
	args := m.Called(b)
	return args.Int(0), args.Error(1)
}

// Close closes the connection
func (m *MockConn) Close() error {
	args := m.Called()
	return args.Error(0)
}

// LocalAddr returns the local network address
func (m *MockConn) LocalAddr() net.Addr {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(net.Addr)
}

// RemoteAddr returns the remote network address
func (m *MockConn) RemoteAddr() net.Addr {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(net.Addr)
}

// SetDeadline sets the read and write deadlines
func (m *MockConn) SetDeadline(t time.Time) error {
	args := m.Called(t)
	return args.Error(0)
}

// SetReadDeadline sets the read deadline
func (m *MockConn) SetReadDeadline(t time.Time) error {
	args := m.Called(t)
	return args.Error(0)
}

// SetWriteDeadline sets the write deadline
func (m *MockConn) SetWriteDeadline(t time.Time) error {
	args := m.Called(t)
	return args.Error(0)
}

// Conn returns the underlying net.Conn
func (m *MockConn) Conn() net.Conn {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(net.Conn)
}

// IsActive returns whether the connection is active
func (m *MockConn) IsActive() bool {
	args := m.Called()
	return args.Bool(0)
}

// Ensure MockConn implements Conn interface
var _ Conn = (*MockConn)(nil)
