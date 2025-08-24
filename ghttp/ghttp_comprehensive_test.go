package ghttp

import (
	"context"
	"net"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/yetiz-org/gone/channel"
)

// MockNetConn is a mock implementation of net.Conn for testing
type MockNetConn struct {
	mock.Mock
}

func (m *MockNetConn) Read(b []byte) (n int, err error) {
	args := m.Called(b)
	return args.Int(0), args.Error(1)
}

func (m *MockNetConn) Write(b []byte) (n int, err error) {
	args := m.Called(b)
	return args.Int(0), args.Error(1)
}

func (m *MockNetConn) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockNetConn) LocalAddr() net.Addr {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(net.Addr)
}

func (m *MockNetConn) RemoteAddr() net.Addr {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(net.Addr)
}

func (m *MockNetConn) SetDeadline(t time.Time) error {
	args := m.Called(t)
	return args.Error(0)
}

func (m *MockNetConn) SetReadDeadline(t time.Time) error {
	args := m.Called(t)
	return args.Error(0)
}

func (m *MockNetConn) SetWriteDeadline(t time.Time) error {
	args := m.Called(t)
	return args.Error(0)
}


// TestServerChannel_ServeHTTP tests the HTTP request handling
func TestServerChannel_ServeHTTP(t *testing.T) {
	t.Parallel()

	t.Run("ServeHTTP_NoConn", func(t *testing.T) {
		t.Parallel()
		
		serverCh := &ServerChannel{}
		
		// Create request without conn context
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		
		// Should handle missing conn gracefully
		assert.NotPanics(t, func() {
			serverCh.ServeHTTP(w, req)
		})
	})

	t.Run("ServeHTTP_WithConnButNoChannel", func(t *testing.T) {
		t.Parallel()
		
		serverCh := &ServerChannel{}
		
		// Create mock conn
		mockConn := &MockNetConn{}
		
		// Create request with conn but no channel context
		req := httptest.NewRequest("GET", "/test", nil)
		ctx := context.WithValue(req.Context(), ConnCtx, mockConn)
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()
		
		// Should handle missing channel gracefully
		assert.NotPanics(t, func() {
			serverCh.ServeHTTP(w, req)
		})
	})

	t.Run("ServeHTTP_MaxBodyBytes", func(t *testing.T) {
		t.Parallel()
		
		serverCh := &ServerChannel{maxBodyBytes: 10}
		
		// Create mock objects
		mockConn := &MockNetConn{}
		mockChannel := &Channel{}
		
		// Create request with both contexts
		body := strings.NewReader("test body content that exceeds limit")
		req := httptest.NewRequest("POST", "/test", body)
		ctx := context.WithValue(req.Context(), ConnCtx, mockConn)
		ctx = context.WithValue(ctx, ConnChCtx, mockChannel)
		req = req.WithContext(ctx)
		w := httptest.NewRecorder()
		
		// Should apply max body bytes
		assert.NotPanics(t, func() {
			serverCh.ServeHTTP(w, req)
		})
	})
}

// TestServerChannel_UnsafeBind tests server binding functionality
func TestServerChannel_UnsafeBind(t *testing.T) {
	t.Parallel()

	t.Run("UnsafeBind_Success", func(t *testing.T) {
		t.Parallel()
		
		serverCh := &ServerChannel{}
		serverCh.DefaultNetServerChannel.Name = "TestServer"
		
		// Create local address
		addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
		assert.NoError(t, err)
		
		err = serverCh.UnsafeBind(addr)
		assert.NoError(t, err)
		assert.True(t, serverCh.active)
		assert.NotNil(t, serverCh.server)
		assert.NotNil(t, serverCh.newChChan)
		
		// Cleanup
		serverCh.UnsafeClose()
	})

	t.Run("UnsafeBind_WithDefaultName", func(t *testing.T) {
		t.Parallel()
		
		serverCh := &ServerChannel{}
		
		// Create local address  
		addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
		assert.NoError(t, err)
		
		err = serverCh.UnsafeBind(addr)
		assert.NoError(t, err)
		assert.True(t, serverCh.active)
		assert.Contains(t, serverCh.Name, "SERVER_")
		
		// Cleanup
		serverCh.UnsafeClose()
	})

	t.Run("UnsafeBind_WithParameters", func(t *testing.T) {
		t.Parallel()
		
		serverCh := &ServerChannel{}
		
		// Set parameters directly on serverCh - it inherits SetParams from DefaultNetServerChannel
		params := map[string]any{
			string(ParamAcceptWaitCount):    512,
			string(ParamMaxBodyBytes):       int64(1024),
			string(ParamIdleTimeout):        int64(30),
			string(ParamReadTimeout):        int64(30),
			string(ParamReadHeaderTimeout):  int64(30),
			string(ParamWriteTimeout):       int64(30),
			string(ParamMaxHeaderBytes):     1024*1024*2,
		}
		// ServerChannel embeds DefaultNetServerChannel, so we can set params directly
		for k, v := range params {
			serverCh.SetParam(channel.ParamKey(k), v)
		}
		
		addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
		assert.NoError(t, err)
		
		err = serverCh.UnsafeBind(addr)
		assert.NoError(t, err)
		assert.True(t, serverCh.active)
		assert.Equal(t, int64(1024), serverCh.maxBodyBytes)
		
		// Cleanup
		serverCh.UnsafeClose()
	})
}

// TestServerChannel_UnsafeClose tests server closing functionality
func TestServerChannel_UnsafeClose(t *testing.T) {
	t.Parallel()

	t.Run("UnsafeClose_NotActive", func(t *testing.T) {
		t.Parallel()
		
		serverCh := &ServerChannel{active: false}
		
		err := serverCh.UnsafeClose()
		assert.NoError(t, err)
	})

	t.Run("UnsafeClose_Active", func(t *testing.T) {
		t.Parallel()
		
		serverCh := &ServerChannel{}
		
		// Bind first
		addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
		assert.NoError(t, err)
		
		err = serverCh.UnsafeBind(addr)
		assert.NoError(t, err)
		assert.True(t, serverCh.active)
		
		// Now close
		err = serverCh.UnsafeClose()
		assert.NoError(t, err)
		assert.False(t, serverCh.active)
	})

	t.Run("UnsafeClose_WithConnections", func(t *testing.T) {
		t.Parallel()
		
		serverCh := &ServerChannel{}
		
		// Bind first
		addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
		assert.NoError(t, err)
		
		err = serverCh.UnsafeBind(addr)
		assert.NoError(t, err)
		
		// Test active state and close functionality
		serverCh.active = true
		
		// Close should handle active state
		err = serverCh.UnsafeClose()
		assert.NoError(t, err)
		assert.False(t, serverCh.active)
	})
}

// TestServerChannel_IsActive tests active state checking
func TestServerChannel_IsActive(t *testing.T) {
	t.Parallel()

	t.Run("IsActive_False", func(t *testing.T) {
		t.Parallel()
		
		serverCh := &ServerChannel{active: false}
		assert.False(t, serverCh.IsActive())
	})

	t.Run("IsActive_True", func(t *testing.T) {
		t.Parallel()
		
		serverCh := &ServerChannel{active: true}
		assert.True(t, serverCh.IsActive())
	})
}

// TestDefaultHTTPHandlerTask_HTTPMethods tests HTTP method handlers
func TestDefaultHTTPHandlerTask_HTTPMethods(t *testing.T) {
	t.Parallel()

	task := &DefaultHTTPHandlerTask{}
	ctx := channel.NewMockHandlerContext()
	req := &Request{}
	resp := &Response{}
	params := map[string]any{}

	t.Run("Index_ReturnsNotImplemented", func(t *testing.T) {
		t.Parallel()
		
		err := task.Index(ctx, req, resp, params)
		assert.Equal(t, NotImplemented, err)
	})

	t.Run("Get_ReturnsNil", func(t *testing.T) {
		t.Parallel()
		
		err := task.Get(ctx, req, resp, params)
		assert.Nil(t, err)
	})

	t.Run("Create_ReturnsNotImplemented", func(t *testing.T) {
		t.Parallel()
		
		err := task.Create(ctx, req, resp, params)
		assert.Equal(t, NotImplemented, err)
	})

	t.Run("Post_ReturnsNil", func(t *testing.T) {
		t.Parallel()
		
		err := task.Post(ctx, req, resp, params)
		assert.Nil(t, err)
	})

	t.Run("Put_ReturnsNil", func(t *testing.T) {
		t.Parallel()
		
		err := task.Put(ctx, req, resp, params)
		assert.Nil(t, err)
	})

	t.Run("Delete_ReturnsNil", func(t *testing.T) {
		t.Parallel()
		
		err := task.Delete(ctx, req, resp, params)
		assert.Nil(t, err)
	})

	t.Run("Options_ReturnsNil", func(t *testing.T) {
		t.Parallel()
		
		err := task.Options(ctx, req, resp, params)
		assert.Nil(t, err)
	})

	t.Run("Patch_ReturnsNil", func(t *testing.T) {
		t.Parallel()
		
		err := task.Patch(ctx, req, resp, params)
		assert.Nil(t, err)
	})

	t.Run("Trace_ReturnsNil", func(t *testing.T) {
		t.Parallel()
		
		err := task.Trace(ctx, req, resp, params)
		assert.Nil(t, err)
	})

	t.Run("Connect_ReturnsNil", func(t *testing.T) {
		t.Parallel()
		
		err := task.Connect(ctx, req, resp, params)
		assert.Nil(t, err)
	})
}

// TestDefaultHTTPHandlerTask_CORSHelper tests CORS functionality
func TestDefaultHTTPHandlerTask_CORSHelper(t *testing.T) {
	t.Parallel()

	task := &DefaultHTTPHandlerTask{}

	t.Run("CORSHelper_NullOrigin", func(t *testing.T) {
		t.Parallel()
		
		// Create mock HTTP request for testing
		httpReq := httptest.NewRequest("GET", "/", nil)
		httpReq.Header.Set("Origin", "null")
		req := &Request{request: httpReq}
		
		// Create response with proper header initialization
		resp := NewResponse(req)
		
		params := map[string]any{}
		
		task.CORSHelper(req, resp, params)
		
		assert.Equal(t, "*", resp.Header().Get("Access-Control-Allow-Origin"))
	})

	t.Run("CORSHelper_WithOrigin", func(t *testing.T) {
		t.Parallel()
		
		// Create mock HTTP request for testing
		httpReq := httptest.NewRequest("GET", "/", nil)
		httpReq.Header.Set("Origin", "https://example.com")
		req := &Request{request: httpReq}
		
		// Create response with proper header initialization
		resp := NewResponse(req)
		
		params := map[string]any{}
		
		task.CORSHelper(req, resp, params)
		
		assert.Equal(t, "https://example.com", resp.Header().Get("Access-Control-Allow-Origin"))
	})

	t.Run("CORSHelper_WithRequestHeaders", func(t *testing.T) {
		t.Parallel()
		
		// Create mock HTTP request for testing
		httpReq := httptest.NewRequest("GET", "/", nil)
		httpReq.Header.Set("Origin", "https://example.com")
		httpReq.Header.Set("Access-Control-Request-Headers", "Content-Type,Authorization")
		req := &Request{request: httpReq}
		
		// Create response with proper header initialization
		resp := NewResponse(req)
		
		params := map[string]any{}
		
		task.CORSHelper(req, resp, params)
		
		assert.Equal(t, "Content-Type,Authorization", resp.Header().Get("Access-Control-Allow-Headers"))
	})

	t.Run("CORSHelper_WithRequestMethod", func(t *testing.T) {
		t.Parallel()
		
		// Create mock HTTP request for testing
		httpReq := httptest.NewRequest("GET", "/", nil)
		httpReq.Header.Set("Origin", "https://example.com")
		httpReq.Header.Set("Access-Control-Request-Method", "POST")
		req := &Request{request: httpReq}
		
		// Create response with proper header initialization
		resp := NewResponse(req)
		
		params := map[string]any{}
		
		task.CORSHelper(req, resp, params)
		
		assert.Equal(t, "POST", resp.Header().Get("Access-Control-Allow-Methods"))
	})
}

// TestDefaultHTTPHandlerTask_LifecycleMethods tests lifecycle methods
func TestDefaultHTTPHandlerTask_LifecycleMethods(t *testing.T) {
	t.Parallel()

	task := &DefaultHTTPHandlerTask{}
	req := &Request{}
	resp := &Response{}
	params := map[string]any{}

	t.Run("PreCheck_ReturnsNil", func(t *testing.T) {
		t.Parallel()
		
		err := task.PreCheck(req, resp, params)
		assert.Nil(t, err)
	})

	t.Run("Before_ReturnsNil", func(t *testing.T) {
		t.Parallel()
		
		err := task.Before(req, resp, params)
		assert.Nil(t, err)
	})

	t.Run("After_ReturnsNil", func(t *testing.T) {
		t.Parallel()
		
		err := task.After(req, resp, params)
		assert.Nil(t, err)
	})

	t.Run("ErrorCaught_HandlesError", func(t *testing.T) {
		t.Parallel()
		
		// Create a proper request and response for testing with mock channel
		httpReq := httptest.NewRequest("GET", "/", nil)
		mockChannel := channel.NewMockChannel()
		mockChannel.On("ID").Return("test-channel-id")
		
		testReq := &Request{
			request: httpReq,
			channel: mockChannel,
		}
		
		// Create response with proper request initialization to avoid nil pointer
		realResp := NewResponse(testReq)
		
		err := task.ErrorCaught(testReq, realResp, params, NotImplemented)
		assert.NoError(t, err)
		
		mockChannel.AssertExpectations(t)
	})

	t.Run("ThrowErrorResponse_Panics", func(t *testing.T) {
		t.Parallel()
		
		assert.Panics(t, func() {
			task.ThrowErrorResponse(NotImplemented)
		})
	})
}

// TestDefaultHandlerTask tests DefaultHandlerTask methods
func TestDefaultHandlerTask(t *testing.T) {
	t.Parallel()

	t.Run("NewDefaultHandlerTask", func(t *testing.T) {
		t.Parallel()
		
		task := NewDefaultHandlerTask()
		assert.NotNil(t, task)
	})

	t.Run("Register_DoesNothing", func(t *testing.T) {
		t.Parallel()
		
		task := &DefaultHandlerTask{}
		assert.NotPanics(t, func() {
			task.Register()
		})
	})

	t.Run("IsIndex_False", func(t *testing.T) {
		t.Parallel()
		
		task := &DefaultHandlerTask{}
		params := map[string]any{}
		
		result := task.IsIndex(params)
		assert.False(t, result)
	})

	t.Run("IsIndex_True", func(t *testing.T) {
		t.Parallel()
		
		task := &DefaultHandlerTask{}
		params := map[string]any{
			"[gone-http]is_index": true,
		}
		
		result := task.IsIndex(params)
		assert.True(t, result)
	})

	t.Run("GetNodeName_Empty", func(t *testing.T) {
		t.Parallel()
		
		task := &DefaultHandlerTask{}
		params := map[string]any{}
		
		result := task.GetNodeName(params)
		assert.Equal(t, "", result)
	})

	t.Run("GetNodeName_WithValue", func(t *testing.T) {
		t.Parallel()
		
		task := &DefaultHandlerTask{}
		params := map[string]any{
			"[gone-http]node_name": "test-node",
		}
		
		result := task.GetNodeName(params)
		assert.Equal(t, "test-node", result)
	})

	t.Run("GetID_Empty", func(t *testing.T) {
		t.Parallel()
		
		task := &DefaultHandlerTask{}
		params := map[string]any{}
		
		result := task.GetID("user", params)
		assert.Equal(t, "", result)
	})

	t.Run("GetID_WithValue", func(t *testing.T) {
		t.Parallel()
		
		task := &DefaultHandlerTask{}
		params := map[string]any{
			"[gone-http]user_id": "123",
		}
		
		result := task.GetID("user", params)
		assert.Equal(t, "123", result)
	})

	t.Run("LogExtend_NewExtend", func(t *testing.T) {
		t.Parallel()
		
		task := &DefaultHandlerTask{}
		params := map[string]any{}
		
		task.LogExtend("key1", "value1", params)
		
		extend, exists := params["[gone-http]extend"]
		assert.True(t, exists)
		assert.Equal(t, "value1", extend.(map[string]any)["key1"])
	})

	t.Run("LogExtend_ExistingExtend", func(t *testing.T) {
		t.Parallel()
		
		task := &DefaultHandlerTask{}
		params := map[string]any{
			"[gone-http]extend": map[string]any{"existing": "value"},
		}
		
		task.LogExtend("key2", "value2", params)
		
		extend := params["[gone-http]extend"].(map[string]any)
		assert.Equal(t, "value", extend["existing"])
		assert.Equal(t, "value2", extend["key2"])
	})
}

// TestSSEMessage tests SSE message functionality
func TestSSEMessage(t *testing.T) {
	t.Parallel()

	t.Run("SSEMessage_Validate_Empty", func(t *testing.T) {
		t.Parallel()
		
		msg := SSEMessage{}
		assert.False(t, msg.Validate())
	})

	t.Run("SSEMessage_Validate_WithComment", func(t *testing.T) {
		t.Parallel()
		
		msg := SSEMessage{Comment: "test comment"}
		assert.True(t, msg.Validate())
	})

	t.Run("SSEMessage_Validate_WithEvent", func(t *testing.T) {
		t.Parallel()
		
		msg := SSEMessage{Event: "test-event"}
		assert.True(t, msg.Validate())
	})

	t.Run("SSEMessage_Validate_WithData", func(t *testing.T) {
		t.Parallel()
		
		msg := SSEMessage{Data: []string{"test data"}}
		assert.True(t, msg.Validate())
	})

	t.Run("SSEMessage_Validate_WithID", func(t *testing.T) {
		t.Parallel()
		
		msg := SSEMessage{Id: "test-id"}
		assert.True(t, msg.Validate())
	})

	t.Run("SSEMessage_Validate_WithRetry", func(t *testing.T) {
		t.Parallel()
		
		msg := SSEMessage{Retry: 1000}
		assert.True(t, msg.Validate())
	})

	t.Run("SSEMessage_Validate_Complete", func(t *testing.T) {
		t.Parallel()
		
		msg := SSEMessage{
			Comment: "test comment",
			Event:   "test-event",
			Data:    []string{"line1", "line2"},
			Id:      "test-id",
			Retry:   1000,
		}
		assert.True(t, msg.Validate())
	})
}
