package gudp

import (
	"fmt"
	"net"

	"github.com/yetiz-org/gone/channel"
	buf "github.com/yetiz-org/goth-bytebuf"
)

// Channel represents a UDP client channel implementation
type Channel struct {
	channel.DefaultNetChannel
}

var ErrNotUDPAddr = fmt.Errorf("not udp addr")

// Init leaves idle detection to the server's ParamPeerIdleTimeout, which counts datagrams in both
// directions, so a peer that only receives is not closed for reading nothing.
func (c *Channel) Init() (ch channel.Channel) {
	c.DefaultNetChannel.Init()
	c.ReadIdleTimeout = 0
	return c
}

// UnsafeConnect establishes a UDP connection to the remote address
// Unlike TCP, UDP is connectionless, but this creates a connected UDP socket
func (c *Channel) UnsafeConnect(localAddr net.Addr, remoteAddr net.Addr) error {
	if remoteAddr == nil {
		return channel.ErrNilObject
	}

	// Validate that the remote address is a UDP address
	if _, ok := remoteAddr.(*net.UDPAddr); !ok {
		return ErrNotUDPAddr
	}

	// Validate local address if provided
	if localAddr != nil {
		if _, ok := localAddr.(*net.UDPAddr); !ok {
			return ErrNotUDPAddr
		}
	}

	return c.DefaultNetChannel.UnsafeConnect(localAddr, remoteAddr)
}

// UnsafeWrite sends each ByteBuf as one datagram. DefaultNetChannel would write a CompositeByteBuf
// segment by segment, which UDP delivers as separate datagrams.
func (c *Channel) UnsafeWrite(obj any) (rtErr error) {
	if v, ok := obj.(buf.ByteBuf); ok {
		obj = v.Bytes()
	}

	return c.DefaultNetChannel.UnsafeWrite(obj)
}

// UnsafeClose releases this peer's connection and server admission quota without closing the shared server socket.
func (c *Channel) UnsafeClose() (rtErr error) {
	return c.UnsafeDisconnect()
}
