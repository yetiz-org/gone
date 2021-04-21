package websocket

import (
	"github.com/kklab-com/gone/channel"
	"github.com/kklab-com/gone/http"
	"github.com/kklab-com/gone/websocket"
)

type ClientHandler struct {
	websocket.DefaultHandlerTask
}

func (h *ClientHandler) WSText(ctx channel.HandlerContext, message *websocket.DefaultMessage, params map[string]interface{}) {
	println(message.StringMessage())
}

func (h *ClientHandler) WSClose(ctx channel.HandlerContext, message *websocket.CloseMessage, params map[string]interface{}) {
	println("client WSClose")
}

func (h *ClientHandler) WSBinary(ctx channel.HandlerContext, message *websocket.DefaultMessage, params map[string]interface{}) {
	println("client WSBinary")
}

func (h *ClientHandler) WSConnected(req *http.Request, resp *http.Response, params map[string]interface{}) {
	println("client WSConnected")
}

func (h *ClientHandler) WSDisconnected(req *http.Request, resp *http.Response, params map[string]interface{}) {
	println("client WSDisconnected")
}
