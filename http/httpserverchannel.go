package http

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"reflect"
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

var ClientChannelType = reflect.TypeOf(DefaultClientChannel{})

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

	cch := c.Child(conn.(net.Conn)).(*DefaultClientChannel)
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
	cch.FireReadCompleted()
}

func (c *DefaultServerChannel) panicCatch() {
	kkpanic.Call(func(r kkpanic.Caught) {
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
				cch := c.DeriveClientChannel(ClientChannelType, conn)
				cch.SetParam(ParamMaxMultiPartMemory, MaxMultiPartMemory)
			case http.StateActive:
			case http.StateIdle:
			case http.StateHijacked:
			case http.StateClosed:
				c.Abandon(c.Child(conn).Conn().Conn())
			default:
			}
		},
		ConnContext: func(ctx context.Context, conn net.Conn) context.Context {
			ctx = context.WithValue(ctx, ConnCtx, conn)
			return ctx
		},
	}

	c.active = true
	go c.server.ListenAndServe()
	return nil
}

func (c *DefaultServerChannel) close() error {
	if !c.active {
		return nil
	}

	shutdownTimeout, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	if err := c.server.Shutdown(shutdownTimeout); err != nil {
		kklogger.ErrorJ("HttpServerChannel", err.Error())
	}

	c.Unsafe.CloseLock.Unlock()
	c.active = false
	return nil
}

func (c *DefaultServerChannel) IsActive() bool {
	return c.active
}
