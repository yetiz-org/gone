package example

import (
	"fmt"

	"github.com/yetiz-org/gone/channel"
	"github.com/yetiz-org/gone/ghttp"
	websocket "github.com/yetiz-org/gone/gws"
)

type ClientHandlerTask struct {
	websocket.DefaultHandlerTask
}

func (h *ClientHandlerTask) WSPing(ctx channel.HandlerContext, message *websocket.PingMessage, params map[string]any) {
	println("client WSPing")
	h.DefaultHandlerTask.WSPing(ctx, message, params)
}

func (h *ClientHandlerTask) WSPong(ctx channel.HandlerContext, message *websocket.PongMessage, params map[string]any) {
	println("client WSPong")
}

func (h *ClientHandlerTask) WSText(ctx channel.HandlerContext, message *websocket.DefaultMessage, params map[string]any) {
	println("client WSText")
	println(message.StringMessage())
}

func (h *ClientHandlerTask) WSClose(ctx channel.HandlerContext, message *websocket.CloseMessage, params map[string]any) {
	println(fmt.Sprintf("%s client WSClose %s", ctx.Channel().ID(), message.StringMessage()))
}

func (h *ClientHandlerTask) WSBinary(ctx channel.HandlerContext, message *websocket.DefaultMessage, params map[string]any) {
	println("client WSBinary")
}

func (h *ClientHandlerTask) WSConnected(ch channel.Channel, req *ghttp.Request, resp *ghttp.Response, params map[string]any) {
	println(fmt.Sprintf("%s client WSConnected", ch.ID()))
	ch.Write(h.Builder.Ping(nil, nil)).Sync()
}

func (h *ClientHandlerTask) WSDisconnected(ch channel.Channel, req *ghttp.Request, resp *ghttp.Response, params map[string]any) {
	println(fmt.Sprintf("%s client WSDisconnected", ch.ID()))
}
