package websocket

import (
	"github.com/kklab-com/gone/channel"
	"github.com/kklab-com/goth-kklogger"
)

type ClientHandler struct {
	channel.DefaultHandler
	HandlerTask
	params map[string]interface{}
}

func (h *ClientHandler) Added(ctx channel.HandlerContext) {
	h.params = map[string]interface{}{}
}

func (h *ClientHandler) Read(ctx channel.HandlerContext, obj interface{}) {
	if msg, ok := obj.(Message); ok {
		h._Call(ctx, msg, h.params)
	} else {
		ctx.FireRead(obj)
	}
}

func (h *ClientHandler) _Call(ctx channel.HandlerContext, msg Message, params map[string]interface{}) {
	switch msg.Type() {
	case TextMessageType:
		h.WSText(ctx, msg.(*DefaultMessage), params)
	case BinaryMessageType:
		h.WSBinary(ctx, msg.(*DefaultMessage), params)
	case CloseMessageType:
		h.WSClose(ctx, msg.(*CloseMessage), params)
	case PingMessageType:
		h.WSPing(ctx, msg.(*PingMessage), params)
	case PongMessageType:
		h.WSPong(ctx, msg.(*PongMessage), params)
	}
}

func (h *ClientHandler) WSPing(ctx channel.HandlerContext, message *PingMessage, params map[string]interface{}) {
}

func (h *ClientHandler) WSPong(ctx channel.HandlerContext, message *PongMessage, params map[string]interface{}) {
}

func (h *ClientHandler) WSClose(ctx channel.HandlerContext, message *CloseMessage, params map[string]interface{}) {
}

func (h *ClientHandler) WSBinary(ctx channel.HandlerContext, message *DefaultMessage, params map[string]interface{}) {
}

func (h *ClientHandler) WSText(ctx channel.HandlerContext, message *DefaultMessage, params map[string]interface{}) {
}

func (h *ClientHandler) ErrorCaught(ctx channel.HandlerContext, err error) {
	kklogger.ErrorJ("websocket:ClientHandler", err.Error())
	ctx.Channel().Disconnect()
}
