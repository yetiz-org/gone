package http

import (
	"fmt"
	"github.com/yetiz-org/gone/channel"
	"github.com/yetiz-org/gone/erresponse"
	"github.com/yetiz-org/gone/http/httpheadername"
	buf "github.com/yetiz-org/goth-bytebuf"
	"net/http"
)

type HttpTask interface {
	Index(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) ErrorResponse
	Get(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) ErrorResponse
	Create(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) ErrorResponse
	Post(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) ErrorResponse
	Put(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) ErrorResponse
	Delete(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) ErrorResponse
	Options(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) ErrorResponse
	Patch(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) ErrorResponse
	Trace(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) ErrorResponse
	Connect(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) ErrorResponse
}

type HandlerTask interface {
	GetNodeName(params map[string]any) string
	GetID(name string, params map[string]any) string
}

type HttpHandlerTask interface {
	HttpTask
	CORSHelper(req *Request, resp *Response, params map[string]any)
	PreCheck(req *Request, resp *Response, params map[string]any) ErrorResponse
	Before(req *Request, resp *Response, params map[string]any) ErrorResponse
	After(req *Request, resp *Response, params map[string]any) ErrorResponse
	ErrorCaught(req *Request, resp *Response, params map[string]any, err ErrorResponse) error
}

type SSEOperation interface {
	WriteHeader(ctx channel.HandlerContext, header http.Header, params map[string]any) channel.Future
	WriteMessage(ctx channel.HandlerContext, message SSEMessage, params map[string]any) channel.Future
	WriteMessages(ctx channel.HandlerContext, messages []SSEMessage, params map[string]any) channel.Future
}

var NotImplemented = erresponse.NotImplemented

type DefaultHTTPHandlerTask struct {
	DefaultHandlerTask
}

func (h *DefaultHTTPHandlerTask) Index(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) ErrorResponse {
	return NotImplemented
}

func (h *DefaultHTTPHandlerTask) Get(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) ErrorResponse {
	return nil
}

func (h *DefaultHTTPHandlerTask) Create(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) ErrorResponse {
	return NotImplemented
}

func (h *DefaultHTTPHandlerTask) Post(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) ErrorResponse {
	return nil
}

func (h *DefaultHTTPHandlerTask) Put(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) ErrorResponse {
	return nil
}

func (h *DefaultHTTPHandlerTask) Delete(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) ErrorResponse {
	return nil
}

func (h *DefaultHTTPHandlerTask) Options(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) ErrorResponse {
	return nil
}

func (h *DefaultHTTPHandlerTask) Patch(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) ErrorResponse {
	return nil
}

func (h *DefaultHTTPHandlerTask) Trace(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) ErrorResponse {
	return nil
}

func (h *DefaultHTTPHandlerTask) Connect(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) ErrorResponse {
	return nil
}

func (h *DefaultHTTPHandlerTask) SSEMode(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) SSEOperation {
	if obj, f := params["[gone-http]context_pack"]; f && obj != nil {
		response := obj.(*Pack).Response
		response.SetHeader(httpheadername.ContentType, "text/event-stream")
		response.SetHeader(httpheadername.CacheControl, "no-cache")
		response.SetHeader(httpheadername.Connection, "keep-alive")
		response.SetHeader(httpheadername.TransferEncoding, "identity")
		obj.(*Pack).writeSeparateMode = true
		return _DefaultSSEOperation
	}

	return nil
}

var _DefaultSSEOperation = &DefaultSSEOperation{}

type DefaultSSEOperation struct {
}

type SSEMessage struct {
	Comment string   `json:"comment"`
	Event   string   `json:"event"`
	Data    []string `json:"data"`
	Id      string   `json:"id"`
	Retry   int      `json:"retry"`
}

func (m SSEMessage) Validate() bool {
	return !(m.Comment == "" && m.Event == "" && len(m.Data) == 0 && m.Id == "" && m.Retry == 0)
}

type SSEMessages []SSEMessage

func (h *DefaultSSEOperation) WriteHeader(ctx channel.HandlerContext, header http.Header, params map[string]any) channel.Future {
	if obj, f := params["[gone-http]context_pack"]; f && obj != nil {
		pack := obj.(*Pack)
		if dispatcher, f := params["[gone-http]dispatcher"]; f {
			pack.Response.header = header
			return dispatcher.(*DispatchHandler).callWriteHeader(ctx, obj).Sync()
		}
	}

	chCtx := channel.NewFuture(ctx.Channel())
	chCtx.Completable().Fail(fmt.Errorf("not found pack"))
	return chCtx
}

func (h *DefaultSSEOperation) WriteMessage(ctx channel.HandlerContext, message SSEMessage, params map[string]any) channel.Future {
	if obj, f := params["[gone-http]context_pack"]; f && obj != nil {
		pack := obj.(*Pack)
		if !pack.Response.headerWritten {
			if !h.WriteHeader(ctx, pack.Response.Header(), params).IsSuccess() {
				chCtx := channel.NewFuture(ctx.Channel())
				chCtx.Completable().Fail(fmt.Errorf("header write error"))
				return chCtx
			}
		}

		body := buf.EmptyByteBuf()
		if message.Comment != "" {
			body.WriteString(fmt.Sprintf(": %s\n", message.Comment))
		}

		if message.Event != "" {
			body.WriteString(fmt.Sprintf("event: %s\n", message.Event))
		}

		if len(message.Data) > 0 {
			for _, datum := range message.Data {
				body.WriteString(fmt.Sprintf("data: %s\n", datum))
			}
		}

		if message.Id != "" {
			body.WriteString(fmt.Sprintf("id: %s\n", message.Id))
		}

		if message.Retry > 0 {
			body.WriteString(fmt.Sprintf("retry: %d\n", message.Retry))
		}

		if body.ReadableBytes() == 0 {
			chCtx := channel.NewFuture(ctx.Channel())
			chCtx.Completable().Fail(fmt.Errorf("message is empty"))
			return chCtx
		}

		body.WriteByte('\n')
		pack.Response.SetBody(body)
		return ctx.Write(obj, channel.NewFuture(ctx.Channel())).Sync()
	}

	chCtx := channel.NewFuture(ctx.Channel())
	chCtx.Completable().Fail(fmt.Errorf("not found pack"))
	return chCtx
}

func (h *DefaultSSEOperation) WriteMessages(ctx channel.HandlerContext, messages []SSEMessage, params map[string]any) channel.Future {
	if len(messages) == 0 {
		chCtx := channel.NewFuture(ctx.Channel())
		chCtx.Completable().Fail(fmt.Errorf("messages is empty"))
		return chCtx
	}

	var chCtx channel.Future
	for _, message := range messages {
		if !message.Validate() {
			chCtx = channel.NewFuture(ctx.Channel())
			chCtx.Completable().Fail(fmt.Errorf("message is empty"))
			return chCtx
		}
	}

	for _, message := range messages {
		chCtx = h.WriteMessage(ctx, message, params)
		if chCtx.IsFail() {
			return chCtx
		}
	}

	return chCtx
}

func (h *DefaultHTTPHandlerTask) ThrowErrorResponse(err ErrorResponse) {
	panic(err)
}

func (h *DefaultHTTPHandlerTask) CORSHelper(req *Request, resp *Response, params map[string]any) {
	if req.Origin() == "null" {
		resp.Header().Set(httpheadername.AccessControlAllowOrigin, "*")
	} else {
		resp.Header().Set(httpheadername.AccessControlAllowOrigin, req.Origin())
	}

	if str := req.Header().Get(httpheadername.AccessControlRequestHeaders); str != "" {
		resp.Header().Set(httpheadername.AccessControlAllowHeaders, str)
	}

	if str := req.Header().Get(httpheadername.AccessControlRequestMethod); str != "" {
		resp.Header().Set(httpheadername.AccessControlAllowMethods, str)
	}
}

func (h *DefaultHTTPHandlerTask) PreCheck(req *Request, resp *Response, params map[string]any) ErrorResponse {
	return nil
}

func (h *DefaultHTTPHandlerTask) Before(req *Request, resp *Response, params map[string]any) ErrorResponse {
	return nil
}

func (h *DefaultHTTPHandlerTask) After(req *Request, resp *Response, params map[string]any) ErrorResponse {
	return nil
}

func (h *DefaultHTTPHandlerTask) ErrorCaught(req *Request, resp *Response, params map[string]any, err ErrorResponse) error {
	resp.ResponseError(err)
	return nil
}

type DefaultHandlerTask struct {
}

func NewDefaultHandlerTask() *DefaultHandlerTask {
	return new(DefaultHandlerTask)
}

func (h *DefaultHandlerTask) IsIndex(params map[string]any) bool {
	if rtn := params["[gone-http]is_index"]; rtn != nil {
		if is, ok := rtn.(bool); ok && is {
			return true
		}
	}

	return false
}

func (h *DefaultHandlerTask) GetNodeName(params map[string]any) string {
	if rtn := params["[gone-http]node_name"]; rtn != nil {
		return rtn.(string)
	}

	return ""
}

func (h *DefaultHandlerTask) GetID(name string, params map[string]any) string {
	if rtn := params[fmt.Sprintf("[gone-http]%s_id", name)]; rtn != nil {
		return rtn.(string)
	}

	return ""
}

func (h *DefaultHandlerTask) LogExtend(key string, value any, params map[string]any) {
	if rtn := params["[gone-http]extend"]; rtn == nil {
		rtn = map[string]any{key: value}
		params["[gone-http]extend"] = rtn
	} else {
		rtn.(map[string]any)[key] = value
	}
}
