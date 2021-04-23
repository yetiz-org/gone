package http

import (
	"fmt"

	"github.com/kklab-com/gone/channel"
	"github.com/kklab-com/goth-erresponse"
)

type HttpTask interface {
	Index(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]interface{}) ErrorResponse
	Get(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]interface{}) ErrorResponse
	Post(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]interface{}) ErrorResponse
	Put(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]interface{}) ErrorResponse
	Delete(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]interface{}) ErrorResponse
	Options(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]interface{}) ErrorResponse
	Patch(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]interface{}) ErrorResponse
	Trace(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]interface{}) ErrorResponse
	Connect(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]interface{}) ErrorResponse
}

type HandlerTask interface {
	GetNodeName(params map[string]interface{}) string
	GetID(name string, params map[string]interface{}) string
}

type HttpHandlerTask interface {
	HttpTask
	PreCheck(req *Request, resp *Response, params map[string]interface{}) ErrorResponse
	Before(req *Request, resp *Response, params map[string]interface{}) ErrorResponse
	After(req *Request, resp *Response, params map[string]interface{}) ErrorResponse
	ErrorCaught(req *Request, resp *Response, params map[string]interface{}, err ErrorResponse) error
}

var NotImplemented = erresponse.NotImplemented

type DefaultHTTPHandlerTask struct {
	DefaultHandlerTask
}

func (h *DefaultHTTPHandlerTask) Index(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]interface{}) ErrorResponse {
	return NotImplemented
}

func (h *DefaultHTTPHandlerTask) Get(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]interface{}) ErrorResponse {
	return nil
}

func (h *DefaultHTTPHandlerTask) Post(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]interface{}) ErrorResponse {
	return nil
}

func (h *DefaultHTTPHandlerTask) Put(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]interface{}) ErrorResponse {
	return nil
}

func (h *DefaultHTTPHandlerTask) Delete(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]interface{}) ErrorResponse {
	return nil
}

func (h *DefaultHTTPHandlerTask) Options(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]interface{}) ErrorResponse {
	return nil
}

func (h *DefaultHTTPHandlerTask) Patch(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]interface{}) ErrorResponse {
	return nil
}

func (h *DefaultHTTPHandlerTask) Trace(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]interface{}) ErrorResponse {
	return nil
}

func (h *DefaultHTTPHandlerTask) Connect(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]interface{}) ErrorResponse {
	return nil
}

func (h *DefaultHTTPHandlerTask) ThrowErrorResponse(err ErrorResponse) {
	panic(err)
}

func (h *DefaultHTTPHandlerTask) PreCheck(req *Request, resp *Response, params map[string]interface{}) ErrorResponse {
	return nil
}

func (h *DefaultHTTPHandlerTask) Before(req *Request, resp *Response, params map[string]interface{}) ErrorResponse {
	return nil
}

func (h *DefaultHTTPHandlerTask) After(req *Request, resp *Response, params map[string]interface{}) ErrorResponse {
	return nil
}

func (h *DefaultHTTPHandlerTask) ErrorCaught(req *Request, resp *Response, params map[string]interface{}, err ErrorResponse) error {
	resp.ResponseError(err)
	return nil
}

type DefaultHandlerTask struct {
}

func NewDefaultHandlerTask() *DefaultHandlerTask {
	return new(DefaultHandlerTask)
}

func (h *DefaultHandlerTask) IsIndex(params map[string]interface{}) string {
	if rtn := params["[gone-http]is_index"]; rtn != nil {
		return rtn.(string)
	}

	return ""
}

func (h *DefaultHandlerTask) GetNodeName(params map[string]interface{}) string {
	if rtn := params["[gone-http]node_name"]; rtn != nil {
		return rtn.(string)
	}

	return ""
}

func (h *DefaultHandlerTask) GetID(name string, params map[string]interface{}) string {
	if rtn := params[fmt.Sprintf("[gone-http]%s_id", name)]; rtn != nil {
		return rtn.(string)
	}

	return ""
}

func (h *DefaultHandlerTask) LogExtend(key string, value interface{}, params map[string]interface{}) {
	if rtn := params["[gone-http]extend"]; rtn == nil {
		rtn = map[string]interface{}{key: value}
		params["[gone-http]extend"] = rtn
	} else {
		rtn.(map[string]interface{})[key] = value
	}
}
