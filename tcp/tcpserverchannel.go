package tcp

import (
	"fmt"
	"net"
	"os"

	"github.com/kklab-com/gone/channel"
	"github.com/kklab-com/goth-kklogger"
)

type DefaultTCPServerChannel struct {
	channel.DefaultNetServerChannel
	listen net.Listener
	active bool
}

func (c *DefaultTCPServerChannel) Init() channel.Channel {
	c.ChannelPipeline = channel.NewDefaultPipeline(c)
	c.Unsafe.BindFunc = c.bind
	c.Unsafe.CloseFunc = c.close
	c.Unsafe.CloseLock.Lock()
	return c
}

func (c *DefaultTCPServerChannel) bind(localAddr net.Addr) error {
	if c.Name == "" {
		c.Name = fmt.Sprintf("TCPSERV_%s", localAddr.String())
	}

	if c.active {
		kklogger.Error("DefaultTCPServerChannel.bind", fmt.Sprintf("%s bind twice", c.Name))
		os.Exit(1)
	}

	if listen, err := net.Listen("tcp4", localAddr.String()); err != nil {
		kklogger.ErrorJ("DefaultTCPServerChannel.bind", fmt.Sprintf("bind fail, %s", err.Error()))
		return err
	} else {
		c.active = true
		c.listen = listen
	}

	go c.acceptLoop()
	return nil
}

func (c *DefaultTCPServerChannel) acceptLoop() {
	for c.active {
		if accept, err := c.listen.Accept(); err != nil {
			if !c.active {
				return
			}

			kklogger.ErrorJ("DefaultTCPServerChannel.acceptLoop", err.Error())
		} else {
			cch := c._NewClientChannel(accept)
			cch.Init()
			cch.Pipeline().AddLast("", c.ChildHandler())
		}
	}
}

func (c *DefaultTCPServerChannel) close() error {
	c.active = false
	c.listen.Close()
	c.Unsafe.CloseLock.Unlock()
	return nil
}

func (c *DefaultTCPServerChannel) _NewClientChannel(conn net.Conn) *DefaultTCPClientChannel {
	if conn == nil {
		return nil
	}

	cc := DefaultTCPClientChannel{
		DefaultNetClientChannel: *c.DeriveNetClientChannel(conn),
	}

	cc.Name = conn.RemoteAddr().String()
	c.Params().Range(func(k channel.ParamKey, v interface{}) bool {
		cc.SetParam(k, v)
		return true
	})

	return &cc
}

func (c *DefaultTCPServerChannel) IsActive() bool {
	return c.active
}
