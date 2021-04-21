package websocket

import (
	"github.com/kklab-com/gone/channel"
	"github.com/kklab-com/gone/http"
	"github.com/kklab-com/gone/websocket"
)

type ClientHandlerTask struct {
	websocket.DefaultHandlerTask
}

func (h *ClientHandlerTask) WSText(ctx channel.HandlerContext, message *websocket.DefaultMessage, params map[string]interface{}) {
	println(message.StringMessage())
	println("client WSText")
}

func (h *ClientHandlerTask) WSClose(ctx channel.HandlerContext, message *websocket.CloseMessage, params map[string]interface{}) {
	println("client WSClose")
}

func (h *ClientHandlerTask) WSBinary(ctx channel.HandlerContext, message *websocket.DefaultMessage, params map[string]interface{}) {
	println("client WSBinary")
}

func (h *ClientHandlerTask) WSConnected(req *http.Request, resp *http.Response, params map[string]interface{}) {
	println("client WSConnected")
}

func (h *ClientHandlerTask) WSDisconnected(req *http.Request, resp *http.Response, params map[string]interface{}) {
	println("client WSDisconnected")
}
