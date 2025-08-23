package gws

import (
	"time"

	"github.com/stretchr/testify/mock"
)

// MockMessage is a mock implementation of Message interface
// It provides complete testify/mock integration for testing WebSocket message behaviors
type MockMessage struct {
	mock.Mock
}

// NewMockMessage creates a new MockMessage instance
func NewMockMessage() *MockMessage {
	return &MockMessage{}
}

// Type returns the message type
func (m *MockMessage) Type() MessageType {
	args := m.Called()
	return MessageType(args.Int(0))
}

// Encoded returns the encoded message bytes
func (m *MockMessage) Encoded() []byte {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).([]byte)
}

// Deadline returns the message deadline
func (m *MockMessage) Deadline() *time.Time {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*time.Time)
}

// MockMessageBuilder is a mock implementation of MessageBuilder interface
// It provides complete testify/mock integration for testing WebSocket message building behaviors
type MockMessageBuilder struct {
	mock.Mock
}

// NewMockMessageBuilder creates a new MockMessageBuilder instance
func NewMockMessageBuilder() *MockMessageBuilder {
	return &MockMessageBuilder{}
}

// Text creates a text message
func (m *MockMessageBuilder) Text(msg string) *DefaultMessage {
	args := m.Called(msg)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*DefaultMessage)
}

// Binary creates a binary message
func (m *MockMessageBuilder) Binary(msg []byte) *DefaultMessage {
	args := m.Called(msg)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*DefaultMessage)
}

// Close creates a close message
func (m *MockMessageBuilder) Close(msg []byte, closeCode CloseCode) *CloseMessage {
	args := m.Called(msg, closeCode)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*CloseMessage)
}

// Ping creates a ping message
func (m *MockMessageBuilder) Ping(msg []byte, deadline time.Time) *PingMessage {
	args := m.Called(msg, deadline)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*PingMessage)
}

// Pong creates a pong message
func (m *MockMessageBuilder) Pong(msg []byte, deadline time.Time) *PongMessage {
	args := m.Called(msg, deadline)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*PongMessage)
}

// Ensure interface compliance
var _ Message = (*MockMessage)(nil)
var _ MessageBuilder = (*MockMessageBuilder)(nil)
