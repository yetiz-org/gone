package websocket

import (
	"github.com/kklab-com/gone/channel"
	"github.com/kklab-com/gone/websocket"
)

type ClientHandler struct {
	websocket.ClientHandler
}

func (h *ClientHandler) WSText(ctx channel.HandlerContext, message *websocket.DefaultMessage, params map[string]interface{}) {
	println(message.StringMessage())
}

func (h *ClientHandler) ReadCompleted(ctx channel.HandlerContext) {
	println("client read_completed")
}

func (h *ClientHandler) Disconnect(ctx channel.HandlerContext) {
	println("client disconnect")
	ctx.Disconnect()
}

func (h *ClientHandler) Active(ctx channel.HandlerContext) {
	println("client active")
	ctx.FireActive()
}

func (h *ClientHandler) Inactive(ctx channel.HandlerContext) {
	println("client inactive")
	ctx.FireInactive()
}
