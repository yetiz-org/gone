package channel

import (
	"io"
	"net"
	"reflect"

	"github.com/kklab-com/goth-kklogger"
	"github.com/kklab-com/goth-kkutil/buf"
	kkpanic "github.com/kklab-com/goth-panic"
	"github.com/pkg/errors"
)

type NetChannel interface {
	Channel
	Conn() Conn
	RemoteAddr() net.Addr
}

type NetChannelPostActive interface {
	PostActive(conn net.Conn)
}

type DefaultNetChannel struct {
	DefaultChannel
	conn         Conn
	bufferSize   int
	readTimeout  int
	writeTimeout int
}

func (c *DefaultNetChannel) Init() Channel {
	c.bufferSize = GetParamIntDefault(c, ParamReadBufferSize, 1024)
	c.readTimeout = GetParamIntDefault(c, ParamReadTimeout, 6000)
	c.writeTimeout = GetParamIntDefault(c, ParamWriteTimeout, 3000)
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

func (c *DefaultNetChannel) PostActive(conn net.Conn) {
	c.conn = WrapConn(conn)
	c.Pipeline().fireActive()
}

func (c *DefaultNetChannel) UnsafeWrite(obj interface{}) error {
	if c.Conn() == nil {
		return ErrNilObject
	}

	if !c.IsActive() {
		return net.ErrClosed
	}

	var bs []byte
	switch v := obj.(type) {
	case buf.ByteBuf:
		bs = v.Bytes()
	case []byte:
		bs = v
	default:
		kklogger.ErrorJ("DefaultNetChannel.UnsafeWrite", errors.Wrap(ErrUnknownObjectType, reflect.TypeOf(v).String()))
		return ErrUnknownObjectType
	}

	if _, err := c.Conn().Write(bs); err != nil {
		kklogger.ErrorJ("DefaultNetChannel.UnsafeWrite", err.Error())
		return err
	}

	return nil
}

func (c *DefaultNetChannel) UnsafeRead() error {
	if c.Conn() == nil {
		return ErrNilObject
	}

	for c.IsActive() {
		if !c.IsActive() {
			return net.ErrClosed
		}

		bs := make([]byte, 1024)
		if rl, err := c.Conn().Read(bs); err != nil {
			if c.IsActive() {
				if err != io.EOF {
					kklogger.WarnJ("DefaultNetChannel.UnsafeRead", err.Error())
					return ErrReadError
				}
			} else if err == io.EOF {
				return ErrReadError
			}
		} else {
			kkpanic.Catch(func() {
				c.FireRead(buf.NewByteBuf(bs[:rl]))
				c.FireReadCompleted()
			}, func(r kkpanic.Caught) {
				kklogger.ErrorJ("DefaultNetChannel.UnsafeRead", r.String())
			})
		}
	}

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
