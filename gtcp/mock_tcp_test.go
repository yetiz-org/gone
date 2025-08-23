package gtcp

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/yetiz-org/gone/channel"
)

// TestMockTcpChannel_InterfaceCompliance verifies that MockTcpChannel implements all required interfaces
func TestMockTcpChannel_InterfaceCompliance(t *testing.T) {
	mockTcp := NewMockTcpChannel()
	
	// Test that MockTcpChannel implements Channel interface
	var _ channel.Channel = mockTcp
	
	// Test that MockTcpChannel implements NetChannel interface
	var _ channel.NetChannel = mockTcp
	
	assert.NotNil(t, mockTcp, "MockTcpChannel should not be nil")
	assert.NotNil(t, mockTcp.MockNetChannel, "Embedded MockNetChannel should not be nil")
}

// TestMockTcpServerChannel_InterfaceCompliance verifies that MockTcpServerChannel implements all required interfaces
func TestMockTcpServerChannel_InterfaceCompliance(t *testing.T) {
	mockTcpServer := NewMockTcpServerChannel()
	
	// Test that MockTcpServerChannel implements Channel interface
	var _ channel.Channel = mockTcpServer
	
	// Test that MockTcpServerChannel implements ServerChannel interface
	var _ channel.ServerChannel = mockTcpServer
	
	assert.NotNil(t, mockTcpServer, "MockTcpServerChannel should not be nil")
	assert.NotNil(t, mockTcpServer.MockServerChannel, "Embedded MockServerChannel should not be nil")
}

// TestMockTcpChannel_BasicFunctionality tests basic mock functionality
func TestMockTcpChannel_BasicFunctionality(t *testing.T) {
	mockTcp := NewMockTcpChannel()
	
	// Test UnsafeConnect method
	localAddr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0}
	remoteAddr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080}
	
	mockTcp.On("UnsafeConnect", localAddr, remoteAddr).Return(nil)
	
	err := mockTcp.UnsafeConnect(localAddr, remoteAddr)
	assert.NoError(t, err)
	
	// Verify the call was made
	mockTcp.AssertExpectations(t)
}

// TestMockTcpServerChannel_BasicFunctionality tests basic server mock functionality
func TestMockTcpServerChannel_BasicFunctionality(t *testing.T) {
	mockTcpServer := NewMockTcpServerChannel()
	
	// Test UnsafeBind method
	localAddr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080}
	
	mockTcpServer.On("UnsafeBind", localAddr).Return(nil)
	
	err := mockTcpServer.UnsafeBind(localAddr)
	assert.NoError(t, err)
	
	// Test UnsafeAccept method
	mockChannel := channel.NewMockChannel()
	mockFuture := channel.NewMockFuture(mockChannel)
	
	mockTcpServer.On("UnsafeAccept").Return(mockChannel, mockFuture)
	
	ch, future := mockTcpServer.UnsafeAccept()
	assert.Equal(t, mockChannel, ch)
	assert.Equal(t, mockFuture, future)
	
	// Test IsActive method
	mockTcpServer.On("IsActive").Return(true)
	
	isActive := mockTcpServer.IsActive()
	assert.True(t, isActive)
	
	// Verify all calls were made
	mockTcpServer.AssertExpectations(t)
}

// TestMockTcpChannel_MethodDelegation tests that methods are properly delegated to embedded mock
func TestMockTcpChannel_MethodDelegation(t *testing.T) {
	mockTcp := NewMockTcpChannel()
	
	// Test that Serial method works (inherited from MockChannel)
	expectedSerial := uint64(123)
	mockTcp.On("Serial").Return(expectedSerial)
	
	serial := mockTcp.Serial()
	assert.Equal(t, expectedSerial, serial)
	
	// Test that ID method works (inherited from MockChannel)
	expectedID := "test-tcp-channel-id"
	mockTcp.On("ID").Return(expectedID)
	
	id := mockTcp.ID()
	assert.Equal(t, expectedID, id)
	
	// Test that IsActive method works (inherited from MockChannel)
	mockTcp.On("IsActive").Return(true)
	
	isActive := mockTcp.IsActive()
	assert.True(t, isActive)
	
	// Verify all calls were made
	mockTcp.AssertExpectations(t)
}

// TestMockTcpChannel_ProperInheritance verifies the mock inherits from MockNetChannel, not DefaultNetChannel
func TestMockTcpChannel_ProperInheritance(t *testing.T) {
	mockTcp := NewMockTcpChannel()
	
	// This test ensures we're using the Mock hierarchy, not Default hierarchy
	// If this compiles and runs, it means our inheritance is correct
	
	// Test accessing embedded MockNetChannel directly
	assert.NotNil(t, &mockTcp.MockNetChannel, "Should have access to embedded MockNetChannel")
	
	// Test that we can call inherited mock methods
	mockTcp.On("RemoteAddr").Return(nil)
	
	addr := mockTcp.RemoteAddr()
	assert.Nil(t, addr)
	
	mockTcp.AssertExpectations(t)
}
