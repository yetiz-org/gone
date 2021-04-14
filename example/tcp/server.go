package tcp

import (
	"net"
	"reflect"
	"time"

	"github.com/kklab-com/gone/channel"
	"github.com/kklab-com/gone/tcp"
)

type Server struct {
}

func (k *Server) Start(localAddr net.Addr) {
	bootstrap := channel.NewServerBootstrap()
	bootstrap.ChannelType(reflect.TypeOf(tcp.DefaultTCPServerChannel{}))
	bootstrap.ChildHandler(channel.NewInitializer(func(ch channel.Channel) {
		ch.Pipeline().AddLast("DECODE_HANDLER", NewDecodeHandler())
		ch.Pipeline().AddLast("HANDLER", &ServerChildHandler{})
	}))

	ch := bootstrap.Bind(localAddr).Sync().Channel()
	go func() {
		time.Sleep(time.Minute * 1)
		ch.Close()
	}()

	ch.CloseFuture().Sync()
}
