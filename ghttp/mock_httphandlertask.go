package ghttp

import (
	"github.com/stretchr/testify/mock"
	"github.com/yetiz-org/gone/channel"
)

// MockHttpHandlerTask is a mock implementation of HttpHandlerTask interface
// It provides complete testify/mock integration for testing HTTP handler behaviors
type MockHttpHandlerTask struct {
	mock.Mock
}

// NewMockHttpHandlerTask creates a new MockHttpHandlerTask instance
func NewMockHttpHandlerTask() *MockHttpHandlerTask {
	return &MockHttpHandlerTask{}
}

// HttpTask methods implementation
func (m *MockHttpHandlerTask) Index(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) ErrorResponse {
	args := m.Called(ctx, req, resp, params)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(ErrorResponse)
}

func (m *MockHttpHandlerTask) Get(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) ErrorResponse {
	args := m.Called(ctx, req, resp, params)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(ErrorResponse)
}

func (m *MockHttpHandlerTask) Create(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) ErrorResponse {
	args := m.Called(ctx, req, resp, params)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(ErrorResponse)
}

func (m *MockHttpHandlerTask) Post(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) ErrorResponse {
	args := m.Called(ctx, req, resp, params)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(ErrorResponse)
}

func (m *MockHttpHandlerTask) Put(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) ErrorResponse {
	args := m.Called(ctx, req, resp, params)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(ErrorResponse)
}

func (m *MockHttpHandlerTask) Delete(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) ErrorResponse {
	args := m.Called(ctx, req, resp, params)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(ErrorResponse)
}

func (m *MockHttpHandlerTask) Options(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) ErrorResponse {
	args := m.Called(ctx, req, resp, params)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(ErrorResponse)
}

func (m *MockHttpHandlerTask) Patch(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) ErrorResponse {
	args := m.Called(ctx, req, resp, params)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(ErrorResponse)
}

func (m *MockHttpHandlerTask) Trace(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) ErrorResponse {
	args := m.Called(ctx, req, resp, params)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(ErrorResponse)
}

func (m *MockHttpHandlerTask) Connect(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) ErrorResponse {
	args := m.Called(ctx, req, resp, params)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(ErrorResponse)
}

// HandlerTask methods implementation
func (m *MockHttpHandlerTask) Register() {
	m.Called()
}

func (m *MockHttpHandlerTask) GetNodeName(params map[string]any) string {
	args := m.Called(params)
	return args.String(0)
}

func (m *MockHttpHandlerTask) GetID(name string, params map[string]any) string {
	args := m.Called(name, params)
	return args.String(0)
}

// HttpHandlerTask specific methods implementation
func (m *MockHttpHandlerTask) CORSHelper(req *Request, resp *Response, params map[string]any) {
	m.Called(req, resp, params)
}

func (m *MockHttpHandlerTask) PreCheck(req *Request, resp *Response, params map[string]any) ErrorResponse {
	args := m.Called(req, resp, params)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(ErrorResponse)
}

func (m *MockHttpHandlerTask) Before(req *Request, resp *Response, params map[string]any) ErrorResponse {
	args := m.Called(req, resp, params)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(ErrorResponse)
}

func (m *MockHttpHandlerTask) After(req *Request, resp *Response, params map[string]any) ErrorResponse {
	args := m.Called(req, resp, params)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(ErrorResponse)
}

func (m *MockHttpHandlerTask) ErrorCaught(req *Request, resp *Response, params map[string]any, err ErrorResponse) error {
	args := m.Called(req, resp, params, err)
	return args.Error(0)
}

// Ensure interface compliance
var _ HttpHandlerTask = (*MockHttpHandlerTask)(nil)
var _ HttpTask = (*MockHttpHandlerTask)(nil)
var _ HandlerTask = (*MockHttpHandlerTask)(nil)
