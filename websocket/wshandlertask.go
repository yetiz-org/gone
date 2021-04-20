package websocket

import (
	"time"

	"github.com/kklab-com/gone/channel"
	"github.com/kklab-com/gone/http"
	kklogger "github.com/kklab-com/goth-kklogger"
)

type WSTask interface {
	WSPing(ctx channel.HandlerContext, message *PingMessage, params map[string]interface{})
	WSPong(ctx channel.HandlerContext, message *PongMessage, params map[string]interface{})
	WSClose(ctx channel.HandlerContext, message *CloseMessage, params map[string]interface{})
	WSBinary(ctx channel.HandlerContext, message *DefaultMessage, params map[string]interface{})
	WSText(ctx channel.HandlerContext, message *DefaultMessage, params map[string]interface{})
}

type ServerHandlerTask interface {
	channel.HandlerTask
	WSTask
	WSUpgrade(req *http.Request, resp *http.Response, params map[string]interface{}) bool
}

type WSHandler struct {
	channel.DefaultHandler
	Builder DefaultMessageBuilder
}

func (h *WSHandler) Read(ctx channel.HandlerContext, obj interface{}) {
	pack := _HttpWebsocketPackCast(obj)
	if pack == nil {
		ctx.FireRead(obj)
		return
	}

	h._Call(ctx, pack.Request, pack.HandlerTask, pack.Message, pack.Params)
}

func (h *WSHandler) _Call(ctx channel.HandlerContext, req *http.Request, task ServerHandlerTask, msg Message, params map[string]interface{}) {
	switch msg.Type() {
	case TextMessageType:
		task.WSText(ctx, msg.(*DefaultMessage), params)
	case BinaryMessageType:
		task.WSBinary(ctx, msg.(*DefaultMessage), params)
	case CloseMessageType:
		task.WSClose(ctx, msg.(*CloseMessage), params)
	case PingMessageType:
		task.WSPing(ctx, msg.(*PingMessage), params)
	case PongMessageType:
		task.WSPong(ctx, msg.(*PongMessage), params)
	}
}

func (h *WSHandler) ErrorCaught(ctx channel.HandlerContext, err error) {
	kklogger.ErrorJ("websocket:WSHandler", err.Error())
}

func (h *WSHandler) WSPing(ctx channel.HandlerContext, message *PingMessage, params map[string]interface{}) {
	dead := time.Now().Add(time.Minute)
	var obj interface{} = PongMessage{
		DefaultMessage: DefaultMessage{
			MessageType: PongMessageType,
			Message:     message.Message,
			Dead:        &dead,
		},
	}

	ctx.Write(&obj, nil)
}

func (h *WSHandler) WSPong(ctx channel.HandlerContext, message *PongMessage, params map[string]interface{}) {
}

func (h *WSHandler) WSClose(ctx channel.HandlerContext, message *CloseMessage, params map[string]interface{}) {
}

func (h *WSHandler) WSBinary(ctx channel.HandlerContext, message *DefaultMessage, params map[string]interface{}) {
}

func (h *WSHandler) WSText(ctx channel.HandlerContext, message *DefaultMessage, params map[string]interface{}) {
}

type WSServerHandler struct {
	WSHandler
}

func (h *WSServerHandler) WSUpgrade(req *http.Request, resp *http.Response, params map[string]interface{}) bool {
	return true
}

type MessageBuilder interface {
	Text(msg string) *DefaultMessage
	Binary(msg []byte) *DefaultMessage
	Close(msg []byte, closeCode CloseCode) *CloseMessage
	Ping(msg []byte, deadline time.Time) *PingMessage
	Pong(msg []byte, deadline time.Time) *PongMessage
}

type DefaultMessageBuilder struct{}

func (b *DefaultMessageBuilder) Text(msg string) *DefaultMessage {
	return &DefaultMessage{
		MessageType: TextMessageType,
		Message:     []byte(msg),
	}
}

func (b *DefaultMessageBuilder) Binary(msg []byte) *DefaultMessage {
	return &DefaultMessage{
		MessageType: BinaryMessageType,
		Message:     msg,
	}
}

func (b *DefaultMessageBuilder) Close(msg []byte, closeCode CloseCode) *CloseMessage {
	return &CloseMessage{
		DefaultMessage: DefaultMessage{
			MessageType: CloseMessageType,
			Message:     msg,
		},
		CloseCode: closeCode,
	}
}

func (b *DefaultMessageBuilder) Ping(msg []byte, deadline time.Time) *PingMessage {
	return &PingMessage{
		DefaultMessage: DefaultMessage{
			MessageType: PingMessageType,
			Message:     msg,
			Dead:        &deadline,
		},
	}
}

func (b *DefaultMessageBuilder) Pong(msg []byte, deadline time.Time) *PongMessage {
	return &PongMessage{
		DefaultMessage: DefaultMessage{
			MessageType: PongMessageType,
			Message:     msg,
			Dead:        &deadline,
		},
	}
}
