package websocket

import (
	"github.com/kklab-com/gone/channel"
	"github.com/kklab-com/gone/http"
	"github.com/kklab-com/gone/websocket"
	"github.com/kklab-com/goth-kkutil/value"
)

type ServerHandlerTask struct {
	websocket.DefaultServerHandlerTask
}

func (t *ServerHandlerTask) WSPing(ctx channel.HandlerContext, message *websocket.PingMessage, params map[string]interface{}) {
	println("server WSPing")
}

func (t *ServerHandlerTask) WSText(ctx channel.HandlerContext, message *websocket.DefaultMessage, params map[string]interface{}) {
	println(message.StringMessage())
	println("server WSText")
	var obj interface{} = t.Builder.Text(value.JsonMarshal(struct {
		Params  map[string]interface{} `json:"params"`
		Message string                 `json:"message"`
	}{
		Params:  params,
		Message: message.StringMessage(),
	}))

	ctx.Write(obj, nil)
}

func (t *ServerHandlerTask) WSClose(ctx channel.HandlerContext, message *websocket.CloseMessage, params map[string]interface{}) {
	println("server WSClose")
}

func (t *ServerHandlerTask) WSConnected(req *http.Request, resp *http.Response, params map[string]interface{}) {
	println("server WSConnected")
}

func (t *ServerHandlerTask) WSDisconnected(req *http.Request, resp *http.Response, params map[string]interface{}) {
	println("server WSDisconnected")
	req.Channel().Parent().Close()
}
