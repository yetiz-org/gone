package websocket

import (
	"fmt"
	"time"

	"github.com/kklab-com/gone/channel"
	"github.com/kklab-com/gone/http"
)

type WSTask interface {
	Ping(ctx channel.HandlerContext, message *PingMessage, params map[string]interface{})
	Pong(ctx channel.HandlerContext, message *PongMessage, params map[string]interface{})
	Close(ctx channel.HandlerContext, message *CloseMessage, params map[string]interface{})
	Binary(ctx channel.HandlerContext, message *DefaultMessage, params map[string]interface{})
	Text(ctx channel.HandlerContext, message *DefaultMessage, params map[string]interface{})
}

type HandlerTask interface {
	channel.HandlerTask
	WSTask
	Upgrade(req *http.Request, resp *http.Response, params map[string]interface{}) bool
	WSConnected(req *http.Request, params map[string]interface{})
	WSDisconnect(req *http.Request, params map[string]interface{})
	ErrorCaught(ctx channel.HandlerContext, req *http.Request, msg Message, err error)
}

func (h *WSHandlerTask) Ping(ctx channel.HandlerContext, message *PingMessage, params map[string]interface{}) {
	dead := time.Now().Add(time.Minute)
	var obj interface{} = PongMessage{
		DefaultMessage: DefaultMessage{
			MessageType: PongMessageType,
			Message:     message.Message,
			Dead:        &dead,
		},
	}

	ctx.FireWrite(&obj)
}

func (h *WSHandlerTask) Pong(ctx channel.HandlerContext, message *PongMessage, params map[string]interface{}) {
}

func (h *WSHandlerTask) Close(ctx channel.HandlerContext, message *CloseMessage, params map[string]interface{}) {
}

func (h *WSHandlerTask) Binary(ctx channel.HandlerContext, message *DefaultMessage, params map[string]interface{}) {
}

func (h *WSHandlerTask) Text(ctx channel.HandlerContext, message *DefaultMessage, params map[string]interface{}) {
}

type WSHandlerTask struct {
	Builder DefaultMessageBuilder
}

func (h *WSHandlerTask) Upgrade(req *http.Request, resp *http.Response, params map[string]interface{}) bool {
	return true
}

func (h *WSHandlerTask) WSConnected(req *http.Request, params map[string]interface{}) {
}

func (h *WSHandlerTask) WSDisconnect(req *http.Request, params map[string]interface{}) {
}

func (h *WSHandlerTask) ErrorCaught(ctx channel.HandlerContext, req *http.Request, msg Message, err error) {
}

func (h *WSHandlerTask) GetNodeName(params map[string]interface{}) string {
	if rtn := params["[gone]node_name"]; rtn != nil {
		return rtn.(string)
	}

	return ""
}

func (h *WSHandlerTask) IsIndex(params map[string]interface{}) string {
	if rtn := params["[gone]is_index"]; rtn != nil {
		return rtn.(string)
	}

	return ""
}

func (h *WSHandlerTask) GetID(name string, params map[string]interface{}) string {
	if rtn := params[fmt.Sprintf("[gone]%s_id", name)]; rtn != nil {
		return rtn.(string)
	}

	return ""
}

func (h *WSHandlerTask) LogExtend(key string, value interface{}, params map[string]interface{}) {
	if rtn := params["[gone]extend"]; rtn == nil {
		rtn = map[string]interface{}{key: value}
		params["[gone]extend"] = rtn
	} else {
		rtn.(map[string]interface{})[key] = value
	}
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
