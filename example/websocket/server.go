package websocket

import (
	"net"
	"reflect"
	"time"

	"github.com/kklab-com/gone/channel"
	"github.com/kklab-com/gone/http"
	"github.com/kklab-com/gone/websocket"
)

type Server struct {
	ch channel.Channel
}

func (k *Server) Start(localAddr net.Addr) {
	upgrader := &websocket.UpgradeHandler{}
	bootstrap := channel.NewServerBootstrap()
	bootstrap.ChannelType(reflect.TypeOf(http.DefaultServerChannel{}))
	bootstrap.SetParams(websocket.ParamCheckOrigin, false)
	bootstrap.ChildHandler(channel.NewInitializer(func(ch channel.Channel) {
		ch.Pipeline().AddLast("DISPATCHER", http.NewDispatchHandler(NewRoute())).
			AddLast("WS_UPGRADE", upgrader).
			AddLast("WS_INVOKER", new(websocket.InvokeHandler))
	}))

	k.ch = bootstrap.Bind(localAddr).Sync().Channel()
	go func() {
		time.Sleep(time.Minute * 1)
		k.ch.Close()
	}()

	k.ch.CloseFuture().Sync()
	time.Sleep(time.Second * 1)
}
