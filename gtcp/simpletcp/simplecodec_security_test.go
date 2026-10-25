package simpletcp

import (
	"bytes"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/yetiz-org/gone/channel"
	"github.com/yetiz-org/gone/utils"
	buf "github.com/yetiz-org/goth-bytebuf"
)

func TestSimpleCodec_RejectsOversizedFrames(t *testing.T) {
	for _, length := range []uint64{16<<20 + 1, uint64(math.MaxInt) + 1, math.MaxUint64} {
		t.Run("length", func(t *testing.T) {
			codec := NewSimpleCodec()
			ctx := channel.NewMockHandlerContext()
			ctx.On("Channel").Return(&channel.DefaultChannel{}).Maybe()
			var caught error
			closed := false
			ctx.On("FireErrorCaught", mock.Anything).Run(func(args mock.Arguments) {
				caught = args.Error(0)
			}).Return(ctx).Maybe()
			ctx.On("Disconnect", mock.Anything).Run(func(args mock.Arguments) {
				closed = true
			}).Return(channel.NewFuture(nil)).Maybe()
			codec.Added(ctx)
			codec.Read(ctx, utils.VarIntEncode(length))
			require.ErrorIs(t, caught, channel.ErrFrameTooLarge)
			require.True(t, closed)
			codec.Read(ctx, buf.NewByteBuf([]byte{1, 'x'}))
			ctx.AssertNotCalled(t, "FireRead", mock.Anything)
		})
	}
}

func TestSimpleCodec_StreamFraming(t *testing.T) {
	codec := NewSimpleCodec()
	ctx := channel.NewMockHandlerContext()
	ctx.On("Channel").Return(&channel.DefaultChannel{}).Maybe()
	var messages [][]byte
	ctx.On("FireRead", mock.Anything).Run(func(args mock.Arguments) {
		messages = append(messages, args.Get(0).(buf.ByteBuf).BytesCopy())
	}).Return(ctx)
	codec.Added(ctx)
	payload := bytes.Repeat([]byte{'a'}, 300)
	header := utils.VarIntEncode(uint64(len(payload))).BytesCopy()
	for _, part := range [][]byte{{0, header[0]}, header[1:2], header[2:], payload[:100]} {
		codec.Read(ctx, buf.NewByteBuf(part))
		require.Empty(t, messages)
	}
	last := append(bytes.Clone(payload[100:]), 0, 1, 'z', 0)
	codec.Read(ctx, buf.NewByteBuf(last))
	require.Equal(t, [][]byte{payload, {'z'}}, messages)
}

func TestSimpleCodec_FrameBoundary(t *testing.T) {
	for _, n := range []int{16 << 20, 16<<20 + 1} {
		t.Run("write", func(t *testing.T) {
			codec := NewSimpleCodec()
			ctx := channel.NewMockHandlerContext()
			ctx.On("Channel").Return(&channel.DefaultChannel{}).Maybe()
			future := channel.NewFuture(nil)
			ctx.On("Disconnect", mock.Anything).Return(channel.NewFuture(nil)).Maybe()
			var written buf.ByteBuf
			var caught error
			ctx.On("Write", mock.Anything, future).Run(func(args mock.Arguments) {
				written = args.Get(0).(buf.ByteBuf)
			}).Return(future).Maybe()
			ctx.On("FireErrorCaught", mock.Anything).Run(func(args mock.Arguments) {
				caught = args.Error(0)
			}).Return(ctx).Maybe()
			codec.Write(ctx, buf.NewByteBuf(make([]byte, n)), future)
			if n > 16<<20 {
				require.Nil(t, written)
				require.True(t, future.IsDone())
				require.ErrorIs(t, future.Error(), channel.ErrFrameTooLarge)
				require.NoError(t, caught, "an oversized write fails its future without an inbound error")
				return
			}
			require.NoError(t, caught)
			require.NotNil(t, written)
			codec.Added(ctx)
			var decoded []int
			ctx.On("FireRead", mock.Anything).Run(func(args mock.Arguments) {
				decoded = append(decoded, args.Get(0).(buf.ByteBuf).ReadableBytes())
			}).Return(ctx)
			coalesced := buf.NewCompositeByteBuf(written, utils.VarIntEncode(64), buf.NewByteBuf(make([]byte, 64)))
			codec.Read(ctx, coalesced)
			require.NoError(t, caught)
			require.Equal(t, []int{n, 64}, decoded)
			require.Equal(t, n+5, written.ReadableBytes(), "decoding must preserve the input reader index")
		})
	}
}

func TestSimpleCodecAssemblyCheckpointsDoNotRenewDeadline(t *testing.T) {
	codec := NewSimpleCodec()
	ch := &channel.DefaultChannel{}
	ch.SetParam(channel.ParamFrameAssemblyTimeout, 400)
	ctx := channel.NewMockHandlerContext()
	ctx.On("Channel").Return(ch)
	expired := make(chan struct{}, 1)
	caught := make(chan error, 1)
	ctx.On("Disconnect", mock.Anything).Run(func(mock.Arguments) { expired <- struct{}{} }).Return(channel.NewFuture(nil))
	ctx.On("FireErrorCaught", mock.Anything).Run(func(args mock.Arguments) { caught <- args.Error(0) }).Return(ctx)
	codec.Added(ctx)
	t.Cleanup(func() { codec.Removed(ctx) })
	header := utils.VarIntEncode(300).BytesCopy()
	codec.Read(ctx, buf.NewByteBuf(header[:1]))
	time.Sleep(100 * time.Millisecond)
	codec.Read(ctx, buf.NewByteBuf(header[1:2]))
	time.Sleep(100 * time.Millisecond)
	codec.Read(ctx, buf.NewByteBuf(header[2:]))
	time.Sleep(100 * time.Millisecond)
	codec.Read(ctx, buf.NewByteBuf([]byte{'a'}))
	select {
	case <-expired:
	case <-time.After(150 * time.Millisecond):
		t.Fatal("partial header/body checkpoints renewed the first frame's absolute deadline")
	}
	select {
	case err := <-caught:
		require.ErrorIs(t, err, channel.ErrFrameAssemblyTimeout)
	case <-time.After(time.Second):
		t.Fatal("assembly expiry did not report its terminal error")
	}
	ctx.AssertNotCalled(t, "FireRead", mock.Anything)
}
