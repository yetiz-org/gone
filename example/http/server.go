package http

import (
	"net"
	"time"

	"github.com/kklab-com/gone/channel"
	"github.com/kklab-com/gone/http"
)

type Server struct {
}

func (k *Server) Start(localAddr net.Addr) {
	bootstrap := channel.NewServerBootstrap()
	bootstrap.ChannelType(&http.ServerChannel{})
	bootstrap.Handler(channel.NewInitializer(func(ch channel.Channel) {
		ch.Pipeline().AddLast("INDICATE_HANDLER_INBOUND", &channel.IndicateHandlerInbound{})
		ch.Pipeline().AddLast("INDICATE_HANDLER_OUTBOUND", &channel.IndicateHandlerOutbound{})
	}))

	bootstrap.ChildHandler(channel.NewInitializer(func(ch channel.Channel) {
		ch.Pipeline().AddLast("INDICATE_HANDLER_INBOUND", &channel.IndicateHandlerInbound{})
		ch.Pipeline().AddLast("GZIP_HANDLER", new(http.GZipHandler))
		ch.Pipeline().AddLast("LOG_HANDLER", http.NewLogHandler(false))
		ch.Pipeline().AddLast("DISPATCHER", http.NewDispatchHandler(NewRoute()))
		ch.Pipeline().AddLast("INDICATE_HANDLER_OUTBOUND", &channel.IndicateHandlerOutbound{})
	}))

	ch := bootstrap.Bind(localAddr).Sync().Channel()
	go func() {
		time.Sleep(time.Second * 3)
		ch.Close()
	}()

	ch.CloseFuture().Sync()
}
