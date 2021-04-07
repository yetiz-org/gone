package tcp

import (
	"bytes"
	"fmt"
	"net"

	"github.com/kklab-com/gone/channel"
	kklogger "github.com/kklab-com/goth-kklogger"
)

type DefaultTCPClientChannel struct {
	channel.DefaultNetClientChannel
	conn net.Conn
}

var UnknownObjectType = fmt.Errorf("unknown object type")

func (c *DefaultTCPClientChannel) Init() channel.Channel {
	c.ChannelPipeline = channel.NewDefaultPipeline(c)
	c.Unsafe.WriteFunc = c.write
	return c
}

func (c *DefaultTCPClientChannel) write(obj interface{}) error {
	var bs []byte
	switch v := obj.(type) {
	case bytes.Buffer:
		bs = v.Bytes()
	case []byte:
		bs = v
	default:
		kklogger.ErrorJ("DefaultTCPClientChannel.write", UnknownObjectType)
		return UnknownObjectType
	}

	if _, err := c.conn.Write(bs); err != nil {
		kklogger.ErrorJ("DefaultTCPClientChannel.write", err.Error())
		return err
	}

	return nil
}
