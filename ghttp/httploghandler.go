package ghttp

import (
	"compress/gzip"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/yetiz-org/gone/channel"
	"github.com/yetiz-org/gone/ghttp/httpheadername"
	buf "github.com/yetiz-org/goth-bytebuf"
	"github.com/yetiz-org/goth-kklogger"
)

const LogHandlerDefaultMaxBodySize = 204800
const LogHandlerDefaultMaxReqBodySize = 100000
const LogHandlerDefaultMaxRespBodySize = 100000

// LogHandlerRedacted replaces the values of redacted headers in logs.
const LogHandlerRedacted = "[REDACTED]"

// LogHandlerDefaultRedactHeaders lists the credential headers NewLogHandler redacts.
var LogHandlerDefaultRedactHeaders = []string{"Authorization", "Proxy-Authorization", "Cookie", "Set-Cookie"}

type LogHandler struct {
	channel.DefaultHandler
	printBody       bool
	ReqMaxBodySize  int
	RespMaxBodySize int
	FilterFunc      func(req *Request, resp *Response, params map[string]any) bool
	// RedactHeaders lists headers, matched case-insensitively, whose values are logged as LogHandlerRedacted.
	RedactHeaders []string
}

var defaultFilter = func(req *Request, resp *Response, params map[string]any) bool { return true }

func NewLogHandler(printBody bool) (logHandler *LogHandler) {
	handler := LogHandler{}
	handler.printBody = printBody
	handler.ReqMaxBodySize = LogHandlerDefaultMaxReqBodySize
	handler.RespMaxBodySize = LogHandlerDefaultMaxRespBodySize
	handler.FilterFunc = defaultFilter
	handler.RedactHeaders = slices.Clone(LogHandlerDefaultRedactHeaders)
	return &handler
}

func (h *LogHandler) Read(ctx channel.HandlerContext, obj any) {
	pack := _UnPack(obj)
	if pack != nil {
		if !h.FilterFunc(pack.Request, pack.Response, pack.Params) {
			ctx.FireRead(obj)
			return
		}

		logStruct := ReadRequestLogStruct{
			ChannelID:  pack.Request.Channel().ID(),
			TrackID:    pack.Request.TrackID(),
			RemoteAddr: pack.Request.Request().RemoteAddr,
			RemoteAddrs: func(addrs []string) string {
				sb := strings.Builder{}
				for _, addr := range addrs {
					sb.WriteString(addr + ", ")
				}

				r := sb.String()
				return r[:len(r)-2]
			}(pack.Request.RemoteAddrs()),
			URI:     pack.Request.LogURI(),
			Request: h.constructReq(pack.Request),
		}

		kklogger.InfoJ("ghttp:LogHandler.Read#log_request!read", logStruct)
	}

	ctx.FireRead(obj)
}

func (h *LogHandler) constructReq(req *Request) (reqLog *RequestLogStruct) {
	logStruct := RequestLogStruct{
		Method:  req.Method(),
		Headers: h.logHeaders(req.Header()),
		HOST:    req.Host(),
		URI:     req.LogURI(),
	}

	bodyLength := 0
	if h.printBody {
		body := req.Body()
		bs := body.Bytes()
		if len(bs) > h.ReqMaxBodySize {
			logStruct.Body = string(bs[:h.ReqMaxBodySize])
		} else {
			logStruct.Body = string(bs)
		}

		bodyLength = body.ReadableBytes()
	}

	logStruct.BodyLength = bodyLength
	return &logStruct
}

// logHeaders joins each header's values with ";" and redacts the RedactHeaders values.
func (h *LogHandler) logHeaders(header http.Header) (headers map[string]any) {
	headers = map[string]any{}
	for name, value := range header {
		if slices.ContainsFunc(h.RedactHeaders, func(redact string) bool { return strings.EqualFold(redact, name) }) {
			headers[name] = LogHandlerRedacted
			continue
		}

		headers[name] = strings.Join(value, ";")
	}

	return headers
}

func (h *LogHandler) constructResp(resp *Response) (respLog *ResponseLogStruct) {
	logStruct := ResponseLogStruct{
		StatusCode: resp.StatusCode(),
		Headers:    h.logHeaders(resp.Header()),
		URI:        resp.request.LogURI(),
	}

	if h.printBody {
		maxBodySize := h.RespMaxBodySize
		if resp.GetHeader(httpheadername.ContentEncoding) == "gzip" {
			if reader, err := gzip.NewReader(buf.NewSharedByteBuf(resp.body.Bytes())); err == nil {
				defer reader.Close()
				bus := buf.EmptyByteBuf().WriteReader(reader)
				decoded := bus.Bytes()
				if len(decoded) > maxBodySize {
					logStruct.Body = string(decoded[:maxBodySize])
				} else {
					logStruct.Body = string(decoded)
				}

				logStruct.PreCompressLength = len(logStruct.Body)
			}
		} else {
			bs := resp.body.Bytes()
			if len(bs) > maxBodySize {
				logStruct.Body = string(bs[:maxBodySize])
			} else {
				logStruct.Body = string(bs)
			}
		}
	}

	logStruct.OutBodyLength = resp.body.ReadableBytes()
	return &logStruct
}

func (h *LogHandler) Write(ctx channel.HandlerContext, obj any, future channel.Future) {
	pack := _UnPack(obj)
	if pack == nil {
		ctx.Write(obj, future)
		return
	}

	req, resp, params := pack.Request, pack.Response, pack.Params
	//go func(cid string, req *Request, resp *Response, params map[string]any) {
	if !h.FilterFunc(req, resp, params) {
		ctx.Write(obj, future)
		return
	}

	defer deferError()
	if !pack.writeSeparateMode {
		logStruct := LogStruct{
			ChannelID:  ctx.Channel().ID(),
			TrackID:    req.TrackID(),
			Method:     req.Method(),
			URI:        req.LogURI(),
			StatusCode: resp.StatusCode(),
			RemoteAddr: req.Request().RemoteAddr,
			RemoteAddrs: func(addrs []string) string {
				sb := strings.Builder{}
				for _, addr := range addrs {
					sb.WriteString(addr + ", ")
				}

				r := sb.String()
				return r[:len(r)-2]
			}(req.RemoteAddrs()),
			Request:     h.constructReq(req),
			Response:    h.constructResp(resp),
			AcceptTime:  req.CreatedAt().UnixNano(),
			ProcessTime: time.Now().UnixNano() - req.CreatedAt().UnixNano(),
		}

		if v := params["[gone-http]h_locate_time"]; v != nil {
			logStruct.HLocateTime = v.(int64)
		}

		if v := params["[gone-http]h_acceptance_time"]; v != nil {
			logStruct.HAcceptanceTime = v.(int64)
		}

		if v := params["[gone-http]handler_time"]; v != nil {
			logStruct.HandlerTime = v.(int64)
		}

		if v := params["[gone-http]h_error_time"]; v != nil {
			logStruct.HErrorTime = v.(int64)
		}

		if v := params["[gone-http]compress_time"]; v != nil {
			logStruct.CompressTime = v.(int64)
		}

		if v := params["[gone-http]extend"]; v != nil {
			logStruct.Extend = v
		}

		kklogger.InfoJ("ghttp:LogHandler.Write#log_response!write", logStruct)
	} else {
		logStruct := LogStruct{
			ChannelID:  ctx.Channel().ID(),
			TrackID:    req.TrackID(),
			Method:     req.Method(),
			URI:        req.LogURI(),
			StatusCode: resp.StatusCode(),
			RemoteAddr: req.Request().RemoteAddr,
			RemoteAddrs: func(addrs []string) string {
				sb := strings.Builder{}
				for _, addr := range addrs {
					sb.WriteString(addr + ", ")
				}

				r := sb.String()
				return r[:len(r)-2]
			}(req.RemoteAddrs()),
			Response:    h.constructResp(resp),
			AcceptTime:  req.CreatedAt().UnixNano(),
			ProcessTime: time.Now().UnixNano() - req.CreatedAt().UnixNano(),
		}

		if v := params["[gone-http]h_locate_time"]; v != nil {
			logStruct.HLocateTime = v.(int64)
		}

		if v := params["[gone-http]h_acceptance_time"]; v != nil {
			logStruct.HAcceptanceTime = v.(int64)
		}

		if v := params["[gone-http]extend"]; v != nil {
			logStruct.Extend = v
		}

		kklogger.InfoJ("ghttp:LogHandler.Write#log_response!write", logStruct)
	}

	ctx.Write(obj, future)
}

func deferError() {
	if err := recover(); err != nil {
		kklogger.ErrorJ("ghttp:LogHandler.deferError#defer_error!error", err)
	}
}

type ReadRequestLogStruct struct {
	ChannelID   string            `json:"cid,omitempty"`
	TrackID     string            `json:"tid,omitempty"`
	RemoteAddr  string            `json:"remote_addr,omitempty"`
	RemoteAddrs string            `json:"remote_addrs,omitempty"`
	URI         string            `json:"uri,omitempty"`
	Request     *RequestLogStruct `json:"request"`
}

type LogStruct struct {
	ChannelID       string             `json:"cid,omitempty"`
	TrackID         string             `json:"tid,omitempty"`
	Method          string             `json:"method,omitempty"`
	URI             string             `json:"uri,omitempty"`
	StatusCode      int                `json:"status_code,omitempty"`
	RemoteAddr      string             `json:"remote_addr,omitempty"`
	RemoteAddrs     string             `json:"remote_addrs,omitempty"`
	Request         *RequestLogStruct  `json:"request,omitempty"`
	Response        *ResponseLogStruct `json:"response,omitempty"`
	AcceptTime      int64              `json:"accept_time,omitempty"`
	HLocateTime     int64              `json:"h_locate_time,omitempty"`
	HAcceptanceTime int64              `json:"h_acceptance_time,omitempty"`
	HandlerTime     int64              `json:"handler_time,omitempty"`
	HErrorTime      int64              `json:"h_error_time,omitempty"`
	CompressTime    int64              `json:"compress_time,omitempty"`
	ProcessTime     int64              `json:"process_time,omitempty"`
	Extend          any                `json:"extend,omitempty"`
}

type RequestLogStruct struct {
	URI        string         `json:"uri,omitempty"`
	Method     string         `json:"method,omitempty"`
	Headers    map[string]any `json:"headers,omitempty"`
	HOST       string         `json:"host,omitempty"`
	Body       string         `json:"body,omitempty"`
	BodyLength int            `json:"body_length,omitempty"`
}

type ResponseLogStruct struct {
	URI               string         `json:"uri,omitempty"`
	StatusCode        int            `json:"status_code,omitempty"`
	Headers           map[string]any `json:"headers,omitempty"`
	Body              string         `json:"body,omitempty"`
	OutBodyLength     int            `json:"out_body_length,omitempty"`
	PreCompressLength int            `json:"pre_compress_length,omitempty"`
}
