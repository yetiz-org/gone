package gtcp

import (
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/yetiz-org/gone/channel"
	"github.com/yetiz-org/goth-kklogger"
)

type ServerChannel struct {
	channel.DefaultNetServerChannel
	listen      net.Listener
	mu          sync.Mutex
	active      atomic.Bool
	acceptDelay time.Duration // touched only by the accept loop
}

var ErrBindTwice = fmt.Errorf("bind twice")

func (c *ServerChannel) UnsafeBind(localAddr net.Addr) (rtErr error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.Name == "" {
		c.Name = fmt.Sprintf("TCPSERV_%s", localAddr.String())
	}

	if c.IsActive() {
		err := fmt.Errorf("%s: %w", c.Name, ErrBindTwice)
		kklogger.ErrorJ("gtcp:ServerChannel.UnsafeBind#unsafe_bind!bind_twice", err.Error())
		return err
	}

	if listen, err := net.Listen("tcp4", localAddr.String()); err != nil {
		kklogger.ErrorJ("gtcp:ServerChannel.UnsafeBind#unsafe_bind!bind_error", fmt.Sprintf("bind at %s fail %s", localAddr.String(), err.Error()))
		return err
	} else {
		c.listen = channel.LimitListener(listen, int(channel.GetParamInt64Default(c, channel.ParamMaxConnections, channel.DefaultMaxConnections)))
		c.active.Store(true)
	}

	return nil
}

func (c *ServerChannel) UnsafeAccept() (child channel.Channel, future channel.Future) {
	c.mu.Lock()
	listen := c.listen
	c.mu.Unlock()

	if listen == nil {
		return nil, c.Pipeline().NewFuture()
	}

	if conn, err := listen.Accept(); err != nil {
		if !c.IsActive() {
			return nil, c.Pipeline().NewFuture()
		}

		// Back off like net/http, so an error such as running out of descriptors does not spin the loop.
		c.acceptDelay = min(max(2*c.acceptDelay, 5*time.Millisecond), time.Second)
		kklogger.ErrorJ("gtcp:ServerChannel.UnsafeAccept#unsafe_accept!accept_error", fmt.Sprintf("%s; retrying in %s", err, c.acceptDelay))
		time.Sleep(c.acceptDelay)
		return nil, c.Pipeline().NewFuture()
	} else {
		c.acceptDelay = 0
		ch := c.DeriveNetChildChannel(&Channel{}, c, conn)
		return ch, ch.Pipeline().NewFuture()
	}
}

func (c *ServerChannel) UnsafeClose() error {
	c.DefaultNetServerChannel.UnsafeClose()

	c.mu.Lock()
	c.active.Store(false)
	listen := c.listen
	c.listen = nil
	c.mu.Unlock()

	if listen != nil {
		return listen.Close()
	}
	return nil
}

func (c *ServerChannel) IsActive() bool {
	return c.active.Load()
}
