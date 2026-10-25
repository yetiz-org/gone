package ghttp

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/yetiz-org/gone/channel"
	"github.com/yetiz-org/gone/erresponse"
	"github.com/yetiz-org/gone/ghttp/httpheadername"
	buf "github.com/yetiz-org/goth-bytebuf"
)

type HttpTask interface {
	Index(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) ErrorResponse
	Get(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) ErrorResponse
	Head(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) ErrorResponse
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
	Register()
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

// AutoRangeSupporter enables automatic HTTP Range request handling by the dispatcher
type AutoRangeSupporter interface {
	EnableAutoRangeSupport() bool
}

// PreCheckSkipOptions allows skipping PreCheck for OPTIONS method
type PreCheckSkipOptions interface {
	SkipPreCheckForOptions() bool
}

// RequestBodyStreamer lets a task read Request().Body as a stream. When StreamRequestBody reports true,
// the dispatcher skips buffering the body before the acceptances run, so the task owns reading it and
// handling an *http.MaxBytesError; calling Request.Body still buffers it on demand.
type RequestBodyStreamer interface {
	StreamRequestBody() bool
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

func (h *DefaultHTTPHandlerTask) Head(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) ErrorResponse {
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
	if obj, f := params["[gone-http]context_pack"]; f && obj != nil && obj.(*Pack).Writer != nil {
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

// RawMode hands the raw http.ResponseWriter to the handler. After it returns ok,
// ghttp writes nothing else for this response; the handler owns status, headers and body.
func (h *DefaultHTTPHandlerTask) RawMode(req *Request, resp *Response, params map[string]any) (writer http.ResponseWriter, ok bool) {
	pack, ok := params["[gone-http]context_pack"].(*Pack)
	if !ok || pack.Writer == nil || (pack.writeSeparateMode && !pack.rawMode) {
		return nil, false
	}

	pack.writeSeparateMode = true
	pack.rawMode = true
	pack.Response.headerWritten = true
	return pack.Writer, true
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

func (m SSEMessage) validate() (rtErr error) {
	if strings.ContainsAny(m.Comment, "\r\n") {
		return fmt.Errorf("invalid SSE comment")
	}
	if strings.ContainsAny(m.Event, "\r\n") {
		return fmt.Errorf("invalid SSE event")
	}
	if strings.ContainsAny(m.Id, "\r\n\x00") {
		return fmt.Errorf("invalid SSE id")
	}

	if m.Comment == "" && m.Event == "" && len(m.Data) == 0 && m.Id == "" && m.Retry <= 0 {
		return fmt.Errorf("message is empty")
	}

	return nil
}

func (m SSEMessage) encode() (body buf.ByteBuf) {
	body = buf.EmptyByteBuf()
	if m.Comment != "" {
		body.WriteString(": " + m.Comment + "\n")
	}
	if m.Event != "" {
		body.WriteString("event: " + m.Event + "\n")
	}
	for _, datum := range m.Data {
		datum = strings.ReplaceAll(strings.ReplaceAll(datum, "\r\n", "\n"), "\r", "\n")
		for line := range strings.SplitSeq(datum, "\n") {
			body.WriteString("data: " + line + "\n")
		}
	}
	if m.Id != "" {
		body.WriteString("id: " + m.Id + "\n")
	}
	if m.Retry > 0 {
		body.WriteString("retry: " + strconv.Itoa(m.Retry) + "\n")
	}
	body.WriteByte('\n')
	return body
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

func (h *DefaultSSEOperation) WriteMessage(ctx channel.HandlerContext, message SSEMessage, params map[string]any) (future channel.Future) {
	return h.WriteMessages(ctx, []SSEMessage{message}, params)
}

func (h *DefaultSSEOperation) WriteMessages(ctx channel.HandlerContext, messages []SSEMessage, params map[string]any) (future channel.Future) {
	future = channel.NewFuture(ctx.Channel())
	if len(messages) == 0 {
		future.Completable().Fail(fmt.Errorf("messages is empty"))
		return future
	}

	for _, message := range messages {
		if err := message.validate(); err != nil {
			future.Completable().Fail(err)
			return future
		}
	}

	obj := params["[gone-http]context_pack"]
	if obj == nil {
		future.Completable().Fail(fmt.Errorf("not found pack"))
		return future
	}
	pack := obj.(*Pack)
	if !pack.Response.headerWritten {
		if !h.WriteHeader(ctx, pack.Response.Header(), params).IsSuccess() {
			future.Completable().Fail(fmt.Errorf("header write error"))
			return future
		}
	}

	for _, message := range messages {
		pack.Response.SetBody(message.encode())
		future = ctx.Write(obj, channel.NewFuture(ctx.Channel())).Sync()
		if future.IsFail() {
			return future
		}
	}

	return future
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

func (h *DefaultHTTPHandlerTask) EnableAutoRangeSupport() bool {
	return true
}

func (h *DefaultHTTPHandlerTask) SkipPreCheckForOptions() bool {
	return true
}

// ParseRange parses HTTP Range header and returns start, end, valid
func ParseRange(rangeHeader string, contentSize int64) (rangeStart, rangeEnd int64, valid bool) {
	if contentSize <= 0 {
		return 0, 0, false
	}

	rangeSpec, found := strings.CutPrefix(rangeHeader, "bytes=")
	if !found {
		return 0, 0, false
	}

	if strings.Contains(rangeSpec, ",") {
		return 0, 0, false
	}

	parts := strings.Split(rangeSpec, "-")
	if len(parts) != 2 {
		return 0, 0, false
	}

	var start, end int64
	var err error

	if parts[0] == "" {
		if parts[1] == "" {
			return 0, 0, false
		}
		suffixLength, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil || suffixLength <= 0 {
			return 0, 0, false
		}
		if suffixLength > contentSize {
			suffixLength = contentSize
		}
		start = contentSize - suffixLength
		end = contentSize - 1
	} else if parts[1] == "" {
		start, err = strconv.ParseInt(parts[0], 10, 64)
		if err != nil || start < 0 {
			return 0, 0, false
		}
		if start >= contentSize {
			return 0, 0, false
		}
		end = contentSize - 1
	} else {
		start, err = strconv.ParseInt(parts[0], 10, 64)
		if err != nil || start < 0 {
			return 0, 0, false
		}
		end, err = strconv.ParseInt(parts[1], 10, 64)
		if err != nil || end < start || start >= contentSize {
			return 0, 0, false
		}
		if end >= contentSize {
			end = contentSize - 1
		}
	}

	return start, end, true
}

type DefaultHandlerTask struct {
}

func NewDefaultHandlerTask() *DefaultHandlerTask {
	return new(DefaultHandlerTask)
}

func (h *DefaultHandlerTask) Register() {
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
	// 1. Brace syntax: {param} stores as [gone-http]p:param
	if rtn := params[fmt.Sprintf("[gone-http]p:%s", name)]; rtn != nil {
		return rtn.(string)
	}

	// 2. Colon syntax: :param stores as [gone-http]param
	if rtn := params[fmt.Sprintf("[gone-http]%s", name)]; rtn != nil {
		return rtn.(string)
	}

	// 3. Default syntax: node name stores as [gone-http]name_id
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
