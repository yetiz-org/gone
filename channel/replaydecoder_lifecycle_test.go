package channel

import (
	"net"
	"testing"
	"testing/synctest"
	"time"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	buf "github.com/yetiz-org/goth-bytebuf"
	"github.com/yetiz-org/goth-util/structs"
)

func TestReplayDecoderSelfRemoval(t *testing.T) {
	ch := &DefaultChannel{}
	ch.init(ch)
	var decoder *ReplayDecoder
	decoder = NewReplayDecoder(0, func(ctx HandlerContext, in buf.ByteBuf, out structs.Queue) {
		ctx.Channel().Pipeline().Remove(decoder)
		in.Skip(in.ReadableBytes())
		decoder.Checkpoint(1)
	})
	ch.Pipeline().AddLast("decoder", decoder)
	done := make(chan struct{})
	go func() { ch.FireRead(buf.NewByteBuf([]byte{1})); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("self-removal blocked Decode")
	}
	require.Zero(t, decoder.in.ReadableBytes())
	require.Zero(t, decoder.State())
	require.Zero(t, InboundBudgetFor(ch).used.Load())
}

func TestReplayDecoderBadInputDoesNotPoisonNextRead(t *testing.T) {
	ch := &DefaultChannel{}
	ch.init(ch)
	decoded := make(chan struct{}, 1)
	decoder := NewReplayDecoder(0, func(ctx HandlerContext, in buf.ByteBuf, out structs.Queue) {
		in.Skip(in.ReadableBytes())
		decoded <- struct{}{}
	})
	ch.Pipeline().AddLast("decoder", decoder)
	ch.FireRead("not a buffer")
	done := make(chan struct{})
	go func() { ch.FireRead(buf.NewByteBuf([]byte{1})); close(done) }()
	select {
	case <-decoded:
		<-done
	case <-time.After(time.Second):
		t.Fatal("bad input poisoned next read")
	}
}

func TestReplayDecoderTerminalDuringDecode(t *testing.T) {
	for _, terminal := range []string{"removed", "inactive"} {
		t.Run(terminal, func(t *testing.T) {
			ch := &DefaultChannel{}
			ch.init(ch)
			entered, release, readDone := make(chan struct{}), make(chan struct{}), make(chan struct{})
			var decoder *ReplayDecoder
			decoder = NewReplayDecoder(0, func(ctx HandlerContext, in buf.ByteBuf, out structs.Queue) {
				if in.ReadableBytes() < 2 {
					panic(buf.ErrInsufficientSize)
				}
				close(entered)
				<-release
				in.Skip(in.ReadableBytes())
				decoder.Checkpoint(1)
			})
			ctx := &DefaultHandlerContext{pipeline: ch.Pipeline(), _handler: decoder}
			decoder.Added(ctx)
			decoder.Read(ctx, buf.NewByteBuf([]byte{1}))
			require.Positive(t, InboundBudgetFor(ch).used.Load())
			go func() { decoder.Read(ctx, buf.NewByteBuf([]byte{1})); close(readDone) }()
			<-entered
			terminalDone := make(chan struct{})
			go func() {
				if terminal == "removed" {
					decoder.Removed(ctx)
				} else {
					decoder.Inactive(ctx)
				}
				close(terminalDone)
			}()
			select {
			case <-terminalDone:
			case <-time.After(time.Second):
				close(release)
				<-readDone
				t.Fatal("terminal signal waited for Decode")
			}
			close(release)
			<-readDone
			require.Zero(t, decoder.in.ReadableBytes())
			require.Zero(t, decoder.State())
			require.Zero(t, InboundBudgetFor(ch).used.Load())
		})
	}
}

func TestReplayDecoderCompleteReadNeedsNoSharedReservation(t *testing.T) {
	ch := &DefaultChannel{}
	ch.init(ch)
	ch.SetParam(ParamMaxInProgressBytes, 1)
	budget := InboundBudgetFor(ch)
	require.True(t, budget.Reserve(1))
	defer budget.Release(1)
	decoded := 0
	decoder := NewReplayDecoder(0, func(ctx HandlerContext, in buf.ByteBuf, out structs.Queue) {
		decoded += in.ReadableBytes()
		in.Skip(in.ReadableBytes())
	})
	ch.Pipeline().AddLast("decoder", decoder)
	ch.FireRead(buf.NewByteBuf(make([]byte, 1024)))
	require.Equal(t, 1024, decoded, "complete read competed with retained-input budget")
	ch.Pipeline().Remove(decoder)
}

func TestReplayDecoderRetainedChunksReleaseBudget(t *testing.T) {
	ch := &DefaultChannel{}
	ch.init(ch)
	ch.SetParam(ParamMaxInProgressBytes, 600)
	ctx := &DefaultHandlerContext{pipeline: ch.Pipeline()}
	decoder := NewReplayDecoder(0, func(ctx HandlerContext, in buf.ByteBuf, out structs.Queue) {
		if in.ReadableBytes() >= 3 {
			in.Skip(2)
		}
	})
	decoder.Added(ctx)
	budget := InboundBudgetFor(ch)
	decoder.Read(ctx, buf.NewByteBuf([]byte{1}))
	decoder.Read(ctx, buf.NewByteBuf([]byte{2}))
	require.True(t, budget.Reserve(80), "two tiny chunks charged more than their memory bound")
	budget.Release(80)
	decoder.Read(ctx, buf.NewByteBuf([]byte{3}))
	require.True(t, budget.Reserve(300), "consumed chunks retained their charge")
	budget.Release(300)
	decoder.Removed(ctx)
	require.True(t, budget.Reserve(600), "removal leaked shared input budget")
	budget.Release(600)
}

func TestReplayDecoderTimeoutDuringDecode(t *testing.T) {
	ch := &DefaultChannel{}
	ch.init(ch)
	ch.SetParam(ParamFrameAssemblyTimeout, 30)
	ctx := NewMockHandlerContext()
	ctx.On("Channel").Return(ch)
	ctx.On("Disconnect", mock.Anything).Return(NewFuture(nil))
	timedOut := make(chan error, 1)
	ctx.On("FireErrorCaught", mock.Anything).Run(func(args mock.Arguments) { timedOut <- args.Error(0) }).Return(ctx)
	entered, release, readDone := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var decoder *ReplayDecoder
	decoder = NewReplayDecoder(0, func(ctx HandlerContext, in buf.ByteBuf, out structs.Queue) {
		if in.ReadableBytes() == 1 {
			panic(buf.ErrInsufficientSize)
		}
		close(entered)
		<-release
		in.Skip(in.ReadableBytes())
		decoder.Checkpoint(1)
	})
	decoder.Added(ctx)
	decoder.Read(ctx, buf.NewByteBuf([]byte{1}))
	go func() { decoder.Read(ctx, buf.NewByteBuf([]byte{2})); close(readDone) }()
	<-entered
	select {
	case err := <-timedOut:
		require.ErrorIs(t, err, ErrFrameAssemblyTimeout)
	case <-time.After(time.Second):
		close(release)
		<-readDone
		t.Fatal("timeout waited for Decode")
	}
	close(release)
	<-readDone
	require.Zero(t, InboundBudgetFor(ch).used.Load())
	require.Zero(t, decoder.in.ReadableBytes())
	require.Zero(t, decoder.State())
}

func TestReplayDecoderDatagramEmittedInputRemainsReadable(t *testing.T) {
	ch := &DefaultChannel{}
	ch.init(ch)
	messages := make(chan buf.ByteBuf, 2)
	decoder := NewReplayDecoder(0, func(ctx HandlerContext, in buf.ByteBuf, out structs.Queue) { out.Push(in) })
	decoder.Datagram = true
	ch.Pipeline().AddLast("decoder", decoder)
	ch.Pipeline().AddLast("consume", NewRWHandler(func(ctx HandlerContext, obj any) { messages <- obj.(buf.ByteBuf) }, nil))
	ch.FireRead(buf.NewByteBuf([]byte("first")))
	first := <-messages
	ch.FireRead(buf.NewByteBuf([]byte("second")))
	second := <-messages
	require.Equal(t, "first", string(first.Bytes()))
	require.Equal(t, "second", string(second.Bytes()))
	ch.Pipeline().Remove(decoder)
}

func BenchmarkReplayDecoderCompleteRead(b *testing.B) {
	for _, datagram := range []bool{false, true} {
		name := "stream"
		if datagram {
			name = "datagram"
		}
		b.Run(name, func(b *testing.B) {
			ch := &DefaultChannel{}
			ch.init(ch)
			decoder := NewReplayDecoder(0, func(ctx HandlerContext, in buf.ByteBuf, out structs.Queue) { in.Skip(in.ReadableBytes()) })
			decoder.Datagram = datagram
			ctx := &DefaultHandlerContext{pipeline: ch.Pipeline(), _handler: decoder}
			decoder.Added(ctx)
			input := buf.NewByteBuf(make([]byte, 64))
			b.ReportAllocs()
			for b.Loop() {
				decoder.Read(ctx, input)
			}
			decoder.Removed(ctx)
		})
	}
}

func TestReplayDecoderTailDoesNotPinMergedInput(t *testing.T) {
	ch := &DefaultChannel{}
	ch.init(ch)
	var merged []byte
	decoder := NewReplayDecoder(0, func(ctx HandlerContext, in buf.ByteBuf, out structs.Queue) {
		if in.ReadableBytes() == 1<<20 {
			return
		}
		merged = in.Bytes()
		in.Skip(in.ReadableBytes() - 1)
	})
	ctx := &DefaultHandlerContext{pipeline: ch.Pipeline()}
	decoder.Added(ctx)
	defer decoder.Removed(ctx)

	decoder.Read(ctx, buf.NewSharedByteBuf(make([]byte, 1<<20)))
	decoder.Read(ctx, buf.NewSharedByteBuf([]byte{1}))

	tail := uintptr(unsafe.Pointer(unsafe.SliceData(decoder.in.Bytes())))
	start := uintptr(unsafe.Pointer(unsafe.SliceData(merged)))
	require.False(t, tail >= start && tail < start+uintptr(len(merged)), "the retained tail must not alias the merged input")
	require.Equal(t, int64(1+replayInputOverhead), InboundBudgetFor(ch).used.Load())
}

type clearErrorHandler struct {
	DefaultHandler
	errors chan error
}

func (h *clearErrorHandler) ErrorCaught(ctx HandlerContext, err error) {
	h.errors <- err
}

func TestReplayDecoderClearReleasesPartialFrame(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		local, peer := net.Pipe()
		defer local.Close()
		defer peer.Close()
		ch := &DefaultNetChannel{BufferSize: 1024}
		ch.init(ch)
		ch.SetConn(local)
		ch.SetParam(ParamMaxInProgressBytes, 600)
		ch.SetParam(ParamFrameAssemblyTimeout, 10)
		ch.activeChannel()
		defer func() { ch.Disconnect().Sync() }()
		decoder := NewReplayDecoder(0, func(ctx HandlerContext, in buf.ByteBuf, out structs.Queue) {
			if in.ReadableBytes() < 2 {
				panic(buf.ErrInsufficientSize)
			}
			out.Push(in.ReadBytes(2))
		})
		defer decoder.Removed(nil)
		errorsCaught := make(chan error, 1)
		observer := &clearErrorHandler{errors: errorsCaught}
		ch.Pipeline().AddLast("observer", observer)
		ch.Pipeline().AddLast("partial", decoder)
		ch.FireRead(buf.NewByteBuf([]byte{1}))
		budget := InboundBudgetFor(ch)
		require.False(t, budget.Reserve(600), "partial input must hold a shared reservation")

		ch.Pipeline().Clear()

		if assert.True(t, budget.Reserve(600), "Clear must return the partial frame's reservation immediately") {
			budget.Release(600)
		}
		messages := make(chan []byte, 1)
		replacement := NewReplayDecoder(0, func(ctx HandlerContext, in buf.ByteBuf, out structs.Queue) {
			out.Push(in.ReadBytes(in.ReadableBytes()))
		})
		ch.Pipeline().AddLast("replacement", replacement)
		ch.Pipeline().AddLast("consume", NewRWHandler(func(ctx HandlerContext, obj any) { messages <- obj.([]byte) }, nil))
		time.Sleep(20 * time.Millisecond)
		synctest.Wait()
		assert.Empty(t, errorsCaught, "Clear must cancel the removed decoder's assembly timer")
		require.True(t, ch.IsActive(), "the removed decoder's old deadline must not close the replacement pipeline")
		ch.FireRead(buf.NewByteBuf([]byte{2, 3}))
		require.Equal(t, []byte{2, 3}, <-messages)
	})
}
