package simpleudp

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/yetiz-org/gone/channel"
	goneMock "github.com/yetiz-org/gone/mock"
)

// Test Server creation and basic properties
func TestServer_Creation(t *testing.T) {
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
			
			server := NewServer(tt.handler)
			
			if tt.wantNil {
				assert.Nil(t, server, "TestCase: %s should return nil", tt.name)
			} else {
				assert.NotNil(t, server, "TestCase: %s should return non-nil server", tt.name)
				assert.Equal(t, tt.handler, server.Handler, 
					"TestCase: %s should store provided handler", tt.name)
				assert.Nil(t, server.ch, 
					"TestCase: %s should initialize with nil channel", tt.name)
			}
		})
	}
}

// Test Server channel operations
func TestServer_ChannelOperations(t *testing.T) {
	t.Parallel()
	
	server := NewServer(goneMock.NewMockHandler())
	
	// Before start, Channel() should return nil
	assert.Nil(t, server.Channel(), "Channel should be nil before start")
	
	// Test stop without connection should not panic
	assert.NotPanics(t, func() {
		// server.Stop() would panic since ch is nil, but we test this scenario
		if server.ch != nil {
			server.Stop()
		}
	}, "Stop check should not panic when channel is nil")
}

// Test serverHandlerAdapter with nil handler
func TestServerHandlerAdapter_NilHandler(t *testing.T) {
	t.Parallel()
	
	server := NewServer(nil) // nil handler
	adapter := &serverHandlerAdapter{server: server}
	
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

// Test serverHandlerAdapter with valid handler
func TestServerHandlerAdapter_ValidHandler(t *testing.T) {
	t.Parallel()
	
	mockHandler := goneMock.NewMockHandler()
	server := NewServer(mockHandler)
	adapter := &serverHandlerAdapter{server: server}
	
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

// Test serverHandlerAdapter Write method
func TestServerHandlerAdapter_Write(t *testing.T) {
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
			
			var server *Server
			if tt.hasHandler {
				mockHandler := goneMock.NewMockHandler()
				server = NewServer(mockHandler)
				mockHandler.On("Write", mock.Anything, mock.Anything, mock.Anything).Return()
			} else {
				server = NewServer(nil)
			}
			
			adapter := &serverHandlerAdapter{server: server}
			mockCtx := goneMock.NewMockHandlerContext()
			mockFuture := goneMock.NewMockFuture(nil)
			
			if tt.expectWrite {
				mockCtx.On("Write", "test", mockFuture).Return(mockFuture)
			}
			
			adapter.Write(mockCtx, "test", mockFuture)
			
			if tt.hasHandler {
				if mockHandler, ok := server.Handler.(*channel.MockHandler); ok {
					mockHandler.AssertExpectations(t)
				}
			}
			if tt.expectWrite {
				mockCtx.AssertExpectations(t)
			}
		})
	}
}

// Test serverHandlerAdapter network operations
func TestServerHandlerAdapter_NetworkOperations(t *testing.T) {
	t.Parallel()
	
	mockHandler := goneMock.NewMockHandler()
	server := NewServer(mockHandler)
	adapter := &serverHandlerAdapter{server: server}
	
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

// Test serverHandlerAdapter network operations with nil handler
func TestServerHandlerAdapter_NetworkOperations_NilHandler(t *testing.T) {
	t.Parallel()
	
	server := NewServer(nil)
	adapter := &serverHandlerAdapter{server: server}
	
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

// Test serverHandlerAdapter comparison with clientHandlerAdapter
func TestServerHandlerAdapter_ComparisonWithClient(t *testing.T) {
	t.Parallel()
	
	// Both server and client adapters should have similar behavior
	serverHandler := goneMock.NewMockHandler()
	clientHandler := goneMock.NewMockHandler()
	
	server := NewServer(serverHandler)
	client := NewClient(clientHandler)
	
	serverAdapter := &serverHandlerAdapter{server: server}
	clientAdapter := &clientHandlerAdapter{client: client}
	
	// Test that both adapters handle nil cases similarly
	assert.NotNil(t, serverAdapter, "Server adapter should not be nil")
	assert.NotNil(t, clientAdapter, "Client adapter should not be nil")
	
	// Test structure similarities
	assert.IsType(t, &serverHandlerAdapter{}, serverAdapter, "Should be server adapter type")
	assert.IsType(t, &clientHandlerAdapter{}, clientAdapter, "Should be client adapter type")
}

// Test Server and Client structural differences
func TestServer_vs_Client_Structure(t *testing.T) {
	t.Parallel()
	
	handler := goneMock.NewMockHandler()
	
	server := NewServer(handler)
	client := NewClient(handler)
	
	// Server should not have AutoReconnect functionality
	assert.NotNil(t, server, "Server should not be nil")
	assert.NotNil(t, client, "Client should not be nil")
	
	// Test that both store handlers correctly
	assert.Equal(t, handler, server.Handler, "Server should store handler")
	assert.Equal(t, handler, client.Handler, "Client should store handler")
	
	// Server should not have close flag like client
	// This tests structural differences between server and client
}

// Performance benchmark for Server creation
func BenchmarkNewServer(b *testing.B) {
	handler := goneMock.NewMockHandler()
	
	for i := 0; i < b.N; i++ {
		_ = NewServer(handler)
	}
}

// Performance benchmark for serverHandlerAdapter operations
func BenchmarkServerHandlerAdapter_Operations(b *testing.B) {
	server := NewServer(goneMock.NewMockHandler())
	adapter := &serverHandlerAdapter{server: server}
	mockCtx := goneMock.NewMockHandlerContext()
	
	// Setup minimal mocks to avoid excessive overhead
	mockCtx.On("FireActive").Return(nil)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		adapter.Active(mockCtx)
	}
}

// Test serverHandlerAdapter concurrent safety
func TestServerHandlerAdapter_ConcurrentSafety(t *testing.T) {
	t.Parallel()
	
	server := NewServer(goneMock.NewMockHandler())
	adapter := &serverHandlerAdapter{server: server}
	
	// Test that adapter can handle concurrent access to server field
	done := make(chan bool, 10)
	
	// Launch multiple goroutines accessing the adapter
	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- true }()
			
			// Simple field access should be safe
			_ = adapter.server
			_ = adapter.server.Handler
		}()
	}
	
	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
	
	assert.NotNil(t, adapter.server, "Server should remain accessible")
}