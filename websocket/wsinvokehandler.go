package websocket

import (
	"fmt"

	"github.com/kklab-com/gone/channel"
	"github.com/kklab-com/gone/http"
	"github.com/kklab-com/goth-kklogger"
	kkpanic "github.com/kklab-com/goth-panic"
)

type InvokeHandler struct {
	WSHandler
}

func (h *InvokeHandler) Read(ctx channel.HandlerContext, obj interface{}) {
	pack := _HttpWebsocketPackCast(obj)
	if pack == nil {
		ctx.FireRead(obj)
		return
	}

	h._Call(ctx, pack.Request, pack.HandlerTask, pack.Message, pack.Params)
}

func (h *InvokeHandler) _Call(ctx channel.HandlerContext, req *http.Request, task ServerHandlerTask, msg Message, params map[string]interface{}) {
	kkpanic.Catch(func() {
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
	}, func(r kkpanic.Caught) {
		kklogger.ErrorJ("websocket:InvokeHandler._Call", fmt.Sprintf("error occurred, %s", r.Error()))
		task.WSErrorCaught(ctx, req, msg, r)
	})
}

func (h *InvokeHandler) ErrorCaught(ctx channel.HandlerContext, err error) {
	kklogger.ErrorJ("websocket:InvokeHandler", err.Error())
}
