package channel

import (
	"context"
	"net"
	"time"

	"github.com/stretchr/testify/mock"
)

// MockHandlerContext is a mock implementation of HandlerContext interface
// It provides complete testify/mock integration for testing handler context behaviors
type MockHandlerContext struct {
	mock.Mock
	ctx context.Context
}

// NewMockHandlerContext creates a new MockHandlerContext instance
func NewMockHandlerContext() *MockHandlerContext {
	return &MockHandlerContext{
		ctx: context.Background(),
	}
}

// NewMockHandlerContextWithContext creates a new MockHandlerContext with specified context
func NewMockHandlerContextWithContext(ctx context.Context) *MockHandlerContext {
	return &MockHandlerContext{
		ctx: ctx,
	}
}

// WithValue returns a new context with the specified key-value pair
func (m *MockHandlerContext) WithValue(key, val any) HandlerContext {
	args := m.Called(key, val)
	return args.Get(0).(HandlerContext)
}

// Name returns the handler context name
func (m *MockHandlerContext) Name() string {
	args := m.Called()
	return args.String(0)
}

// Channel returns the associated channel
func (m *MockHandlerContext) Channel() Channel {
	args := m.Called()
	return args.Get(0).(Channel)
}

// FireRegistered fires a registered event
func (m *MockHandlerContext) FireRegistered() HandlerContext {
	args := m.Called()
	return args.Get(0).(HandlerContext)
}

// FireUnregistered fires an unregistered event
func (m *MockHandlerContext) FireUnregistered() HandlerContext {
	args := m.Called()
	return args.Get(0).(HandlerContext)
}

// FireActive fires an active event
func (m *MockHandlerContext) FireActive() HandlerContext {
	args := m.Called()
	return args.Get(0).(HandlerContext)
}

// FireInactive fires an inactive event
func (m *MockHandlerContext) FireInactive() HandlerContext {
	args := m.Called()
	return args.Get(0).(HandlerContext)
}

// FireRead fires a read event
func (m *MockHandlerContext) FireRead(obj any) HandlerContext {
	args := m.Called(obj)
	return args.Get(0).(HandlerContext)
}

// FireReadCompleted fires a read completed event
func (m *MockHandlerContext) FireReadCompleted() HandlerContext {
	args := m.Called()
	return args.Get(0).(HandlerContext)
}

// FireErrorCaught fires an error caught event
func (m *MockHandlerContext) FireErrorCaught(err error) HandlerContext {
	args := m.Called(err)
	return args.Get(0).(HandlerContext)
}

// Write writes data through the context
func (m *MockHandlerContext) Write(obj any, future Future) Future {
	args := m.Called(obj, future)
	return args.Get(0).(Future)
}

// Bind binds to local address
func (m *MockHandlerContext) Bind(localAddr net.Addr, future Future) Future {
	args := m.Called(localAddr, future)
	return args.Get(0).(Future)
}

// Close closes the context
func (m *MockHandlerContext) Close(future Future) Future {
	args := m.Called(future)
	return args.Get(0).(Future)
}

// Connect connects to remote address
func (m *MockHandlerContext) Connect(localAddr net.Addr, remoteAddr net.Addr, future Future) Future {
	args := m.Called(localAddr, remoteAddr, future)
	return args.Get(0).(Future)
}

// Disconnect disconnects the context
func (m *MockHandlerContext) Disconnect(future Future) Future {
	args := m.Called(future)
	return args.Get(0).(Future)
}

// Deregister deregisters the context
func (m *MockHandlerContext) Deregister(future Future) Future {
	args := m.Called(future)
	return args.Get(0).(Future)
}

// Deadline returns the deadline of the context
func (m *MockHandlerContext) Deadline() (deadline time.Time, ok bool) {
	args := m.Called()
	return args.Get(0).(time.Time), args.Bool(1)
}

// Done returns the done channel
func (m *MockHandlerContext) Done() <-chan struct{} {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(<-chan struct{})
}

// Err returns the context error
func (m *MockHandlerContext) Err() error {
	args := m.Called()
	return args.Error(0)
}

// Value returns the value associated with key
func (m *MockHandlerContext) Value(key any) any {
	args := m.Called(key)
	return args.Get(0)
}

// _Context returns the underlying context.Context (internal method)
func (m *MockHandlerContext) _Context() context.Context {
	args := m.Called()
	if args.Get(0) == nil {
		return context.Background()
	}
	return args.Get(0).(context.Context)
}

// Internal methods for MockHandlerContext (required for interface compliance)
func (m *MockHandlerContext) prev() HandlerContext {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(HandlerContext)
}

func (m *MockHandlerContext) setPrev(prev HandlerContext) HandlerContext {
	args := m.Called(prev)
	return args.Get(0).(HandlerContext)
}

func (m *MockHandlerContext) next() HandlerContext {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(HandlerContext)
}

func (m *MockHandlerContext) setNext(next HandlerContext) HandlerContext {
	args := m.Called(next)
	return args.Get(0).(HandlerContext)
}

func (m *MockHandlerContext) deferErrorCaught() {
	m.Called()
}

func (m *MockHandlerContext) checkFuture(future Future) Future {
	args := m.Called(future)
	return args.Get(0).(Future)
}

func (m *MockHandlerContext) handler() Handler {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(Handler)
}

// Ensure MockHandlerContext implements HandlerContext interface
var _ HandlerContext = (*MockHandlerContext)(nil)

// MockHandlerContext implements HandlerContext interface