package websocket

import (
	"github.com/kklab-com/gone/channel"
	"github.com/kklab-com/gone/websocket"
	"github.com/kklab-com/goth-kkutil/value"
)

type DefaultTask struct {
	websocket.DefaultServerHandlerTask
}

func (t *DefaultTask) WSPing(ctx channel.HandlerContext, message *websocket.PingMessage, params map[string]interface{}) {
}

func (t *DefaultTask) WSText(ctx channel.HandlerContext, message *websocket.DefaultMessage, params map[string]interface{}) {
	println(message.StringMessage())
	var obj interface{} = t.Builder.Text(value.JsonMarshal(struct {
		Params  map[string]interface{} `json:"params"`
		Message string                 `json:"message"`
	}{
		Params:  params,
		Message: message.StringMessage(),
	}))

	ctx.Write(obj, nil)
}

func (t *DefaultTask) WSClose(ctx channel.HandlerContext, message *websocket.CloseMessage, params map[string]interface{}) {
	println("server ws close")
}
