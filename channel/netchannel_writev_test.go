package channel

import (
	"errors"
	"io"
	"net"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	buf "github.com/yetiz-org/goth-bytebuf"
	concurrent "github.com/yetiz-org/goth-concurrent"
	"github.com/yetiz-org/goth-util/structs"
)

type errorNetConn struct {
	readErr  error
	writeErr error
}

func (c *errorNetConn) Read(_ []byte) (int, error) {
	return 0, c.readErr
}

func (c *errorNetConn) Write(_ []byte) (int, error) {
	return 0, c.writeErr
}

func (c *errorNetConn) Close() error {
	return nil
}

func (c *errorNetConn) LocalAddr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1}
}

func (c *errorNetConn) RemoteAddr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 2}
}

func (c *errorNetConn) SetDeadline(time.Time) error {
	return nil
}

func (c *errorNetConn) SetReadDeadline(time.Time) error {
	return nil
}

func (c *errorNetConn) SetWriteDeadline(time.Time) error {
	return nil
}

// dialLoopbackPair brings up a TCP listener on 127.0.0.1, dials back to it,
// and returns (client, server). Both sides are *net.TCPConn so the
// scatter-gather writev(2) path inside CompositeByteBuf.WriteTo activates.
func dialLoopbackPair(t *testing.T) (net.Conn, net.Conn) {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	type result struct {
		conn net.Conn
		err  error
	}
	done := make(chan result, 1)
	go func() {
		c, err := lis.Accept()
		done <- result{c, err}
	}()

	client, err := net.Dial("tcp", lis.Addr().String())
	require.NoError(t, err)
	r := <-done
	require.NoError(t, r.err)
	require.NoError(t, lis.Close())

	t.Cleanup(func() {
		_ = client.Close()
		_ = r.conn.Close()
	})
	return client, r.conn
}

// newNetChannelFor wires a DefaultNetChannel to the supplied net.Conn so the
// UnsafeWrite path can be exercised against a real socket.
func newNetChannelFor(t *testing.T, c net.Conn) *DefaultNetChannel {
	t.Helper()
	ch := &DefaultNetChannel{BufferSize: 4096, WriteTimeout: 2 * time.Second, ReadTimeout: 2 * time.Second}
	ch.setConn(c)
	return ch
}

// readN drains exactly n bytes from r within timeout.
func readN(t *testing.T, r net.Conn, n int, timeout time.Duration) []byte {
	t.Helper()
	require.NoError(t, r.SetReadDeadline(time.Now().Add(timeout)))
	out := make([]byte, 0, n)
	tmp := make([]byte, 4096)
	for len(out) < n {
		k, err := r.Read(tmp)
		if k > 0 {
			out = append(out, tmp[:k]...)
		}
		if err != nil {
			break
		}
	}
	return out
}

// TestUnsafeWrite_CompositeByteBuf_WritevPath confirms that submitting a
// CompositeByteBuf to UnsafeWrite transmits every component in order and
// exercises the io.WriterTo branch (which routes to writev(2) for TCP).
func TestUnsafeWrite_CompositeByteBuf_WritevPath(t *testing.T) {
	client, server := dialLoopbackPair(t)
	ch := newNetChannelFor(t, client)

	head := buf.NewSharedByteBuf([]byte("HEAD|"))
	mid := buf.NewSharedByteBuf([]byte("MID|"))
	tail := buf.NewSharedByteBuf([]byte("TAIL"))
	composite := buf.NewCompositeByteBuf(head, mid, tail)

	_, ok := any(composite).(io.WriterTo)
	assert.True(t, ok, "CompositeByteBuf must satisfy io.WriterTo so the writev path is taken")

	require.NoError(t, ch.UnsafeWrite(composite))

	got := readN(t, server, len("HEAD|MID|TAIL"), 2*time.Second)
	assert.Equal(t, "HEAD|MID|TAIL", string(got))
}

// TestUnsafeWrite_PlainByteBuf_DefaultPath verifies that a plain *DefaultByteBuf
// (no io.WriterTo) flows through DefaultConn.Write.
func TestUnsafeWrite_PlainByteBuf_DefaultPath(t *testing.T) {
	client, server := dialLoopbackPair(t)
	ch := newNetChannelFor(t, client)

	bb := buf.EmptyByteBuf()
	bb.WriteString("hello world")

	_, wantsWriteTo := any(bb).(io.WriterTo)
	assert.False(t, wantsWriteTo, "plain DefaultByteBuf must NOT expose io.WriterTo so the non-writev path is selected")

	require.NoError(t, ch.UnsafeWrite(bb))
	got := readN(t, server, len("hello world"), 2*time.Second)
	assert.Equal(t, "hello world", string(got))
}

// TestUnsafeWrite_BytesSlice verifies []byte still works.
func TestUnsafeWrite_BytesSlice(t *testing.T) {
	client, server := dialLoopbackPair(t)
	ch := newNetChannelFor(t, client)

	require.NoError(t, ch.UnsafeWrite([]byte("raw-slice")))
	got := readN(t, server, len("raw-slice"), 2*time.Second)
	assert.Equal(t, "raw-slice", string(got))
}

func TestUnsafeRead_ReturnedByteBufIsDetachedFromPooledReadBuffer(t *testing.T) {
	client, server := net.Pipe()
	t.Cleanup(func() {
		_ = client.Close()
		_ = server.Close()
	})

	ch := &DefaultNetChannel{BufferSize: 1024, ReadTimeout: 2 * time.Second}
	ch.setConn(client)
	ch.alive = concurrent.NewFuture()

	writeAndRead := func(payload string) buf.ByteBuf {
		t.Helper()
		done := make(chan error, 1)
		go func() {
			_, err := server.Write([]byte(payload))
			done <- err
		}()

		obj, err := ch.UnsafeRead()
		require.NoError(t, err)
		require.NoError(t, <-done)

		bb, ok := obj.(buf.ByteBuf)
		require.True(t, ok)
		return bb
	}

	first := writeAndRead("first")
	second := writeAndRead("second")

	require.Equal(t, "first", string(first.Bytes()), "later reads must not mutate a previously returned ByteBuf")
	require.Equal(t, "second", string(second.Bytes()))
}

// TestUnsafeWrite_UnknownType_Rejected verifies non-ByteBuf / non-[]byte
// values return ErrUnknownObjectType.
func TestUnsafeWrite_UnknownType_Rejected(t *testing.T) {
	client, _ := dialLoopbackPair(t)
	ch := newNetChannelFor(t, client)
	err := ch.UnsafeWrite(12345)
	assert.ErrorIs(t, err, ErrUnknownObjectType)
}

// TestUnsafeWrite_CompositeError_MarksConnInactive confirms that when the
// writev path fails, the wrapping DefaultConn is flipped into the inactive
// state (mirroring DefaultConn.Write's behavior on non-deadline errors).
func TestUnsafeWrite_CompositeError_MarksConnInactive(t *testing.T) {
	ch := newNetChannelFor(t, &errorNetConn{writeErr: errors.New("write failed")})

	head := buf.NewSharedByteBuf([]byte("X"))
	composite := buf.NewCompositeByteBuf(head)

	err := ch.UnsafeWrite(composite)
	require.Error(t, err)
	assert.False(t, ch.Conn().IsActive(), "non-deadline write failure must mark the conn inactive")
}

func TestUnsafeWrite_CompositeDeadlineError_KeepsConnActive(t *testing.T) {
	ch := newNetChannelFor(t, &errorNetConn{writeErr: os.ErrDeadlineExceeded})

	composite := buf.NewCompositeByteBuf(buf.NewSharedByteBuf([]byte("X")))

	err := ch.UnsafeWrite(composite)
	require.ErrorIs(t, err, os.ErrDeadlineExceeded)
	assert.True(t, ch.Conn().IsActive(), "deadline write failures must not mark the conn inactive")
}

func TestUnsafeRead_ErrorBranches(t *testing.T) {
	t.Run("nil conn", func(t *testing.T) {
		ch := &DefaultNetChannel{BufferSize: 1024, ReadTimeout: time.Second}

		obj, err := ch.UnsafeRead()

		require.Nil(t, obj)
		require.ErrorIs(t, err, ErrNilObject)
	})

	t.Run("inactive channel", func(t *testing.T) {
		ch := newNetChannelFor(t, &errorNetConn{})

		obj, err := ch.UnsafeRead()

		require.Nil(t, obj)
		require.ErrorIs(t, err, net.ErrClosed)
	})

	t.Run("deadline while conn active skips", func(t *testing.T) {
		ch := newNetChannelFor(t, &errorNetConn{readErr: os.ErrDeadlineExceeded})
		ch.alive = concurrent.NewFuture()

		obj, err := ch.UnsafeRead()

		require.Nil(t, obj)
		require.ErrorIs(t, err, ErrSkip)
	})

	t.Run("deadline while conn inactive reports not active", func(t *testing.T) {
		ch := newNetChannelFor(t, &errorNetConn{readErr: os.ErrDeadlineExceeded})
		ch.alive = concurrent.NewFuture()
		ch.Conn().(*DefaultConn).markInactive()

		obj, err := ch.UnsafeRead()

		require.Nil(t, obj)
		require.ErrorIs(t, err, ErrNotActive)
	})
}

// TestReplayDecoder_Composite_ZeroCopyAccumulation exercises the composite
// accumulator: successive ByteBufs pushed via Read should be visible to the
// Decode callback in-order; a frame split across Read calls reassembles
// correctly without per-call copies into a growing DefaultByteBuf.
func TestReplayDecoder_Composite_ZeroCopyAccumulation(t *testing.T) {
	var decoded []string

	dec := NewReplayDecoder(ReplayState(0), func(ctx HandlerContext, in buf.ByteBuf, out structs.Queue) {
		for in.ReadableBytes() >= 1 {
			in.MarkReaderIndex()
			n := int(in.MustReadByte())
			if in.ReadableBytes() < n {
				in.ResetReaderIndex()
				panic(buf.ErrInsufficientSize)
			}
			bb := in.ReadByteBuf(n)
			out.Push(string(bb.Bytes()))
		}
	})

	ctx := NewMockHandlerContext()
	ctx.On("FireRead", mock.Anything).Run(func(args mock.Arguments) {
		decoded = append(decoded, args.Get(0).(string))
	}).Return(ctx)
	ctx.On("FireReadCompleted").Return(ctx)

	dec.Added(ctx)

	// Frame #1: len=2, "AB"
	first := buf.EmptyByteBuf()
	first.WriteByte(2)
	first.WriteBytes([]byte("AB"))

	// Frame #2 split across two inputs: len=3, then "CDE"
	second := buf.EmptyByteBuf()
	second.WriteByte(3)
	second.WriteBytes([]byte("C"))
	third := buf.EmptyByteBuf()
	third.WriteBytes([]byte("DE"))

	dec.Read(ctx, first)
	assert.Equal(t, []string{"AB"}, decoded)
	dec.Read(ctx, second)
	assert.Equal(t, []string{"AB"}, decoded, "second partial frame must not decode yet")
	dec.Read(ctx, third)
	assert.Equal(t, []string{"AB", "CDE"}, decoded, "third chunk completes the second frame")

	// Sanity: the accumulator is a composite, not a DefaultByteBuf. After
	// full consumption ReadableBytes is zero, so Compact is a no-op.
	assert.Equal(t, 0, dec.in.ReadableBytes())
	dec.in.Compact()
}

// TestReplayDecoder_Composite_CompactDropsConsumedComponents verifies that
// the truncation fast-path actually drops fully-consumed components so the
// accumulator memory footprint does not grow unbounded.
func TestReplayDecoder_Composite_CompactDropsConsumedComponents(t *testing.T) {
	dec := NewReplayDecoder(ReplayState(0), func(ctx HandlerContext, in buf.ByteBuf, out structs.Queue) {
		for in.ReadableBytes() >= 1 {
			_ = in.MustReadByte()
		}
	})
	ctx := NewMockHandlerContext()
	ctx.On("FireRead", mock.Anything).Return(ctx)
	ctx.On("FireReadCompleted").Return(ctx)
	dec.Added(ctx)

	// Push two components, consume both.
	a := buf.EmptyByteBuf()
	a.WriteBytes([]byte{1, 2, 3})
	b := buf.EmptyByteBuf()
	b.WriteBytes([]byte{4, 5, 6})

	dec.Read(ctx, a)
	dec.Read(ctx, b)

	assert.Equal(t, 0, dec.in.ReadableBytes())
	// Before Compact, Cap equals the total bytes buffered across both
	// components (3 + 3). After Compact, every fully-consumed component is
	// dropped and Cap goes to zero.
	assert.Equal(t, 6, dec.in.Cap())
	dec.in.Compact()
	assert.Equal(t, 0, dec.in.Cap(), "Compact on a fully-consumed composite drops all components")
}
