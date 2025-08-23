package gws

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/yetiz-org/gone/channel"
	"github.com/yetiz-org/gone/ghttp"
)

// TestMockWebSocketChannel_InterfaceCompliance verifies that MockWebSocketChannel implements all required interfaces
func TestMockWebSocketChannel_InterfaceCompliance(t *testing.T) {
	mockWS := NewMockWebSocketChannel()
	
	// Test that MockWebSocketChannel implements Channel interface
	var _ channel.Channel = mockWS
	
	// Test that MockWebSocketChannel implements NetChannel interface
	var _ channel.NetChannel = mockWS
	
	assert.NotNil(t, mockWS, "MockWebSocketChannel should not be nil")
	assert.NotNil(t, mockWS.MockNetChannel, "Embedded MockNetChannel should not be nil")
}

// TestMockWebSocketChannel_BasicFunctionality tests basic mock functionality
func TestMockWebSocketChannel_BasicFunctionality(t *testing.T) {
	mockWS := NewMockWebSocketChannel()
	
	// Test BootstrapPreInit method
	mockWS.On("BootstrapPreInit").Return()
	
	mockWS.BootstrapPreInit()
	
	// Test Init method
	expectedChannel := channel.NewMockChannel()
	mockWS.On("Init").Return(expectedChannel)
	
	result := mockWS.Init()
	assert.Equal(t, expectedChannel, result)
	
	// Test UnsafeWrite method
	testData := []byte("test websocket message")
	mockWS.On("UnsafeWrite", testData).Return(nil)
	
	err := mockWS.UnsafeWrite(testData)
	assert.NoError(t, err)
	
	// Test UnsafeRead method
	expectedData := []byte("received websocket message")
	mockWS.On("UnsafeRead").Return(expectedData, nil)
	
	data, err := mockWS.UnsafeRead()
	assert.NoError(t, err)
	assert.Equal(t, expectedData, data)
	
	// Verify all calls were made
	mockWS.AssertExpectations(t)
}

// TestMockWebSocketChannel_MethodDelegation tests that methods are properly delegated to embedded mock
func TestMockWebSocketChannel_MethodDelegation(t *testing.T) {
	mockWS := NewMockWebSocketChannel()
	
	// Test that Serial method works (inherited from MockChannel)
	expectedSerial := uint64(789)
	mockWS.On("Serial").Return(expectedSerial)
	
	serial := mockWS.Serial()
	assert.Equal(t, expectedSerial, serial)
	
	// Test that ID method works (inherited from MockChannel)
	expectedID := "test-websocket-channel-id"
	mockWS.On("ID").Return(expectedID)
	
	id := mockWS.ID()
	assert.Equal(t, expectedID, id)
	
	// Test that IsActive method works (inherited from MockChannel)
	mockWS.On("IsActive").Return(true)
	
	isActive := mockWS.IsActive()
	assert.True(t, isActive)
	
	// Verify all calls were made
	mockWS.AssertExpectations(t)
}

// TestMockWebSocketChannel_ProperInheritance verifies the mock inherits from MockNetChannel, not DefaultNetChannel
func TestMockWebSocketChannel_ProperInheritance(t *testing.T) {
	mockWS := NewMockWebSocketChannel()
	
	// This test ensures we're using the Mock hierarchy, not Default hierarchy
	// If this compiles and runs, it means our inheritance is correct
	
	// Test accessing embedded MockNetChannel directly
	assert.NotNil(t, &mockWS.MockNetChannel, "Should have access to embedded MockNetChannel")
	
	// Test that we can call inherited mock methods
	mockWS.On("RemoteAddr").Return(nil)
	
	addr := mockWS.RemoteAddr()
	assert.Nil(t, addr)
	
	// Test Conn method (specific to NetChannel)
	mockConn := channel.NewMockConn()
	mockWS.On("Conn").Return(mockConn)
	
	conn := mockWS.Conn()
	assert.Equal(t, mockConn, conn)
	
	mockWS.AssertExpectations(t)
}

// TestMockWebSocketChannel_WebSocketSpecificFields tests WebSocket-specific fields
func TestMockWebSocketChannel_WebSocketSpecificFields(t *testing.T) {
	mockWS := NewMockWebSocketChannel()
	
	// Test that WebSocket-specific fields can be accessed
	assert.Nil(t, mockWS.wsConn, "wsConn should be nil initially")
	assert.Nil(t, mockWS.Response, "Response should be nil initially")
	assert.Nil(t, mockWS.Request, "Request should be nil initially")
	
	// Test setting WebSocket-specific fields
	mockWS.Response = &ghttp.Response{}
	mockWS.Request = &ghttp.Request{}
	
	assert.NotNil(t, mockWS.Response, "Response should not be nil after setting")
	assert.NotNil(t, mockWS.Request, "Request should not be nil after setting")
}

// TestMockWebSocketChannel_ComplexScenario tests a more complex usage scenario
func TestMockWebSocketChannel_ComplexScenario(t *testing.T) {
	mockWS := NewMockWebSocketChannel()
	
	// Setup a complex scenario
	mockWS.On("BootstrapPreInit").Return()
	mockWS.On("Init").Return(mockWS)
	mockWS.On("IsActive").Return(true)
	
	// Initialize and verify
	mockWS.BootstrapPreInit()
	result := mockWS.Init()
	assert.Equal(t, mockWS, result)
	
	isActive := mockWS.IsActive()
	assert.True(t, isActive)
	
	// Test write/read cycle
	writeData := []byte("Hello WebSocket")
	readData := []byte("WebSocket Response")
	
	mockWS.On("UnsafeWrite", writeData).Return(nil)
	mockWS.On("UnsafeRead").Return(readData, nil)
	
	err := mockWS.UnsafeWrite(writeData)
	assert.NoError(t, err)
	
	received, err := mockWS.UnsafeRead()
	assert.NoError(t, err)
	assert.Equal(t, readData, received)
	
	// Verify all expectations
	mockWS.AssertExpectations(t)
}
