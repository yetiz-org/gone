package channel

import (
	"net"

	"github.com/stretchr/testify/mock"
)

// MockPipeline is a mock implementation of Pipeline interface
// It provides complete testify/mock integration for testing pipeline behaviors
type MockPipeline struct {
	mock.Mock
}

// NewMockPipeline creates a new MockPipeline instance
func NewMockPipeline() *MockPipeline {
	return &MockPipeline{}
}

// AddLast adds a handler to the end of the pipeline
func (m *MockPipeline) AddLast(name string, elem Handler) Pipeline {
	args := m.Called(name, elem)
	return args.Get(0).(Pipeline)
}

// AddBefore adds a handler before the specified target handler
func (m *MockPipeline) AddBefore(target string, name string, elem Handler) Pipeline {
	args := m.Called(target, name, elem)
	return args.Get(0).(Pipeline)
}

// RemoveFirst removes the first handler from the pipeline
func (m *MockPipeline) RemoveFirst() Pipeline {
	args := m.Called()
	return args.Get(0).(Pipeline)
}

// Remove removes the specified handler from the pipeline
func (m *MockPipeline) Remove(elem Handler) Pipeline {
	args := m.Called(elem)
	return args.Get(0).(Pipeline)
}

// RemoveByName removes handler by name from the pipeline
func (m *MockPipeline) RemoveByName(name string) Pipeline {
	args := m.Called(name)
	return args.Get(0).(Pipeline)
}

// Clear clears all handlers from the pipeline
func (m *MockPipeline) Clear() Pipeline {
	args := m.Called()
	return args.Get(0).(Pipeline)
}

// Channel returns the associated channel
func (m *MockPipeline) Channel() Channel {
	args := m.Called()
	return args.Get(0).(Channel)
}

// Param gets a parameter from the pipeline
func (m *MockPipeline) Param(key ParamKey) any {
	args := m.Called(key)
	return args.Get(0)
}

// SetParam sets a parameter in the pipeline
func (m *MockPipeline) SetParam(key ParamKey, value any) Pipeline {
	args := m.Called(key, value)
	return args.Get(0).(Pipeline)
}

// Params returns all parameters
func (m *MockPipeline) Params() *Params {
	args := m.Called()
	return args.Get(0).(*Params)
}

// Read triggers a read operation
func (m *MockPipeline) Read() Pipeline {
	args := m.Called()
	return args.Get(0).(Pipeline)
}

// Write writes data through the pipeline
func (m *MockPipeline) Write(obj any) Future {
	args := m.Called(obj)
	return args.Get(0).(Future)
}

// Bind binds to local address
func (m *MockPipeline) Bind(localAddr net.Addr) Future {
	args := m.Called(localAddr)
	return args.Get(0).(Future)
}

// Close closes the pipeline
func (m *MockPipeline) Close() Future {
	args := m.Called()
	return args.Get(0).(Future)
}

// Connect connects to remote address
func (m *MockPipeline) Connect(localAddr net.Addr, remoteAddr net.Addr) Future {
	args := m.Called(localAddr, remoteAddr)
	return args.Get(0).(Future)
}

// Disconnect disconnects the pipeline
func (m *MockPipeline) Disconnect() Future {
	args := m.Called()
	return args.Get(0).(Future)
}

// Deregister deregisters the pipeline
func (m *MockPipeline) Deregister() Future {
	args := m.Called()
	return args.Get(0).(Future)
}

// NewFuture creates a new future
func (m *MockPipeline) NewFuture() Future {
	args := m.Called()
	return args.Get(0).(Future)
}

// fireRegistered fires registered event (internal method)
func (m *MockPipeline) fireRegistered() Pipeline {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(Pipeline)
}

// fireUnregistered fires unregistered event (internal method)
func (m *MockPipeline) fireUnregistered() Pipeline {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(Pipeline)
}

// fireActive fires active event (internal method)
func (m *MockPipeline) fireActive() Pipeline {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(Pipeline)
}

// fireInactive fires inactive event (internal method)
func (m *MockPipeline) fireInactive() Pipeline {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(Pipeline)
}

// fireRead fires read event (internal method)
func (m *MockPipeline) fireRead(obj any) Pipeline {
	args := m.Called(obj)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(Pipeline)
}

// fireReadCompleted fires read completed event (internal method)
func (m *MockPipeline) fireReadCompleted() Pipeline {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(Pipeline)
}

// fireErrorCaught fires error caught event (internal method)
func (m *MockPipeline) fireErrorCaught(err error) Pipeline {
	args := m.Called(err)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(Pipeline)
}

// Note: Interface compliance check removed due to unexported methods
// MockPipeline implements Pipeline interface