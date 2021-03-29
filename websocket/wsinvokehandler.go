package websocket

import (
	"fmt"
	"sync"

	"github.com/kklab-com/gone/channel"
	"github.com/kklab-com/gone/http"
	"github.com/kklab-com/goth-kklogger"
	"github.com/pkg/errors"
)

type InvokeHandler struct {
	channel.DefaultHandler
}

func (h *InvokeHandler) Read(ctx channel.HandlerContext, obj interface{}) {
	pack := _UnPack(obj)
	if pack == nil {
		ctx.FireRead(obj)
		return
	}

	h.invokeMethod(ctx, pack.Req, pack.Task, pack.Message, pack.Params)
}

func (h *InvokeHandler) invokeMethod(ctx channel.HandlerContext, req *http.Request, task HandlerTask, msg Message, params map[string]interface{}) {
	defer func() {
		var err error = nil
		if r := recover(); r != nil {
			switch er := r.(type) {
			case error:
				err = er
			case string:
				err = errors.Errorf(er)
			default:
				panic(er)
			}

			kklogger.ErrorJ("InvokeHandler.invokeMethod#Error", fmt.Sprintf("error occurred, %s", err.Error()))
			task.ErrorCaught(ctx, req, msg, err)
			ctx.Channel().Param(ParamWSDisconnectOnce).(*sync.Once).Do(func() {
				ctx.Channel().Disconnect()
			})
		}
	}()

	switch msg.Type() {
	case TextMessageType:
		task.Text(ctx, msg.(*DefaultMessage), params)
	case BinaryMessageType:
		task.Binary(ctx, msg.(*DefaultMessage), params)
	case CloseMessageType:
		task.Close(ctx, msg.(*CloseMessage), params)
	case PingMessageType:
		task.Ping(ctx, msg.(*PingMessage), params)
	case PongMessageType:
		task.Pong(ctx, msg.(*PongMessage), params)
	}
}

func (h *InvokeHandler) ErrorCaught(ctx channel.HandlerContext, err error) {
	kklogger.ErrorJ("InvokeHandler", err.Error())
}
