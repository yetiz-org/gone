package tcp

import (
	"bytes"
	"fmt"
	"io"

	"github.com/kklab-com/gone/channel"
	kklogger "github.com/kklab-com/goth-kklogger"
	kkpanic "github.com/kklab-com/goth-panic"
)

type DefaultTCPClientChannel struct {
	*channel.DefaultNetClientChannel
	bufferSize   int
	readTimeout  int
	writeTimeout int
}

var UnknownObjectType = fmt.Errorf("unknown object type")

func (c *DefaultTCPClientChannel) Init() channel.Channel {
	c.ChannelPipeline = channel.NewDefaultPipeline(c)
	c.Unsafe.WriteFunc = c.write
	c.bufferSize = channel.GetParamIntDefault(c, ParamReadBufferSize, 1024)
	c.readTimeout = channel.GetParamIntDefault(c, ParamReadTimeout, 6000)
	c.writeTimeout = channel.GetParamIntDefault(c, ParamWriteTimeout, 3000)
	return c
}

func (c *DefaultTCPClientChannel) write(obj interface{}) error {
	var bs []byte
	switch v := obj.(type) {
	case *bytes.Buffer:
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
	for c.IsActive() {
		if c.Conn() == nil {
			c.Disconnect()
			return
		}

		bs := make([]byte, c.bufferSize)
		if rl, err := c.Conn().Read(bs); err != nil {
			if err != io.EOF {
				kklogger.WarnJ("DefaultTCPClientChannel.read", err.Error())
			}

			c.Disconnect()
		} else {
			kkpanic.Catch(func() {
				c.FireRead(bytes.NewBuffer(bs[:rl]))
				c.FireReadCompleted()
			}, func(r *kkpanic.Caught) {
				kklogger.ErrorJ("DefaultTCPClientChannel.read", r.String())
			})
		}
	}
}
