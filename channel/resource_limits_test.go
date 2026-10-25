package channel

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type blockedResourceWriter struct {
	DefaultChannel
	entered      chan struct{}
	releaseWrite chan struct{}
	releaseOnce  sync.Once
}

func (c *blockedResourceWriter) UnsafeWrite(obj any) (rtErr error) {
	select {
	case c.entered <- struct{}{}:
	default:
	}
	<-c.releaseWrite
	return nil
}

func (c *blockedResourceWriter) UnsafeIsAutoRead() (auto bool) {
	return false
}

func TestUnsafeWriteQueueBoundsAndSettlement(t *testing.T) {
	ch := &blockedResourceWriter{entered: make(chan struct{}, 1), releaseWrite: make(chan struct{})}
	ch.init(ch)
	ch.SetParam(ParamMaxQueuedWriteBytes, 200)
	ch.activeChannel()
	t.Cleanup(func() { ch.inactiveChannel() })
	t.Cleanup(func() { ch.releaseOnce.Do(func() { close(ch.releaseWrite) }) })
	first := ch.Write([]byte("1234"))
	select {
	case <-ch.entered:
	case <-time.After(time.Second):
		t.Fatal("writer did not begin")
	}
	second := ch.Write([]byte("5678"))
	overflow := ch.Write([]byte("x"))
	select {
	case <-overflow.Done():
	case <-time.After(200 * time.Millisecond):
		t.Fatal("entry overhead did not bound queued memory")
	}
	require.ErrorIs(t, overflow.Error(), ErrWriteQueueFull)
	ch.releaseOnce.Do(func() { close(ch.releaseWrite) })
	requireDoneFuture(t, first)
	requireDoneFuture(t, second)
}

func TestUnsafeWriteQueueAcceptsSmallBurstWithinByteBudget(t *testing.T) {
	ch := &blockedResourceWriter{entered: make(chan struct{}, 1), releaseWrite: make(chan struct{})}
	ch.init(ch)
	ch.SetParam(ParamMaxQueuedWriteBytes, 128<<10)
	ch.activeChannel()
	t.Cleanup(func() { ch.inactiveChannel() })
	t.Cleanup(func() { ch.releaseOnce.Do(func() { close(ch.releaseWrite) }) })
	first := ch.Write([]byte("x"))
	<-ch.entered
	futures := []Future{first}
	for range 1100 {
		futures = append(futures, ch.Write([]byte("x")))
	}
	require.True(t, ch.IsActive(), "small burst below byte limit disconnected at old item cap")
	ch.releaseOnce.Do(func() { close(ch.releaseWrite) })
	for _, future := range futures {
		requireDoneFuture(t, future)
		require.NoError(t, future.Error())
	}
}

func BenchmarkUnsafeWriteBurst(b *testing.B) {
	for b.Loop() {
		ch := &blockedResourceWriter{entered: make(chan struct{}, 1), releaseWrite: make(chan struct{})}
		ch.init(ch)
		ch.activeChannel()
		futures := make([]Future, 128)
		for i := range futures {
			futures[i] = ch.Write("x")
		}
		<-ch.entered
		close(ch.releaseWrite)
		for _, future := range futures {
			future.Sync()
		}
		ch.inactiveChannel()
	}
}
