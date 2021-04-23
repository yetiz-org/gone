package http

import (
	"time"

	"github.com/kklab-com/gone-httpstatus"
	"github.com/kklab-com/gone/channel"
	"github.com/kklab-com/gone/http"
	"github.com/kklab-com/goth-kkutil/buf"
)

type DefaultTask struct {
	http.DefaultHTTPHandlerTask
}

func (l *DefaultTask) Get(ctx channel.HandlerContext, req *http.Request, resp *http.Response, params map[string]interface{}) http.ErrorResponse {
	resp.SetStatusCode(httpstatus.OK)
	resp.TextResponse(buf.NewByteBuf([]byte("feeling good")))
	return nil
}

type DefaultHomeTask struct {
	http.DefaultHTTPHandlerTask
}

func (l *DefaultHomeTask) Get(ctx channel.HandlerContext, req *http.Request, resp *http.Response, params map[string]interface{}) http.ErrorResponse {
	resp.SetStatusCode(httpstatus.OK)
	resp.TextResponse(buf.NewByteBuf([]byte(req.RequestURI())))
	go func() {
		<-time.After(time.Millisecond * 100)
		ctx.Channel().Disconnect()
	}()

	return nil
}

type CloseTask struct {
	http.DefaultHTTPHandlerTask
}

func (l *CloseTask) Get(ctx channel.HandlerContext, req *http.Request, resp *http.Response, params map[string]interface{}) http.ErrorResponse {
	resp.SetStatusCode(httpstatus.OK)
	resp.TextResponse(buf.NewByteBuf([]byte(req.RequestURI())))
	go func() {
		<-time.After(time.Second)
		ctx.Channel().Parent().Close()
	}()

	return nil
}
