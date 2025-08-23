package ghttp

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/yetiz-org/gone/channel"
)

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
