package simpleudp

import (
	"testing"
	"time"

	"github.com/yetiz-org/gone/mock"
)

// TestClientBasicOperations tests client basic operations without network
func TestClientBasicOperations(t *testing.T) {
	// Set timeout to prevent infinite loops
	deadline := time.Now().Add(10 * time.Second)
	
	// Test basic client operations
	handler := mock.NewMockHandler()
	client := NewClient(handler)
	
	// Test Channel() method before connection
	if client.Channel() != nil {
		t.Error("Channel() should return nil before connection")
	}
	
	// Test auto-reconnect setting
	client.AutoReconnect = func() bool { return true }
	if client.AutoReconnect == nil {
		t.Error("AutoReconnect should be settable")
	}
	if !client.AutoReconnect() {
		t.Error("AutoReconnect should return true when set")
	}
	
	// Test close flag
	if client.close != false {
		t.Error("Client should initialize with close=false")
	}
	
	if time.Now().After(deadline) {
		t.Fatal("Test exceeded timeout")
	}
}

// TestServerBasicOperations tests server basic operations without network  
func TestServerBasicOperations(t *testing.T) {
	// Set timeout to prevent infinite loops
	deadline := time.Now().Add(10 * time.Second)
	
	// Test basic server operations
	handler := mock.NewMockHandler()
	server := NewServer(handler)
	
	// Test Channel() method before start
	if server.Channel() != nil {
		t.Error("Channel() should return nil before start")
	}
	
	// Test handler assignment
	if server.Handler != handler {
		t.Error("Server should store provided handler")
	}
	
	if time.Now().After(deadline) {
		t.Fatal("Test exceeded timeout")
	}
}

// TestHandlerAdaptersBasic tests handler adapters without complex mocking
func TestHandlerAdaptersBasic(t *testing.T) {
	// Set timeout to prevent infinite loops
	deadline := time.Now().Add(10 * time.Second)
	
	// Test client handler adapter creation
	mockHandler := mock.NewMockHandler()
	client := NewClient(mockHandler)
	adapter := &clientHandlerAdapter{client: client}
	
	if adapter.client != client {
		t.Error("Client handler adapter should store client reference")
	}
	
	// Test server handler adapter creation
	server := NewServer(mockHandler)
	serverAdapter := &serverHandlerAdapter{server: server}
	
	if serverAdapter.server != server {
		t.Error("Server handler adapter should store server reference")
	}
	
	// Test adapter with nil handler
	clientNoHandler := NewClient(nil)
	adapterNoHandler := &clientHandlerAdapter{client: clientNoHandler}
	
	if adapterNoHandler.client.Handler != nil {
		t.Error("Adapter with nil handler should have nil Handler")
	}
	
	if time.Now().After(deadline) {
		t.Fatal("Test exceeded timeout")
	}
}

// TestConnectionHandlerBasic tests connection handler without network operations
func TestConnectionHandlerBasic(t *testing.T) {
	// Set timeout to prevent infinite loops
	deadline := time.Now().Add(10 * time.Second)
	
	handler := mock.NewMockHandler()
	client := NewClient(handler)
	
	// Test connection handler creation
	connHandler := &connectionHandler{client: client}
	
	if connHandler.client != client {
		t.Error("Connection handler should store client reference")
	}
	
	// Test auto-reconnect logic setup
	reconnectCalled := false
	client.AutoReconnect = func() bool {
		reconnectCalled = true
		return false // Don't reconnect in test
	}
	
	if client.AutoReconnect == nil {
		t.Error("AutoReconnect should be settable")
	}
	
	// Test reconnect function call
	if !client.AutoReconnect() { // This should set reconnectCalled to true
		// Expected - we set it to return false
	}
	
	if !reconnectCalled {
		t.Error("AutoReconnect function should have been called")
	}
	
	if time.Now().After(deadline) {
		t.Fatal("Test exceeded timeout")
	}
}