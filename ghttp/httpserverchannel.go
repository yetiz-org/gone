package ghttp

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/yetiz-org/gone/channel"
	"github.com/yetiz-org/goth-kklogger"
	kkpanic "github.com/yetiz-org/goth-panic"
)

type ServerChannel struct {
	channel.DefaultNetServerChannel
	server       *http.Server
	active       bool
	newChChan    chan *serverChannelAccept
	chMap        sync.Map
	maxBodyBytes int64
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

	if c.maxBodyBytes > 0 {
		r.Body = http.MaxBytesReader(w, r.Body, c.maxBodyBytes)
	}

	request := WrapRequest(cch, r)
	if request == nil {
		kklogger.WarnJ("http:ServerChannel.ServeHTTP", fmt.Sprintf("conn from %s, target: %s, body is too large", r.RemoteAddr, r.RequestURI))
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		w.Write([]byte{})
		return
	}

	var writer = w
	var pkg = &Pack{
		Request:  request,
		Response: NewResponse(request),
		Params:   map[string]any{},
		Writer:   writer,
	}

	var obj any = pkg
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

	c.newChChan = make(chan *serverChannelAccept, channel.GetParamIntDefault(c, ParamAcceptWaitCount, 1024))
	c.maxBodyBytes = channel.GetParamInt64Default(c, ParamMaxBodyBytes, 0)
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
				if v, f := c.chMap.LoadAndDelete(conn); f {
					ch := v.(channel.NetChannel)
					kklogger.TraceJ("http:ServerChannel.StateHijacked", fmt.Sprintf("channel_id: %s", ch.ID()))
				}
			case http.StateClosed:
				if v, f := c.chMap.LoadAndDelete(conn); f {
					ch := v.(channel.NetChannel)
					kklogger.TraceJ("http:ServerChannel.StateClosed", fmt.Sprintf("channel_id: %s", ch.ID()))
					if ch.IsActive() {
						ch.Deregister()
					}
				}
			default:
			}
		},
		ConnContext: func(ctx context.Context, conn net.Conn) context.Context {
			var ch = &Channel{}
			c.DeriveNetChildChannel(ch, c, conn)
			ctx = context.WithValue(ctx, ConnCtx, conn)
			ctx = context.WithValue(ctx, ConnChCtx, ch)
			accept := &serverChannelAccept{
				Channel: ch,
				Future:  ch.Pipeline().NewFuture(),
			}

			c.newChChan <- accept
			c.chMap.Store(conn, ch)
			accept.Future.Await()
			return ctx
		},
	}

	c.active = true
	go c.server.ListenAndServe()
	return nil
}

func (c *ServerChannel) UnsafeAccept() (channel.Channel, channel.Future) {
	accept := <-c.newChChan
	return accept.Channel, accept.Future
}

type serverChannelAccept struct {
	channel.Channel
	channel.Future
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
	kklogger.InfoJ("htp:ServerChannel.UnsafeClose", fmt.Sprintf("server %s[%s] closed", c.Name, c.LocalAddr().String()))
	return nil
}

func (c *ServerChannel) IsActive() bool {
	return c.active
}
