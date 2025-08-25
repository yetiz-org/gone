package ghttp

// This comprehensive test file merges:
// - mock_sseoperation_test.go
// - mock_route_test.go  
// - mock_httphandlertask_test.go
// Original files will be archived to avoid duplicate execution.

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/yetiz-org/gone/channel"
	"github.com/yetiz-org/gone/erresponse"
)

// =============================================================================
// MockSSEOperation Tests (from mock_sseoperation_test.go)
// =============================================================================

func TestMockSSEOperation_InterfaceCompliance(t *testing.T) {
	// Test that MockSSEOperation implements SSEOperation interface
	var mockSSE interface{} = NewMockSSEOperation()
	assert.Implements(t, (*SSEOperation)(nil), mockSSE, "MockSSEOperation should implement SSEOperation interface")
}

func TestMockSSEOperation_WriteHeader(t *testing.T) {
	mockSSE := NewMockSSEOperation()
	mockCtx := &channel.MockHandlerContext{}
	mockFuture := &channel.MockFuture{}
	header := make(http.Header)
	header.Set("Content-Type", "text/event-stream")
	params := map[string]any{"test": "value"}

	// Test WriteHeader method with successful result
	mockSSE.On("WriteHeader", mockCtx, header, params).Return(mockFuture).Once()
	result := mockSSE.WriteHeader(mockCtx, header, params)
	assert.Equal(t, mockFuture, result, "WriteHeader should return expected future")

	// Test WriteHeader method with nil result
	mockSSE.On("WriteHeader", mockCtx, mock.AnythingOfType("http.Header"), params).Return(nil).Once()
	result = mockSSE.WriteHeader(mockCtx, http.Header{}, params)
	assert.Nil(t, result, "WriteHeader should return nil when configured to do so")

	// Verify all expectations
	mockSSE.AssertExpectations(t)
}

func TestMockSSEOperation_WriteMessage(t *testing.T) {
	mockSSE := NewMockSSEOperation()
	mockCtx := &channel.MockHandlerContext{}
	mockFuture := &channel.MockFuture{}
	params := map[string]any{"test": "value"}

	// Test message with all fields
	message := SSEMessage{
		Comment: "This is a comment",
		Event:   "user-message",
		Data:    []string{"Hello", "World"},
		Id:      "msg-123",
		Retry:   5000,
	}

	// Test WriteMessage method
	mockSSE.On("WriteMessage", mockCtx, message, params).Return(mockFuture).Once()
	result := mockSSE.WriteMessage(mockCtx, message, params)
	assert.Equal(t, mockFuture, result, "WriteMessage should return expected future")

	// Test with minimal message
	minimalMessage := SSEMessage{
		Data: []string{"Simple message"},
	}

	mockSSE.On("WriteMessage", mockCtx, minimalMessage, params).Return(mockFuture).Once()
	result = mockSSE.WriteMessage(mockCtx, minimalMessage, params)
	assert.Equal(t, mockFuture, result, "WriteMessage should handle minimal message")

	// Test WriteMessage with nil result
	mockSSE.On("WriteMessage", mockCtx, mock.AnythingOfType("SSEMessage"), params).Return(nil).Once()
	result = mockSSE.WriteMessage(mockCtx, SSEMessage{}, params)
	assert.Nil(t, result, "WriteMessage should return nil when configured to do so")

	// Verify all expectations
	mockSSE.AssertExpectations(t)
}

func TestMockSSEOperation_WriteMessages(t *testing.T) {
	mockSSE := NewMockSSEOperation()
	mockCtx := &channel.MockHandlerContext{}
	mockFuture := &channel.MockFuture{}
	params := map[string]any{"test": "value"}

	// Test multiple messages
	messages := []SSEMessage{
		{
			Event: "message1",
			Data:  []string{"First message"},
			Id:    "1",
		},
		{
			Event: "message2",
			Data:  []string{"Second message"},
			Id:    "2",
		},
		{
			Event: "message3",
			Data:  []string{"Third message"},
			Id:    "3",
		},
	}

	// Test WriteMessages method
	mockSSE.On("WriteMessages", mockCtx, messages, params).Return(mockFuture).Once()
	result := mockSSE.WriteMessages(mockCtx, messages, params)
	assert.Equal(t, mockFuture, result, "WriteMessages should return expected future")

	// Test with empty message slice
	emptyMessages := []SSEMessage{}
	mockSSE.On("WriteMessages", mockCtx, emptyMessages, params).Return(mockFuture).Once()
	result = mockSSE.WriteMessages(mockCtx, emptyMessages, params)
	assert.Equal(t, mockFuture, result, "WriteMessages should handle empty message slice")

	// Test WriteMessages with nil result using different parameters to avoid conflict
	paramsNil := map[string]any{"test": "nil"}
	mockSSE.On("WriteMessages", mockCtx, mock.AnythingOfType("[]ghttp.SSEMessage"), paramsNil).Return(nil).Once()
	result = mockSSE.WriteMessages(mockCtx, []SSEMessage{}, paramsNil)
	assert.Nil(t, result, "WriteMessages should return nil when configured to do so")

	// Verify all expectations
	mockSSE.AssertExpectations(t)
}

func TestMockSSEOperation_ComplexScenarios(t *testing.T) {
	mockSSE := NewMockSSEOperation()
	mockCtx := &channel.MockHandlerContext{}
	mockFuture := &channel.MockFuture{}
	params := map[string]any{"session_id": "sess_123"}

	header := make(http.Header)
	header.Set("Content-Type", "text/event-stream")
	header.Set("Cache-Control", "no-cache")

	// Test a typical SSE flow: WriteHeader -> WriteMessage -> WriteMessages
	mockSSE.On("WriteHeader", mockCtx, header, params).Return(mockFuture).Once()
	
	message := SSEMessage{
		Event: "connection-established",
		Data:  []string{"Connected successfully"},
		Id:    "conn-1",
	}
	mockSSE.On("WriteMessage", mockCtx, message, params).Return(mockFuture).Once()

	messages := []SSEMessage{
		{Event: "data-update", Data: []string{"Update 1"}, Id: "upd-1"},
		{Event: "data-update", Data: []string{"Update 2"}, Id: "upd-2"},
	}
	mockSSE.On("WriteMessages", mockCtx, messages, params).Return(mockFuture).Once()

	// Execute the flow
	result1 := mockSSE.WriteHeader(mockCtx, header, params)
	assert.Equal(t, mockFuture, result1, "WriteHeader should succeed")

	result2 := mockSSE.WriteMessage(mockCtx, message, params)
	assert.Equal(t, mockFuture, result2, "WriteMessage should succeed")

	result3 := mockSSE.WriteMessages(mockCtx, messages, params)
	assert.Equal(t, mockFuture, result3, "WriteMessages should succeed")

	// Verify all expectations
	mockSSE.AssertExpectations(t)
}

func TestMockSSEOperation_ParameterMatching(t *testing.T) {
	mockSSE := NewMockSSEOperation()
	mockCtx := &channel.MockHandlerContext{}
	mockFuture := &channel.MockFuture{}

	// Test with parameter matching
	mockSSE.On("WriteMessage", 
		mock.AnythingOfType("*channel.MockHandlerContext"),
		mock.MatchedBy(func(msg SSEMessage) bool {
			return msg.Event == "test-event" && len(msg.Data) > 0
		}),
		mock.MatchedBy(func(params map[string]any) bool {
			sessionID, exists := params["session_id"]
			return exists && sessionID == "valid_session"
		}),
	).Return(mockFuture).Times(2)

	// Test matching calls
	validParams := map[string]any{"session_id": "valid_session"}
	validMessage1 := SSEMessage{Event: "test-event", Data: []string{"data1"}}
	validMessage2 := SSEMessage{Event: "test-event", Data: []string{"data2"}}

	result1 := mockSSE.WriteMessage(mockCtx, validMessage1, validParams)
	assert.Equal(t, mockFuture, result1)

	result2 := mockSSE.WriteMessage(mockCtx, validMessage2, validParams)
	assert.Equal(t, mockFuture, result2)

	// Verify all expectations
	mockSSE.AssertExpectations(t)
}

func TestMockSSEOperation_ErrorHandling(t *testing.T) {
	mockSSE := NewMockSSEOperation()
	mockCtx := &channel.MockHandlerContext{}
	params := map[string]any{"test": "error_case"}

	// Test error scenarios where methods return nil (indicating failure)
	mockSSE.On("WriteHeader", mockCtx, mock.AnythingOfType("http.Header"), params).Return(nil).Once()
	mockSSE.On("WriteMessage", mockCtx, mock.AnythingOfType("ghttp.SSEMessage"), params).Return(nil).Once()
	mockSSE.On("WriteMessages", mockCtx, mock.AnythingOfType("[]ghttp.SSEMessage"), params).Return(nil).Once()

	// Execute error scenarios
	header := make(http.Header)
	result1 := mockSSE.WriteHeader(mockCtx, header, params)
	assert.Nil(t, result1, "WriteHeader should return nil on error")

	message := SSEMessage{Data: []string{"test"}}
	result2 := mockSSE.WriteMessage(mockCtx, message, params)
	assert.Nil(t, result2, "WriteMessage should return nil on error")

	messages := []SSEMessage{{Data: []string{"test"}}}
	result3 := mockSSE.WriteMessages(mockCtx, messages, params)
	assert.Nil(t, result3, "WriteMessages should return nil on error")

	// Verify all expectations
	mockSSE.AssertExpectations(t)
}

func TestSSEMessage_Validation(t *testing.T) {
	// Test SSEMessage validation behavior (this tests the actual struct, not the mock)
	
	// Valid messages
	validMessage1 := SSEMessage{Data: []string{"test"}}
	assert.True(t, validMessage1.Validate(), "Message with data should be valid")

	validMessage2 := SSEMessage{Event: "test-event"}
	assert.True(t, validMessage2.Validate(), "Message with event should be valid")

	validMessage3 := SSEMessage{Id: "123"}
	assert.True(t, validMessage3.Validate(), "Message with ID should be valid")

	validMessage4 := SSEMessage{Retry: 1000}
	assert.True(t, validMessage4.Validate(), "Message with retry should be valid")

	validMessage5 := SSEMessage{Comment: "test comment"}
	assert.True(t, validMessage5.Validate(), "Message with comment should be valid")

	// Invalid message (all fields empty/zero)
	invalidMessage := SSEMessage{}
	assert.False(t, invalidMessage.Validate(), "Empty message should be invalid")
}

// =============================================================================
// MockRoute Tests (from mock_route_test.go)
// =============================================================================

func TestMockRoute_InterfaceCompliance(t *testing.T) {
	// Test that MockRoute implements Route interface
	var mockRoute interface{} = NewMockRoute()
	assert.Implements(t, (*Route)(nil), mockRoute, "MockRoute should implement Route interface")
}

func TestMockRouteNode_InterfaceCompliance(t *testing.T) {
	// Test that MockRouteNode implements RouteNode interface
	var mockNode interface{} = NewMockRouteNode()
	assert.Implements(t, (*RouteNode)(nil), mockNode, "MockRouteNode should implement RouteNode interface")
}

func TestMockRoute_RouteNode(t *testing.T) {
	mockRoute := NewMockRoute()
	mockNode := NewMockRouteNode()
	params := map[string]any{"id": "123"}
	path := "/api/users/123"

	// Test RouteNode method with successful result
	mockRoute.On("RouteNode", path).Return(mockNode, params, true).Once()
	
	resultNode, resultParams, isLast := mockRoute.RouteNode(path)
	assert.Equal(t, mockNode, resultNode, "RouteNode should return expected node")
	assert.Equal(t, params, resultParams, "RouteNode should return expected params")
	assert.True(t, isLast, "RouteNode should return true for isLast")

	// Test RouteNode method with nil result
	mockRoute.On("RouteNode", "/not/found").Return(nil, nil, false).Once()
	
	resultNode, resultParams, isLast = mockRoute.RouteNode("/not/found")
	assert.Nil(t, resultNode, "RouteNode should return nil for not found path")
	assert.Nil(t, resultParams, "RouteNode should return nil params for not found path")
	assert.False(t, isLast, "RouteNode should return false for not found path")

	// Verify all expectations
	mockRoute.AssertExpectations(t)
}

func TestMockRouteNode_BasicMethods(t *testing.T) {
	mockNode := NewMockRouteNode()
	mockParent := NewMockRouteNode()
	mockHandlerTask := NewMockHttpHandlerTask()

	// Test Parent method
	mockNode.On("Parent").Return(mockParent).Once()
	result := mockNode.Parent()
	assert.Equal(t, mockParent, result, "Parent should return expected parent node")

	// Test Parent method with nil
	mockNode.On("Parent").Return(nil).Once()
	result = mockNode.Parent()
	assert.Nil(t, result, "Parent should return nil when no parent")

	// Test HandlerTask method
	mockNode.On("HandlerTask").Return(mockHandlerTask).Once()
	handlerResult := mockNode.HandlerTask()
	assert.Equal(t, mockHandlerTask, handlerResult, "HandlerTask should return expected handler")

	// Test HandlerTask method with nil
	mockNode.On("HandlerTask").Return(nil).Once()
	handlerResult = mockNode.HandlerTask()
	assert.Nil(t, handlerResult, "HandlerTask should return nil when no handler")

	// Test Name method
	expectedName := "api-endpoint"
	mockNode.On("Name").Return(expectedName).Once()
	nameResult := mockNode.Name()
	assert.Equal(t, expectedName, nameResult, "Name should return expected name")

	// Verify all expectations
	mockNode.AssertExpectations(t)
}

func TestMockRouteNode_AcceptanceMethods(t *testing.T) {
	mockNode := NewMockRouteNode()
	expectedAcceptances := []Acceptance{
		// Note: Acceptance is an interface, would need actual implementations for real testing
	}
	
	// Test AggregatedAcceptances method
	mockNode.On("AggregatedAcceptances").Return(expectedAcceptances).Once()
	result := mockNode.AggregatedAcceptances()
	assert.Equal(t, expectedAcceptances, result, "AggregatedAcceptances should return expected acceptances")

	// Test AggregatedAcceptances method with nil
	mockNode.On("AggregatedAcceptances").Return(nil).Once()
	result = mockNode.AggregatedAcceptances()
	assert.Nil(t, result, "AggregatedAcceptances should return nil when none")

	// Test Acceptances method
	mockNode.On("Acceptances").Return(expectedAcceptances).Once()
	result = mockNode.Acceptances()
	assert.Equal(t, expectedAcceptances, result, "Acceptances should return expected acceptances")

	// Test Acceptances method with nil
	mockNode.On("Acceptances").Return(nil).Once()
	result = mockNode.Acceptances()
	assert.Nil(t, result, "Acceptances should return nil when none")

	// Verify all expectations
	mockNode.AssertExpectations(t)
}

func TestMockRouteNode_ResourcesAndType(t *testing.T) {
	mockNode := NewMockRouteNode()
	mockChildNode := NewMockRouteNode()
	expectedResources := map[string]RouteNode{
		"child": mockChildNode,
	}

	// Test Resources method
	mockNode.On("Resources").Return(expectedResources).Once()
	result := mockNode.Resources()
	assert.Equal(t, expectedResources, result, "Resources should return expected resources map")

	// Test Resources method with nil
	mockNode.On("Resources").Return(nil).Once()
	result = mockNode.Resources()
	assert.Nil(t, result, "Resources should return nil when no resources")

	// Test RouteType method
	expectedType := RouteTypeEndPoint
	mockNode.On("RouteType").Return(int(expectedType)).Once()
	typeResult := mockNode.RouteType()
	assert.Equal(t, expectedType, typeResult, "RouteType should return expected type")

	// Test different route types
	testTypes := []RouteType{
		RouteTypeEndPoint,
		RouteTypeGroup,
		RouteTypeRecursiveEndPoint,
		RouteTypeRootEndPoint,
	}

	for _, routeType := range testTypes {
		mockNode.On("RouteType").Return(int(routeType)).Once()
		result := mockNode.RouteType()
		assert.Equal(t, routeType, result, "RouteType should return correct type for %v", routeType)
	}

	// Verify all expectations
	mockNode.AssertExpectations(t)
}

func TestMockRoute_ComplexRouting(t *testing.T) {
	mockRoute := NewMockRoute()
	
	// Test complex routing scenarios
	testCases := []struct {
		path       string
		expectNode bool
		expectLast bool
		params     map[string]any
	}{
		{"/api/v1/users", true, false, map[string]any{"version": "v1"}},
		{"/api/v1/users/123", true, true, map[string]any{"version": "v1", "id": "123"}},
		{"/api/v2/posts/456/comments", true, false, map[string]any{"version": "v2", "post_id": "456"}},
		{"/invalid/path", false, false, nil},
	}

	for _, tc := range testCases {
		var expectedNode RouteNode
		if tc.expectNode {
			expectedNode = NewMockRouteNode()
		}

		mockRoute.On("RouteNode", tc.path).Return(expectedNode, tc.params, tc.expectLast).Once()
		
		node, params, isLast := mockRoute.RouteNode(tc.path)
		
		if tc.expectNode {
			assert.NotNil(t, node, "Expected node for path %s", tc.path)
		} else {
			assert.Nil(t, node, "Expected nil node for path %s", tc.path)
		}
		
		assert.Equal(t, tc.params, params, "Expected params for path %s", tc.path)
		assert.Equal(t, tc.expectLast, isLast, "Expected isLast value for path %s", tc.path)
	}

	// Verify all expectations
	mockRoute.AssertExpectations(t)
}

func TestMockRouteNode_ChainedCalls(t *testing.T) {
	// Test chained method calls
	mockRoot := NewMockRouteNode()
	mockChild := NewMockRouteNode()
	mockGrandchild := NewMockRouteNode()

	// Set up a chain: root -> child -> grandchild
	mockRoot.On("Name").Return("root").Maybe()
	mockRoot.On("Resources").Return(map[string]RouteNode{"child": mockChild}).Maybe()
	
	mockChild.On("Name").Return("child").Maybe()
	mockChild.On("Parent").Return(mockRoot).Maybe()
	mockChild.On("Resources").Return(map[string]RouteNode{"grandchild": mockGrandchild}).Maybe()
	
	mockGrandchild.On("Name").Return("grandchild").Maybe()
	mockGrandchild.On("Parent").Return(mockChild).Maybe()
	mockGrandchild.On("Resources").Return(map[string]RouteNode{}).Maybe()

	// Test the chain
	rootName := mockRoot.Name()
	assert.Equal(t, "root", rootName)
	
	rootResources := mockRoot.Resources()
	assert.Contains(t, rootResources, "child")
	
	childNode := rootResources["child"]
	childName := childNode.Name()
	assert.Equal(t, "child", childName)
	
	parent := childNode.Parent()
	assert.Equal(t, mockRoot, parent)

	// Note: AssertExpectations might be tricky with Maybe() calls in complex scenarios
	// but this demonstrates the mock can handle chained navigation
}

// =============================================================================
// MockHttpHandlerTask Tests (from mock_httphandlertask_test.go)
// =============================================================================

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
