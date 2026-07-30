package channel

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	buf "github.com/yetiz-org/goth-bytebuf"
	"github.com/yetiz-org/goth-util/structs"
)

// TestByteToMessageDecoderDrainsQueuePastNilElement pins the queue-drain
// contract: a decoder is allowed to queue a nil element, and every element
// queued after it must still be fired. Draining with a nil-terminated loop
// stops at the first nil and silently drops the remaining decoded messages.
func TestByteToMessageDecoderDrainsQueuePastNilElement(t *testing.T) {
	decoder := &ByteToMessageDecoder{
		Decode: func(ctx HandlerContext, in buf.ByteBuf, out structs.Queue) {
			out.Push(nil)
			out.Push("after-nil")
		},
	}

	ctx := NewMockHandlerContext()
	var fired []any
	ctx.On("FireRead", mock.Anything).Run(func(args mock.Arguments) {
		fired = append(fired, args.Get(0))
	}).Return(ctx)
	ctx.On("FireReadCompleted").Return(ctx)

	decoder.Read(ctx, buf.NewByteBufString("payload"))

	assert.Len(t, fired, 2, "elements queued after a nil element must still be fired")
	assert.Nil(t, fired[0])
	assert.Equal(t, "after-nil", fired[1])
	ctx.AssertExpectations(t)
}

// TestByteToMessageDecoderEmptyQueueFiresNothing covers the loop-termination
// side of the same contract.
func TestByteToMessageDecoderEmptyQueueFiresNothing(t *testing.T) {
	decoder := &ByteToMessageDecoder{
		Decode: func(ctx HandlerContext, in buf.ByteBuf, out structs.Queue) {},
	}

	ctx := NewMockHandlerContext()
	ctx.On("FireReadCompleted").Return(ctx)

	decoder.Read(ctx, buf.NewByteBufString("payload"))

	ctx.AssertNotCalled(t, "FireRead", mock.Anything)
	ctx.AssertExpectations(t)
}
