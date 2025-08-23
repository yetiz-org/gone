package channel

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	buf "github.com/yetiz-org/goth-bytebuf"
)

func TestMockMessageEncoder_InterfaceCompliance(t *testing.T) {
	// Test that MockMessageEncoder implements MessageEncoder interface
	var mockEncoder interface{} = NewMockMessageEncoder()
	assert.Implements(t, (*MessageEncoder)(nil), mockEncoder, "MockMessageEncoder should implement MessageEncoder interface")
}

func TestMockMessageEncoder_Encode(t *testing.T) {
	mockEncoder := NewMockMessageEncoder()
	mockCtx := NewMockHandlerContext()
	testMessage := "test message"
	outputBuffer := buf.EmptyByteBuf()

	// Test basic Encode functionality
	mockEncoder.On("Encode", mockCtx, testMessage, outputBuffer).Once()
	
	// Call the method
	mockEncoder.Encode(mockCtx, testMessage, outputBuffer)

	// Verify expectations were met
	mockEncoder.AssertExpectations(t)
}

func TestMockMessageEncoder_EncodeWithDifferentTypes(t *testing.T) {
	mockEncoder := NewMockMessageEncoder()
	mockCtx := NewMockHandlerContext()
	outputBuffer := buf.EmptyByteBuf()

	// Test encoding different message types
	testCases := []struct {
		name    string
		message interface{}
	}{
		{"string message", "hello world"},
		{"integer message", 12345},
		{"struct message", struct{ Name string }{"test"}},
		{"byte slice", []byte("binary data")},
		{"nil message", nil},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockEncoder.On("Encode", mockCtx, tc.message, outputBuffer).Once()
			mockEncoder.Encode(mockCtx, tc.message, outputBuffer)
		})
	}

	// Verify all expectations
	mockEncoder.AssertExpectations(t)
}

func TestMockMessageEncoder_EncodeWithMatchers(t *testing.T) {
	mockEncoder := NewMockMessageEncoder()
	mockCtx := NewMockHandlerContext()
	outputBuffer := buf.EmptyByteBuf()

	// Test with custom matchers
	mockEncoder.On("Encode", 
		mock.AnythingOfType("*channel.MockHandlerContext"),
		mock.MatchedBy(func(msg interface{}) bool {
			str, ok := msg.(string)
			return ok && len(str) > 0
		}),
		mock.AnythingOfType("*buf.DefaultByteBuf"),
	).Times(3)

	// Test multiple calls with different string messages
	testMessages := []string{"first", "second", "third"}
	for _, msg := range testMessages {
		mockEncoder.Encode(mockCtx, msg, outputBuffer)
	}

	// Verify all expectations
	mockEncoder.AssertExpectations(t)
}

func TestMockMessageEncoder_MultipleContexts(t *testing.T) {
	mockEncoder := NewMockMessageEncoder()
	ctx1 := NewMockHandlerContext()
	ctx2 := NewMockHandlerContext()
	outputBuffer := buf.EmptyByteBuf()
	message := "test message"

	// Test encoding with different contexts
	mockEncoder.On("Encode", ctx1, message, outputBuffer).Once()
	mockEncoder.On("Encode", ctx2, message, outputBuffer).Once()

	mockEncoder.Encode(ctx1, message, outputBuffer)
	mockEncoder.Encode(ctx2, message, outputBuffer)

	// Verify all expectations
	mockEncoder.AssertExpectations(t)
}

func TestMockMessageEncoder_ConcurrentUsage(t *testing.T) {
	mockEncoder := NewMockMessageEncoder()
	mockCtx := NewMockHandlerContext()
	outputBuffer := buf.EmptyByteBuf()
	
	// Test that the mock can handle concurrent expectations
	numCalls := 10
	mockEncoder.On("Encode", mockCtx, mock.AnythingOfType("string"), outputBuffer).Times(numCalls)

	// Simulate concurrent calls
	done := make(chan bool, numCalls)
	for i := 0; i < numCalls; i++ {
		go func(id int) {
			defer func() { done <- true }()
			message := fmt.Sprintf("message-%d", id)
			mockEncoder.Encode(mockCtx, message, outputBuffer)
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numCalls; i++ {
		<-done
	}

	// Verify all expectations
	mockEncoder.AssertExpectations(t)
}
