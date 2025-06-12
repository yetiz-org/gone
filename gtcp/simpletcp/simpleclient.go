package simpletcp

import (
	"github.com/yetiz-org/gone/gtcp"
	"net"
	"time"

	"github.com/yetiz-org/gone/channel"
	buf "github.com/yetiz-org/goth-bytebuf"
)

type Client struct {
	AutoReconnect func() bool
	Handler       channel.Handler
	bootstrap     channel.Bootstrap
	remoteAddr    net.Addr
	ch            channel.Channel
	close         bool
}

func NewClient(handler channel.Handler) *Client {
	return &Client{
		Handler: handler,
	}
}

func (c *Client) Start(remoteAddr net.Addr) channel.Channel {
	c.bootstrap = channel.NewBootstrap()
	c.bootstrap.ChannelType(&gtcp.Channel{})
	c.bootstrap.Handler(channel.NewInitializer(func(ch channel.Channel) {
		ch.Pipeline().AddLast("SIMPLE_CODEC", NewSimpleCodec())
		ch.Pipeline().AddLast("RECONNECT", &connectionHandler{client: c})
		ch.Pipeline().AddLast("HANDLER", &clientHandlerAdapter{client: c})
	}))

	c.remoteAddr = remoteAddr
	return c.start()
}

func (c *Client) start() channel.Channel {
	c.ch = c.bootstrap.Connect(nil, c.remoteAddr).Sync().Channel()
	return c.ch
}

func (c *Client) Channel() channel.Channel {
	return c.ch
}

func (c *Client) Write(buf buf.ByteBuf) channel.Future {
	return c.ch.Write(buf)
}

func (c *Client) Disconnect() channel.Future {
	c.close = true
	return c.ch.Disconnect()
}

type connectionHandler struct {
	channel.DefaultHandler
	client *Client
}

func (h *connectionHandler) Active(ctx channel.HandlerContext) {
	go func(ch channel.Channel) {
		wait := time.Second * 5
		for ch.IsActive() {
			ch.Write(buf.EmptyByteBuf())
			time.Sleep(wait)
		}

	}(ctx.Channel())

	ctx.FireActive()
}

func (h *connectionHandler) Unregistered(ctx channel.HandlerContext) {
	if !h.client.close && h.client.AutoReconnect != nil {
		if h.client.AutoReconnect() {
			h.client.start()
		} else {
			h.client.close = true
		}
	}

	ctx.FireUnregistered()
}

type clientHandlerAdapter struct {
	channel.DefaultHandler
	client *Client
}

func (h *clientHandlerAdapter) Added(ctx channel.HandlerContext) {
	if h.client.Handler != nil {
		h.client.Handler.Added(ctx)
	}
}

func (h *clientHandlerAdapter) Removed(ctx channel.HandlerContext) {
	if h.client.Handler != nil {
		h.client.Handler.Removed(ctx)
	}
}

func (h *clientHandlerAdapter) Registered(ctx channel.HandlerContext) {
	if h.client.Handler != nil {
		h.client.Handler.Registered(ctx)
	} else {
		ctx.FireRegistered()
	}
}

func (h *clientHandlerAdapter) Unregistered(ctx channel.HandlerContext) {
	if h.client.Handler != nil {
		h.client.Handler.Unregistered(ctx)
	} else {
		ctx.FireUnregistered()
	}
}

func (h *clientHandlerAdapter) Active(ctx channel.HandlerContext) {
	if h.client.Handler != nil {
		h.client.Handler.Active(ctx)
	} else {
		ctx.FireActive()
	}
}

func (h *clientHandlerAdapter) Inactive(ctx channel.HandlerContext) {
	if h.client.Handler != nil {
		h.client.Handler.Inactive(ctx)
	} else {
		ctx.FireInactive()
	}
}

func (h *clientHandlerAdapter) Read(ctx channel.HandlerContext, obj any) {
	if h.client.Handler != nil {
		h.client.Handler.Read(ctx, obj)
	}
}

func (h *clientHandlerAdapter) ReadCompleted(ctx channel.HandlerContext) {
	if h.client.Handler != nil {
		h.client.Handler.ReadCompleted(ctx)
	}
}

func (h *clientHandlerAdapter) Write(ctx channel.HandlerContext, obj any, future channel.Future) {
	if h.client.Handler != nil {
		h.client.Handler.Write(ctx, obj, future)
	} else {
		ctx.Write(obj, future)
	}
}

func (h *clientHandlerAdapter) Bind(ctx channel.HandlerContext, localAddr net.Addr, future channel.Future) {
	if h.client.Handler != nil {
		h.client.Handler.Bind(ctx, localAddr, future)
	} else {
		ctx.Bind(localAddr, future)
	}
}

func (h *clientHandlerAdapter) Close(ctx channel.HandlerContext, future channel.Future) {
	if h.client.Handler != nil {
		h.client.Handler.Close(ctx, future)
	} else {
		ctx.Close(future)
	}
}

func (h *clientHandlerAdapter) Connect(ctx channel.HandlerContext, localAddr net.Addr, remoteAddr net.Addr, future channel.Future) {
	if h.client.Handler != nil {
		h.client.Handler.Connect(ctx, localAddr, remoteAddr, future)
	} else {
		ctx.Connect(localAddr, remoteAddr, future)
	}
}

func (h *clientHandlerAdapter) Disconnect(ctx channel.HandlerContext, future channel.Future) {
	if h.client.Handler != nil {
		h.client.Handler.Disconnect(ctx, future)
	} else {
		ctx.Disconnect(future)
	}
}

func (h *clientHandlerAdapter) Deregister(ctx channel.HandlerContext, future channel.Future) {
	if h.client.Handler != nil {
		h.client.Handler.Deregister(ctx, future)
	} else {
		ctx.Deregister(future)
	}
}

func (h *clientHandlerAdapter) ErrorCaught(ctx channel.HandlerContext, err error) {
	if h.client.Handler != nil {
		h.client.Handler.ErrorCaught(ctx, err)
	} else {
		ctx.FireErrorCaught(err)
	}
}
