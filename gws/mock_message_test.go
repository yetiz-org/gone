package gws

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestMockMessage_InterfaceCompliance(t *testing.T) {
	// Test that MockMessage implements Message interface
	var mockMessage interface{} = NewMockMessage()
	assert.Implements(t, (*Message)(nil), mockMessage, "MockMessage should implement Message interface")
}

func TestMockMessageBuilder_InterfaceCompliance(t *testing.T) {
	// Test that MockMessageBuilder implements MessageBuilder interface
	var mockBuilder interface{} = NewMockMessageBuilder()
	assert.Implements(t, (*MessageBuilder)(nil), mockBuilder, "MockMessageBuilder should implement MessageBuilder interface")
}

func TestMockMessage_BasicMethods(t *testing.T) {
	mockMessage := NewMockMessage()

	// Test Type method
	expectedType := TextMessageType
	mockMessage.On("Type").Return(int(expectedType)).Once()
	result := mockMessage.Type()
	assert.Equal(t, expectedType, result, "Type should return expected message type")

	// Test Encoded method
	expectedBytes := []byte("test message data")
	mockMessage.On("Encoded").Return(expectedBytes).Once()
	result2 := mockMessage.Encoded()
	assert.Equal(t, expectedBytes, result2, "Encoded should return expected bytes")

	// Test Encoded method with nil
	mockMessage.On("Encoded").Return(nil).Once()
	result2 = mockMessage.Encoded()
	assert.Nil(t, result2, "Encoded should return nil when configured to do so")

	// Test Deadline method
	expectedTime := time.Now().Add(time.Hour)
	mockMessage.On("Deadline").Return(&expectedTime).Once()
	result3 := mockMessage.Deadline()
	assert.Equal(t, &expectedTime, result3, "Deadline should return expected time")

	// Test Deadline method with nil
	mockMessage.On("Deadline").Return(nil).Once()
	result3 = mockMessage.Deadline()
	assert.Nil(t, result3, "Deadline should return nil when no deadline")

	// Verify all expectations
	mockMessage.AssertExpectations(t)
}

func TestMockMessage_AllMessageTypes(t *testing.T) {
	mockMessage := NewMockMessage()

	// Test all message types
	messageTypes := []MessageType{
		TextMessageType,
		BinaryMessageType,
		CloseMessageType,
		PingMessageType,
		PongMessageType,
	}

	for _, msgType := range messageTypes {
		mockMessage.On("Type").Return(int(msgType)).Once()
		result := mockMessage.Type()
		assert.Equal(t, msgType, result, "Type should return correct type for %v", msgType)
	}

	// Verify all expectations
	mockMessage.AssertExpectations(t)
}

func TestMockMessageBuilder_TextMessage(t *testing.T) {
	mockBuilder := NewMockMessageBuilder()
	expectedMessage := &DefaultMessage{
		MessageType: TextMessageType,
		Message:     []byte("hello world"),
	}

	// Test Text method
	mockBuilder.On("Text", "hello world").Return(expectedMessage).Once()
	result := mockBuilder.Text("hello world")
	assert.Equal(t, expectedMessage, result, "Text should return expected DefaultMessage")

	// Test Text method with nil return
	mockBuilder.On("Text", "empty").Return(nil).Once()
	result = mockBuilder.Text("empty")
	assert.Nil(t, result, "Text should return nil when configured to do so")

	// Verify all expectations
	mockBuilder.AssertExpectations(t)
}

func TestMockMessageBuilder_BinaryMessage(t *testing.T) {
	mockBuilder := NewMockMessageBuilder()
	binaryData := []byte{0x01, 0x02, 0x03, 0x04}
	expectedMessage := &DefaultMessage{
		MessageType: BinaryMessageType,
		Message:     binaryData,
	}

	// Test Binary method
	mockBuilder.On("Binary", binaryData).Return(expectedMessage).Once()
	result := mockBuilder.Binary(binaryData)
	assert.Equal(t, expectedMessage, result, "Binary should return expected DefaultMessage")

	// Test Binary method with nil return
	mockBuilder.On("Binary", mock.AnythingOfType("[]uint8")).Return(nil).Once()
	result = mockBuilder.Binary([]byte{})
	assert.Nil(t, result, "Binary should return nil when configured to do so")

	// Verify all expectations
	mockBuilder.AssertExpectations(t)
}

func TestMockMessageBuilder_CloseMessage(t *testing.T) {
	mockBuilder := NewMockMessageBuilder()
	closeData := []byte("connection closing")
	closeCode := CloseNormalClosure
	expectedMessage := &CloseMessage{
		DefaultMessage: DefaultMessage{
			MessageType: CloseMessageType,
			Message:     closeData,
		},
		CloseCode: closeCode,
	}

	// Test Close method
	mockBuilder.On("Close", closeData, closeCode).Return(expectedMessage).Once()
	result := mockBuilder.Close(closeData, closeCode)
	assert.Equal(t, expectedMessage, result, "Close should return expected CloseMessage")

	// Test Close method with nil return
	mockBuilder.On("Close", mock.AnythingOfType("[]uint8"), mock.AnythingOfType("CloseCode")).Return(nil).Once()
	result = mockBuilder.Close([]byte{}, CloseGoingAway)
	assert.Nil(t, result, "Close should return nil when configured to do so")

	// Verify all expectations
	mockBuilder.AssertExpectations(t)
}

func TestMockMessageBuilder_PingPongMessages(t *testing.T) {
	mockBuilder := NewMockMessageBuilder()
	pingData := []byte("ping data")
	pongData := []byte("pong data")
	deadline := time.Now().Add(time.Minute)

	expectedPing := &PingMessage{
		DefaultMessage: DefaultMessage{
			MessageType: PingMessageType,
			Message:     pingData,
			Dead:        &deadline,
		},
	}

	expectedPong := &PongMessage{
		DefaultMessage: DefaultMessage{
			MessageType: PongMessageType,
			Message:     pongData,
			Dead:        &deadline,
		},
	}

	// Test Ping method
	mockBuilder.On("Ping", pingData, deadline).Return(expectedPing).Once()
	result := mockBuilder.Ping(pingData, deadline)
	assert.Equal(t, expectedPing, result, "Ping should return expected PingMessage")

	// Test Pong method
	mockBuilder.On("Pong", pongData, deadline).Return(expectedPong).Once()
	result2 := mockBuilder.Pong(pongData, deadline)
	assert.Equal(t, expectedPong, result2, "Pong should return expected PongMessage")

	// Test with nil returns
	mockBuilder.On("Ping", mock.AnythingOfType("[]uint8"), mock.AnythingOfType("time.Time")).Return(nil).Once()
	result = mockBuilder.Ping([]byte{}, time.Now())
	assert.Nil(t, result, "Ping should return nil when configured to do so")

	mockBuilder.On("Pong", mock.AnythingOfType("[]uint8"), mock.AnythingOfType("time.Time")).Return(nil).Once()
	result2 = mockBuilder.Pong([]byte{}, time.Now())
	assert.Nil(t, result2, "Pong should return nil when configured to do so")

	// Verify all expectations
	mockBuilder.AssertExpectations(t)
}

func TestMockMessageBuilder_ComplexScenarios(t *testing.T) {
	mockBuilder := NewMockMessageBuilder()

	// Test complex scenario with multiple message types in sequence
	textMsg := mockBuilder
	binaryMsg := mockBuilder
	closeMsg := mockBuilder

	// Set up expectations for a conversation flow
	mockBuilder.On("Text", "Hello").Return(&DefaultMessage{MessageType: TextMessageType}).Once()
	mockBuilder.On("Binary", mock.AnythingOfType("[]uint8")).Return(&DefaultMessage{MessageType: BinaryMessageType}).Once()
	mockBuilder.On("Close", mock.AnythingOfType("[]uint8"), CloseNormalClosure).Return(&CloseMessage{}).Once()

	// Execute the flow
	textResult := textMsg.Text("Hello")
	assert.NotNil(t, textResult, "Text message should be created")

	binaryResult := binaryMsg.Binary([]byte("binary data"))
	assert.NotNil(t, binaryResult, "Binary message should be created")

	closeResult := closeMsg.Close([]byte("goodbye"), CloseNormalClosure)
	assert.NotNil(t, closeResult, "Close message should be created")

	// Verify all expectations
	mockBuilder.AssertExpectations(t)
}

func TestMockMessage_TimestampBehavior(t *testing.T) {
	mockMessage := NewMockMessage()
	
	// Test different timestamp scenarios
	pastTime := time.Now().Add(-time.Hour)
	futureTime := time.Now().Add(time.Hour)
	
	// Test past deadline
	mockMessage.On("Deadline").Return(&pastTime).Once()
	result := mockMessage.Deadline()
	assert.True(t, result.Before(time.Now()), "Past deadline should be before current time")

	// Test future deadline
	mockMessage.On("Deadline").Return(&futureTime).Once()
	result = mockMessage.Deadline()
	assert.True(t, result.After(time.Now()), "Future deadline should be after current time")

	// Verify all expectations
	mockMessage.AssertExpectations(t)
}
