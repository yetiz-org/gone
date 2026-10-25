package gudp

// This consolidated test file merges:
// - gudp_test.go
// - udpserverchannel_comprehensive_test.go
// Original files will be archived to avoid duplicate execution.

import (
	"fmt"
	"maps"
	"net"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yetiz-org/gone/channel"
	buf "github.com/yetiz-org/goth-bytebuf"
)

// =============================================================================
// Real UDP Connection Tests (from gudp_test.go)
// =============================================================================

// Test real UDP client-server communication
func TestUDPChannel_RealConnection(t *testing.T) {
	// Create server
	server := &ServerChannel{}
	server.Init()

	serverAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	err := server.UnsafeBind(serverAddr)
	assert.NoError(t, err, "Server should bind successfully")
	defer server.UnsafeClose()

	actualServerAddr := server.conn.Load().LocalAddr()
	t.Logf("Server bound to: %s", actualServerAddr.String())

	// Test simple UDP packet sending
	conn, err := net.Dial("udp", actualServerAddr.String())
	assert.NoError(t, err, "Should be able to create UDP connection")
	defer conn.Close()

	_, err = conn.Write([]byte("test"))
	assert.NoError(t, err, "Should be able to send UDP packet")

	t.Logf("UDP communication test completed successfully")
}

// Test UDP client-server message exchange
func TestUDPChannel_MessageExchange(t *testing.T) {
	// Create and start server
	server := &ServerChannel{}
	server.Init()

	serverAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	err := server.UnsafeBind(serverAddr)
	assert.NoError(t, err, "Server should bind successfully")
	defer server.UnsafeClose()

	actualServerAddr := server.conn.Load().LocalAddr()

	// Test message exchange using direct UDP connection
	testMessage := []byte("Hello UDP Server!")

	var wg sync.WaitGroup
	var serverReceivedMessage []byte

	// Start server receive loop
	wg.Go(func() {

		// Read directly from UDP connection
		buffer := make([]byte, 1024)
		n, clientAddr, err := server.conn.Load().ReadFromUDP(buffer)
		if err == nil {
			serverReceivedMessage = buffer[:n]
			t.Logf("Server received message from %s: %s", clientAddr, string(serverReceivedMessage))
		}
	})

	// Client sends message
	wg.Go(func() {
		time.Sleep(100 * time.Millisecond) // Let server start first

		// Connect to server using standard UDP connection
		conn, err := net.Dial("udp", actualServerAddr.String())
		assert.NoError(t, err, "Client should connect to server")
		defer conn.Close()

		// Send test message
		_, err = conn.Write(testMessage)
		assert.NoError(t, err, "Client should send message successfully")

		t.Logf("Client sent message: %s", string(testMessage))
	})

	// Wait for both operations to complete
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(5 * time.Second):
		t.Fatal("Test timed out")
	}

	// Verify message was received
	assert.Equal(t, testMessage, serverReceivedMessage, "Server should receive the correct message")
	t.Logf("Server received message: %s", string(serverReceivedMessage))
}

// Test multiple concurrent UDP clients
func TestUDPChannel_MultipleConcurrentClients(t *testing.T) {
	// Create server
	server := &ServerChannel{}
	server.Init()

	serverAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	err := server.UnsafeBind(serverAddr)
	assert.NoError(t, err, "Server should bind successfully")
	defer server.UnsafeClose()

	actualServerAddr := server.conn.Load().LocalAddr()

	const numClients = 10
	const messagesPerClient = 5

	var wg sync.WaitGroup
	var serverMessagesReceived int64
	var serverMutex sync.Mutex
	var receivedMessages []string

	// Start server receive loop
	wg.Go(func() {

		expectedMessages := numClients * messagesPerClient
		buffer := make([]byte, 1024)

		for range expectedMessages {
			n, clientAddr, err := server.conn.Load().ReadFromUDP(buffer)
			if err != nil {
				t.Errorf("Server failed to read message: %v", err)
				break
			}

			message := string(buffer[:n])
			serverMutex.Lock()
			receivedMessages = append(receivedMessages, message)
			serverMutex.Unlock()
			serverMessagesReceived++

			t.Logf("Server received message from %s: %s", clientAddr, message)
		}
	})

	// Create multiple clients
	for clientID := range numClients {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			time.Sleep(time.Duration(id*10) * time.Millisecond) // Stagger client starts

			for msgID := range messagesPerClient {
				conn, err := net.Dial("udp", actualServerAddr.String())
				if err != nil {
					t.Errorf("Client %d failed to connect: %v", id, err)
					return
				}

				message := fmt.Sprintf("Message from client %d, msg %d", id, msgID)
				_, err = conn.Write([]byte(message))
				if err != nil {
					t.Errorf("Client %d failed to send message: %v", id, err)
				}
				conn.Close()

				// Small delay between messages
				time.Sleep(10 * time.Millisecond)
			}
		}(clientID)
	}

	// Wait for all operations to complete
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(10 * time.Second):
		t.Fatal("Test timed out")
	}

	// Verify all messages were received
	expectedMessageCount := int64(numClients * messagesPerClient)
	assert.Equal(t, expectedMessageCount, serverMessagesReceived,
		fmt.Sprintf("Server should receive %d messages", expectedMessageCount))

	t.Logf("Server received %d messages from %d clients", serverMessagesReceived, numClients)
	for i, msg := range receivedMessages {
		t.Logf("Message %d: %s", i+1, msg)
	}
}

// Test UDP connection error handling
func TestUDPChannel_ErrorHandling(t *testing.T) {
	// Test binding to invalid address
	server := &ServerChannel{}
	server.Init()

	// Try to bind with TCP address (should fail)
	invalidAddr, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:8080")
	err := server.UnsafeBind(invalidAddr)
	assert.Error(t, err, "Should fail to bind with TCP address")
	assert.Equal(t, ErrNotUDPAddr, err, "Should return ErrNotUDPAddr")

	// Test double bind
	validAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	err = server.UnsafeBind(validAddr)
	assert.NoError(t, err, "First bind should succeed")

	err = server.UnsafeBind(validAddr)
	assert.Error(t, err, "Second bind should fail")
	assert.Contains(t, err.Error(), "bind twice", "Should indicate bind twice error")

	server.UnsafeClose()

	// Test client connection with invalid address
	client := &Channel{}
	client.Init()

	invalidTCPAddr, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:8080")
	validUDPAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:8080")

	err = client.UnsafeConnect(invalidTCPAddr, validUDPAddr)
	assert.Error(t, err, "Should fail with invalid local address")
	assert.Equal(t, ErrNotUDPAddr, err, "Should return ErrNotUDPAddr for invalid local addr")

	err = client.UnsafeConnect(validUDPAddr, invalidTCPAddr)
	assert.Error(t, err, "Should fail with invalid remote address")
	assert.Equal(t, ErrNotUDPAddr, err, "Should return ErrNotUDPAddr for invalid remote addr")

	err = client.UnsafeConnect(validUDPAddr, nil)
	assert.Error(t, err, "Should fail with nil remote address")
	assert.Equal(t, channel.ErrNilObject, err, "Should return ErrNilObject for nil remote addr")
}

// Test UDP server lifecycle
func TestUDPServerChannel_Lifecycle(t *testing.T) {
	server := &ServerChannel{}
	server.Init()

	// Initially inactive
	assert.False(t, server.IsActive(), "Server should start inactive")

	// Bind and become active
	addr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	err := server.UnsafeBind(addr)
	assert.NoError(t, err, "Server should bind successfully")
	assert.True(t, server.IsActive(), "Server should be active after binding")

	// Close and become inactive
	err = server.UnsafeClose()
	assert.NoError(t, err, "Server should close successfully")
	assert.False(t, server.IsActive(), "Server should be inactive after closing")

	// Multiple close calls should be safe
	err = server.UnsafeClose()
	assert.NoError(t, err, "Multiple close calls should be safe")
	assert.False(t, server.IsActive(), "Server should remain inactive")
}

// =============================================================================
// Concurrent Testing (from gudp_test.go)
// =============================================================================

// Test UDP Channel interface compliance
func TestUDPChannel_InterfaceCompliance(t *testing.T) {
	ch := &Channel{}

	// Verify interface implementations
	assert.Implements(t, (*channel.Channel)(nil), ch)
	assert.NotNil(t, ch)
}

// Test concurrent UDP connection attempts
func TestUDPChannel_ConcurrentConnections(t *testing.T) {
	const numGoroutines = 100
	const connectionsPerGoroutine = 20
	peer, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	require.NoError(t, err)
	defer peer.Close()
	remoteAddr := peer.LocalAddr().(*net.UDPAddr)
	localAddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)}
	slots := make(chan struct{}, 10)
	var successfulConnections atomic.Int64
	var rejectedConnections atomic.Int64
	var wg sync.WaitGroup
	for range numGoroutines {
		wg.Go(func() {
			for range connectionsPerGoroutine {
				slots <- struct{}{}
				ch := &Channel{}
				ch.Init()
				if assert.NoError(t, ch.UnsafeConnect(localAddr, remoteAddr)) {
					assert.Equal(t, remoteAddr.String(), ch.RemoteAddr().String())
					assert.True(t, ch.Conn().IsActive())
					if assert.NoError(t, ch.UnsafeDisconnect()) {
						assert.False(t, ch.Conn().IsActive())
						successfulConnections.Add(1)
					}
				}
				<-slots
			}
			invalid := &Channel{}
			invalid.Init()
			if assert.ErrorIs(t, invalid.UnsafeConnect(localAddr, &net.TCPAddr{}), ErrNotUDPAddr) {
				assert.Nil(t, invalid.Conn())
				rejectedConnections.Add(1)
			}
		})
	}
	wg.Wait()
	expected := int64(numGoroutines * connectionsPerGoroutine)
	assert.Equal(t, expected, successfulConnections.Load())
	assert.Equal(t, int64(numGoroutines), rejectedConnections.Load())
}

// Test concurrent server channel operations
func TestUDPServerChannel_ConcurrentOperations(t *testing.T) {
	const numServers = 50
	const operationsPerServer = 30

	var successfulBinds int64
	var failedBinds int64
	var wg sync.WaitGroup

	// Test concurrent server creation and binding
	for range numServers {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for range operationsPerServer {
				server := &ServerChannel{}
				server.Init()

				localAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:0")

				err := server.UnsafeBind(localAddr)
				if err != nil {
					atomic.AddInt64(&failedBinds, 1)
				} else {
					atomic.AddInt64(&successfulBinds, 1)
					// Ensure proper cleanup
					server.UnsafeClose()
				}
			}
		}()
	}

	wg.Wait()

	// Verify that all operations were attempted
	totalOperations := atomic.LoadInt64(&successfulBinds) + atomic.LoadInt64(&failedBinds)
	expectedTotal := int64(numServers * operationsPerServer)
	assert.Equal(t, expectedTotal, totalOperations, "All bind attempts should be counted")

	t.Logf("Server bind attempts: %d successful, %d failed out of %d total",
		successfulBinds, failedBinds, totalOperations)
}

// =============================================================================
// UDPClientConn Tests (from udpserverchannel_comprehensive_test.go)
// =============================================================================

func TestServerChannel_UnsafeAcceptDeliversFirstPacket(t *testing.T) {
	server := &ServerChannel{}
	server.Init()
	localAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	assert.NoError(t, server.UnsafeBind(localAddr))
	defer server.UnsafeClose()

	client, err := net.DialUDP("udp", nil, server.conn.Load().LocalAddr().(*net.UDPAddr))
	assert.NoError(t, err)
	defer client.Close()
	_, err = client.Write([]byte("first-packet"))
	assert.NoError(t, err)

	child, future := server.UnsafeAccept()
	assert.NotNil(t, child)
	assert.NotNil(t, future)

	netChild := child.(channel.NetChannel)
	readBuffer := make([]byte, 32)
	n, err := netChild.Conn().Read(readBuffer)
	assert.NoError(t, err)
	assert.Equal(t, "first-packet", string(readBuffer[:n]))
}

type udpPeerRecorder struct {
	channel.DefaultHandler
	reads    chan string
	inactive chan string
}

func (h *udpPeerRecorder) Read(ctx channel.HandlerContext, obj any) {
	h.reads <- ctx.Channel().ID() + "|" + string(obj.(buf.ByteBuf).Bytes())
}

func (h *udpPeerRecorder) Inactive(ctx channel.HandlerContext) {
	h.inactive <- ctx.Channel().ID()
	ctx.FireInactive()
}

func startUDPPeerServer(t *testing.T, params map[channel.ParamKey]any) (recorder *udpPeerRecorder, addr *net.UDPAddr, serverCh channel.Channel) {
	t.Helper()
	probe, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	require.NoError(t, err)
	addr = probe.LocalAddr().(*net.UDPAddr)
	require.NoError(t, probe.Close())

	recorder = &udpPeerRecorder{reads: make(chan string, 64), inactive: make(chan string, 64)}
	bootstrap := channel.NewServerBootstrap()
	bootstrap.ChildHandler(recorder)
	bootstrap.ChannelType(&ServerChannel{})
	for key, value := range params {
		bootstrap.SetParams(key, value)
	}

	serverCh = bootstrap.Bind(addr).Sync().Channel()
	t.Cleanup(func() { serverCh.Close().Sync() })
	return recorder, addr, serverCh
}

func dialUDPPeer(t *testing.T, addr *net.UDPAddr) (conn *net.UDPConn) {
	t.Helper()
	conn, err := net.DialUDP("udp4", nil, addr)
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })
	return conn
}

func nextUDPRead(t *testing.T, recorder *udpPeerRecorder) (channelID string, payload string) {
	t.Helper()
	select {
	case got := <-recorder.reads:
		channelID, payload, _ = strings.Cut(got, "|")
		return channelID, payload
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for a datagram")
		return "", ""
	}
}

func TestServerChannel_ServesEachPeerThroughOneChild(t *testing.T) {
	recorder, addr, _ := startUDPPeerServer(t, nil)
	first, second := dialUDPPeer(t, addr), dialUDPPeer(t, addr)
	perChild := map[string][]string{}
	for i := range 3 {
		_, err := first.Write([]byte(fmt.Sprintf("a%d", i)))
		require.NoError(t, err)
		id, payload := nextUDPRead(t, recorder)
		perChild[id] = append(perChild[id], payload)
		_, err = second.Write([]byte(fmt.Sprintf("b%d", i)))
		require.NoError(t, err)
		id, payload = nextUDPRead(t, recorder)
		perChild[id] = append(perChild[id], payload)
	}

	require.Len(t, perChild, 2, "each remote address must be served by one child")
	require.ElementsMatch(t, [][]string{{"a0", "a1", "a2"}, {"b0", "b1", "b2"}}, slices.Collect(maps.Values(perChild)))
}

func TestServerChannel_DropsNewPeersAtLimit(t *testing.T) {
	recorder, addr, _ := startUDPPeerServer(t, map[channel.ParamKey]any{ParamMaxPeers: 1})
	first, second := dialUDPPeer(t, addr), dialUDPPeer(t, addr)
	_, err := first.Write([]byte("first"))
	require.NoError(t, err)
	_, payload := nextUDPRead(t, recorder)
	require.Equal(t, "first", payload)

	_, err = second.Write([]byte("dropped"))
	require.NoError(t, err)
	_, err = first.Write([]byte("served"))
	require.NoError(t, err)

	_, payload = nextUDPRead(t, recorder)
	require.Equal(t, "served", payload, "a new address over the limit must not get a child")
}

func TestServerChannel_ClosesIdlePeer(t *testing.T) {
	recorder, addr, _ := startUDPPeerServer(t, map[channel.ParamKey]any{ParamPeerIdleTimeout: 200})
	peer := dialUDPPeer(t, addr)
	_, err := peer.Write([]byte("before"))
	require.NoError(t, err)
	firstID, _ := nextUDPRead(t, recorder)

	select {
	case id := <-recorder.inactive:
		require.Equal(t, firstID, id)
	case <-time.After(3 * time.Second):
		t.Fatal("an idle peer's child must close")
	}

	_, err = peer.Write([]byte("after"))
	require.NoError(t, err)
	secondID, payload := nextUDPRead(t, recorder)
	require.Equal(t, "after", payload)
	require.NotEqual(t, firstID, secondID, "the next datagram opens a new child")
}

func TestServerChannel_CloseWhileNewPeersArrive(t *testing.T) {
	bootstrap := channel.NewServerBootstrap()
	bootstrap.ChildHandler(&channel.DefaultHandler{})
	bootstrap.ChannelType(&ServerChannel{})
	server := bootstrap.Bind(&net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)}).Sync().Channel().(*ServerChannel)
	addr := server.conn.Load().LocalAddr().(*net.UDPAddr)
	stop := make(chan struct{})
	sent := make(chan struct{})
	go func() {
		defer close(sent)
		for {
			select {
			case <-stop:
				return
			default:
			}

			if conn, err := net.DialUDP("udp4", nil, addr); err == nil {
				conn.Write([]byte("hello"))
				conn.Close()
			}
		}
	}()

	time.Sleep(100 * time.Millisecond)
	closed := server.Close()
	select {
	case <-closed.Done():
	case <-time.After(3 * time.Second):
		t.Error("Close must not wait for peers that arrive while the server closes")
	}

	close(stop)
	<-sent
}

type udpPusher struct {
	channel.DefaultHandler
}

func (h *udpPusher) Read(ctx channel.HandlerContext, obj any) {
	go func() {
		for i := range 10 {
			time.Sleep(50 * time.Millisecond)
			ctx.Channel().Write(buf.NewByteBufString(strconv.Itoa(i)))
		}
	}()
}

func TestServerChannel_KeepsPeerThatOnlyReceives(t *testing.T) {
	bootstrap := channel.NewServerBootstrap()
	bootstrap.ChildHandler(&udpPusher{})
	bootstrap.ChannelType(&ServerChannel{})
	bootstrap.SetChildParams(channel.ParamReadTimeout, 20)
	bootstrap.SetChildParams(channel.ParamReadIdleTimeout, 100)
	server := bootstrap.Bind(&net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)}).Sync().Channel().(*ServerChannel)
	t.Cleanup(func() { server.Close().Sync() })
	peer := dialUDPPeer(t, server.conn.Load().LocalAddr().(*net.UDPAddr))
	_, err := peer.Write([]byte("subscribe"))
	require.NoError(t, err)

	received := 0
	peer.SetReadDeadline(time.Now().Add(2 * time.Second))
	for received < 10 {
		if _, err := peer.Read(make([]byte, 16)); err != nil {
			break
		}

		received++
	}

	require.Equal(t, 10, received, "the peer idle timeout, not the read idle timeout, decides when a UDP peer is idle")
}

func TestServerChannel_CloseReleasesPeers(t *testing.T) {
	recorder, addr, server := startUDPPeerServer(t, map[channel.ParamKey]any{ParamPeerIdleTimeout: 0})
	peer := dialUDPPeer(t, addr)
	_, err := peer.Write([]byte("hello"))
	require.NoError(t, err)
	childID, _ := nextUDPRead(t, recorder)

	server.Close().Sync()

	select {
	case id := <-recorder.inactive:
		require.Equal(t, childID, id)
	case <-time.After(3 * time.Second):
		t.Fatal("closing the server must close its peer children even without an idle timeout")
	}

	require.Eventually(t, func() bool { return server.(*ServerChannel).peerCount.Load() == 0 }, 3*time.Second, 20*time.Millisecond)
}

func newTestUDPClientConn(t *testing.T) (clientConn *UDPClientConn, peer *net.UDPConn) {
	t.Helper()
	socket, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	require.NoError(t, err)
	t.Cleanup(func() { socket.Close() })
	peer, err = net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	require.NoError(t, err)
	t.Cleanup(func() { peer.Close() })

	clientConn = &UDPClientConn{
		server:          &ServerChannel{},
		socket:          socket,
		clientAddr:      peer.LocalAddr().(*net.UDPAddr),
		packets:         make(chan []byte, peerQueueLength),
		closed:          make(chan struct{}),
		deadlineChanged: make(chan struct{}, 1),
	}

	return clientConn, peer
}

func TestUDPClientConn_Read(t *testing.T) {
	t.Parallel()

	t.Run("Read_TruncatesToBuffer", func(t *testing.T) {
		t.Parallel()
		clientConn, _ := newTestUDPClientConn(t)
		clientConn.deliver([]byte("very long datagram"))

		buffer := make([]byte, 4)
		n, err := clientConn.Read(buffer)

		assert.NoError(t, err)
		assert.Equal(t, "very", string(buffer[:n]))
	})

	t.Run("Read_WithTimeout", func(t *testing.T) {
		t.Parallel()
		clientConn, _ := newTestUDPClientConn(t)
		assert.NoError(t, clientConn.SetReadDeadline(time.Now().Add(50*time.Millisecond)))

		_, err := clientConn.Read(make([]byte, 8))

		assert.ErrorIs(t, err, os.ErrDeadlineExceeded)
	})

	t.Run("Read_WakesOnShorterDeadline", func(t *testing.T) {
		t.Parallel()
		clientConn, _ := newTestUDPClientConn(t)
		go func() {
			time.Sleep(50 * time.Millisecond)
			_ = clientConn.SetReadDeadline(time.Now())
		}()

		started := time.Now()
		_, err := clientConn.Read(make([]byte, 8))

		assert.ErrorIs(t, err, os.ErrDeadlineExceeded)
		assert.Less(t, time.Since(started), time.Second, "a blocked Read must apply a deadline set while it waits")
	})

	t.Run("Read_AfterClose", func(t *testing.T) {
		t.Parallel()
		clientConn, _ := newTestUDPClientConn(t)
		assert.NoError(t, clientConn.Close())

		_, err := clientConn.Read(make([]byte, 8))

		assert.ErrorIs(t, err, net.ErrClosed)
	})
}

func TestUDPClientConn_QueueBudget(t *testing.T) {
	t.Parallel()
	clientConn, _ := newTestUDPClientConn(t)
	clientConn.server.queuedBytes.Store(maxQueuedBytes - 4)

	clientConn.deliver([]byte("fits"))
	clientConn.deliver([]byte("x"))
	buffer := make([]byte, 8)
	n, err := clientConn.Read(buffer)
	require.NoError(t, err)
	assert.Equal(t, "fits", string(buffer[:n]), "a datagram over the shared budget must be dropped")
	assert.Equal(t, int64(maxQueuedBytes-4), clientConn.server.queuedBytes.Load(), "reading releases the budget")

	clientConn.deliver([]byte("abc"))
	require.NoError(t, clientConn.Close())
	assert.Equal(t, int64(maxQueuedBytes-4), clientConn.server.queuedBytes.Load(), "closing releases queued datagrams")
}

func TestUDPClientConn_Write(t *testing.T) {
	t.Parallel()
	clientConn, peer := newTestUDPClientConn(t)

	n, err := clientConn.Write([]byte("test message"))
	require.NoError(t, err)
	assert.Equal(t, len("test message"), n)
	buffer := make([]byte, 32)
	require.NoError(t, peer.SetReadDeadline(time.Now().Add(time.Second)))
	n, err = peer.Read(buffer)
	require.NoError(t, err)
	assert.Equal(t, "test message", string(buffer[:n]))

	require.NoError(t, clientConn.Close())
	_, err = clientConn.Write([]byte("late"))
	assert.ErrorIs(t, err, net.ErrClosed)
}

func TestUDPClientConn_Close(t *testing.T) {
	t.Parallel()
	clientConn, _ := newTestUDPClientConn(t)

	assert.NoError(t, clientConn.Close())
	assert.NoError(t, clientConn.Close(), "multiple closes should succeed")
}

// =============================================================================
// Benchmark Tests
// =============================================================================

// Benchmark UDP operations
func BenchmarkUDPChannel_ConcurrentOperations(b *testing.B) {
	const numGoroutines = 50

	b.ResetTimer()

	for b.Loop() {
		var wg sync.WaitGroup

		for i := range numGoroutines {
			wg.Add(1)
			go func(routineID int) {
				defer wg.Done()

				ch := &Channel{}
				ch.Init()

				// Perform address validation and connection attempt
				localAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:0")
				remoteAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:99999")
				ch.UnsafeConnect(localAddr, remoteAddr)
			}(i)
		}

		wg.Wait()
	}
}

// Benchmark UDP server operations
func BenchmarkUDPServerChannel_ConcurrentOperations(b *testing.B) {
	const numGoroutines = 50

	b.ResetTimer()

	for b.Loop() {
		var wg sync.WaitGroup

		for range numGoroutines {
			wg.Add(1)
			go func() {
				defer wg.Done()

				server := &ServerChannel{}
				server.Init()

				localAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:0")

				server.UnsafeBind(localAddr)
				server.IsActive()
				server.UnsafeClose()
			}()
		}

		wg.Wait()
	}
}
