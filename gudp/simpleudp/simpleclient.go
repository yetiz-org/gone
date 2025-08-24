package simpleudp

import (
	"net"
	"time"

	"github.com/yetiz-org/gone/gudp"

	"github.com/yetiz-org/gone/channel"
	buf "github.com/yetiz-org/goth-bytebuf"
)

// Client represents a simple UDP client implementation
type Client struct {
	AutoReconnect func() bool
	Handler       channel.Handler
	bootstrap     channel.Bootstrap
	remoteAddr    net.Addr
	ch            channel.Channel
	close         bool
}

// NewClient creates a new simple UDP client with the specified handler
func NewClient(handler channel.Handler) *Client {
	return &Client{
		Handler: handler,
	}
}

// Start initializes and starts the UDP client connection
func (c *Client) Start(remoteAddr net.Addr) channel.Channel {
	c.bootstrap = channel.NewBootstrap()
	c.bootstrap.ChannelType(&gudp.Channel{})
	c.bootstrap.Handler(channel.NewInitializer(func(ch channel.Channel) {
		ch.Pipeline().AddLast("SIMPLE_CODEC", NewSimpleCodec())
		ch.Pipeline().AddLast("RECONNECT", &connectionHandler{client: c})
		ch.Pipeline().AddLast("HANDLER", &clientHandlerAdapter{client: c})
	}))

	c.remoteAddr = remoteAddr
	return c.start()
}

// start establishes the UDP connection
func (c *Client) start() channel.Channel {
	if c.bootstrap == nil {
		return nil // Return nil if bootstrap is not initialized
	}
	c.ch = c.bootstrap.Connect(nil, c.remoteAddr).Sync().Channel()
	return c.ch
}

// Channel returns the underlying channel
func (c *Client) Channel() channel.Channel {
	return c.ch
}

// Write sends data through the UDP connection
func (c *Client) Write(buf buf.ByteBuf) channel.Future {
	return c.ch.Write(buf)
}

// Disconnect closes the UDP connection
func (c *Client) Disconnect() channel.Future {
	c.close = true
	return c.ch.Disconnect()
}

// connectionHandler handles UDP connection lifecycle events
type connectionHandler struct {
	channel.DefaultHandler
	client *Client
}

// Active is called when the UDP connection becomes active
func (h *connectionHandler) Active(ctx channel.HandlerContext) {
	// UDP doesn't need keep-alive like TCP, but we can implement periodic communication if needed
	go func(ch channel.Channel) {
		wait := time.Second * 30 // Less frequent for UDP
		for ch.IsActive() {
			// For UDP, we might want to send periodic heartbeat messages
			// This is optional and depends on application requirements
			time.Sleep(wait)
		}
	}(ctx.Channel())

	ctx.FireActive()
}

// Unregistered handles UDP connection cleanup and reconnection logic
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

// clientHandlerAdapter adapts user handlers to the UDP client framework
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
