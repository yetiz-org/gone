package gudp

import (
	"fmt"
	"net"
	"time"

	"github.com/pkg/errors"
	"github.com/yetiz-org/gone/channel"
	"github.com/yetiz-org/gone/utils"
	"github.com/yetiz-org/goth-kklogger"
)

// ServerChannel represents a UDP server channel implementation
type ServerChannel struct {
	channel.DefaultNetServerChannel
	conn   *net.UDPConn
	active bool
}

var ErrBindTwice = fmt.Errorf("bind twice")

// UnsafeBind binds the UDP server to the specified local address
func (c *ServerChannel) UnsafeBind(localAddr net.Addr) error {
	if c.Name == "" {
		c.Name = fmt.Sprintf("UDPSERV_%s", localAddr.String())
	}

	if c.IsActive() {
		err := errors.Wrap(ErrBindTwice, c.Name)
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
		c.conn = conn
		c.active = true
	}

	return nil
}

// UnsafeAccept for UDP servers - since UDP is connectionless, this creates a virtual channel
// that represents communication with a specific remote address
func (c *ServerChannel) UnsafeAccept() (channel.Channel, channel.Future) {
	if !c.IsActive() || c.conn == nil {
		return nil, c.Pipeline().NewFuture()
	}

	// Get buffer from pool for memory optimization - using 64KB buffer for UDP max packet size
	buffer := utils.GetLargeBuffer()
	defer utils.PutLargeBuffer(buffer) // Return buffer to pool when done

	// For UDP, we need to read a packet to know which client is connecting
	n, clientAddr, err := c.conn.ReadFromUDP(buffer)
	if err != nil {
		if !c.IsActive() {
			return nil, c.Pipeline().NewFuture()
		}

		kklogger.ErrorJ("gudp:ServerChannel.UnsafeAccept#unsafe_accept!read_error", err.Error())
		return nil, c.Pipeline().NewFuture()
	}

	// Create a copy of the data since we're returning the buffer to the pool
	data := make([]byte, n)
	copy(data, buffer[:n])

	// Create a virtual UDP connection for this client
	clientConn := &UDPClientConn{
		server:     c.conn,
		clientAddr: clientAddr,
		lastData:   data, // Store the first packet (copied data)
	}

	// Create child channel for this client
	ch := c.DeriveNetChildChannel(&Channel{}, c, clientConn)
	return ch, ch.Pipeline().NewFuture()
}

// UnsafeClose closes the UDP server connection
func (c *ServerChannel) UnsafeClose() error {
	c.DefaultNetServerChannel.UnsafeClose()
	c.active = false

	// Prevent nil pointer dereference and double close - check if connection exists before closing
	if c.conn != nil {
		conn := c.conn
		c.conn = nil // Set to nil first to prevent double close
		return conn.Close()
	}
	return nil
}

// IsActive returns whether the UDP server is currently active
func (c *ServerChannel) IsActive() bool {
	return c.active
}

// UDPClientConn represents a virtual connection to a specific UDP client
// This allows UDP to work with the existing channel framework
type UDPClientConn struct {
	server     *net.UDPConn
	clientAddr *net.UDPAddr
	lastData   []byte // Buffer for the first received packet
	firstRead  bool   // Flag to track if this is the first read
}

// Read implements net.Conn interface for UDP client connections
func (c *UDPClientConn) Read(b []byte) (n int, err error) {
	if !c.firstRead && len(c.lastData) > 0 {
		// Return the first packet that was read during accept
		c.firstRead = true
		n = copy(b, c.lastData)
		if n < len(c.lastData) {
			// If buffer is too small, we lose data - this is a limitation of UDP
			kklogger.WarnJ("gudp:UDPClientConn.Read#read!buffer_too_small",
				fmt.Sprintf("Buffer size %d smaller than packet size %d", len(b), len(c.lastData)))
		}
		return n, nil
	}

	// For subsequent reads, we should read from the specific client
	// Since UDP is connectionless, we need to filter packets by client address
	for {
		n, addr, err := c.server.ReadFromUDP(b)
		if err != nil {
			return 0, err
		}

		// Only return packets from our specific client
		if addr.String() == c.clientAddr.String() {
			return n, nil
		}
		// Otherwise, continue reading until we get a packet from our client
		// Note: This is not ideal for performance but necessary for the channel abstraction
	}
}

// Write implements net.Conn interface for UDP client connections
func (c *UDPClientConn) Write(b []byte) (n int, err error) {
	return c.server.WriteToUDP(b, c.clientAddr)
}

// Close implements net.Conn interface for UDP client connections
func (c *UDPClientConn) Close() error {
	// For UDP client connections, we don't actually close anything
	// The server connection remains open for other clients
	return nil
}

// LocalAddr implements net.Conn interface for UDP client connections
func (c *UDPClientConn) LocalAddr() net.Addr {
	return c.server.LocalAddr()
}

// RemoteAddr implements net.Conn interface for UDP client connections
func (c *UDPClientConn) RemoteAddr() net.Addr {
	return c.clientAddr
}

// SetDeadline implements net.Conn interface for UDP client connections
func (c *UDPClientConn) SetDeadline(t time.Time) error {
	return c.server.SetDeadline(t)
}

// SetReadDeadline implements net.Conn interface for UDP client connections
func (c *UDPClientConn) SetReadDeadline(t time.Time) error {
	return c.server.SetReadDeadline(t)
}

// SetWriteDeadline implements net.Conn interface for UDP client connections
func (c *UDPClientConn) SetWriteDeadline(t time.Time) error {
	return c.server.SetWriteDeadline(t)
}
