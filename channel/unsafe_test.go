package channel

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func requireDoneFuture(t *testing.T, future Future) {
	t.Helper()

	select {
	case <-future.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("future did not settle")
	}
}

func TestDefaultUnsafeUnsupportedOperationsSettleFuture(t *testing.T) {
	ch := &DefaultChannel{}
	ch.init(ch)

	bindFuture := ch.Bind(&net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	requireDoneFuture(t, bindFuture)
	assert.ErrorIs(t, bindFuture.Error(), ErrUnsupportedOperation)

	closeFuture := ch.Close()
	requireDoneFuture(t, closeFuture)
	assert.ErrorIs(t, closeFuture.Error(), ErrUnsupportedOperation)
}

type blockingBindChannel struct {
	DefaultChannel
	started   chan struct{}
	releaseCh chan struct{}
}

func (c *blockingBindChannel) UnsafeBind(localAddr net.Addr) error {
	close(c.started)
	<-c.releaseCh
	return nil
}

type blockingConnectChannel struct {
	DefaultChannel
	started   chan struct{}
	releaseCh chan struct{}
}

func (c *blockingConnectChannel) UnsafeConnect(localAddr net.Addr, remoteAddr net.Addr) error {
	close(c.started)
	<-c.releaseCh
	return nil
}

type blockingDisconnectChannel struct {
	DefaultChannel
	started   chan struct{}
	releaseCh chan struct{}
}

func (c *blockingDisconnectChannel) UnsafeDisconnect() error {
	close(c.started)
	<-c.releaseCh
	return nil
}

type blockingCloseChannel struct {
	DefaultChannel
	started   chan struct{}
	releaseCh chan struct{}
}

func (c *blockingCloseChannel) UnsafeClose() error {
	close(c.started)
	<-c.releaseCh
	return nil
}

func TestDefaultUnsafeBusyOperationsSettleSecondFuture(t *testing.T) {
	addr := &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1}

	t.Run("bind", func(t *testing.T) {
		ch := &blockingBindChannel{started: make(chan struct{}), releaseCh: make(chan struct{})}
		ch.init(ch)

		first := ch.Bind(addr)
		<-ch.started
		second := ch.Bind(addr)

		requireDoneFuture(t, second)
		assert.ErrorIs(t, second.Error(), ErrOperationInProgress)

		close(ch.releaseCh)
		requireDoneFuture(t, first)
		require.NoError(t, first.Error())
	})

	t.Run("connect", func(t *testing.T) {
		ch := &blockingConnectChannel{started: make(chan struct{}), releaseCh: make(chan struct{})}
		ch.init(ch)

		first := ch.Connect(nil, addr)
		<-ch.started
		second := ch.Connect(nil, addr)

		requireDoneFuture(t, second)
		assert.ErrorIs(t, second.Error(), ErrOperationInProgress)

		close(ch.releaseCh)
		requireDoneFuture(t, first)
		require.NoError(t, first.Error())
	})

	t.Run("disconnect", func(t *testing.T) {
		ch := &blockingDisconnectChannel{started: make(chan struct{}), releaseCh: make(chan struct{})}
		ch.init(ch)

		first := ch.Disconnect()
		<-ch.started
		second := ch.Disconnect()

		requireDoneFuture(t, second)
		assert.ErrorIs(t, second.Error(), ErrOperationInProgress)

		close(ch.releaseCh)
		requireDoneFuture(t, first)
		require.NoError(t, first.Error())
	})
}

func TestDefaultUnsafeCloseIsIdempotent(t *testing.T) {
	ch := &blockingCloseChannel{started: make(chan struct{}), releaseCh: make(chan struct{})}
	ch.init(ch)
	ch.activeChannel()

	first := ch.Close()
	<-ch.started
	second := ch.Close()

	requireDoneFuture(t, second)
	require.NoError(t, second.Error())

	close(ch.releaseCh)
	requireDoneFuture(t, first)
	require.NoError(t, first.Error())

	third := ch.Close()
	requireDoneFuture(t, third)
	require.NoError(t, third.Error())
}
