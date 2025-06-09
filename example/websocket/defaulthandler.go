package example

import (
	"time"

	buf "github.com/kklab-com/goth-bytebuf"
	"github.com/yetiz-org/gone/channel"
	"github.com/yetiz-org/gone/http"
)

type DefaultTask struct {
	http.DefaultHTTPHandlerTask
}

func (l *DefaultTask) Get(ctx channel.HandlerContext, req *http.Request, resp *http.Response, params map[string]any) http.ErrorResponse {
	resp.SetStatusCode(httpstatus.OK)
	resp.TextResponse(buf.NewByteBuf([]byte("feeling good")))
	return nil
}

type DefaultHomeTask struct {
	http.DefaultHTTPHandlerTask
}

func (l *DefaultHomeTask) Get(ctx channel.HandlerContext, req *http.Request, resp *http.Response, params map[string]any) http.ErrorResponse {
	resp.SetStatusCode(httpstatus.OK)
	resp.TextResponse(buf.NewByteBuf([]byte(req.RequestURI())))
	go func() {
		<-time.After(time.Millisecond * 100)
		if ctx.Channel().IsActive() {
			ctx.Channel().Disconnect()
		}
	}()

	return nil
}

type CloseTask struct {
	http.DefaultHTTPHandlerTask
}

func (l *CloseTask) Get(ctx channel.HandlerContext, req *http.Request, resp *http.Response, params map[string]any) http.ErrorResponse {
	resp.SetStatusCode(httpstatus.OK)
	resp.TextResponse(buf.NewByteBuf([]byte(req.RequestURI())))
	go func() {
		<-time.After(time.Second)
		ctx.Channel().Parent().Close()
	}()

	return nil
}
