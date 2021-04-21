package channel

import (
	"errors"
	"io"
	"net"
	"os"
	"reflect"
	"sync/atomic"
	"time"

	"github.com/kklab-com/goth-kklogger"
	"github.com/kklab-com/goth-kkutil/buf"
	errors2 "github.com/pkg/errors"
)

type NetChannel interface {
	Channel
	Conn() Conn
	RemoteAddr() net.Addr
	setConn(conn net.Conn)
}

type NetChannelSetConn interface {
	SetConn(conn net.Conn)
}

type DefaultNetChannel struct {
	DefaultChannel
	conn         Conn
	BufferSize   int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	rState       int32
}

func (c *DefaultNetChannel) Init() Channel {
	c.BufferSize = GetParamIntDefault(c, ParamReadBufferSize, 1024)
	c.ReadTimeout = time.Duration(GetParamIntDefault(c, ParamReadTimeout, 1000)) * time.Millisecond
	c.WriteTimeout = time.Duration(GetParamIntDefault(c, ParamWriteTimeout, 100)) * time.Millisecond
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
		}
	}

	return nil
}

func (c *DefaultNetChannel) setConn(conn net.Conn) {
	c.conn = WrapConn(conn)
}

func (c *DefaultNetChannel) IsActive() bool {
	return c.active
}

func (c *DefaultNetChannel) SetConn(conn net.Conn) {
	c.setConn(conn)
}

func (c *DefaultNetChannel) DoRead() bool {
	return atomic.CompareAndSwapInt32(&c.rState, 0, 1)
}

func (c *DefaultNetChannel) ReleaseRead() {
	atomic.StoreInt32(&c.rState, 0)
}

func (c *DefaultNetChannel) UnsafeWrite(obj interface{}) error {
	if c.Conn() == nil {
		return ErrNilObject
	}

	if !c.Conn().IsActive() {
		return net.ErrClosed
	}

	var bs []byte
	switch v := obj.(type) {
	case buf.ByteBuf:
		bs = v.Bytes()
	case []byte:
		bs = v
	default:
		kklogger.ErrorJ("DefaultNetChannel.UnsafeWrite", errors2.Wrap(ErrUnknownObjectType, reflect.TypeOf(v).String()))
		return ErrUnknownObjectType
	}

	if c.WriteTimeout > 0 {
		c.Conn().SetWriteDeadline(time.Now().Add(c.WriteTimeout))
	}

	if _, err := c.Conn().Write(bs); err != nil {
		kklogger.WarnJ("DefaultNetChannel.UnsafeWrite", err.Error())
		return err
	}

	return nil
}

func (c *DefaultNetChannel) UnsafeRead() error {
	if c.Conn() == nil {
		return ErrNilObject
	}

	if !c.IsActive() {
		return net.ErrClosed
	}

	if !c.DoRead() {
		return nil
	}

	kklogger.TraceJ("DefaultNetChannel.UnsafeRead", "change read state to 1")
	go func() {
		for c.IsActive() {
			bs := make([]byte, c.BufferSize)
			if c.ReadTimeout > 0 {
				c.Conn().SetReadDeadline(time.Now().Add(c.ReadTimeout))
			}

			rc, err := c.Conn().Read(bs)
			if err != nil {
				if errors.Is(err, os.ErrDeadlineExceeded) && c.Conn().IsActive() {
					continue
				}

				if c.IsActive() {
					if err != io.EOF {
						kklogger.WarnJ("DefaultNetChannel.UnsafeRead", err.Error())
					}

					if !c.Conn().IsActive() {
						c.Deregister()
						break
					}
				} else if err == io.EOF {
					break
				}
			} else {
				if rc == 0 {
					continue
				}

				c.FireRead(buf.NewByteBuf(bs[:rc]))
				c.FireReadCompleted()
			}
		}

		c.ReleaseRead()
		kklogger.TraceJ("DefaultNetChannel.UnsafeRead", "change read state to 0")
	}()

	return nil
}

func (c *DefaultNetChannel) UnsafeDisconnect() error {
	if c.Conn() != nil {
		return c.Conn().Close()
	}

	return ErrNilObject
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
