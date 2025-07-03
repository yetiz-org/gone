package gws

import (
	"fmt"

	"github.com/yetiz-org/gone/channel"
	"github.com/yetiz-org/gone/ghttp"
	"github.com/yetiz-org/goth-kklogger"
	kkpanic "github.com/yetiz-org/goth-panic"
)

type InvokeHandler struct {
	channel.DefaultHandler
	DefaultHandlerTask
	task   HandlerTask
	params map[string]any
}

func NewInvokeHandler(task HandlerTask, params map[string]any) *InvokeHandler {
	if params == nil {
		params = map[string]any{}
	}

	return &InvokeHandler{task: task, params: params}
}

func (h *InvokeHandler) Read(ctx channel.HandlerContext, obj any) {
	if ch, ok := ctx.Channel().(*Channel); ok {
		if msg, ok := obj.(Message); ok {
			h._Call(ctx, ch.Request, ch.Response, h.task, msg, h.params)
			return
		}
	}

	ctx.FireRead(obj)
	return
}

func (h *InvokeHandler) Active(ctx channel.HandlerContext) {
	if ch, ok := ctx.Channel().(*Channel); ok {
		h.task.WSConnected(ch, ch.Request, ch.Response, h.params)
	}

	ctx.FireActive()
}

func (h *InvokeHandler) Inactive(ctx channel.HandlerContext) {
	if ch, ok := ctx.Channel().(*Channel); ok {
		h.task.WSDisconnected(ch, ch.Request, ch.Response, h.params)
	}

	ctx.FireInactive()
}

func (h *InvokeHandler) _Call(ctx channel.HandlerContext, req *ghttp.Request, resp *ghttp.Response, task HandlerTask, msg Message, params map[string]any) {
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
		kklogger.ErrorJ("gws:InvokeHandler._Call#invoke_call!error", fmt.Sprintf("error occurred, %s", r.Error()))
		task.WSErrorCaught(ctx, req, resp, msg, r)
	})
}

func (h *InvokeHandler) ErrorCaught(ctx channel.HandlerContext, err error) {
	kklogger.ErrorJ("gws:InvokeHandler.ErrorCaught#error_caught!error", err.Error())
}
