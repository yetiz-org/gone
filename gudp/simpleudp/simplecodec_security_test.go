package simpleudp

import (
	"math"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/yetiz-org/gone/channel"
	"github.com/yetiz-org/gone/utils"
	buf "github.com/yetiz-org/goth-bytebuf"
)

func TestSimpleCodec_DatagramIsolation(t *testing.T) {
	for _, partial := range [][]byte{{3, 'A'}, {0xfd, 0}, {1, 'A', 3, 'B'}} {
		t.Run("partial", func(t *testing.T) {
			codec := NewSimpleCodec()
			ctx := channel.NewMockHandlerContext()
			var messages [][]byte
			var caught error
			ctx.On("FireRead", mock.Anything).Run(func(args mock.Arguments) {
				messages = append(messages, args.Get(0).(buf.ByteBuf).BytesCopy())
			}).Return(ctx).Maybe()
			ctx.On("FireErrorCaught", mock.Anything).Run(func(args mock.Arguments) {
				caught = args.Error(0)
			}).Return(ctx).Maybe()
			ctx.On("Disconnect", mock.Anything).Return(channel.NewFuture(nil)).Maybe()
			codec.Added(ctx)
			codec.Read(ctx, buf.NewByteBuf(partial))
			require.ErrorIs(t, caught, channel.ErrMalformedFrame)
			require.Empty(t, messages, "a malformed datagram must not deliver partial messages")
			codec.Read(ctx, buf.NewByteBuf([]byte{0, 1, 'Z', 1, 'Y', 0}))
			require.Equal(t, [][]byte{{'Z'}, {'Y'}}, messages)
			ctx.AssertNotCalled(t, "Close", mock.Anything)
		})
	}
}

func TestSimpleCodec_RejectsOversizedFrames(t *testing.T) {
	for _, length := range []uint64{65505, uint64(math.MaxInt) + 1, math.MaxUint64} {
		t.Run("length", func(t *testing.T) {
			codec := NewSimpleCodec()
			ctx := channel.NewMockHandlerContext()
			var caught error
			ctx.On("FireErrorCaught", mock.Anything).Run(func(args mock.Arguments) {
				caught = args.Error(0)
			}).Return(ctx).Maybe()
			ctx.On("Disconnect", mock.Anything).Return(channel.NewFuture(nil)).Maybe()
			codec.Added(ctx)
			codec.Read(ctx, utils.VarIntEncode(length))
			require.ErrorIs(t, caught, channel.ErrFrameTooLarge)
			ctx.AssertNotCalled(t, "FireRead", mock.Anything)
		})
	}
}

func TestSimpleCodec_DatagramBoundary(t *testing.T) {
	for _, n := range []int{65504, 65505} {
		t.Run("write", func(t *testing.T) {
			codec := NewSimpleCodec()
			ctx := channel.NewMockHandlerContext()
			future := channel.NewFuture(nil)
			var written buf.ByteBuf
			var caught error
			ctx.On("Write", mock.Anything, future).Run(func(args mock.Arguments) {
				written = args.Get(0).(buf.ByteBuf)
			}).Return(future).Maybe()
			ctx.On("FireErrorCaught", mock.Anything).Run(func(args mock.Arguments) {
				caught = args.Error(0)
			}).Return(ctx).Maybe()
			codec.Write(ctx, buf.NewByteBuf(make([]byte, n)), future)
			if n > 65504 {
				require.Nil(t, written)
				require.True(t, future.IsDone())
				require.ErrorIs(t, future.Error(), channel.ErrFrameTooLarge)
				require.NoError(t, caught, "an oversized write fails its future without an inbound error")
				return
			}
			require.NoError(t, caught)
			require.NotNil(t, written)
			require.Equal(t, 65507, written.ReadableBytes())
			codec.Added(ctx)
			decoded := 0
			ctx.On("FireRead", mock.Anything).Run(func(args mock.Arguments) {
				decoded = args.Get(0).(buf.ByteBuf).ReadableBytes()
			}).Return(ctx)
			codec.Read(ctx, written)
			require.NoError(t, caught)
			require.Equal(t, n, decoded)
		})
	}
}
