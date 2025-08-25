package ghttp

import (
	"context"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/yetiz-org/gone/channel"
)

// MockNetChannel implements NetChannel interface for testing
type MockNetChannel struct {
	channel.DefaultNetChannel
	deregisterCount int32
	isActiveMock    bool
	idMock          string
	mu              sync.Mutex
	deactivated     int32 // tracks if channel has been deactivated
}

func NewMockNetChannel(id string, isActive bool) *MockNetChannel {
	mock := &MockNetChannel{
		isActiveMock: isActive,
		idMock:       id,
	}
	mock.Init()
	return mock
}

func (m *MockNetChannel) Deregister() channel.Future {
	atomic.AddInt32(&m.deregisterCount, 1)
	// Simulate atomic behavior - only first call actually deactivates
	if atomic.CompareAndSwapInt32(&m.deactivated, 0, 1) {
		m.SetActive(false)
	}
	return channel.NewFuture(m)
}

func (m *MockNetChannel) IsActive() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Channel is active if it was initially set as active AND hasn't been deactivated
	return m.isActiveMock && atomic.LoadInt32(&m.deactivated) == 0
}

func (m *MockNetChannel) SetActive(active bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.isActiveMock = active
}

func (m *MockNetChannel) ID() string {
	return m.idMock
}

func (m *MockNetChannel) GetDeregisterCount() int32 {
	return atomic.LoadInt32(&m.deregisterCount)
}

func (m *MockNetChannel) IsDeactivated() bool {
	return atomic.LoadInt32(&m.deactivated) == 1
}

// MockServer wraps http.Server to allow shutdown behavior override
type MockServer struct {
	*http.Server
	shutdownBehavior func(context.Context) error
}

func NewMockServer(addr string, connState func(net.Conn, http.ConnState)) *MockServer {
	server := &http.Server{
		Addr:      addr,
		ConnState: connState,
	}
	return &MockServer{
		Server: server,
	}
}

func (m *MockServer) Shutdown(ctx context.Context) error {
	if m.shutdownBehavior != nil {
		return m.shutdownBehavior(ctx)
	}
	return m.Server.Shutdown(ctx)
}

func (m *MockServer) SetShutdownBehavior(behavior func(context.Context) error) {
	m.shutdownBehavior = behavior
}

// MockAddr implements net.Addr for testing
type MockAddr struct {
	network string
	address string
}

func (m *MockAddr) Network() string { return m.network }
func (m *MockAddr) String() string  { return m.address }

// TestChannelRangeLoopDeregistration tests the Range loop deregistration logic directly
func TestChannelRangeLoopDeregistration(t *testing.T) {
	serverChannel := &ServerChannel{}
	serverChannel.Init()
	serverChannel.Name = "test-server"

	// Create mock channels that are initially active
	mockCh1 := NewMockNetChannel("test-ch-1", true)
	mockCh2 := NewMockNetChannel("test-ch-2", true)

	// Add them directly to chMap (simulating active connections)
	conn1 := &mockConn{id: "conn1"}
	conn2 := &mockConn{id: "conn2"}
	serverChannel.chMap.Store(conn1, mockCh1)
	serverChannel.chMap.Store(conn2, mockCh2)

	// Verify initial state
	assert.True(t, mockCh1.IsActive(), "Channel 1 should be initially active")
	assert.True(t, mockCh2.IsActive(), "Channel 2 should be initially active")
	assert.Equal(t, int32(0), mockCh1.GetDeregisterCount(), "Channel 1 should have 0 deregister calls initially")
	assert.Equal(t, int32(0), mockCh2.GetDeregisterCount(), "Channel 2 should have 0 deregister calls initially")

	// Simulate the Range loop logic from UnsafeClose (when server.Shutdown fails)
	serverChannel.chMap.Range(func(key, value interface{}) bool {
		ch := value.(channel.NetChannel)
		if ch.IsActive() {
			ch.Deregister()
		}
		serverChannel.chMap.Delete(key)
		return true
	})

	// Verify that channels were deregistered exactly once through Range loop
	assert.Equal(t, int32(1), mockCh1.GetDeregisterCount(), "Channel 1 should be deregistered exactly once")
	assert.Equal(t, int32(1), mockCh2.GetDeregisterCount(), "Channel 2 should be deregistered exactly once")
	assert.True(t, mockCh1.IsDeactivated(), "Channel 1 should be deactivated")
	assert.True(t, mockCh2.IsDeactivated(), "Channel 2 should be deactivated")
	assert.False(t, mockCh1.IsActive(), "Channel 1 should not be active after deregistration")
	assert.False(t, mockCh2.IsActive(), "Channel 2 should not be active after deregistration")
}

// TestLoadAndDeleteRaceCondition tests concurrent LoadAndDelete and Range operations
func TestLoadAndDeleteRaceCondition(t *testing.T) {
	serverChannel := &ServerChannel{}
	serverChannel.Init()

	mockCh := NewMockNetChannel("test-ch", true)
	conn := &mockConn{id: "test-conn"}
	serverChannel.chMap.Store(conn, mockCh)

	var wg sync.WaitGroup

	// Simulate StateClosed callback doing LoadAndDelete
	wg.Add(1)
	go func() {
		defer wg.Done()
		if v, found := serverChannel.chMap.LoadAndDelete(conn); found {
			ch := v.(channel.NetChannel)
			if ch.IsActive() {
				ch.Deregister()
			}
		}
	}()

	// Simulate Range loop trying to access the same connection
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(1 * time.Millisecond) // Small delay to create race condition
		serverChannel.chMap.Range(func(key, value interface{}) bool {
			if key == conn {
				ch := value.(channel.NetChannel)
				if ch.IsActive() {
					ch.Deregister()
				}
				serverChannel.chMap.Delete(key)
			}
			return true
		})
	}()

	wg.Wait()

	// In race condition, channel should be deregistered at least once but not more than twice
	// Due to atomic protection in MockNetChannel, actual deactivation happens only once
	count := mockCh.GetDeregisterCount()
	assert.True(t, count >= 1, "Channel should be deregistered at least once")
	assert.True(t, count <= 2, "Channel should not be deregistered more than twice")
	assert.True(t, mockCh.IsDeactivated(), "Channel should be deactivated exactly once")
	assert.False(t, mockCh.IsActive(), "Channel should be inactive")
}

// TestServerChannelMultipleDeregisterSafety tests that multiple Deregister calls are safe
func TestServerChannelMultipleDeregisterSafety(t *testing.T) {
	mockCh := NewMockNetChannel("test-ch", true)

	// Call Deregister multiple times concurrently
	var wg sync.WaitGroup
	concurrentCalls := 10

	for i := 0; i < concurrentCalls; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mockCh.Deregister()
		}()
	}

	wg.Wait()

	// All calls should be counted, but only first one actually deactivates
	count := mockCh.GetDeregisterCount()
	assert.Equal(t, int32(concurrentCalls), count, "All Deregister calls should be counted")
	assert.True(t, mockCh.IsDeactivated(), "Channel should be deactivated after first deregistration")
	assert.False(t, mockCh.IsActive(), "Channel should be inactive after deregistration")
}

// mockConn is a minimal implementation of net.Conn for testing
type mockConn struct {
	id string
}

func (m *mockConn) Read(b []byte) (n int, err error)   { return 0, nil }
func (m *mockConn) Write(b []byte) (n int, err error)  { return len(b), nil }
func (m *mockConn) Close() error                       { return nil }
func (m *mockConn) LocalAddr() net.Addr                { return &net.TCPAddr{} }
func (m *mockConn) RemoteAddr() net.Addr               { return &net.TCPAddr{} }
func (m *mockConn) SetDeadline(t time.Time) error      { return nil }
func (m *mockConn) SetReadDeadline(t time.Time) error  { return nil }
func (m *mockConn) SetWriteDeadline(t time.Time) error { return nil }
