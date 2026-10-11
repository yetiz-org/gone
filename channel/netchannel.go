package channel

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"reflect"
	"time"

	"github.com/yetiz-org/gone/utils"
	buf "github.com/yetiz-org/goth-bytebuf"
	concurrent "github.com/yetiz-org/goth-concurrent"
	kklogger "github.com/yetiz-org/goth-kklogger"
)

// markConnInactiveOnError mirrors DefaultConn.Write's error tracking for
// write paths that bypass DefaultConn.Write. Non-deadline errors flip the
// connection into the inactive state.
func markConnInactiveOnError(c Conn, err error) {
	if err == nil || c == nil {
		return
	}
	if errors.Is(err, os.ErrDeadlineExceeded) {
		return
	}
	if dc, ok := c.(*DefaultConn); ok {
		dc.markInactive()
	}
}

// Get buffer from pool with specified size - uses appropriate pool based on size.
// Returned slice is uncleared; callers only read bytes they themselves wrote
// via the subsequent net.Conn.Read (i.e. bs[:rc]).
func getNetBuffer(size int) []byte {
	return utils.GetDirtyBufferForSize(size)
}

// Put buffer back to appropriate pool based on size
func putNetBuffer(buffer []byte) {
	utils.PutBufferForSize(buffer)
}

type NetChannel interface {
	Channel
	Conn() Conn
	RemoteAddr() net.Addr
	setConn(conn net.Conn)
}

type NetChannelSetConn interface {
	SetConn(conn net.Conn)
}

// ErrReadIdle closes a net channel that received nothing for its ParamReadIdleTimeout.
var ErrReadIdle = fmt.Errorf("read idle timeout")

type DefaultNetChannel struct {
	DefaultChannel
	conn            Conn
	BufferSize      int
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ReadIdleTimeout time.Duration
	lastRead        time.Time // touched only by the read loop
}

func (c *DefaultNetChannel) Init() (ch Channel) {
	c.BufferSize = GetParamIntDefault(c, ParamReadBufferSize, 1024)
	c.ReadTimeout = time.Duration(GetParamIntDefault(c, ParamReadTimeout, 1000)) * time.Millisecond
	c.WriteTimeout = time.Duration(GetParamIntDefault(c, ParamWriteTimeout, 100)) * time.Millisecond
	idleDefault := int64(0)
	if c.Parent() != nil {
		// Only connections a server accepted close when idle by default; a client keeps its own connection.
		idleDefault = DefaultReadIdleTimeout
	}

	c.ReadIdleTimeout = time.Duration(GetParamInt64Default(c, ParamReadIdleTimeout, idleDefault)) * time.Millisecond
	return c
}

func (c *DefaultNetChannel) Conn() Conn {
	return c.conn
}

func (c *DefaultNetChannel) RemoteAddr() net.Addr {
	if c.conn != nil {
		return c.conn.RemoteAddr()
	}

	return nil
}

func (c *DefaultNetChannel) LocalAddr() net.Addr {
	if c.localAddr == nil {
		if c.conn != nil {
			c.localAddr = c.conn.LocalAddr()
			return c.localAddr
		} else {
			return nil
		}
	}

	return c.localAddr
}

func (c *DefaultNetChannel) setConn(conn net.Conn) {
	c.conn = WrapConn(conn)
}

func (c *DefaultNetChannel) SetConn(conn net.Conn) {
	c.setConn(conn)
}

func (c *DefaultNetChannel) UnsafeWrite(obj any) error {
	if c.Conn() == nil {
		return ErrNilObject
	}

	if !c.Conn().IsActive() {
		return net.ErrClosed
	}

	if c.WriteTimeout > 0 {
		if err := c.Conn().SetWriteDeadline(time.Now().Add(c.WriteTimeout)); err != nil {
			return err
		}
	}

	var err error
	switch v := obj.(type) {
	case buf.ByteBuf:
		// Buffers that expose io.WriterTo (CompositeByteBuf) go through the
		// underlying net.Conn so net.Buffers.WriteTo can coalesce components
		// into a writev(2) syscall for *net.TCPConn / *net.UnixConn targets.
		if wt, ok := v.(io.WriterTo); ok {
			target := c.Conn().Conn()
			if limited, ok := target.(*limitConn); ok {
				// Write to the accepted connection itself so a *net.TCPConn keeps its writev path.
				target = limited.Conn
			}

			_, err = wt.WriteTo(target)
			if err != nil {
				markConnInactiveOnError(c.Conn(), err)
			}
		} else {
			_, err = c.Conn().Write(v.Bytes())
		}
	case []byte:
		_, err = c.Conn().Write(v)
	default:
		kklogger.ErrorJ("channel:DefaultNetChannel.UnsafeWrite#unsafe_write!type_error", fmt.Errorf("%s: %w", reflect.TypeOf(v).String(), ErrUnknownObjectType))
		return ErrUnknownObjectType
	}

	if err != nil {
		kklogger.WarnJ("channel:DefaultNetChannel.UnsafeWrite#unsafe_write!write_error", err.Error())
		return err
	}

	return nil
}

func (c *DefaultNetChannel) UnsafeRead() (obj any, rtErr error) {
	if c.Conn() == nil {
		return nil, ErrNilObject
	}

	if !c.IsActive() {
		return nil, net.ErrClosed
	}

	// Get buffer from pool to reduce memory allocations
	bs := getNetBuffer(c.BufferSize)
	defer putNetBuffer(bs) // Return buffer to pool when done

	wait := c.ReadTimeout
	if wait <= 0 {
		// Without a ReadTimeout the idle timeout is the deadline, so a silent peer still gets closed.
		wait = c.ReadIdleTimeout
	}

	if wait > 0 {
		if err := c.Conn().SetReadDeadline(time.Now().Add(wait)); err != nil {
			return nil, err
		}
	}

	if c.ReadIdleTimeout > 0 && c.lastRead.IsZero() {
		c.lastRead = time.Now()
	}

	if rc, err := c.Conn().Read(bs); err != nil {
		if errors.Is(err, os.ErrDeadlineExceeded) {
			if c.ReadIdleTimeout > 0 && time.Since(c.lastRead) >= c.ReadIdleTimeout {
				return nil, ErrReadIdle
			}

			if c.Conn().IsActive() {
				return nil, ErrSkip
			} else {
				return nil, ErrNotActive
			}
		}

		if c.IsActive() && err != io.EOF {
			kklogger.TraceJ("channel:DefaultNetChannel.UnsafeRead#unsafe_read!read_trace", err.Error())
		}

		return nil, err
	} else if rc == 0 {
		return nil, ErrSkip
	} else {
		if c.ReadIdleTimeout > 0 {
			c.lastRead = time.Now()
		}

		// Create a copy of the data since we're returning the buffer to pool
		data := make([]byte, rc)
		copy(data, bs[:rc])
		return buf.NewSharedByteBuf(data), nil
	}
}

// UnsafeDisconnect closes the connection even after a failed read or write marked it inactive, so its
// file descriptor is released at once instead of by the garbage collector. Closing it again is not an error.
func (c *DefaultNetChannel) UnsafeDisconnect() (rtErr error) {
	if c.Conn() == nil {
		return ErrNilObject
	}

	if err := c.Conn().Close(); err != nil && !errors.Is(err, net.ErrClosed) {
		return err
	}

	return nil
}

func (c *DefaultNetChannel) UnsafeConnect(localAddr net.Addr, remoteAddr net.Addr) error {
	if remoteAddr == nil {
		return ErrNilObject
	}

	if conn, err := net.Dial(remoteAddr.Network(), remoteAddr.String()); err != nil {
		return err
	} else {
		c.conn = WrapConn(conn)
	}

	return nil
}

func (c *DefaultNetChannel) inactiveChannel() (bool, concurrent.Future) {
	success, future := c.DefaultChannel.inactiveChannel()
	if success {
		c.UnsafeDisconnect()
	}

	return success, future
}
