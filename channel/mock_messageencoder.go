package channel

import (
	"github.com/stretchr/testify/mock"
	buf "github.com/yetiz-org/goth-bytebuf"
)

// MockMessageEncoder is a mock implementation of MessageEncoder interface
// It provides complete testify/mock integration for testing message encoding behaviors
type MockMessageEncoder struct {
	mock.Mock
}

// NewMockMessageEncoder creates a new MockMessageEncoder instance
func NewMockMessageEncoder() *MockMessageEncoder {
	return &MockMessageEncoder{}
}

// Encode encodes a message to ByteBuf
func (m *MockMessageEncoder) Encode(ctx HandlerContext, msg any, out buf.ByteBuf) {
	m.Called(ctx, msg, out)
}

// Ensure interface compliance
var _ MessageEncoder = (*MockMessageEncoder)(nil)
