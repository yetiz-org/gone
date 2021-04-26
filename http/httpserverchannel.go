package http

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/kklab-com/gone/channel"
	"github.com/kklab-com/goth-kklogger"
	kkpanic "github.com/kklab-com/goth-panic"
)

type ServerChannel struct {
	channel.DefaultNetServerChannel
	server    *http.Server
	active    bool
	newChChan chan channel.Channel
	chMap     sync.Map
}

const ConnCtx = "conn"
const ConnChCtx = "conn_ch"

func (c *ServerChannel) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer c.panicCatch()
	conn := r.Context().Value(ConnCtx)
	if conn == nil {
		kklogger.ErrorJ("http:ServerChannel.ServeHTTP", "can't get conn")
		return
	}

	cch := r.Context().Value(ConnChCtx).(*Channel)
	if cch == nil {
		kklogger.ErrorJ("http:ServerChannel.ServeHTTP", "can't get Channel")
		return
	}

	cch.writer = w
	request := WrapRequest(cch, r)
	var pkg = &Pack{
		Request:  request,
		Response: NewResponse(request),
		Params:   map[string]interface{}{},
		Writer:   w,
	}

	var obj interface{} = pkg
	cch.FireRead(obj)
	cch.FireReadCompleted()
}

func (c *ServerChannel) panicCatch() {
	kkpanic.Call(func(r kkpanic.Caught) {
		kklogger.ErrorJ("http:ServerChannelPanicCatch", r.String())
	})
}

func (c *ServerChannel) UnsafeBind(localAddr net.Addr) error {
	var handler http.Handler = c
	if c.Name == "" {
		c.Name = fmt.Sprintf("SERVER_%s", localAddr.String())
	}

	if c.active {
		kklogger.Error("http:ServerChannel.bind", fmt.Sprintf("%s bind twice", c.Name))
		os.Exit(1)
	}

	c.newChChan = make(chan channel.Channel, channel.GetParamIntDefault(c, ParamAcceptWaitCount, 1024))
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
				c.chMap.Delete(conn)
			case http.StateClosed:
				if v, f := c.chMap.LoadAndDelete(conn); f {
					ch := v.(channel.Channel)
					if ch.IsActive() {
						ch.Deregister()
					}
				}
			default:
			}
		},
		ConnContext: func(ctx context.Context, conn net.Conn) context.Context {
			ch := &Channel{}
			ch.SetParam(ParamMaxMultiPartMemory, MaxMultiPartMemory)
			c.DeriveNetChildChannel(ch, c, conn)
			ctx = context.WithValue(ctx, ConnCtx, conn)
			ctx = context.WithValue(ctx, ConnChCtx, ch)
			c.newChChan <- ch
			c.chMap.Store(conn, ch)
			return ctx
		},
	}

	c.active = true
	go c.server.ListenAndServe()
	return nil
}

func (c *ServerChannel) UnsafeAccept() channel.Channel {
	return <-c.newChChan
}

func (c *ServerChannel) UnsafeClose() error {
	if !c.active {
		return nil
	}

	c.DefaultNetServerChannel.UnsafeClose()
	shutdownTimeout, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	if err := c.server.Shutdown(shutdownTimeout); err != nil {
		kklogger.ErrorJ("http:ServerChannel#UnsafeClose", err.Error())
	}

	c.active = false
	return nil
}

func (c *ServerChannel) IsActive() bool {
	return c.active
}
