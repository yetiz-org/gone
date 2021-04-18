package tcp

import (
	"fmt"
	"net"

	"github.com/kklab-com/gone/channel"
)

type Channel struct {
	channel.DefaultNetChannel
	bufferSize   int
	readTimeout  int
	writeTimeout int
}

var ErrNotTCPAddr = fmt.Errorf("not tcp addr")

func (c *Channel) Init() channel.Channel {
	c.bufferSize = channel.GetParamIntDefault(c, ParamReadBufferSize, 1024)
	c.readTimeout = channel.GetParamIntDefault(c, ParamReadTimeout, 6000)
	c.writeTimeout = channel.GetParamIntDefault(c, ParamWriteTimeout, 3000)
	return c
}

func (c *Channel) UnsafeConnect(localAddr net.Addr, remoteAddr net.Addr) error {
	if remoteAddr == nil {
		return channel.ErrNilObject
	}

	if _, ok := remoteAddr.(*net.TCPAddr); !ok {
		return ErrNotTCPAddr
	}

	if localAddr != nil {
		if _, ok := localAddr.(*net.TCPAddr); !ok {
			return ErrNotTCPAddr
		}
	}

	return c.DefaultNetChannel.UnsafeConnect(localAddr, remoteAddr)
}
