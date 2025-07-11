package example

import (
	"fmt"
	"github.com/yetiz-org/gone/channel"
	"github.com/yetiz-org/gone/erresponse"
	"github.com/yetiz-org/gone/ghttp"
	"github.com/yetiz-org/gone/ghttp/httpstatus"
	buf "github.com/yetiz-org/goth-bytebuf"
	"runtime/pprof"
	"time"
)

type DefaultTask struct {
	ghttp.DefaultHTTPHandlerTask
}

func (l *DefaultTask) Get(ctx channel.HandlerContext, req *ghttp.Request, resp *ghttp.Response, params map[string]any) ghttp.ErrorResponse {
	resp.SetStatusCode(httpstatus.OK)
	resp.TextResponse(buf.NewByteBuf([]byte("feeling good")))
	return nil
}

func (l *DefaultTask) Post(ctx channel.HandlerContext, req *ghttp.Request, resp *ghttp.Response, params map[string]any) ghttp.ErrorResponse {
	return l.Get(ctx, req, resp, params)
}

type DefaultHomeTask struct {
	ghttp.DefaultHTTPHandlerTask
}

func (l *DefaultHomeTask) Get(ctx channel.HandlerContext, req *ghttp.Request, resp *ghttp.Response, params map[string]any) ghttp.ErrorResponse {
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

var longMsg = "{\"msg\":\"UnhandledPromiseRejectionWarning: Unhandled promise rejection. This error originated either by throwing inside of an async function without a catch block, or by rejecting a promise which was not handled with .catch(). To terminate the node process on unhandled promise rejection, use the CLI flag `--unhandled-rejections=strict` (see https://nodejs.org/api/cli.html#cli_unhandled_rejections_mode). (rejection id: 11)210(node:44) UnhandledPromiseRejectionWarning: FetchError: Caught error after test environment was torn down\"}"

type LongTask struct {
	ghttp.DefaultHTTPHandlerTask
}

func (l *LongTask) Get(ctx channel.HandlerContext, req *ghttp.Request, resp *ghttp.Response, params map[string]any) ghttp.ErrorResponse {
	resp.SetStatusCode(httpstatus.OK)
	resp.TextResponse(buf.NewByteBufString(longMsg + req.FormValue("v")))
	return nil
}

type CloseTask struct {
	ghttp.DefaultHTTPHandlerTask
}

func (l *CloseTask) Get(ctx channel.HandlerContext, req *ghttp.Request, resp *ghttp.Response, params map[string]any) ghttp.ErrorResponse {
	resp.SetStatusCode(httpstatus.OK)
	resp.TextResponse(buf.NewByteBuf([]byte(req.RequestURI())))
	go func() {
		<-time.After(time.Second)
		ctx.Channel().Parent().Close()
	}()

	return nil
}

type Routine struct {
	ghttp.DefaultHTTPHandlerTask
}

func (a *Routine) Index(ctx channel.HandlerContext, req *ghttp.Request, resp *ghttp.Response, params map[string]any) ghttp.ErrorResponse {
	buffer := buf.EmptyByteBuf()
	pprof.Lookup("goroutine").WriteTo(buffer, 1)
	resp.TextResponse(buffer)
	return nil
}

type Acceptance400 struct {
	ghttp.DispatchAcceptance
}

func (a *Acceptance400) Do(ctx channel.HandlerContext, req *ghttp.Request, resp *ghttp.Response, params map[string]any) error {
	return erresponse.InvalidRequest
}

type SSE struct {
	ghttp.DefaultHTTPHandlerTask
}

func (h *SSE) Get(ctx channel.HandlerContext, req *ghttp.Request, resp *ghttp.Response, params map[string]any) ghttp.ErrorResponse {
	sse := h.SSEMode(ctx, req, resp, params)
	resp.SetHeader("Validate", "true")
	sse.WriteHeader(ctx, resp.Header(), params)
	for i := 0; i < 3; i++ {
		sse.WriteMessage(ctx, ghttp.SSEMessage{Event: "event", Data: []string{fmt.Sprintf("%d", i)}}, params)
		time.Sleep(time.Millisecond * 300)
	}

	sse.WriteMessages(ctx, []ghttp.SSEMessage{
		{Event: "event2", Data: []string{"4"}},
		{Event: "event2", Data: []string{"5", "5-1"}},
		{Event: "event2", Data: []string{"6"}},
	}, params)

	return nil
}
