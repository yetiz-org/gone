package gws

import "github.com/yetiz-org/gone/ghttp"

type DefaultServerHandlerTask struct {
	DefaultHandlerTask
}

func (h *DefaultServerHandlerTask) WSUpgrade(req *ghttp.Request, resp *ghttp.Response, params map[string]any) bool {
	return true
}
