package http

import (
	"bytes"

	"github.com/kklab-com/gone-httpstatus"
	"github.com/kklab-com/gone/http"
)

type DefaultTask struct {
	http.DefaultHandlerTask
}

func (l *DefaultTask) Get(req *http.Request, resp *http.Response, params map[string]interface{}) http.ErrorResponse {
	resp.SetStatusCode(httpstatus.OK)
	resp.TextResponse(bytes.NewBufferString("feeling good"))
	return nil
}

type DefaultHomeTask struct {
	http.DefaultHandlerTask
}

func (l *DefaultHomeTask) Get(req *http.Request, resp *http.Response, params map[string]interface{}) http.ErrorResponse {
	resp.SetStatusCode(httpstatus.OK)
	resp.TextResponse(bytes.NewBufferString(req.RequestURI))
	return nil
}
