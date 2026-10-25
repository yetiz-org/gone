package mock

import (
	"net"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/yetiz-org/gone/channel"
	"github.com/yetiz-org/gone/ghttp"
	"github.com/yetiz-org/gone/ghttp/httpsession"
	"github.com/yetiz-org/gone/gtcp"
	"github.com/yetiz-org/gone/gudp"
	"github.com/yetiz-org/gone/gws"
	"github.com/yetiz-org/gone/utils"
)

// TestInterfaceCompliance verifies mocks can be used as their respective interfaces
func TestInterfaceCompliance(t *testing.T) {
	t.Run("Channel_Interface_Compliance", func(t *testing.T) {
		mockChannel := NewMockChannel()

		// Test that MockChannel can be used as channel.Channel
		var ch channel.Channel = mockChannel
		assert.NotNil(t, ch, "MockChannel should implement Channel interface")

		// Setup basic expectations
		mockChannel.On("ID").Return("test-id")
		mockChannel.On("IsActive").Return(true)

		// Test interface methods
		id := ch.ID()
		isActive := ch.IsActive()

		assert.Equal(t, "test-id", id)
		assert.True(t, isActive)

		mockChannel.AssertExpectations(t)
	})

	t.Run("HTTPResponse_Interface_Compliance", func(t *testing.T) {
		mockResponse := NewMockResponse()

		// Test that MockResponse can be used as http.ResponseWriter
		var w http.ResponseWriter = mockResponse
		assert.NotNil(t, w, "MockResponse should implement http.ResponseWriter")

		// Setup expectations
		mockResponse.On("Header").Return(make(http.Header))
		mockResponse.On("WriteHeader", 200).Return()
		mockResponse.On("Write", mock.AnythingOfType("[]uint8")).Return(5, nil)

		// Test interface methods
		header := w.Header()
		w.WriteHeader(200)
		n, err := w.Write([]byte("hello"))

		assert.NotNil(t, header)
		assert.Equal(t, 5, n)
		assert.NoError(t, err)

		mockResponse.AssertExpectations(t)
	})

	t.Run("TCP_Interface_Compliance", func(t *testing.T) {
		mockTcpChannel := NewMockTcpChannel()

		// Test TCP-specific functionality
		localAddr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0}
		remoteAddr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080}

		mockTcpChannel.On("UnsafeConnect", localAddr, remoteAddr).Return(nil)

		err := mockTcpChannel.UnsafeConnect(localAddr, remoteAddr)
		assert.NoError(t, err)

		mockTcpChannel.AssertExpectations(t)
	})
}

// TestMockExpectationsAndVerification tests testify/mock functionality
func TestMockExpectationsAndVerification(t *testing.T) {
	t.Run("Basic_Expectations", func(t *testing.T) {
		mockChannel := NewMockChannel()

		// Set up expectations
		mockChannel.On("ID").Return("expected-id")
		mockChannel.On("IsActive").Return(false)

		// Call methods
		id := mockChannel.ID()
		isActive := mockChannel.IsActive()

		// Verify results
		assert.Equal(t, "expected-id", id)
		assert.False(t, isActive)

		// Verify expectations were met
		mockChannel.AssertExpectations(t)
	})

	t.Run("Method_Call_Counting", func(t *testing.T) {
		mockHandler := NewMockHandler()
		mockContext := NewMockHandlerContext()

		// Expect method to be called exactly twice
		mockHandler.On("Active", mockContext).Times(2)

		// Call method twice
		mockHandler.Active(mockContext)
		mockHandler.Active(mockContext)

		// Verify expectations
		mockHandler.AssertExpectations(t)
	})

	t.Run("Argument_Matching", func(t *testing.T) {
		mockResponse := NewMockResponse()

		// Use argument matchers
		mockResponse.On("Write", mock.MatchedBy(func(data []byte) bool {
			return len(data) > 0
		})).Return(10, nil)

		n, err := mockResponse.Write([]byte("test"))
		assert.Equal(t, 10, n)
		assert.NoError(t, err)

		mockResponse.AssertExpectations(t)
	})
}

// TestDirectPackageUsage tests using mocks directly from their packages
func TestDirectPackageUsage(t *testing.T) {
	t.Run("Direct_Channel_Mock", func(t *testing.T) {
		// Use mock directly from channel package
		directMockChannel := channel.NewMockChannel()

		directMockChannel.On("ID").Return("direct-channel")

		id := directMockChannel.ID()
		assert.Equal(t, "direct-channel", id)

		directMockChannel.AssertExpectations(t)
	})

	t.Run("Direct_HTTP_Mock", func(t *testing.T) {
		// Use mock directly from ghttp package
		directMockResponse := ghttp.NewMockResponse()

		directMockResponse.On("WriteHeader", 404).Return()
		directMockResponse.WriteHeader(404)

		directMockResponse.AssertExpectations(t)
	})

	t.Run("Direct_TCP_Mock", func(t *testing.T) {
		// Use mock directly from gtcp package
		directMockTcpChannel := gtcp.NewMockTcpChannel()

		addr := &net.TCPAddr{IP: net.ParseIP("192.168.1.1"), Port: 9000}
		directMockTcpChannel.On("UnsafeConnect", mock.Anything, addr).Return(nil)

		err := directMockTcpChannel.UnsafeConnect(nil, addr)
		assert.NoError(t, err)

		directMockTcpChannel.AssertExpectations(t)
	})
}

// TestRealWorldUsageScenarios tests mocks in realistic scenarios
func TestRealWorldUsageScenarios(t *testing.T) {
	t.Run("Channel_Pipeline_Scenario", func(t *testing.T) {
		mockChannel := NewMockChannel()
		mockPipeline := NewMockPipeline()
		mockHandler := NewMockHandler()

		// Setup realistic channel pipeline scenario
		mockChannel.On("Pipeline").Return(mockPipeline)
		mockPipeline.On("AddLast", "test-handler", mockHandler).Return(mockPipeline)

		// Execute scenario
		pipeline := mockChannel.Pipeline()
		resultPipeline := pipeline.AddLast("test-handler", mockHandler)

		assert.NotNil(t, resultPipeline)

		mockChannel.AssertExpectations(t)
		mockPipeline.AssertExpectations(t)
	})

	t.Run("HTTP_Server_Response_Scenario", func(t *testing.T) {
		mockResponse := NewMockResponse()

		// Setup HTTP response scenario
		headers := make(http.Header)
		mockResponse.On("Header").Return(headers)
		mockResponse.On("WriteHeader", 200).Return()
		mockResponse.On("Write", mock.AnythingOfType("[]uint8")).Return(13, nil)

		// Simulate HTTP response handling
		var w http.ResponseWriter = mockResponse
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		n, err := w.Write([]byte("Hello, World!"))

		assert.Equal(t, 13, n)
		assert.NoError(t, err)
		assert.Equal(t, "application/json", headers.Get("Content-Type"))

		mockResponse.AssertExpectations(t)
	})

	t.Run("TCP_Client_Connection_Scenario", func(t *testing.T) {
		mockTcpChannel := NewMockTcpChannel()

		// Setup TCP connection scenario - focus on the specific UnsafeConnect method
		localAddr := &net.TCPAddr{IP: net.ParseIP("0.0.0.0"), Port: 0}
		remoteAddr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 80}

		mockTcpChannel.On("UnsafeConnect", localAddr, remoteAddr).Return(nil)

		// Test the TCP-specific functionality
		err := mockTcpChannel.UnsafeConnect(localAddr, remoteAddr)
		assert.NoError(t, err, "UnsafeConnect should succeed")

		// Verify TCP mock is functional
		assert.NotNil(t, mockTcpChannel, "TCP Channel mock should be created successfully")

		mockTcpChannel.AssertExpectations(t)
	})
}

// TestErrorHandling tests error scenarios with mocks
func TestErrorHandling(t *testing.T) {
	t.Run("Connection_Failure_Simulation", func(t *testing.T) {
		mockTcpChannel := NewMockTcpChannel()

		// Simulate connection failure
		expectedError := assert.AnError
		mockTcpChannel.On("UnsafeConnect", mock.Anything, mock.Anything).Return(expectedError)

		err := mockTcpChannel.UnsafeConnect(nil, &net.TCPAddr{Port: 8080})
		assert.Error(t, err)
		assert.Equal(t, expectedError, err)

		mockTcpChannel.AssertExpectations(t)
	})

	t.Run("Queue_Empty_Error", func(t *testing.T) {
		mockQueue := NewMockQueue()

		// Simulate empty queue error - Pop() only returns one value
		mockQueue.On("Pop").Return(nil)

		item := mockQueue.Pop()
		assert.Nil(t, item)

		mockQueue.AssertExpectations(t)
	})
}

func TestMockConstructors(t *testing.T) {
	ch := NewMockChannel()
	future := NewMockFuture(ch)
	constructors := []struct {
		name string
		got  any
		want any
	}{
		{"NewMockChannel", ch, (*channel.MockChannel)(nil)},
		{"NewMockPipeline", NewMockPipeline(), (*channel.MockPipeline)(nil)},
		{"NewMockHandler", NewMockHandler(), (*channel.MockHandler)(nil)},
		{"NewMockFuture", future, (*channel.MockFuture)(nil)},
		{"NewMockNetChannel", NewMockNetChannel(), (*channel.MockNetChannel)(nil)},
		{"NewMockServerChannel", NewMockServerChannel(), (*channel.MockServerChannel)(nil)},
		{"NewMockHandlerContext", NewMockHandlerContext(), (*channel.MockHandlerContext)(nil)},
		{"NewMockConn", NewMockConn(), (*channel.MockConn)(nil)},
		{"NewMockMessageEncoder", NewMockMessageEncoder(), (*channel.MockMessageEncoder)(nil)},
		{"NewMockMessageDecoder", NewMockMessageDecoder(), (*channel.MockMessageDecoder)(nil)},
		{"NewMockHTTPServerChannel", NewMockHTTPServerChannel(), (*ghttp.MockHTTPServerChannel)(nil)},
		{"NewMockRequest", NewMockRequest(), (*ghttp.MockRequest)(nil)},
		{"NewMockResponse", NewMockResponse(), (*ghttp.MockResponse)(nil)},
		{"NewMockHttpHandlerTask", NewMockHttpHandlerTask(), (*ghttp.MockHttpHandlerTask)(nil)},
		{"NewMockRoute", NewMockRoute(), (*ghttp.MockRoute)(nil)},
		{"NewMockRouteNode", NewMockRouteNode(), (*ghttp.MockRouteNode)(nil)},
		{"NewMockSSEOperation", NewMockSSEOperation(), (*ghttp.MockSSEOperation)(nil)},
		{"NewMockSessionProvider", NewMockSessionProvider(), (*httpsession.MockSessionProvider)(nil)},
		{"NewMockSession", NewMockSession(), (*httpsession.MockSession)(nil)},
		{"NewMockTcpChannel", NewMockTcpChannel(), (*gtcp.MockTcpChannel)(nil)},
		{"NewMockTcpServerChannel", NewMockTcpServerChannel(), (*gtcp.MockTcpServerChannel)(nil)},
		{"NewMockUdpChannel", NewMockUdpChannel(), (*gudp.MockUdpChannel)(nil)},
		{"NewMockUdpServerChannel", NewMockUdpServerChannel(), (*gudp.MockUdpServerChannel)(nil)},
		{"NewMockWebSocketChannel", NewMockWebSocketChannel(), (*gws.MockWebSocketChannel)(nil)},
		{"NewMockHandlerTask", NewMockHandlerTask(), (*gws.MockHandlerTask)(nil)},
		{"NewMockServerHandlerTask", NewMockServerHandlerTask(), (*gws.MockServerHandlerTask)(nil)},
		{"NewMockMessage", NewMockMessage(), (*gws.MockMessage)(nil)},
		{"NewMockMessageBuilder", NewMockMessageBuilder(), (*gws.MockMessageBuilder)(nil)},
		{"NewMockQueue", NewMockQueue(), (*utils.MockQueue)(nil)},
	}
	for _, constructor := range constructors {
		t.Run(constructor.name, func(t *testing.T) {
			assert.IsType(t, constructor.want, constructor.got)
			assert.NotNil(t, constructor.got)
		})
	}

	ch.On("ID").Return(nil).Once()
	ch.On("Serial").Return(nil).Once()
	assert.NotEmpty(t, ch.ID())
	assert.NotZero(t, ch.Serial())
	ch.AssertExpectations(t)
	future.On("Channel").Return(nil).Once()
	assert.Same(t, ch, future.Channel())
	future.AssertExpectations(t)
	nilFuture := NewMockFuture(nil)
	if assert.NotNil(t, nilFuture) {
		nilFuture.On("Channel").Return(nil).Once()
		assert.Nil(t, nilFuture.Channel())
		nilFuture.AssertExpectations(t)
	}
}
