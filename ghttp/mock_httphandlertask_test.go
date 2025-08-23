package ghttp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/yetiz-org/gone/channel"
	"github.com/yetiz-org/gone/erresponse"
)

func TestMockHttpHandlerTask_InterfaceCompliance(t *testing.T) {
	// Test that MockHttpHandlerTask implements all required interfaces
	var mockTask interface{} = NewMockHttpHandlerTask()
	
	// Verify interface compliance
	assert.Implements(t, (*HttpHandlerTask)(nil), mockTask, "MockHttpHandlerTask should implement HttpHandlerTask")
	assert.Implements(t, (*HttpTask)(nil), mockTask, "MockHttpHandlerTask should implement HttpTask")
	assert.Implements(t, (*HandlerTask)(nil), mockTask, "MockHttpHandlerTask should implement HandlerTask")
}

func TestMockHttpHandlerTask_HttpTaskMethods(t *testing.T) {
	mockTask := NewMockHttpHandlerTask()
	mockCtx := &channel.MockHandlerContext{}
	// Use actual Request/Response types instead of Mock types
	mockReq := &Request{}
	mockResp := &Response{}
	params := map[string]any{"test": "value"}
	expectedError := erresponse.InvalidRequest

	// Test Index method
	mockTask.On("Index", mockCtx, mockReq, mockResp, params).Return(expectedError).Once()
	result := mockTask.Index(mockCtx, mockReq, mockResp, params)
	assert.Equal(t, expectedError, result, "Index should return expected error")

	// Test Get method  
	mockTask.On("Get", mockCtx, mockReq, mockResp, params).Return(nil).Once()
	result = mockTask.Get(mockCtx, mockReq, mockResp, params)
	assert.Nil(t, result, "Get should return nil")

	// Test Post method
	mockTask.On("Post", mockCtx, mockReq, mockResp, params).Return(expectedError).Once()
	result = mockTask.Post(mockCtx, mockReq, mockResp, params)
	assert.Equal(t, expectedError, result, "Post should return expected error")

	// Test Put method
	mockTask.On("Put", mockCtx, mockReq, mockResp, params).Return(nil).Once()
	result = mockTask.Put(mockCtx, mockReq, mockResp, params)
	assert.Nil(t, result, "Put should return nil")

	// Test Delete method
	mockTask.On("Delete", mockCtx, mockReq, mockResp, params).Return(expectedError).Once()
	result = mockTask.Delete(mockCtx, mockReq, mockResp, params)
	assert.Equal(t, expectedError, result, "Delete should return expected error")

	// Verify all expectations
	mockTask.AssertExpectations(t)
}

func TestMockHttpHandlerTask_HandlerTaskMethods(t *testing.T) {
	mockTask := NewMockHttpHandlerTask()
	params := map[string]any{"node_name": "test_node"}

	// Test Register method
	mockTask.On("Register").Once()
	mockTask.Register()

	// Test GetNodeName method
	mockTask.On("GetNodeName", params).Return("test_node").Once()
	result := mockTask.GetNodeName(params)
	assert.Equal(t, "test_node", result, "GetNodeName should return expected name")

	// Test GetID method
	mockTask.On("GetID", "test", params).Return("test_id").Once()
	result = mockTask.GetID("test", params)
	assert.Equal(t, "test_id", result, "GetID should return expected ID")

	// Verify all expectations
	mockTask.AssertExpectations(t)
}

func TestMockHttpHandlerTask_HttpHandlerTaskMethods(t *testing.T) {
	mockTask := NewMockHttpHandlerTask()
	// Use actual Request/Response types instead of Mock types
	mockReq := &Request{}
	mockResp := &Response{}
	params := map[string]any{"test": "value"}
	expectedError := erresponse.ServerError

	// Test CORSHelper method
	mockTask.On("CORSHelper", mockReq, mockResp, params).Once()
	mockTask.CORSHelper(mockReq, mockResp, params)

	// Test PreCheck method
	mockTask.On("PreCheck", mockReq, mockResp, params).Return(expectedError).Once()
	result := mockTask.PreCheck(mockReq, mockResp, params)
	assert.Equal(t, expectedError, result, "PreCheck should return expected error")

	// Test Before method
	mockTask.On("Before", mockReq, mockResp, params).Return(nil).Once()
	result = mockTask.Before(mockReq, mockResp, params)
	assert.Nil(t, result, "Before should return nil")

	// Test After method
	mockTask.On("After", mockReq, mockResp, params).Return(expectedError).Once()
	result = mockTask.After(mockReq, mockResp, params)
	assert.Equal(t, expectedError, result, "After should return expected error")

	// Test ErrorCaught method
	inputError := erresponse.InvalidRequest
	expectedReturnError := assert.AnError
	mockTask.On("ErrorCaught", mockReq, mockResp, params, inputError).Return(expectedReturnError).Once()
	errorResult := mockTask.ErrorCaught(mockReq, mockResp, params, inputError)
	assert.Equal(t, expectedReturnError, errorResult, "ErrorCaught should return expected error")

	// Verify all expectations
	mockTask.AssertExpectations(t)
}

func TestMockHttpHandlerTask_ReturnsNilForNilMocks(t *testing.T) {
	mockTask := NewMockHttpHandlerTask()
	mockCtx := &channel.MockHandlerContext{}
	// Use actual Request/Response types instead of Mock types
	mockReq := &Request{}
	mockResp := &Response{}
	params := map[string]any{"test": "value"}

	// Test that nil returns work correctly
	mockTask.On("Index", mockCtx, mockReq, mockResp, params).Return(nil).Once()
	result := mockTask.Index(mockCtx, mockReq, mockResp, params)
	assert.Nil(t, result, "Index should return nil when configured to do so")

	// Test PreCheck with nil return
	mockTask.On("PreCheck", mockReq, mockResp, params).Return(nil).Once()
	result = mockTask.PreCheck(mockReq, mockResp, params)
	assert.Nil(t, result, "PreCheck should return nil when configured to do so")

	// Verify all expectations
	mockTask.AssertExpectations(t)
}

func TestMockHttpHandlerTask_MockBehaviorCustomization(t *testing.T) {
	mockTask := NewMockHttpHandlerTask()
	mockCtx := &channel.MockHandlerContext{}
	// Use actual Request/Response types instead of Mock types
	mockReq := &Request{}
	mockResp := &Response{}
	params := map[string]any{"custom": "test"}

	// Test custom mock behavior with multiple calls
	mockTask.On("Get", mockCtx, mockReq, mockResp, mock.MatchedBy(func(p map[string]any) bool {
		return p["custom"] == "test"
	})).Return(nil).Times(3)

	// Make multiple calls
	for i := 0; i < 3; i++ {
		result := mockTask.Get(mockCtx, mockReq, mockResp, params)
		assert.Nil(t, result, "Get should return nil for custom matcher")
	}

	// Verify all expectations
	mockTask.AssertExpectations(t)
}
