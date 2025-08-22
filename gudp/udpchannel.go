package gudp

import (
	"fmt"
	"net"

	"github.com/yetiz-org/gone/channel"
)

// Channel represents a UDP client channel implementation
type Channel struct {
	channel.DefaultNetChannel
}

var ErrNotUDPAddr = fmt.Errorf("not udp addr")

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
