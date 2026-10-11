package channel

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	buf "github.com/yetiz-org/goth-bytebuf"
	"github.com/yetiz-org/goth-util/structs"
)

func TestReplayDecoder_BufferBudget(t *testing.T) {
	for _, chunks := range [][]int{{16<<20 + 1}, {16 << 20, 1}} {
		t.Run("chunks", func(t *testing.T) {
			calls := 0
			decoder := NewReplayDecoder(0, func(ctx HandlerContext, in buf.ByteBuf, out structs.Queue) {
				calls++
				panic(buf.ErrInsufficientSize)
			})
			ctx := NewMockHandlerContext()
			var caught error
			closed := false
			ctx.On("FireErrorCaught", mock.Anything).Run(func(args mock.Arguments) {
				caught = args.Error(0)
			}).Return(ctx).Maybe()
			ctx.On("Disconnect", mock.Anything).Run(func(args mock.Arguments) {
				closed = true
			}).Return(NewFuture(nil)).Maybe()
			decoder.Added(ctx)
			for _, n := range chunks {
				decoder.Read(ctx, buf.NewByteBuf(make([]byte, n)))
			}
			require.Error(t, caught, "excess unread input must be rejected")
			require.True(t, closed, "an over-budget stream must close")
			require.Zero(t, decoder.in.ReadableBytes(), "rejected input must be released")
			previousCalls := calls
			decoder.Read(ctx, buf.NewByteBuf([]byte{1}))
			require.Equal(t, previousCalls, calls, "rejected streams must not decode further input")
		})
	}
}

func TestReplayDecoder_DecodeFailure(t *testing.T) {
	malformed := errors.New("malformed framing")
	calls := 0
	decoder := NewReplayDecoder(0, func(ctx HandlerContext, in buf.ByteBuf, out structs.Queue) {
		calls++
		panic(malformed)
	})
	ctx := NewMockHandlerContext()
	var caught error
	closed := false
	ctx.On("FireErrorCaught", mock.Anything).Run(func(args mock.Arguments) {
		caught = args.Error(0)
	}).Return(ctx).Maybe()
	ctx.On("Disconnect", mock.Anything).Run(func(args mock.Arguments) {
		closed = true
	}).Return(NewFuture(nil)).Maybe()
	decoder.Added(ctx)
	decoder.Read(ctx, buf.NewByteBuf([]byte{1, 2, 3}))
	require.ErrorIs(t, caught, malformed)
	require.True(t, closed)
	require.Zero(t, decoder.in.ReadableBytes())
	decoder.Read(ctx, buf.NewByteBuf([]byte{4}))
	require.Equal(t, 1, calls)
}

func TestReplayDecoder_ConsumedInputBudget(t *testing.T) {
	decoded := 0
	decoder := NewReplayDecoder(0, func(ctx HandlerContext, in buf.ByteBuf, out structs.Queue) {
		decoded += in.ReadableBytes()
		in.Skip(in.ReadableBytes())
	})
	ctx := NewMockHandlerContext()
	decoder.Added(ctx)
	for range 2 {
		decoder.Read(ctx, buf.NewByteBuf(make([]byte, 16<<20)))
	}
	require.Equal(t, 32<<20, decoded)
	require.Zero(t, decoder.in.Cap(), "consumed components must not remain retained indefinitely")
}
