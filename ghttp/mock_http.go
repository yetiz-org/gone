package ghttp

import (
	"net/http"

	"github.com/stretchr/testify/mock"
	"github.com/yetiz-org/gone/channel"
)

// MockHTTPServerChannel is a mock implementation for HTTP server channels
type MockHTTPServerChannel struct {
	channel.MockServerChannel
}

// NewMockHTTPServerChannel creates a new MockHTTPServerChannel instance
func NewMockHTTPServerChannel() *MockHTTPServerChannel {
	return &MockHTTPServerChannel{
		MockServerChannel: *channel.NewMockServerChannel(),
	}
}

// ServeHTTP implements the http.Handler interface for testing
func (m *MockHTTPServerChannel) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.Called(w, r)
}

// MockRequest is a mock implementation for HTTP requests
type MockRequest struct {
	mock.Mock
}

// NewMockRequest creates a new MockRequest instance
func NewMockRequest() *MockRequest {
	return &MockRequest{}
}

// GetRequest returns the underlying HTTP request
func (m *MockRequest) GetRequest() *http.Request {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*http.Request)
}

// Channel returns the associated channel
func (m *MockRequest) Channel() *Channel {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*Channel)
}

// MockResponse is a mock implementation for HTTP responses
type MockResponse struct {
	mock.Mock
}

// NewMockResponse creates a new MockResponse instance
func NewMockResponse() *MockResponse {
	return &MockResponse{}
}

// SetStatus sets the HTTP status code
func (m *MockResponse) SetStatus(code int) {
	m.Called(code)
}

// Status returns the HTTP status code
func (m *MockResponse) Status() int {
	args := m.Called()
	return args.Int(0)
}

// SetHeader sets a response header
func (m *MockResponse) SetHeader(key, value string) {
	m.Called(key, value)
}

// GetHeader gets a response header
func (m *MockResponse) GetHeader(key string) string {
	args := m.Called(key)
	return args.String(0)
}

// Write writes response body
func (m *MockResponse) Write(data []byte) (int, error) {
	args := m.Called(data)
	return args.Int(0), args.Error(1)
}

// WriteString writes string response body
func (m *MockResponse) WriteString(data string) (int, error) {
	args := m.Called(data)
	return args.Int(0), args.Error(1)
}

// Header returns response headers (implementing http.ResponseWriter)
func (m *MockResponse) Header() http.Header {
	args := m.Called()
	if args.Get(0) == nil {
		return make(http.Header)
	}
	return args.Get(0).(http.Header)
}

// WriteHeader writes response status code (implementing http.ResponseWriter)
func (m *MockResponse) WriteHeader(statusCode int) {
	m.Called(statusCode)
}

// Interface compliance checks for ghttp mocks
var (
	_ channel.Channel = (*MockHTTPServerChannel)(nil)
	_ http.ResponseWriter = (*MockResponse)(nil)
	// Note: Request interface compliance depends on actual interface definition
)