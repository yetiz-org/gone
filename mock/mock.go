package mock

// Mock package provides unified access to all mock implementations
// distributed across the gone library packages.
//
// All mocks are now located in their respective packages for interface compliance:
// - Channel mocks: github.com/yetiz-org/gone/channel
// - HTTP mocks: github.com/yetiz-org/gone/ghttp  
// - TCP mocks: github.com/yetiz-org/gone/gtcp
// - WebSocket mocks: github.com/yetiz-org/gone/gws
// - Utility mocks: github.com/yetiz-org/gone/utils
//
// Usage Examples:
//
// Channel Mocks:
//   import "github.com/yetiz-org/gone/channel"
//   mockChannel := channel.NewMockChannel()
//   mockPipeline := channel.NewMockPipeline()
//   mockHandler := channel.NewMockHandler()
//
// HTTP Mocks:
//   import "github.com/yetiz-org/gone/ghttp"
//   mockRequest := ghttp.NewMockRequest()
//   mockResponse := ghttp.NewMockResponse()
//
// TCP Mocks:
//   import "github.com/yetiz-org/gone/gtcp"
//   mockTcpChannel := gtcp.NewMockTcpChannel()
//   mockTcpServerChannel := gtcp.NewMockTcpServerChannel()
//
// WebSocket Mocks:
//   import "github.com/yetiz-org/gone/gws"
//   mockWSChannel := gws.NewMockWebSocketChannel()
//   mockHandlerTask := gws.NewMockHandlerTask()
//
// Utils Mocks:
//   import "github.com/yetiz-org/gone/utils"
//   mockQueue := utils.NewMockQueue()

import (
	// Import all packages with mocks for convenient access
	"github.com/yetiz-org/gone/channel"
	"github.com/yetiz-org/gone/ghttp"
	"github.com/yetiz-org/gone/gtcp"
	"github.com/yetiz-org/gone/gtcp/simpletcp"
	"github.com/yetiz-org/gone/gws"
	"github.com/yetiz-org/gone/utils"
)

// Channel Mock Constructors - wrapper functions for convenience
func NewMockChannel() *channel.MockChannel {
	return channel.NewMockChannel()
}

func NewMockPipeline() *channel.MockPipeline {
	return channel.NewMockPipeline()
}

func NewMockHandler() *channel.MockHandler {
	return channel.NewMockHandler()
}

func NewMockFuture(ch interface{}) *channel.MockFuture {
	return channel.NewMockFuture(ch)
}

func NewMockNetChannel() *channel.MockNetChannel {
	return channel.NewMockNetChannel()
}

func NewMockServerChannel() *channel.MockServerChannel {
	return channel.NewMockServerChannel()
}

func NewMockHandlerContext() *channel.MockHandlerContext {
	return channel.NewMockHandlerContext()
}

func NewMockConn() *channel.MockConn {
	return channel.NewMockConn()
}

// HTTP Mock Constructors
func NewMockHTTPServerChannel() *ghttp.MockHTTPServerChannel {
	return ghttp.NewMockHTTPServerChannel()
}

func NewMockRequest() *ghttp.MockRequest {
	return ghttp.NewMockRequest()
}

func NewMockResponse() *ghttp.MockResponse {
	return ghttp.NewMockResponse()
}

// TCP Mock Constructors
func NewMockTcpChannel() *gtcp.MockTcpChannel {
	return gtcp.NewMockTcpChannel()
}

func NewMockTcpServerChannel() *gtcp.MockTcpServerChannel {
	return gtcp.NewMockTcpServerChannel()
}

// SimpleTCP Mock Constructors
func NewMockSimpleClient() *simpletcp.MockSimpleClient {
	return simpletcp.NewMockSimpleClient()
}

func NewMockSimpleServer() *simpletcp.MockSimpleServer {
	return simpletcp.NewMockSimpleServer()
}

// WebSocket Mock Constructors
func NewMockWebSocketChannel() *gws.MockWebSocketChannel {
	return gws.NewMockWebSocketChannel()
}

func NewMockHandlerTask() *gws.MockHandlerTask {
	return gws.NewMockHandlerTask()
}

func NewMockServerHandlerTask() *gws.MockServerHandlerTask {
	return gws.NewMockServerHandlerTask()
}

// Utility Mock Constructors
func NewMockQueue() *utils.MockQueue {
	return utils.NewMockQueue()
}