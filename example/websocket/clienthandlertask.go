package websocket

import (
	"fmt"

	"github.com/kklab-com/gone/channel"
	"github.com/kklab-com/gone/http"
	"github.com/kklab-com/gone/websocket"
)

type ClientHandlerTask struct {
	websocket.DefaultHandlerTask
}

func (h *ClientHandlerTask) WSPing(ctx channel.HandlerContext, message *websocket.PingMessage, params map[string]interface{}) {
	println("client WSPing")
	h.DefaultHandlerTask.WSPing(ctx, message, params)
}

func (h *ClientHandlerTask) WSPong(ctx channel.HandlerContext, message *websocket.PongMessage, params map[string]interface{}) {
	println("client WSPong")
}

func (h *ClientHandlerTask) WSText(ctx channel.HandlerContext, message *websocket.DefaultMessage, params map[string]interface{}) {
	println("client WSText")
	println(message.StringMessage())
}

func (h *ClientHandlerTask) WSClose(ctx channel.HandlerContext, message *websocket.CloseMessage, params map[string]interface{}) {
	println(fmt.Sprintf("%s client WSClose %s", ctx.Channel().ID(), message.StringMessage()))
}

func (h *ClientHandlerTask) WSBinary(ctx channel.HandlerContext, message *websocket.DefaultMessage, params map[string]interface{}) {
	println("client WSBinary")
}

func (h *ClientHandlerTask) WSConnected(ch channel.Channel, req *http.Request, resp *http.Response, params map[string]interface{}) {
	println(fmt.Sprintf("%s client WSConnected", ch.ID()))
	ch.Write(h.Builder.Ping(nil, nil))
}

func (h *ClientHandlerTask) WSDisconnected(ch channel.Channel, req *http.Request, resp *http.Response, params map[string]interface{}) {
	println(fmt.Sprintf("%s client WSDisconnected", ch.ID()))
}
