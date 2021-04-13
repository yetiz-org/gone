package websocket

import (
	"github.com/kklab-com/gone/channel"
	"github.com/kklab-com/gone/http"
	"github.com/kklab-com/gone/websocket"
	"github.com/kklab-com/goth-kklogger"
	"github.com/kklab-com/goth-kkutil/value"
)

type DefaultTask struct {
	websocket.WSHandlerTask
}

func (t *DefaultTask) Ping(ctx channel.HandlerContext, message *websocket.PingMessage, params map[string]interface{}) {
}

func (t *DefaultTask) Text(ctx channel.HandlerContext, message *websocket.DefaultMessage, params map[string]interface{}) {
	var obj interface{} = t.Builder.Text(value.JsonMarshal(struct {
		Params  map[string]interface{} `json:"params"`
		Message string                 `json:"message"`
	}{
		Params:  params,
		Message: message.StringMessage(),
	}))

	ctx.FireWrite(obj)
}

func (t *DefaultTask) Close(ctx channel.HandlerContext, message *websocket.CloseMessage, params map[string]interface{}) {
	kklogger.Trace("DefaultTask", "Close")
}

func (*DefaultTask) WSConnected(req *http.Request, params map[string]interface{}) {
	kklogger.Trace("DefaultTask", "WSConnected")
}

func (*DefaultTask) WSDisconnect(req *http.Request, params map[string]interface{}) {
	kklogger.Trace("DefaultTask", "WSDisconnect")
	req.Channel().Parent().Close()
}
