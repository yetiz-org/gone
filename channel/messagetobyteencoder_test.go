package channel

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	buf "github.com/yetiz-org/goth-bytebuf"
)

func TestMessageToByteEncoder_DefaultPassesByteBuf(t *testing.T) {
	encoder := &MessageToByteEncoder{}
	ctx := NewMockHandlerContext()
	future := NewFuture(nil)

	encoder.Added(ctx)
	ctx.On("Write", mock.MatchedBy(func(out buf.ByteBuf) bool {
		return string(out.Bytes()) == "payload"
	}), future).Return(future)

	assert.NotPanics(t, func() {
		encoder.Write(ctx, buf.NewByteBufString("payload"), future)
	})
	ctx.AssertExpectations(t)
}

func TestMessageToByteEncoder_DefaultFailsUnsupportedInput(t *testing.T) {
	encoder := &MessageToByteEncoder{}
	ctx := NewMockHandlerContext()
	future := NewFuture(nil)

	encoder.Added(ctx)
	ctx.On("FireErrorCaught", mock.MatchedBy(func(err error) bool {
		return errors.Is(err, ErrUnknownObjectType)
	})).Return(ctx)

	assert.NotPanics(t, func() {
		encoder.Write(ctx, "not a byte buffer", future)
	})
	assert.True(t, future.IsDone())
	assert.ErrorIs(t, future.Error(), ErrUnknownObjectType)
	ctx.AssertExpectations(t)
	ctx.AssertNotCalled(t, "Write", mock.Anything, mock.Anything)
}
