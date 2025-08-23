package channel

import (
	"github.com/stretchr/testify/mock"
	buf "github.com/yetiz-org/goth-bytebuf"
	"github.com/yetiz-org/goth-util/structs"
)

// MockMessageDecoder is a mock implementation of MessageDecoder interface
// It provides complete testify/mock integration for testing message decoding behaviors
type MockMessageDecoder struct {
	mock.Mock
}

// NewMockMessageDecoder creates a new MockMessageDecoder instance
func NewMockMessageDecoder() *MockMessageDecoder {
	return &MockMessageDecoder{}
}

// Decode decodes messages from ByteBuf to Queue
func (m *MockMessageDecoder) Decode(ctx HandlerContext, in buf.ByteBuf, out structs.Queue) {
	m.Called(ctx, in, out)
}

// Ensure interface compliance
var _ MessageDecoder = (*MockMessageDecoder)(nil)
