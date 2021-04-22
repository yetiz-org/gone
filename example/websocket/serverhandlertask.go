package websocket

import (
	"fmt"

	"github.com/kklab-com/gone/channel"
	"github.com/kklab-com/gone/http"
	"github.com/kklab-com/gone/websocket"
	"github.com/kklab-com/goth-kkutil/value"
)

type ServerHandlerTask struct {
	websocket.DefaultServerHandlerTask
}

func (h *ServerHandlerTask) WSPing(ctx channel.HandlerContext, message *websocket.PingMessage, params map[string]interface{}) {
	println("server WSPing")
	h.DefaultServerHandlerTask.WSPing(ctx, message, params)
	ctx.Channel().Write(h.Builder.Ping(nil, nil)).Sync()
}

func (h *ServerHandlerTask) WSPong(ctx channel.HandlerContext, message *websocket.PongMessage, params map[string]interface{}) {
	println("server WSPong")
}

func (h *ServerHandlerTask) WSText(ctx channel.HandlerContext, message *websocket.DefaultMessage, params map[string]interface{}) {
	println("server WSText")
	println(message.StringMessage())
	var obj interface{} = h.Builder.Text(value.JsonMarshal(struct {
		Params  map[string]interface{} `json:"params"`
		Message string                 `json:"message"`
	}{
		Params:  params,
		Message: message.StringMessage(),
	}))

	ctx.Write(obj, nil).Sync()
}

func (h *ServerHandlerTask) WSClose(ctx channel.HandlerContext, message *websocket.CloseMessage, params map[string]interface{}) {
	println(fmt.Sprintf("%s server WSClose %s", ctx.Channel().ID(), message.StringMessage()))
}

func (h *ServerHandlerTask) WSConnected(ch channel.Channel, req *http.Request, resp *http.Response, params map[string]interface{}) {
	println(fmt.Sprintf("%s server WSConnected", ch.ID()))
}

func (h *ServerHandlerTask) WSDisconnected(ch channel.Channel, req *http.Request, resp *http.Response, params map[string]interface{}) {
	println(fmt.Sprintf("%s server WSDisconnected", ch.ID()))
	ch.Parent().Close()
}
