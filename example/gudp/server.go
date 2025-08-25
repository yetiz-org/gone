package gudp

import (
	"net"
	"time"

	"github.com/yetiz-org/gone/channel"
	"github.com/yetiz-org/gone/gudp"
)

type Server struct {
}

func (s *Server) Start(localAddr net.Addr) {
	bootstrap := channel.NewServerBootstrap()
	bootstrap.ChannelType(&gudp.ServerChannel{})
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
