package gudp

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/netip"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/yetiz-org/gone/channel"
	"github.com/yetiz-org/gone/utils"
	"github.com/yetiz-org/goth-kklogger"
)

// ServerChannel represents a UDP server channel implementation. It is the only reader of its socket and
// serves each remote address through one child channel.
type ServerChannel struct {
	channel.DefaultNetServerChannel
	conn            atomic.Pointer[net.UDPConn]
	active          atomic.Bool
	peers           sync.Map // remote address string -> *UDPClientConn
	peerCount       atomic.Int64
	queuedBytes     atomic.Int64
	maxPeers        int64
	idleTimeout     time.Duration
	peerMu          sync.Mutex // guards peer publication/removal and admission accounting
	sourcePeers     map[netip.Addr]int64
	maxPeersPerIP   int64
	newPeersRate    float64
	newPeerBurst    float64
	newPeerTokens   float64
	newPeerRefilled time.Time
}

var ErrBindTwice = fmt.Errorf("bind twice")

// peerQueueLength bounds the datagrams queued for one child, and maxQueuedBytes the datagram bytes queued
// for all children of a server; more are dropped, as a full socket buffer would.
const peerQueueLength = 32
const maxQueuedBytes = 64 << 20

// UnsafeBind binds the UDP server to the specified local address
func (c *ServerChannel) UnsafeBind(localAddr net.Addr) (rtErr error) {
	if c.Name == "" {
		c.Name = fmt.Sprintf("UDPSERV_%s", localAddr.String())
	}

	if c.IsActive() {
		err := fmt.Errorf("%s: %w", c.Name, ErrBindTwice)
		kklogger.ErrorJ("gudp:ServerChannel.UnsafeBind#unsafe_bind!bind_twice", err.Error())
		return err
	}

	// Validate that the local address is a UDP address
	udpAddr, ok := localAddr.(*net.UDPAddr)
	if !ok {
		kklogger.ErrorJ("gudp:ServerChannel.UnsafeBind#unsafe_bind!invalid_addr", ErrNotUDPAddr.Error())
		return ErrNotUDPAddr
	}

	if conn, err := net.ListenUDP("udp4", udpAddr); err != nil {
		kklogger.ErrorJ("gudp:ServerChannel.UnsafeBind#unsafe_bind!bind_error", fmt.Sprintf("bind at %s fail %s", localAddr.String(), err.Error()))
		return err
	} else {
		c.peerMu.Lock()
		c.maxPeers = channel.GetParamInt64Default(c, ParamMaxPeers, DefaultMaxPeers)
		c.idleTimeout = time.Duration(channel.GetParamInt64Default(c, ParamPeerIdleTimeout, DefaultPeerIdleTimeout)) * time.Millisecond
		c.maxPeersPerIP = channel.GetParamInt64Default(c, ParamMaxPeersPerIP, DefaultMaxPeersPerIP)
		if c.maxPeersPerIP <= 0 {
			c.maxPeersPerIP = DefaultMaxPeersPerIP
		}

		rate := channel.GetParamInt64Default(c, ParamNewPeersPerSecond, DefaultNewPeersPerSecond)
		if rate <= 0 {
			rate = DefaultNewPeersPerSecond
		}

		burst := channel.GetParamInt64Default(c, ParamNewPeerBurst, DefaultNewPeerBurst)
		if burst <= 0 {
			burst = DefaultNewPeerBurst
		}

		if c.sourcePeers == nil {
			c.sourcePeers = make(map[netip.Addr]int64)
		}

		c.newPeersRate = float64(rate)
		c.newPeerBurst = float64(burst)
		c.newPeerTokens = c.newPeerBurst
		c.newPeerRefilled = time.Now()
		c.peerMu.Unlock()
		c.conn.Store(conn)
		c.active.Store(true)
	}

	return nil
}

// UnsafeAccept reads datagrams and queues each one for the child of its remote address. It returns a
// new child with the first datagram of an address it does not serve yet, subject to global,
// per-source and new-peer rate limits. Existing peers continue receiving at an admission limit.
func (c *ServerChannel) UnsafeAccept() (child channel.Channel, future channel.Future) {
	conn := c.conn.Load()
	if !c.IsActive() || conn == nil {
		return nil, c.Pipeline().NewFuture()
	}

	// Get buffer from pool for memory optimization - using 64KB buffer for UDP max packet size
	buffer := utils.GetLargeBuffer()
	defer utils.PutLargeBuffer(buffer)
	for {
		n, clientAddr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			if c.IsActive() {
				kklogger.ErrorJ("gudp:ServerChannel.UnsafeAccept#unsafe_accept!read_error", err.Error())
			}

			return nil, c.Pipeline().NewFuture()
		}

		key := clientAddr.String()
		if v, ok := c.peers.Load(key); ok {
			v.(*UDPClientConn).deliver(buffer[:n])
			continue
		}

		sourceIP := clientAddr.AddrPort().Addr().Unmap().WithZone("")
		c.peerMu.Lock()
		if !c.IsActive() || c.conn.Load() != conn {
			c.peerMu.Unlock()
			return nil, c.Pipeline().NewFuture()
		}

		if c.maxPeers > 0 && c.peerCount.Load() >= c.maxPeers {
			limit := c.maxPeers
			c.peerMu.Unlock()
			kklogger.DebugJ("gudp:ServerChannel.UnsafeAccept#unsafe_accept!peer_limit", fmt.Sprintf("drop datagram from %s, %d peers served", key, limit))
			continue
		}

		if c.sourcePeers[sourceIP] >= c.maxPeersPerIP {
			c.peerMu.Unlock()
			continue
		}

		now := time.Now()
		c.newPeerTokens = min(c.newPeerBurst, c.newPeerTokens+now.Sub(c.newPeerRefilled).Seconds()*c.newPeersRate)
		c.newPeerRefilled = now
		if c.newPeerTokens < 1 {
			c.peerMu.Unlock()
			continue
		}

		c.newPeerTokens--
		clientConn := &UDPClientConn{
			server:          c,
			socket:          conn,
			clientAddr:      clientAddr,
			packets:         make(chan []byte, peerQueueLength),
			closed:          make(chan struct{}),
			deadlineChanged: make(chan struct{}, 1),
			idleTimeout:     c.idleTimeout,
		}

		clientConn.deliver(buffer[:n])
		c.peers.Store(key, clientConn)
		c.peerCount.Add(1)
		c.sourcePeers[sourceIP]++
		c.peerMu.Unlock()

		// UnsafeClose turns the server inactive before closing the stored peers, so a peer stored after that
		// sweep is caught here and never gets a child.
		if !c.IsActive() || c.conn.Load() != conn {
			clientConn.Close()
			return nil, c.Pipeline().NewFuture()
		}

		child = c.DeriveNetChildChannel(&Channel{}, c, clientConn)
		return child, child.Pipeline().NewFuture()
	}
}

// UnsafeClose closes the UDP server connection
func (c *ServerChannel) UnsafeClose() (rtErr error) {
	c.DefaultNetServerChannel.UnsafeClose()
	c.active.Store(false)

	// Swap the socket out first so a concurrent close or accept never uses it twice.
	if conn := c.conn.Swap(nil); conn != nil {
		rtErr = conn.Close()
	}

	// Close every peer, including one an accept in progress published after the children were closed.
	c.peers.Range(func(_, value any) bool {
		value.(*UDPClientConn).Close()
		return true
	})

	return rtErr
}

// IsActive returns whether the UDP server is currently active
func (c *ServerChannel) IsActive() bool {
	return c.active.Load()
}

// UDPClientConn is the net.Conn of one remote address served by a ServerChannel. Its reads return the
// datagrams the server queued for that address and io.EOF once the address stays idle past the timeout;
// the channel then closes it, which releases the address for its next datagram.
type UDPClientConn struct {
	server       *ServerChannel
	socket       *net.UDPConn
	clientAddr   *net.UDPAddr
	packets      chan []byte
	closed       chan struct{}
	isClosed     bool
	mu           sync.Mutex // guards isClosed and queuing against Close
	idleTimeout  time.Duration
	lastActive   atomic.Int64 // UnixNano of the latest datagram received or sent
	readDeadline atomic.Int64 // UnixNano, zero for none
	// deadlineChanged wakes a blocked Read to apply a new read deadline.
	deadlineChanged chan struct{}
	truncated       atomic.Bool
}

// deliver queues a copy of datagram, which the caller reuses, unless the queue or the server budget is full.
func (c *UDPClientConn) deliver(datagram []byte) {
	c.lastActive.Store(time.Now().UnixNano())
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.isClosed {
		return
	}

	size := int64(len(datagram))
	if len(c.packets) == cap(c.packets) {
		return
	}

	if c.server.queuedBytes.Add(size) > maxQueuedBytes {
		c.server.queuedBytes.Add(-size)
		return
	}

	select {
	case c.packets <- bytes.Clone(datagram):
	default:
		c.server.queuedBytes.Add(-size)
	}
}

// Read implements net.Conn interface for UDP client connections
func (c *UDPClientConn) Read(b []byte) (n int, rtErr error) {
	var packet []byte
	select {
	case packet = <-c.packets:
	default:
		if packet, rtErr = c.waitPacket(); rtErr != nil {
			return 0, rtErr
		}
	}

	c.server.queuedBytes.Add(-int64(len(packet)))
	n = copy(b, packet)
	if n < len(packet) && c.truncated.CompareAndSwap(false, true) {
		// If the destination buffer is too small, unread packet bytes are discarded.
		kklogger.WarnJ("gudp:UDPClientConn.Read#read!buffer_too_small",
			fmt.Sprintf("Buffer size %d smaller than packet size %d; later truncations from %s are not logged", len(b), len(packet), c.clientAddr))
	}

	return n, nil
}

// waitPacket blocks until a datagram arrives, the conn closes, the read deadline passes or the peer
// stays idle past the idle timeout.
func (c *UDPClientConn) waitPacket() (packet []byte, rtErr error) {
	for {
		// Wake at the earlier of the read deadline and the idle expiry.
		var until time.Time
		if c.idleTimeout > 0 {
			until = time.Unix(0, c.lastActive.Load()).Add(c.idleTimeout)
		}

		if deadline := c.readDeadline.Load(); deadline != 0 && (until.IsZero() || deadline < until.UnixNano()) {
			until = time.Unix(0, deadline)
		}

		var wake <-chan time.Time
		if !until.IsZero() {
			wake = time.After(time.Until(until))
		}

		select {
		case packet = <-c.packets:
			return packet, nil
		case <-c.closed:
			return nil, net.ErrClosed
		case <-c.deadlineChanged:
		case <-wake:
			now := time.Now().UnixNano()
			if c.idleTimeout > 0 && now-c.lastActive.Load() >= int64(c.idleTimeout) {
				return nil, io.EOF
			}

			if deadline := c.readDeadline.Load(); deadline != 0 && now >= deadline {
				return nil, os.ErrDeadlineExceeded
			}
		}
	}
}

// Write implements net.Conn interface for UDP client connections
func (c *UDPClientConn) Write(b []byte) (n int, rtErr error) {
	select {
	case <-c.closed:
		return 0, net.ErrClosed
	default:
	}

	if n, rtErr = c.socket.WriteToUDP(b, c.clientAddr); rtErr == nil {
		c.lastActive.Store(time.Now().UnixNano())
	}

	return n, rtErr
}

// Close stops serving this remote address; the server socket stays open for other clients.
func (c *UDPClientConn) Close() (rtErr error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.isClosed {
		return nil
	}

	c.isClosed = true
	close(c.closed)
	for drained := false; !drained; {
		select {
		case packet := <-c.packets:
			c.server.queuedBytes.Add(-int64(len(packet)))
		default:
			drained = true
		}
	}

	c.server.peerMu.Lock()
	if c.server.peers.CompareAndDelete(c.clientAddr.String(), c) {
		c.server.peerCount.Add(-1)
		sourceIP := c.clientAddr.AddrPort().Addr().Unmap().WithZone("")
		if c.server.sourcePeers[sourceIP] <= 1 {
			delete(c.server.sourcePeers, sourceIP)
		} else {
			c.server.sourcePeers[sourceIP]--
		}
	}
	c.server.peerMu.Unlock()

	return nil
}

// LocalAddr implements net.Conn interface for UDP client connections
func (c *UDPClientConn) LocalAddr() (addr net.Addr) {
	return c.socket.LocalAddr()
}

// RemoteAddr implements net.Conn interface for UDP client connections
func (c *UDPClientConn) RemoteAddr() (addr net.Addr) {
	return c.clientAddr
}

// SetDeadline sets this connection's read deadline and the shared socket's write deadline.
func (c *UDPClientConn) SetDeadline(t time.Time) (rtErr error) {
	c.SetReadDeadline(t)
	return c.SetWriteDeadline(t)
}

// SetReadDeadline sets the read deadline of this connection only.
func (c *UDPClientConn) SetReadDeadline(t time.Time) (rtErr error) {
	if t.IsZero() {
		c.readDeadline.Store(0)
	} else {
		c.readDeadline.Store(t.UnixNano())
	}

	select {
	case c.deadlineChanged <- struct{}{}:
	default:
	}

	return nil
}

// SetWriteDeadline implements net.Conn interface for UDP client connections
func (c *UDPClientConn) SetWriteDeadline(t time.Time) (rtErr error) {
	return c.socket.SetWriteDeadline(t)
}
