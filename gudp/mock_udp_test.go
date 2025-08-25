package gudp

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/yetiz-org/gone/channel"
)

// TestMockUdpChannel_InterfaceCompliance verifies that MockUdpChannel implements all required interfaces
func TestMockUdpChannel_InterfaceCompliance(t *testing.T) {
	mockUdp := NewMockUdpChannel()

	// Test that MockUdpChannel implements Channel interface
	var _ channel.Channel = mockUdp

	// Test that MockUdpChannel implements NetChannel interface
	var _ channel.NetChannel = mockUdp

	assert.NotNil(t, mockUdp, "MockUdpChannel should not be nil")
	assert.NotNil(t, mockUdp.MockNetChannel, "Embedded MockNetChannel should not be nil")
}

// TestMockUdpServerChannel_InterfaceCompliance verifies that MockUdpServerChannel implements all required interfaces
func TestMockUdpServerChannel_InterfaceCompliance(t *testing.T) {
	mockUdpServer := NewMockUdpServerChannel()

	// Test that MockUdpServerChannel implements Channel interface
	var _ channel.Channel = mockUdpServer

	// Test that MockUdpServerChannel implements ServerChannel interface
	var _ channel.ServerChannel = mockUdpServer

	assert.NotNil(t, mockUdpServer, "MockUdpServerChannel should not be nil")
	assert.NotNil(t, mockUdpServer.MockServerChannel, "Embedded MockServerChannel should not be nil")
}

// TestMockUdpChannel_BasicFunctionality tests basic mock functionality
func TestMockUdpChannel_BasicFunctionality(t *testing.T) {
	mockUdp := NewMockUdpChannel()

	// Test UnsafeConnect method
	localAddr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0}
	remoteAddr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080}

	mockUdp.On("UnsafeConnect", localAddr, remoteAddr).Return(nil)

	err := mockUdp.UnsafeConnect(localAddr, remoteAddr)
	assert.NoError(t, err)

	// Verify the call was made
	mockUdp.AssertExpectations(t)
}

// TestMockUdpServerChannel_BasicFunctionality tests basic server mock functionality
func TestMockUdpServerChannel_BasicFunctionality(t *testing.T) {
	mockUdpServer := NewMockUdpServerChannel()

	// Test UnsafeBind method
	localAddr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080}

	mockUdpServer.On("UnsafeBind", localAddr).Return(nil)

	err := mockUdpServer.UnsafeBind(localAddr)
	assert.NoError(t, err)

	// Test UnsafeAccept method
	mockChannel := channel.NewMockChannel()
	mockFuture := channel.NewMockFuture(mockChannel)

	mockUdpServer.On("UnsafeAccept").Return(mockChannel, mockFuture)

	ch, future := mockUdpServer.UnsafeAccept()
	assert.Equal(t, mockChannel, ch)
	assert.Equal(t, mockFuture, future)

	// Test IsActive method
	mockUdpServer.On("IsActive").Return(true)

	isActive := mockUdpServer.IsActive()
	assert.True(t, isActive)

	// Verify all calls were made
	mockUdpServer.AssertExpectations(t)
}

// TestMockUdpChannel_MethodDelegation tests that methods are properly delegated to embedded mock
func TestMockUdpChannel_MethodDelegation(t *testing.T) {
	mockUdp := NewMockUdpChannel()

	// Test that Serial method works (inherited from MockChannel)
	expectedSerial := uint64(456)
	mockUdp.On("Serial").Return(expectedSerial)

	serial := mockUdp.Serial()
	assert.Equal(t, expectedSerial, serial)

	// Test that ID method works (inherited from MockChannel)
	expectedID := "test-udp-channel-id"
	mockUdp.On("ID").Return(expectedID)

	id := mockUdp.ID()
	assert.Equal(t, expectedID, id)

	// Test that IsActive method works (inherited from MockChannel)
	mockUdp.On("IsActive").Return(true)

	isActive := mockUdp.IsActive()
	assert.True(t, isActive)

	// Verify all calls were made
	mockUdp.AssertExpectations(t)
}

// TestMockUdpChannel_ProperInheritance verifies the mock inherits from MockNetChannel, not DefaultNetChannel
func TestMockUdpChannel_ProperInheritance(t *testing.T) {
	mockUdp := NewMockUdpChannel()

	// This test ensures we're using the Mock hierarchy, not Default hierarchy
	// If this compiles and runs, it means our inheritance is correct

	// Test accessing embedded MockNetChannel directly
	assert.NotNil(t, &mockUdp.MockNetChannel, "Should have access to embedded MockNetChannel")

	// Test that we can call inherited mock methods
	mockUdp.On("RemoteAddr").Return(nil)

	addr := mockUdp.RemoteAddr()
	assert.Nil(t, addr)

	// Test Conn method (specific to NetChannel)
	mockConn := channel.NewMockConn()
	mockUdp.On("Conn").Return(mockConn)

	conn := mockUdp.Conn()
	assert.Equal(t, mockConn, conn)

	mockUdp.AssertExpectations(t)
}

// TestMockUdpServerChannel_ProperInheritance verifies the server mock inherits correctly
func TestMockUdpServerChannel_ProperInheritance(t *testing.T) {
	mockUdpServer := NewMockUdpServerChannel()

	// Test accessing embedded MockServerChannel directly
	assert.NotNil(t, &mockUdpServer.MockServerChannel, "Should have access to embedded MockServerChannel")

	// Test that we can call inherited server channel methods
	mockParams := &channel.Params{}
	mockUdpServer.On("ChildParams").Return(mockParams)

	params := mockUdpServer.ChildParams()
	assert.Equal(t, mockParams, params)

	mockUdpServer.AssertExpectations(t)
}
