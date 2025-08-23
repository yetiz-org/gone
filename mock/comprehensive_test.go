package mock

import (
	"testing"
	"net"
	"net/http"

	"github.com/stretchr/testify/assert"
	"github.com/yetiz-org/gone/gws"
)

// TestChannelMocks verifies all channel mock implementations
func TestChannelMocks(t *testing.T) {
	t.Run("MockChannel", func(t *testing.T) {
		mock := NewMockChannel()
		assert.NotNil(t, mock)
		
		// Test basic mock functionality
		mock.On("ID").Return("test-channel-123")
		id := mock.ID()
		assert.Equal(t, "test-channel-123", id)
		mock.AssertExpectations(t)
	})

	t.Run("MockPipeline", func(t *testing.T) {
		mock := NewMockPipeline()
		assert.NotNil(t, mock)
		
		mockFuture := NewMockFuture(nil)
		mock.On("NewFuture").Return(mockFuture)
		
		future := mock.NewFuture()
		assert.Equal(t, mockFuture, future)
		mock.AssertExpectations(t)
	})

	t.Run("MockHandler", func(t *testing.T) {
		mock := NewMockHandler()
		assert.NotNil(t, mock)
		
		mockCtx := NewMockHandlerContext()
		mock.On("Added", mockCtx).Once()
		mock.Added(mockCtx)
		mock.AssertExpectations(t)
	})

	t.Run("MockFuture", func(t *testing.T) {
		mock := NewMockFuture(nil)
		assert.NotNil(t, mock)
		
		mock.On("IsSuccess").Return(true)
		success := mock.IsSuccess()
		assert.True(t, success)
		mock.AssertExpectations(t)
	})

	t.Run("MockConn", func(t *testing.T) {
		mock := NewMockConn()
		assert.NotNil(t, mock)
		
		mock.On("IsActive").Return(true)
		active := mock.IsActive()
		assert.True(t, active)
		mock.AssertExpectations(t)
	})
}

// TestHTTPMocks verifies all HTTP mock implementations
func TestHTTPMocks(t *testing.T) {
	t.Run("MockHTTPServerChannel", func(t *testing.T) {
		mock := NewMockHTTPServerChannel()
		assert.NotNil(t, mock)
		
		req, _ := http.NewRequest("GET", "/test", nil)
		rw := &mockResponseWriter{}
		
		mock.On("ServeHTTP", rw, req).Once()
		mock.ServeHTTP(rw, req)
		mock.AssertExpectations(t)
	})

	t.Run("MockRequest", func(t *testing.T) {
		mock := NewMockRequest()
		assert.NotNil(t, mock)
		
		req, _ := http.NewRequest("POST", "/api", nil)
		mock.On("GetRequest").Return(req)
		
		result := mock.GetRequest()
		assert.Equal(t, req, result)
		mock.AssertExpectations(t)
	})

	t.Run("MockResponse", func(t *testing.T) {
		mock := NewMockResponse()
		assert.NotNil(t, mock)
		
		mock.On("Status").Return(200)
		mock.On("SetStatus", 404).Once()
		
		status := mock.Status()
		assert.Equal(t, 200, status)
		
		mock.SetStatus(404)
		mock.AssertExpectations(t)
	})
}

// TestTCPMocks verifies all TCP mock implementations
func TestTCPMocks(t *testing.T) {
	t.Run("MockTcpChannel", func(t *testing.T) {
		mock := NewMockTcpChannel()
		assert.NotNil(t, mock)
		
		localAddr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0}
		remoteAddr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080}
		
		mock.On("UnsafeConnect", localAddr, remoteAddr).Return(nil)
		err := mock.UnsafeConnect(localAddr, remoteAddr)
		assert.NoError(t, err)
		mock.AssertExpectations(t)
	})

}

// TestWebSocketMocks verifies all WebSocket mock implementations  
func TestWebSocketMocks(t *testing.T) {
	t.Run("MockWebSocketChannel", func(t *testing.T) {
		mock := NewMockWebSocketChannel()
		assert.NotNil(t, mock)
		
		mock.On("BootstrapPreInit").Once()
		mock.BootstrapPreInit()
		mock.AssertExpectations(t)
	})

	t.Run("MockHandlerTask", func(t *testing.T) {
		mock := NewMockHandlerTask()
		assert.NotNil(t, mock)
		
		mockCtx := NewMockHandlerContext()
		params := make(map[string]any)
		
		// Test basic WebSocket handler functionality with properly typed nil
		var pingMsg *gws.PingMessage = nil
		mock.On("WSPing", mockCtx, pingMsg, params).Once()
		mock.WSPing(mockCtx, pingMsg, params)
		mock.AssertExpectations(t)
	})
}

// TestUtilityMocks verifies all utility mock implementations
func TestUtilityMocks(t *testing.T) {
	t.Run("MockQueue", func(t *testing.T) {
		mock := NewMockQueue()
		assert.NotNil(t, mock)
		
		testItem := "test-item"
		mock.On("Push", testItem).Once()
		mock.On("Pop").Return(testItem)
		mock.On("Size").Return(1)
		
		mock.Push(testItem)
		item := mock.Pop()
		size := mock.Size()
		
		assert.Equal(t, testItem, item)
		assert.Equal(t, 1, size)
		mock.AssertExpectations(t)
	})
}

// Mock helper for testing
type mockResponseWriter struct {
	header http.Header
	body   []byte
	status int
}

func (m *mockResponseWriter) Header() http.Header {
	if m.header == nil {
		m.header = make(http.Header)
	}
	return m.header
}

func (m *mockResponseWriter) Write(data []byte) (int, error) {
	m.body = append(m.body, data...)
	return len(data), nil
}

func (m *mockResponseWriter) WriteHeader(statusCode int) {
	m.status = statusCode
}