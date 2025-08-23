package channel

import (
	"time"

	"github.com/stretchr/testify/mock"
	concurrent "github.com/yetiz-org/goth-concurrent"
)

// MockFuture is a mock implementation of Future interface
// It provides complete testify/mock integration for testing future behaviors
type MockFuture struct {
	mock.Mock
	channel Channel
}

// NewMockFuture creates a new MockFuture instance
func NewMockFuture(ch interface{}) *MockFuture {
	var channelRef Channel
	if ch != nil {
		if c, ok := ch.(Channel); ok {
			channelRef = c
		}
	}
	return &MockFuture{
		channel: channelRef,
	}
}

// Sync waits for the future to complete and returns itself
func (m *MockFuture) Sync() Future {
	args := m.Called()
	return args.Get(0).(Future)
}

// Channel returns the associated channel
func (m *MockFuture) Channel() Channel {
	args := m.Called()
	if args.Get(0) != nil {
		return args.Get(0).(Channel)
	}
	return m.channel
}

// Await waits for the future to complete
func (m *MockFuture) Await() concurrent.Future {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(concurrent.Future)
}

// Additional methods for MockFuture (required for interface compliance)
func (m *MockFuture) AddListener(listener concurrent.FutureListener) concurrent.Future {
	args := m.Called(listener)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(concurrent.Future)
}

func (m *MockFuture) AwaitTimeout(timeout time.Duration) concurrent.Future {
	args := m.Called(timeout)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(concurrent.Future)
}

func (m *MockFuture) Chainable() concurrent.ChainFuture {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(concurrent.ChainFuture)
}

func (m *MockFuture) Done() <-chan struct{} {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(<-chan struct{})
}

func (m *MockFuture) Error() error {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(error)
}

func (m *MockFuture) GetNow() any {
	args := m.Called()
	return args.Get(0)
}

func (m *MockFuture) GetTimeout(timeout time.Duration) any {
	args := m.Called(timeout)
	return args.Get(0)
}

func (m *MockFuture) Immutable() concurrent.Immutable {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(concurrent.Immutable)
}

func (m *MockFuture) IsFail() bool {
	args := m.Called()
	return args.Bool(0)
}

// AwaitUninterruptibly waits for the future to complete uninterruptibly
func (m *MockFuture) AwaitUninterruptibly() {
	m.Called()
}

// AwaitWithTimeout waits for the future to complete with timeout
func (m *MockFuture) AwaitWithTimeout(timeout time.Duration) bool {
	args := m.Called(timeout)
	return args.Bool(0)
}

// AwaitUninterruptiblyWithTimeout waits for the future to complete uninterruptibly with timeout
func (m *MockFuture) AwaitUninterruptiblyWithTimeout(timeout time.Duration) bool {
	args := m.Called(timeout)
	return args.Bool(0)
}

// IsDone returns whether the future is done
func (m *MockFuture) IsDone() bool {
	args := m.Called()
	return args.Bool(0)
}

// IsSuccess returns whether the future completed successfully
func (m *MockFuture) IsSuccess() bool {
	args := m.Called()
	return args.Bool(0)
}

// IsCancelled returns whether the future was cancelled
func (m *MockFuture) IsCancelled() bool {
	args := m.Called()
	return args.Bool(0)
}

// Cause returns the cause of failure
func (m *MockFuture) Cause() error {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(error)
}

// Get returns the result
func (m *MockFuture) Get() any {
	args := m.Called()
	return args.Get(0)
}

// Completable returns the completable interface
func (m *MockFuture) Completable() concurrent.Completable {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(concurrent.Completable)
}

// Ensure MockFuture implements Future interface
var _ Future = (*MockFuture)(nil)