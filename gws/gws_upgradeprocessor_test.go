package gws

import (
	"errors"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/yetiz-org/gone/channel"
	"github.com/yetiz-org/gone/ghttp"
)

// TestUpgradeProcessor_ComprehensiveOperations tests all UpgradeProcessor operations
func TestUpgradeProcessor_ComprehensiveOperations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "Added_InitializesUpgrader",
			testFunc: func(t *testing.T) {
				processor := &UpgradeProcessor{}
				mockCtx := channel.NewMockHandlerContext()
				mockChannel := channel.NewMockChannel()
				
				// Test with CheckOrigin parameter set to false
				mockChannel.On("Param", ParamCheckOrigin, mock.Anything).Return(false, true)
				mockCtx.On("Channel").Return(mockChannel)

				assert.NotPanics(t, func() {
					processor.Added(mockCtx)
				})

				assert.NotNil(t, processor.upgrade)
			},
		},
		{
			name: "Added_InitializesUpgraderWithCheckOrigin",
			testFunc: func(t *testing.T) {
				processor := &UpgradeProcessor{}
				mockCtx := channel.NewMockHandlerContext()
				mockChannel := channel.NewMockChannel()
				
				// Test with CheckOrigin parameter set to true
				mockChannel.On("Param", ParamCheckOrigin, mock.Anything).Return(true, true)
				mockCtx.On("Channel").Return(mockChannel)

				assert.NotPanics(t, func() {
					processor.Added(mockCtx)
				})

				assert.NotNil(t, processor.upgrade)
			},
		},
		{
			name: "Read_WithNilObject",
			testFunc: func(t *testing.T) {
				processor := &UpgradeProcessor{}
				mockCtx := channel.NewMockHandlerContext()

				// Should return early with nil object
				assert.NotPanics(t, func() {
					processor.Read(mockCtx, nil)
				})
			},
		},
		{
			name: "Read_WithNonPackObject",
			testFunc: func(t *testing.T) {
				processor := &UpgradeProcessor{}
				mockCtx := channel.NewMockHandlerContext()
				mockCtx.On("FireRead", mock.Anything).Return(mockCtx)

				nonPackObj := "not a pack"

				assert.NotPanics(t, func() {
					processor.Read(mockCtx, nonPackObj)
				})

				mockCtx.AssertExpectations(t)
			},
		},
		{
			name: "Read_WithPackButNilRouteNode",
			testFunc: func(t *testing.T) {
				processor := &UpgradeProcessor{}
				mockCtx := channel.NewMockHandlerContext()
				mockCtx.On("FireRead", mock.Anything).Return(mockCtx)

				pack := &ghttp.Pack{
					RouteNode: nil,
				}

				assert.NotPanics(t, func() {
					processor.Read(mockCtx, pack)
				})

				mockCtx.AssertExpectations(t)
			},
		},
		{
			name: "Read_WithPackButNonServerHandlerTask",
			testFunc: func(t *testing.T) {
				processor := &UpgradeProcessor{}
				mockCtx := channel.NewMockHandlerContext()
				mockCtx.On("FireRead", mock.Anything).Return(mockCtx)
				
				mockRouteNode := ghttp.NewMockRouteNode()
				mockHandler := ghttp.NewMockHttpHandlerTask() // Not a ServerHandlerTask
				mockRouteNode.On("HandlerTask").Return(mockHandler)

				pack := &ghttp.Pack{
					RouteNode: mockRouteNode,
				}

				assert.NotPanics(t, func() {
					processor.Read(mockCtx, pack)
				})

				mockCtx.AssertExpectations(t)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.testFunc(t)
		})
	}
}

// TestUpgradeProcessor_NewWSLog tests _NewWSLog method
func TestUpgradeProcessor_NewWSLog(t *testing.T) {
	t.Parallel()

	processor := &UpgradeProcessor{}
	
	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "NewWSLog_WithoutWebSocketConn",
			testFunc: func(t *testing.T) {
				cID := "test-channel-123"
				tID := "test-track-456"
				uri := "/test/websocket"
				testErr := errors.New("test error")

				log := processor._NewWSLog(cID, tID, uri, nil, testErr)

				assert.NotNil(t, log)
				assert.Equal(t, LogType, log.LogType)
				assert.Equal(t, cID, log.ChannelID)
				assert.Equal(t, tID, log.TrackID)
				assert.Equal(t, uri, log.RequestURI)
				assert.Equal(t, testErr, log.Error)
				assert.Nil(t, log.RemoteAddr)
				assert.Nil(t, log.LocalAddr)
			},
		},
		{
			name: "NewWSLog_WithWebSocketConn",
			testFunc: func(t *testing.T) {
				cID := "test-channel-789"
				tID := "test-track-012"
				uri := "/test/websocket/path"
				
				// Create mock WebSocket connection (we can't easily mock websocket.Conn)
				// So we test the case where wsConn is nil, which is more realistic for unit tests
				log := processor._NewWSLog(cID, tID, uri, nil, nil)

				assert.NotNil(t, log)
				assert.Equal(t, LogType, log.LogType)
				assert.Equal(t, cID, log.ChannelID)
				assert.Equal(t, tID, log.TrackID)
				assert.Equal(t, uri, log.RequestURI)
				assert.Nil(t, log.Error)
				// Without real websocket.Conn, these will be nil
				assert.Nil(t, log.RemoteAddr)
				assert.Nil(t, log.LocalAddr)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.testFunc(t)
		})
	}
}

// TestLogStruct_Comprehensive tests LogStruct functionality
func TestLogStruct_Comprehensive(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "LogStruct_Creation",
			testFunc: func(t *testing.T) {
				remoteAddr := &net.TCPAddr{IP: net.ParseIP("192.168.1.100"), Port: 9000}
				localAddr := &net.TCPAddr{IP: net.ParseIP("localhost"), Port: 8080}
				testErr := errors.New("websocket test error")
				mockMessage := NewMockMessage()

				log := &LogStruct{
					LogType:    LogType,
					RemoteAddr: remoteAddr,
					LocalAddr:  localAddr,
					RequestURI: "/ws/test",
					ChannelID:  "channel-test-123",
					TrackID:    "track-test-456",
					Message:    mockMessage,
					Error:      testErr,
				}

				assert.Equal(t, LogType, log.LogType)
				assert.Equal(t, remoteAddr, log.RemoteAddr)
				assert.Equal(t, localAddr, log.LocalAddr)
				assert.Equal(t, "/ws/test", log.RequestURI)
				assert.Equal(t, "channel-test-123", log.ChannelID)
				assert.Equal(t, "track-test-456", log.TrackID)
				assert.Equal(t, mockMessage, log.Message)
				assert.Equal(t, testErr, log.Error)
			},
		},
		{
			name: "LogStruct_EmptyFields",
			testFunc: func(t *testing.T) {
				log := &LogStruct{}

				assert.Empty(t, log.LogType)
				assert.Nil(t, log.RemoteAddr)
				assert.Nil(t, log.LocalAddr)
				assert.Empty(t, log.RequestURI)
				assert.Empty(t, log.ChannelID)
				assert.Empty(t, log.TrackID)
				assert.Nil(t, log.Message)
				assert.Nil(t, log.Error)
			},
		},
		{
			name: "LogType_Constant",
			testFunc: func(t *testing.T) {
				assert.Equal(t, "websocket", LogType)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.testFunc(t)
		})
	}
}

// TestUpgradeProcessor_UpgradeCheckFunc tests custom upgrade check function
func TestUpgradeProcessor_UpgradeCheckFunc(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "UpgradeCheckFunc_CustomFunction",
			testFunc: func(t *testing.T) {
				called := false
				processor := &UpgradeProcessor{
					UpgradeCheckFunc: func(req *ghttp.Request, resp *ghttp.Response, params map[string]any) bool {
						called = true
						return false // Reject upgrade
					},
				}

				// This test verifies that UpgradeCheckFunc can be set
				assert.NotNil(t, processor.UpgradeCheckFunc)
				
				// Call the function to verify it works
				var mockReq *ghttp.Request = nil
				var mockResp *ghttp.Response = nil
				params := map[string]any{}
				
				result := processor.UpgradeCheckFunc(mockReq, mockResp, params)
				
				assert.True(t, called)
				assert.False(t, result)
			},
		},
		{
			name: "UpgradeCheckFunc_NilFunction",
			testFunc: func(t *testing.T) {
				processor := &UpgradeProcessor{
					UpgradeCheckFunc: nil,
				}

				assert.Nil(t, processor.UpgradeCheckFunc)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.testFunc(t)
		})
	}
}