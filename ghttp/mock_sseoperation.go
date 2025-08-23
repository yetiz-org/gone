package ghttp

import (
	"net/http"

	"github.com/stretchr/testify/mock"
	"github.com/yetiz-org/gone/channel"
)

// MockSSEOperation is a mock implementation of SSEOperation interface
// It provides complete testify/mock integration for testing Server-Sent Events behaviors
type MockSSEOperation struct {
	mock.Mock
}

// NewMockSSEOperation creates a new MockSSEOperation instance
func NewMockSSEOperation() *MockSSEOperation {
	return &MockSSEOperation{}
}

// WriteHeader writes HTTP headers for SSE
func (m *MockSSEOperation) WriteHeader(ctx channel.HandlerContext, header http.Header, params map[string]any) channel.Future {
	args := m.Called(ctx, header, params)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(channel.Future)
}

// WriteMessage writes a single SSE message
func (m *MockSSEOperation) WriteMessage(ctx channel.HandlerContext, message SSEMessage, params map[string]any) channel.Future {
	args := m.Called(ctx, message, params)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(channel.Future)
}

// WriteMessages writes multiple SSE messages
func (m *MockSSEOperation) WriteMessages(ctx channel.HandlerContext, messages []SSEMessage, params map[string]any) channel.Future {
	args := m.Called(ctx, messages, params)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(channel.Future)
}

// Ensure interface compliance
var _ SSEOperation = (*MockSSEOperation)(nil)
