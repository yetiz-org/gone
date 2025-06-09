package websocket

import "github.com/yetiz-org/gone/http"

type DefaultServerHandlerTask struct {
	DefaultHandlerTask
}

func (h *DefaultServerHandlerTask) WSUpgrade(req *http.Request, resp *http.Response, params map[string]any) bool {
	return true
}
