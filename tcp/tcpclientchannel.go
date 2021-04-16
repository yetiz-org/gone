package tcp

import (
	"fmt"
	"io"
	"net"

	"github.com/kklab-com/gone/channel"
	kklogger "github.com/kklab-com/goth-kklogger"
	"github.com/kklab-com/goth-kkutil/buf"
	kkpanic "github.com/kklab-com/goth-panic"
)

type DefaultTCPClientChannel struct {
	*channel.DefaultNetChannel
	bufferSize           int
	readTimeout          int
	writeTimeout         int
	netClientConnectFunc func(remoteAddr net.Addr) error
}

var UnknownObjectType = fmt.Errorf("unknown object type")
var ErrNotTCPAddr = fmt.Errorf("not tcp addr")

func (c *DefaultTCPClientChannel) Init() channel.Channel {
	if c.DefaultNetChannel == nil {
		c.DefaultNetChannel = channel.NewDefaultNetClientChannel()
		c.netClientConnectFunc = c.unsafe.ConnectFunc
	}

	c.pipeline = channel._NewDefaultPipeline(c)
	c.unsafe.ConnectFunc = c.connect
	c.unsafe.WriteFunc = c.write
	c.bufferSize = channel.GetParamIntDefault(c, ParamReadBufferSize, 1024)
	c.readTimeout = channel.GetParamIntDefault(c, ParamReadTimeout, 6000)
	c.writeTimeout = channel.GetParamIntDefault(c, ParamWriteTimeout, 3000)
	return c
}

func (c *DefaultTCPClientChannel) write(obj interface{}) error {
	if !c.IsActive() {
		return net.ErrClosed
	}

	c.WriteLock.Lock()
	defer c.WriteLock.Unlock()
	var bs []byte
	switch v := obj.(type) {
	case buf.ByteBuf:
		bs = v.Bytes()
	default:
		kklogger.ErrorJ("DefaultTCPClientChannel.write", UnknownObjectType)
		return UnknownObjectType
	}

	if _, err := c.Conn().Write(bs); err != nil {
		kklogger.ErrorJ("DefaultTCPClientChannel.write", err.Error())
		return err
	}

	return nil
}

func (c *DefaultTCPClientChannel) read() {
	defer kkpanic.Call(func(r kkpanic.Caught) {
		c.Disconnect()
	})

	for c.IsActive() {
		if c.Conn() == nil {
			c.Disconnect()
			return
		}

		bs := make([]byte, c.bufferSize)
		if rl, err := c.Conn().Read(bs); err != nil {
			if c.IsActive() {
				if err != io.EOF {
					kklogger.WarnJ("DefaultTCPClientChannel.read", err.Error())
				}

				c.Disconnect()
			} else if err == io.EOF {
				c.Disconnect()
			}
		} else {
			kkpanic.Catch(func() {
				c.FireRead(buf.NewByteBuf(bs[:rl]))
				c.FireReadCompleted()
			}, func(r kkpanic.Caught) {
				kklogger.ErrorJ("DefaultTCPClientChannel.read", r.String())
			})
		}
	}
}

func (c *DefaultTCPClientChannel) connect(remoteAddr net.Addr) error {
	if _, ok := remoteAddr.(*net.TCPAddr); !ok {
		return ErrNotTCPAddr
	}

	if err := c.netClientConnectFunc(remoteAddr); err != nil {
		return err
	}

	go c.read()
	return nil
}
