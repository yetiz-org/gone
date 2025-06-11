package tcp

import (
	"fmt"
	"net"

	"github.com/pkg/errors"
	"github.com/yetiz-org/gone/channel"
	"github.com/yetiz-org/goth-kklogger"
)

type ServerChannel struct {
	channel.DefaultNetServerChannel
	listen net.Listener
	active bool
}

var ErrBindTwice = fmt.Errorf("bind twice")

func (c *ServerChannel) UnsafeBind(localAddr net.Addr) error {
	if c.Name == "" {
		c.Name = fmt.Sprintf("TCPSERV_%s", localAddr.String())
	}

	if c.IsActive() {
		err := errors.Wrap(ErrBindTwice, c.Name)
		kklogger.Error("ServerChannel.UnsafeBind", err)
		return err
	}

	if listen, err := net.Listen("tcp4", localAddr.String()); err != nil {
		kklogger.ErrorJ("ServerChannel.UnsafeBind", fmt.Sprintf("bind at %s fail %s", localAddr.String(), err.Error()))
		return err
	} else {
		c.listen = listen
		c.active = true
	}

	return nil
}

func (c *ServerChannel) UnsafeAccept() (channel.Channel, channel.Future) {
	if conn, err := c.listen.Accept(); err != nil {
		if !c.IsActive() {
			return nil, c.Pipeline().NewFuture()
		}

		kklogger.ErrorJ("tcp:ServerChannel.UnsafeAccept", err.Error())
		return nil, c.Pipeline().NewFuture()
	} else {
		ch := c.DeriveNetChildChannel(&Channel{}, c, conn)
		return ch, ch.Pipeline().NewFuture()
	}
}

func (c *ServerChannel) UnsafeClose() error {
	c.DefaultNetServerChannel.UnsafeClose()
	c.active = false
	return c.listen.Close()
}

func (c *ServerChannel) IsActive() bool {
	return c.active
}
