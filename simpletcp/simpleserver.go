package simpletcp

import (
	"net"

	"github.com/yetiz-org/gone/channel"
	"github.com/yetiz-org/gone/tcp"
)

type Server struct {
	ch      channel.Channel
	Handler channel.Handler
}

func NewServer(handler channel.Handler) *Server {
	return &Server{
		Handler: handler,
	}
}

func (s *Server) Start(localAddr net.Addr) channel.Channel {
	bootstrap := channel.NewServerBootstrap()
	bootstrap.ChannelType(&tcp.ServerChannel{})
	bootstrap.ChildHandler(channel.NewInitializer(func(ch channel.Channel) {
		ch.Pipeline().AddLast("SIMPLE_CODEC", NewSimpleCodec())
		ch.Pipeline().AddLast("HANDLER", &serverHandlerAdapter{server: s})
	}))

	s.ch = bootstrap.Bind(localAddr).Sync().Channel()
	return s.ch
}

func (s *Server) Channel() channel.Channel {
	return s.ch
}

func (s *Server) Stop() channel.Future {
	return s.ch.Close()
}

type serverHandlerAdapter struct {
	channel.DefaultHandler
	server *Server
}

func (h *serverHandlerAdapter) Added(ctx channel.HandlerContext) {
	if h.server.Handler != nil {
		h.server.Handler.Added(ctx)
	}
}

func (h *serverHandlerAdapter) Removed(ctx channel.HandlerContext) {
	if h.server.Handler != nil {
		h.server.Handler.Removed(ctx)
	}
}

func (h *serverHandlerAdapter) Registered(ctx channel.HandlerContext) {
	if h.server.Handler != nil {
		h.server.Handler.Registered(ctx)
	} else {
		ctx.FireRegistered()
	}
}

func (h *serverHandlerAdapter) Unregistered(ctx channel.HandlerContext) {
	if h.server.Handler != nil {
		h.server.Handler.Unregistered(ctx)
	} else {
		ctx.FireUnregistered()
	}
}

func (h *serverHandlerAdapter) Active(ctx channel.HandlerContext) {
	if h.server.Handler != nil {
		h.server.Handler.Active(ctx)
	} else {
		ctx.FireActive()
	}
}

func (h *serverHandlerAdapter) Inactive(ctx channel.HandlerContext) {
	if h.server.Handler != nil {
		h.server.Handler.Inactive(ctx)
	} else {
		ctx.FireInactive()
	}
}

func (h *serverHandlerAdapter) Read(ctx channel.HandlerContext, obj any) {
	if h.server.Handler != nil {
		h.server.Handler.Read(ctx, obj)
	}
}

func (h *serverHandlerAdapter) ReadCompleted(ctx channel.HandlerContext) {
	if h.server.Handler != nil {
		h.server.Handler.ReadCompleted(ctx)
	}
}

func (h *serverHandlerAdapter) Write(ctx channel.HandlerContext, obj any, future channel.Future) {
	if h.server.Handler != nil {
		h.server.Handler.Write(ctx, obj, future)
	} else {
		ctx.Write(obj, future)
	}
}

func (h *serverHandlerAdapter) Bind(ctx channel.HandlerContext, localAddr net.Addr, future channel.Future) {
	if h.server.Handler != nil {
		h.server.Handler.Bind(ctx, localAddr, future)
	} else {
		ctx.Bind(localAddr, future)
	}
}

func (h *serverHandlerAdapter) Close(ctx channel.HandlerContext, future channel.Future) {
	if h.server.Handler != nil {
		h.server.Handler.Close(ctx, future)
	} else {
		ctx.Close(future)
	}
}

func (h *serverHandlerAdapter) Connect(ctx channel.HandlerContext, localAddr net.Addr, remoteAddr net.Addr, future channel.Future) {
	if h.server.Handler != nil {
		h.server.Handler.Connect(ctx, localAddr, remoteAddr, future)
	} else {
		ctx.Connect(localAddr, remoteAddr, future)
	}
}

func (h *serverHandlerAdapter) Disconnect(ctx channel.HandlerContext, future channel.Future) {
	if h.server.Handler != nil {
		h.server.Handler.Disconnect(ctx, future)
	} else {
		ctx.Disconnect(future)
	}
}

func (h *serverHandlerAdapter) Deregister(ctx channel.HandlerContext, future channel.Future) {
	if h.server.Handler != nil {
		h.server.Handler.Deregister(ctx, future)
	} else {
		ctx.Deregister(future)
	}
}

func (h *serverHandlerAdapter) ErrorCaught(ctx channel.HandlerContext, err error) {
	if h.server.Handler != nil {
		h.server.Handler.ErrorCaught(ctx, err)
	} else {
		ctx.FireErrorCaught(err)
	}
}
