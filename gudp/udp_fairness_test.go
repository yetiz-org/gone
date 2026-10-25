package gudp

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/yetiz-org/gone/channel"
	buf "github.com/yetiz-org/goth-bytebuf"
)

type udpAdmissionEvent struct {
	peer    channel.Channel
	payload string
}

type udpAdmissionRecorder struct {
	channel.DefaultHandler
	reads chan udpAdmissionEvent
}

func (r *udpAdmissionRecorder) Read(ctx channel.HandlerContext, obj any) {
	r.reads <- udpAdmissionEvent{peer: ctx.Channel(), payload: string(obj.(buf.ByteBuf).Bytes())}
}

func startUDPAdmissionServer(t *testing.T, params map[channel.ParamKey]any) (addr *net.UDPAddr, recorder *udpAdmissionRecorder) {
	t.Helper()
	recorder = &udpAdmissionRecorder{reads: make(chan udpAdmissionEvent, 32)}
	bootstrap := channel.NewServerBootstrap()
	bootstrap.ChannelType(&ServerChannel{})
	bootstrap.ChildHandler(recorder)
	for key, value := range params {
		bootstrap.SetParams(key, value)
	}

	server := bootstrap.Bind(&net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)}).Sync().Channel().(*ServerChannel)
	t.Cleanup(func() { server.Close().Sync() })
	return server.conn.Load().LocalAddr().(*net.UDPAddr), recorder
}

func dialUDPAdmissionPeer(t *testing.T, addr *net.UDPAddr, source net.IP) (peer *net.UDPConn) {
	t.Helper()
	peer, err := net.DialUDP("udp4", &net.UDPAddr{IP: source}, addr)
	require.NoError(t, err)
	t.Cleanup(func() { peer.Close() })
	return peer
}

func readUDPAdmissionEvent(t *testing.T, recorder *udpAdmissionRecorder) (event udpAdmissionEvent) {
	t.Helper()
	select {
	case event = <-recorder.reads:
		return event
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for an admitted UDP peer")
		return event
	}
}

func assertUDPAdmissionDrop(t *testing.T, recorder *udpAdmissionRecorder) {
	t.Helper()
	select {
	case event := <-recorder.reads:
		t.Fatalf("new peer over admission limit reached handler with payload %q", event.payload)
	case <-time.After(80 * time.Millisecond):
	}
}

func TestServerChannel_PerSourcePeerLimit(t *testing.T) {
	addr, recorder := startUDPAdmissionServer(t, map[channel.ParamKey]any{
		ParamMaxPeers:                                4,
		channel.ParamKey("udp_max_peers_per_ip"):     2,
		channel.ParamKey("udp_new_peers_per_second"): 1000,
		channel.ParamKey("udp_new_peer_burst"):       1000,
	})
	first := dialUDPAdmissionPeer(t, addr, net.IPv4(127, 0, 0, 1))
	second := dialUDPAdmissionPeer(t, addr, net.IPv4(127, 0, 0, 1))
	third := dialUDPAdmissionPeer(t, addr, net.IPv4(127, 0, 0, 1))
	_, err := first.Write([]byte("first"))
	require.NoError(t, err)
	firstEvent := readUDPAdmissionEvent(t, recorder)
	_, err = second.Write([]byte("second"))
	require.NoError(t, err)
	require.Equal(t, "second", readUDPAdmissionEvent(t, recorder).payload)
	_, err = third.Write([]byte("over-source-quota"))
	require.NoError(t, err)
	assertUDPAdmissionDrop(t, recorder)

	_, err = first.Write([]byte("established"))
	require.NoError(t, err)
	established := readUDPAdmissionEvent(t, recorder)
	require.Equal(t, firstEvent.peer, established.peer)
	require.Equal(t, "established", established.payload)

	// All traffic still targets loopback; another local interface may supply the distinct source.
	sources := []net.IP{net.IPv4(127, 0, 0, 2)}
	if interfaces, listErr := net.InterfaceAddrs(); listErr == nil {
		for _, address := range interfaces {
			if ipnet, ok := address.(*net.IPNet); ok && ipnet.IP.To4() != nil && !ipnet.IP.IsLoopback() {
				sources = append(sources, ipnet.IP)
			}
		}
	}

	for _, source := range sources {
		other, dialErr := net.DialUDP("udp4", &net.UDPAddr{IP: source}, addr)
		if dialErr != nil {
			continue
		}

		t.Cleanup(func() { other.Close() })
		_, err = other.Write([]byte("other-source"))
		require.NoError(t, err)
		require.Equal(t, "other-source", readUDPAdmissionEvent(t, recorder).payload)
		return
	}

	t.Log("distinct-source assertion unavailable: host has no usable second local IPv4 address")
}

func TestServerChannel_NewPeerRateLimit(t *testing.T) {
	addr, recorder := startUDPAdmissionServer(t, map[channel.ParamKey]any{
		channel.ParamKey("udp_max_peers_per_ip"):     8,
		channel.ParamKey("udp_new_peers_per_second"): 1,
		channel.ParamKey("udp_new_peer_burst"):       1,
	})
	first := dialUDPAdmissionPeer(t, addr, net.IPv4(127, 0, 0, 1))
	second := dialUDPAdmissionPeer(t, addr, net.IPv4(127, 0, 0, 1))
	_, err := first.Write([]byte("first"))
	require.NoError(t, err)
	firstEvent := readUDPAdmissionEvent(t, recorder)
	_, err = second.Write([]byte("over-burst"))
	require.NoError(t, err)
	assertUDPAdmissionDrop(t, recorder)
	_, err = first.Write([]byte("existing-bypasses-bucket"))
	require.NoError(t, err)
	existing := readUDPAdmissionEvent(t, recorder)
	require.Equal(t, firstEvent.peer, existing.peer)
	require.Equal(t, "existing-bypasses-bucket", existing.payload)

	time.Sleep(1100 * time.Millisecond)
	_, err = second.Write([]byte("after-refill"))
	require.NoError(t, err)
	require.Equal(t, "after-refill", readUDPAdmissionEvent(t, recorder).payload)
}

func TestServerChannel_DefaultSourceQuota(t *testing.T) {
	for _, value := range []any{nil, 0, -1} {
		name := "default"
		if value == 0 {
			name = "zero"
		} else if value == -1 {
			name = "negative"
		}

		t.Run(name, func(t *testing.T) {
			params := map[channel.ParamKey]any{}
			if value != nil {
				params[channel.ParamKey("udp_max_peers_per_ip")] = value
			}

			addr, recorder := startUDPAdmissionServer(t, params)
			var first *net.UDPConn
			for i := range 128 {
				peer := dialUDPAdmissionPeer(t, addr, net.IPv4(127, 0, 0, 1))
				if i == 0 {
					first = peer
				}

				_, err := peer.Write([]byte("within-default"))
				require.NoError(t, err)
				require.Equal(t, "within-default", readUDPAdmissionEvent(t, recorder).payload)
			}

			other := dialUDPAdmissionPeer(t, addr, net.IPv4(127, 0, 0, 1))
			_, err := other.Write([]byte("over-default"))
			require.NoError(t, err)
			assertUDPAdmissionDrop(t, recorder)
			_, err = first.Write([]byte("default-existing"))
			require.NoError(t, err)
			require.Equal(t, "default-existing", readUDPAdmissionEvent(t, recorder).payload)
		})
	}
}

func TestServerChannel_SourceQuotaReleasedOnCloseAndIdle(t *testing.T) {
	for _, name := range []string{"conn-close", "channel-close", "idle"} {
		t.Run(name, func(t *testing.T) {
			idleTimeout := 0
			if name == "idle" {
				idleTimeout = 200
			}

			addr, recorder := startUDPAdmissionServer(t, map[channel.ParamKey]any{
				ParamPeerIdleTimeout:                         idleTimeout,
				channel.ParamKey("udp_max_peers_per_ip"):     1,
				channel.ParamKey("udp_new_peers_per_second"): 1000,
				channel.ParamKey("udp_new_peer_burst"):       1000,
			})
			first := dialUDPAdmissionPeer(t, addr, net.IPv4(127, 0, 0, 1))
			second := dialUDPAdmissionPeer(t, addr, net.IPv4(127, 0, 0, 1))
			_, err := first.Write([]byte("first"))
			require.NoError(t, err)
			firstEvent := readUDPAdmissionEvent(t, recorder)
			_, err = second.Write([]byte("blocked"))
			require.NoError(t, err)
			assertUDPAdmissionDrop(t, recorder)
			if name == "idle" {
				select {
				case <-firstEvent.peer.CloseFuture().Done():
				case <-time.After(2 * time.Second):
					t.Fatal("idle peer did not release its child")
				}
			} else if name == "channel-close" {
				require.NoError(t, firstEvent.peer.Close().Sync().Error())
			} else {
				conn := firstEvent.peer.(channel.NetChannel).Conn()
				require.NoError(t, conn.Close())
				require.NoError(t, conn.Close())
				_, err = first.Write([]byte("same-address-reopened"))
				require.NoError(t, err)
				reopened := readUDPAdmissionEvent(t, recorder)
				require.Equal(t, "same-address-reopened", reopened.payload)
				require.NotEqual(t, firstEvent.peer.ID(), reopened.peer.ID())
				require.NoError(t, conn.Close())
				_, err = second.Write([]byte("stale-close-must-not-release-replacement"))
				require.NoError(t, err)
				assertUDPAdmissionDrop(t, recorder)
				require.NoError(t, reopened.peer.(channel.NetChannel).Conn().Close())
			}

			var reused udpAdmissionEvent
			require.Eventually(t, func() bool {
				if _, writeErr := second.Write([]byte("reused-source-slot")); writeErr != nil {
					return false
				}

				select {
				case reused = <-recorder.reads:
					return true
				default:
					return false
				}
			}, 2*time.Second, 20*time.Millisecond)
			require.Equal(t, "reused-source-slot", reused.payload)
			require.NotEqual(t, firstEvent.peer.ID(), reused.peer.ID())
		})
	}
}
