package channel

import (
	"net"

	"github.com/stretchr/testify/mock"
)

// MockHandler is a mock implementation of Handler interface
// It provides complete testify/mock integration for testing handler behaviors
type MockHandler struct {
	mock.Mock
}

// NewMockHandler creates a new MockHandler instance
func NewMockHandler() *MockHandler {
	return &MockHandler{}
}

// Added is called when the handler is added to a pipeline
func (m *MockHandler) Added(ctx HandlerContext) {
	m.Called(ctx)
}

// Removed is called when the handler is removed from a pipeline
func (m *MockHandler) Removed(ctx HandlerContext) {
	m.Called(ctx)
}

// Registered is called when the channel is registered
func (m *MockHandler) Registered(ctx HandlerContext) {
	m.Called(ctx)
}

// Unregistered is called when the channel is unregistered
func (m *MockHandler) Unregistered(ctx HandlerContext) {
	m.Called(ctx)
}

// Active is called when the channel becomes active
func (m *MockHandler) Active(ctx HandlerContext) {
	m.Called(ctx)
}

// Inactive is called when the channel becomes inactive
func (m *MockHandler) Inactive(ctx HandlerContext) {
	m.Called(ctx)
}

// Read is called when data is read from the channel
func (m *MockHandler) Read(ctx HandlerContext, obj any) {
	m.Called(ctx, obj)
}

// ReadCompleted is called when a read operation is completed
func (m *MockHandler) ReadCompleted(ctx HandlerContext) {
	m.Called(ctx)
}

// Write is called when data is written to the channel
func (m *MockHandler) Write(ctx HandlerContext, obj any, future Future) {
	m.Called(ctx, obj, future)
}

// Bind is called when the channel is bound to a local address
func (m *MockHandler) Bind(ctx HandlerContext, localAddr net.Addr, future Future) {
	m.Called(ctx, localAddr, future)
}

// Close is called when the channel is being closed
func (m *MockHandler) Close(ctx HandlerContext, future Future) {
	m.Called(ctx, future)
}

// Connect is called when the channel connects to a remote address
func (m *MockHandler) Connect(ctx HandlerContext, localAddr net.Addr, remoteAddr net.Addr, future Future) {
	m.Called(ctx, localAddr, remoteAddr, future)
}

// Disconnect is called when the channel is being disconnected
func (m *MockHandler) Disconnect(ctx HandlerContext, future Future) {
	m.Called(ctx, future)
}

// Deregister is called when the channel is being deregistered
func (m *MockHandler) Deregister(ctx HandlerContext, future Future) {
	m.Called(ctx, future)
}

// ErrorCaught is called when an error is caught in the pipeline
func (m *MockHandler) ErrorCaught(ctx HandlerContext, err error) {
	m.Called(ctx, err)
}

// Ensure MockHandler implements Handler interface
var _ Handler = (*MockHandler)(nil)