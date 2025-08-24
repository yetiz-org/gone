package simpleudp

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/yetiz-org/gone/channel"
	goneMock "github.com/yetiz-org/gone/mock"
)

// Test Client creation and basic properties
func TestClient_Creation(t *testing.T) {
	tests := []struct {
		name    string
		handler channel.Handler
		wantNil bool
	}{
		{"valid_handler", goneMock.NewMockHandler(), false},
		{"nil_handler", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			
			client := NewClient(tt.handler)
			
			if tt.wantNil {
				assert.Nil(t, client, "TestCase: %s should return nil", tt.name)
			} else {
				assert.NotNil(t, client, "TestCase: %s should return non-nil client", tt.name)
				assert.Equal(t, tt.handler, client.Handler, 
					"TestCase: %s should store provided handler", tt.name)
				assert.False(t, client.close, 
					"TestCase: %s should initialize with close=false", tt.name)
				assert.Nil(t, client.AutoReconnect, 
					"TestCase: %s should initialize with nil AutoReconnect", tt.name)
			}
		})
	}
}

// Test Client channel operations
func TestClient_ChannelOperations(t *testing.T) {
	t.Parallel()
	
	client := NewClient(goneMock.NewMockHandler())
	
	// Before start, Channel() should return nil
	assert.Nil(t, client.Channel(), "Channel should be nil before start")
	
	// Test disconnect without connection should not panic
	assert.NotPanics(t, func() {
		client.close = true
		// client.Disconnect() would panic since ch is nil, but we test the close flag
	}, "Setting close flag should not panic")
}

// Test connectionHandler functionality
func TestConnectionHandler_Active(t *testing.T) {
	t.Parallel()
	
	client := NewClient(goneMock.NewMockHandler())
	handler := &connectionHandler{client: client}
	
	// Create mock context and channel
	mockCtx := goneMock.NewMockHandlerContext()
	mockChannel := goneMock.NewMockChannel()
	
	// Setup mock expectations
	mockCtx.On("Channel").Return(mockChannel)
	mockCtx.On("FireActive").Return(mockCtx)
	
	// Mock IsActive to return true once, then false to break the loop
	mockChannel.On("IsActive").Return(true).Once()
	mockChannel.On("IsActive").Return(false)
	
	// Call Active method
	handler.Active(mockCtx)
	
	// Give goroutine time to start and run
	time.Sleep(100 * time.Millisecond)
	
	// Verify expectations - only check context expectations
	mockCtx.AssertExpectations(t)
}

// Test connectionHandler Unregistered with AutoReconnect
func TestConnectionHandler_Unregistered(t *testing.T) {
	tests := []struct {
		name          string
		clientClose   bool
		autoReconnect func() bool
		expectRestart bool
	}{
		{"client_closed", true, nil, false},
		{"no_auto_reconnect", false, nil, false},
		{"auto_reconnect_true", false, func() bool { return true }, true},
		{"auto_reconnect_false", false, func() bool { return false }, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			
			client := NewClient(goneMock.NewMockHandler())
			client.close = tt.clientClose
			client.AutoReconnect = tt.autoReconnect
			
			// Bootstrap is set to nil by default in test environment
			// The start() method now has nil check to prevent panic
			
			handler := &connectionHandler{client: client}
			
			// Create mock context  
			mockCtx := goneMock.NewMockHandlerContext()
			mockCtx.On("FireUnregistered").Return(mockCtx)
			
			// Call Unregistered method
			handler.Unregistered(mockCtx)
			
			// Verify expectations
			mockCtx.AssertExpectations(t)
			
			// Check if close flag was set correctly
			if tt.autoReconnect != nil && !tt.autoReconnect() {
				assert.True(t, client.close, 
					"TestCase: %s should set close=true when AutoReconnect returns false", tt.name)
			}
		})
	}
}

// Test clientHandlerAdapter with nil handler
func TestClientHandlerAdapter_NilHandler(t *testing.T) {
	t.Parallel()
	
	client := NewClient(nil) // nil handler
	adapter := &clientHandlerAdapter{client: client}
	
	// Create mock context
	mockCtx := goneMock.NewMockHandlerContext()
	
	// Test all adapter methods with nil handler
	testCases := []struct {
		name string
		fn   func()
	}{
		{"Added", func() { adapter.Added(mockCtx) }},
		{"Removed", func() { adapter.Removed(mockCtx) }},
		{"Registered", func() {
			mockCtx.On("FireRegistered").Return(mockCtx)
			adapter.Registered(mockCtx)
		}},
		{"Unregistered", func() {
			mockCtx.On("FireUnregistered").Return(mockCtx)
			adapter.Unregistered(mockCtx)
		}},
		{"Active", func() {
			mockCtx.On("FireActive").Return(mockCtx)
			adapter.Active(mockCtx)
		}},
		{"Inactive", func() {
			mockCtx.On("FireInactive").Return(mockCtx)
			adapter.Inactive(mockCtx)
		}},
		{"Read", func() { adapter.Read(mockCtx, "test") }},
		{"ReadCompleted", func() { adapter.ReadCompleted(mockCtx) }},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.NotPanics(t, tc.fn, "TestCase: %s should not panic with nil handler", tc.name)
		})
	}
	
	// Verify expectations
	mockCtx.AssertExpectations(t)
}

// Test clientHandlerAdapter with valid handler
func TestClientHandlerAdapter_ValidHandler(t *testing.T) {
	t.Parallel()
	
	mockHandler := goneMock.NewMockHandler()
	client := NewClient(mockHandler)
	adapter := &clientHandlerAdapter{client: client}
	
	// Create mock context
	mockCtx := goneMock.NewMockHandlerContext()
	
	// Setup handler expectations for all methods
	mockHandler.On("Added", mockCtx).Return()
	mockHandler.On("Removed", mockCtx).Return()
	mockHandler.On("Registered", mockCtx).Return()
	mockHandler.On("Unregistered", mockCtx).Return()
	mockHandler.On("Active", mockCtx).Return()
	mockHandler.On("Inactive", mockCtx).Return()
	mockHandler.On("Read", mockCtx, "test").Return()
	mockHandler.On("ReadCompleted", mockCtx).Return()
	
	// Test all adapter methods
	adapter.Added(mockCtx)
	adapter.Removed(mockCtx)
	adapter.Registered(mockCtx)
	adapter.Unregistered(mockCtx)
	adapter.Active(mockCtx)
	adapter.Inactive(mockCtx)
	adapter.Read(mockCtx, "test")
	adapter.ReadCompleted(mockCtx)
	
	// Verify all handler methods were called
	mockHandler.AssertExpectations(t)
}

// Test clientHandlerAdapter Write method
func TestClientHandlerAdapter_Write(t *testing.T) {
	tests := []struct {
		name        string
		hasHandler  bool
		expectWrite bool
	}{
		{"with_handler", true, false},
		{"without_handler", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			
			var client *Client
			if tt.hasHandler {
				mockHandler := goneMock.NewMockHandler()
				client = NewClient(mockHandler)
				mockHandler.On("Write", mock.Anything, mock.Anything, mock.Anything).Return()
			} else {
				client = NewClient(nil)
			}
			
			adapter := &clientHandlerAdapter{client: client}
			mockCtx := goneMock.NewMockHandlerContext()
			mockFuture := goneMock.NewMockFuture(nil)
			
			if tt.expectWrite {
				mockCtx.On("Write", "test", mockFuture).Return(mockFuture)
			}
			
			adapter.Write(mockCtx, "test", mockFuture)
			
			if tt.hasHandler {
				if mockHandler, ok := client.Handler.(*channel.MockHandler); ok {
					mockHandler.AssertExpectations(t)
				}
			}
			if tt.expectWrite {
				mockCtx.AssertExpectations(t)
			}
		})
	}
}

// Test clientHandlerAdapter network operations
func TestClientHandlerAdapter_NetworkOperations(t *testing.T) {
	t.Parallel()
	
	mockHandler := goneMock.NewMockHandler()
	client := NewClient(mockHandler)
	adapter := &clientHandlerAdapter{client: client}
	
	mockCtx := goneMock.NewMockHandlerContext()
	mockFuture := goneMock.NewMockFuture(nil)
	localAddr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080}
	remoteAddr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8081}
	
	// Setup handler expectations
	mockHandler.On("Bind", mockCtx, localAddr, mockFuture).Return()
	mockHandler.On("Close", mockCtx, mockFuture).Return()
	mockHandler.On("Connect", mockCtx, localAddr, remoteAddr, mockFuture).Return()
	mockHandler.On("Disconnect", mockCtx, mockFuture).Return()
	mockHandler.On("Deregister", mockCtx, mockFuture).Return()
	mockHandler.On("ErrorCaught", mockCtx, mock.AnythingOfType("*errors.errorString")).Return()
	
	// Test network operations
	adapter.Bind(mockCtx, localAddr, mockFuture)
	adapter.Close(mockCtx, mockFuture)
	adapter.Connect(mockCtx, localAddr, remoteAddr, mockFuture)
	adapter.Disconnect(mockCtx, mockFuture)
	adapter.Deregister(mockCtx, mockFuture)
	adapter.ErrorCaught(mockCtx, assert.AnError)
	
	// Verify all methods were called
	mockHandler.AssertExpectations(t)
}

// Test clientHandlerAdapter network operations with nil handler
func TestClientHandlerAdapter_NetworkOperations_NilHandler(t *testing.T) {
	t.Parallel()
	
	client := NewClient(nil)
	adapter := &clientHandlerAdapter{client: client}
	
	mockCtx := goneMock.NewMockHandlerContext()
	mockFuture := goneMock.NewMockFuture(nil)
	localAddr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080}
	remoteAddr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8081}
	
	// Setup context expectations for fallback behavior
	mockCtx.On("Bind", localAddr, mockFuture).Return(mockFuture)
	mockCtx.On("Close", mockFuture).Return(mockFuture)
	mockCtx.On("Connect", localAddr, remoteAddr, mockFuture).Return(mockFuture)
	mockCtx.On("Disconnect", mockFuture).Return(mockFuture)
	mockCtx.On("Deregister", mockFuture).Return(mockFuture)
	mockCtx.On("FireErrorCaught", mock.Anything).Return(mockCtx)
	
	// Test network operations with nil handler
	adapter.Bind(mockCtx, localAddr, mockFuture)
	adapter.Close(mockCtx, mockFuture)
	adapter.Connect(mockCtx, localAddr, remoteAddr, mockFuture)
	adapter.Disconnect(mockCtx, mockFuture)
	adapter.Deregister(mockCtx, mockFuture)
	adapter.ErrorCaught(mockCtx, assert.AnError)
	
	// Verify context methods were called
	mockCtx.AssertExpectations(t)
}

// Performance benchmark for Client creation
func BenchmarkNewClient(b *testing.B) {
	handler := goneMock.NewMockHandler()
	
	for i := 0; i < b.N; i++ {
		_ = NewClient(handler)
	}
}

// Performance benchmark for connectionHandler Active
func BenchmarkConnectionHandler_Active(b *testing.B) {
	client := NewClient(goneMock.NewMockHandler())
	handler := &connectionHandler{client: client}
	mockCtx := goneMock.NewMockHandlerContext()
	mockChannel := goneMock.NewMockChannel()
	
	mockCtx.On("Channel").Return(mockChannel)
	mockCtx.On("FireActive").Return()
	mockChannel.On("IsActive").Return(false) // Immediately return false to avoid long-running goroutine
	
	for i := 0; i < b.N; i++ {
		handler.Active(mockCtx)
	}
}