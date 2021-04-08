package http

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/kklab-com/gone/channel"
	"github.com/kklab-com/goth-kklogger"
	kkpanic "github.com/kklab-com/goth-panic"
)

type DefaultServerChannel struct {
	channel.DefaultNetServerChannel
	server *http.Server
	active bool
}

const ConnCtx = "conn"
const ClientChannelCtx = "client_channel"

func (c *DefaultServerChannel) Init() channel.Channel {
	c.ChannelPipeline = channel.NewDefaultPipeline(c)
	c.Unsafe.BindFunc = c.bind
	c.Unsafe.CloseFunc = c.close
	c.Unsafe.CloseLock.Lock()
	return c
}

func (c *DefaultServerChannel) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer c.panicCatch()
	conn := r.Context().Value(ConnCtx)
	if conn == nil {
		kklogger.ErrorJ("DefaultServerChannel.ServeHTTP", "can't get conn")
		return
	}

	cch := r.Context().Value(ClientChannelCtx).(*DefaultClientChannel)
	if cch == nil {
		kklogger.ErrorJ("DefaultServerChannel.ServeHTTP", "can't get DefaultClientChannel")
		return
	}

	cch.writer = w
	request := NewRequest(cch, *r)
	var pkg = &Pack{
		Req:    request,
		Resp:   NewResponse(request),
		Params: map[string]interface{}{},
		Writer: w,
	}

	var obj interface{} = pkg
	cch.FireRead(obj)
}

func (c *DefaultServerChannel) panicCatch() {
	kkpanic.Call(func(r *kkpanic.Caught) {
		kklogger.ErrorJ("ServerChannelPanicCatch", r.String())
	})
}

func (c *DefaultServerChannel) bind(localAddr net.Addr) error {
	var handler http.Handler = c
	if c.Name == "" {
		c.Name = fmt.Sprintf("SERVER_%s", localAddr.String())
	}

	if c.active {
		kklogger.Error("DefaultServerChannel.bind", fmt.Sprintf("%s bind twice", c.Name))
		os.Exit(1)
	}

	c.server = &http.Server{
		Addr:              localAddr.String(),
		Handler:           handler,
		IdleTimeout:       time.Second * time.Duration(channel.GetParamInt64Default(c, ParamIdleTimeout, 60)),
		ReadTimeout:       time.Second * time.Duration(channel.GetParamInt64Default(c, ParamReadTimeout, 60)),
		ReadHeaderTimeout: time.Second * time.Duration(channel.GetParamInt64Default(c, ParamReadHeaderTimeout, 60)),
		WriteTimeout:      time.Second * time.Duration(channel.GetParamInt64Default(c, ParamWriteTimeout, 60)),
		MaxHeaderBytes:    channel.GetParamIntDefault(c, ParamMaxHeaderBytes, 1024*1024*4),
		ConnState: func(conn net.Conn, state http.ConnState) {
			switch state {
			case http.StateNew:
			case http.StateActive:
			case http.StateIdle:
			case http.StateHijacked:
			case http.StateClosed:
				if ncc := c.Abandon(conn); ncc != nil {
					ncc.Disconnect()
				}
			default:
			}
		},
		ConnContext: func(ctx context.Context, conn net.Conn) context.Context {
			cch := c._NewClientChannel(conn)
			cch.SetParam(ParamMaxMultiPartMemory, MaxMultiPartMemory)
			cch.Init()
			cch.Pipeline().AddLast("", c.ChildHandler())
			ctx = context.WithValue(ctx, ConnCtx, conn)
			ctx = context.WithValue(ctx, ClientChannelCtx, cch)
			return ctx
		},
	}

	c.active = true
	go c.server.ListenAndServe()
	return nil
}

func (c *DefaultServerChannel) close() error {
	shutdownTimeout, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	if err := c.server.Shutdown(shutdownTimeout); err != nil {
		kklogger.ErrorJ("HttpServerChannel", err.Error())
	}

	c.Unsafe.CloseLock.Unlock()
	c.active = false
	return nil
}

func (c *DefaultServerChannel) _NewClientChannel(conn net.Conn) *DefaultClientChannel {
	if conn == nil {
		return nil
	}

	cc := &DefaultClientChannel{
		DefaultNetClientChannel: c.DeriveNetClientChannel(conn),
	}

	cc.Name = conn.RemoteAddr().String()
	c.Params().Range(func(k channel.ParamKey, v interface{}) bool {
		cc.SetParam(k, v)
		return true
	})

	return cc
}

func (c *DefaultServerChannel) IsActive() bool {
	return c.active
}
