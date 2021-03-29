package http

import (
	"fmt"

	"github.com/kklab-com/gone/channel"
	"github.com/kklab-com/goth-erresponse"
)

type HTTPTask interface {
	Index(req *Request, resp *Response, params map[string]interface{}) ErrorResponse
	Get(req *Request, resp *Response, params map[string]interface{}) ErrorResponse
	Post(req *Request, resp *Response, params map[string]interface{}) ErrorResponse
	Put(req *Request, resp *Response, params map[string]interface{}) ErrorResponse
	Delete(req *Request, resp *Response, params map[string]interface{}) ErrorResponse
	Options(req *Request, resp *Response, params map[string]interface{}) ErrorResponse
	Patch(req *Request, resp *Response, params map[string]interface{}) ErrorResponse
	Trace(req *Request, resp *Response, params map[string]interface{}) ErrorResponse
	Connect(req *Request, resp *Response, params map[string]interface{}) ErrorResponse
}

type HandlerTask interface {
	channel.HandlerTask
	HTTPTask
	PreCheck(req *Request, resp *Response, params map[string]interface{}) ErrorResponse
	Before(req *Request, resp *Response, params map[string]interface{}) ErrorResponse
	After(req *Request, resp *Response, params map[string]interface{}) ErrorResponse
	ErrorCaught(req *Request, resp *Response, params map[string]interface{}, err ErrorResponse) error
}

var NotImplemented = erresponse.NotImplemented

type DefaultHTTPTask struct {
}

func (h *DefaultHTTPTask) Index(req *Request, resp *Response, params map[string]interface{}) ErrorResponse {
	return NotImplemented
}

func (h *DefaultHTTPTask) Get(req *Request, resp *Response, params map[string]interface{}) ErrorResponse {
	return nil
}

func (h *DefaultHTTPTask) Post(req *Request, resp *Response, params map[string]interface{}) ErrorResponse {
	return nil
}

func (h *DefaultHTTPTask) Put(req *Request, resp *Response, params map[string]interface{}) ErrorResponse {
	return nil
}

func (h *DefaultHTTPTask) Delete(req *Request, resp *Response, params map[string]interface{}) ErrorResponse {
	return nil
}

func (h *DefaultHTTPTask) Options(req *Request, resp *Response, params map[string]interface{}) ErrorResponse {
	return nil
}

func (h *DefaultHTTPTask) Patch(req *Request, resp *Response, params map[string]interface{}) ErrorResponse {
	return nil
}

func (h *DefaultHTTPTask) Trace(req *Request, resp *Response, params map[string]interface{}) ErrorResponse {
	return nil
}

func (h *DefaultHTTPTask) Connect(req *Request, resp *Response, params map[string]interface{}) ErrorResponse {
	return nil
}

type DefaultHandlerTask struct {
	DefaultHTTPTask
}

func NewDefaultHandlerTask() *DefaultHandlerTask {
	return new(DefaultHandlerTask)
}

func (h *DefaultHandlerTask) ThrowErrorResponse(err ErrorResponse) {
	panic(err)
}

func (h *DefaultHandlerTask) PreCheck(req *Request, resp *Response, params map[string]interface{}) ErrorResponse {
	return nil
}

func (h *DefaultHandlerTask) Before(req *Request, resp *Response, params map[string]interface{}) ErrorResponse {
	return nil
}

func (h *DefaultHandlerTask) After(req *Request, resp *Response, params map[string]interface{}) ErrorResponse {
	return nil
}

func (h *DefaultHandlerTask) ErrorCaught(req *Request, resp *Response, params map[string]interface{}, err ErrorResponse) error {
	resp.ResponseError(err)
	return nil
}

func (h *DefaultHandlerTask) GetNodeName(params map[string]interface{}) string {
	if rtn := params["[gone]node_name"]; rtn != nil {
		return rtn.(string)
	}

	return ""
}

func (h *DefaultHandlerTask) IsIndex(params map[string]interface{}) string {
	if rtn := params["[gone]is_index"]; rtn != nil {
		return rtn.(string)
	}

	return ""
}

func (h *DefaultHandlerTask) GetID(name string, params map[string]interface{}) string {
	if rtn := params[fmt.Sprintf("[gone]%s_id", name)]; rtn != nil {
		return rtn.(string)
	}

	return ""
}

func (h *DefaultHandlerTask) LogExtend(key string, value interface{}, params map[string]interface{}) {
	if rtn := params["[gone]extend"]; rtn == nil {
		rtn = map[string]interface{}{key: value}
		params["[gone]extend"] = rtn
	} else {
		rtn.(map[string]interface{})[key] = value
	}
}
