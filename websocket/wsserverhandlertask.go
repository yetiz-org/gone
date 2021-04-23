package websocket

import "github.com/kklab-com/gone/http"

type DefaultServerHandlerTask struct {
	DefaultHandlerTask
}

func (h *DefaultServerHandlerTask) WSUpgrade(req *http.Request, resp *http.Response, params map[string]interface{}) bool {
	return true
}
