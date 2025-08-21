package ghttp

import (
	"compress/gzip"
	"fmt"
	"strings"
	"time"

	"github.com/yetiz-org/gone/channel"
	"github.com/yetiz-org/gone/ghttp/httpheadername"
	buf "github.com/yetiz-org/goth-bytebuf"
	"github.com/yetiz-org/goth-kklogger"
)

const LogHandlerDefaultMaxBodySize = 204800

type LogHandler struct {
	channel.DefaultHandler
	printBody   bool
	MaxBodySize int
	FilterFunc  func(req *Request, resp *Response, params map[string]any) bool
}

var defaultFilter = func(req *Request, resp *Response, params map[string]any) bool { return true }

func NewLogHandler(printBody bool) *LogHandler {
	handler := LogHandler{}
	handler.printBody = printBody
	handler.MaxBodySize = LogHandlerDefaultMaxBodySize
	handler.FilterFunc = defaultFilter
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
			URI:     pack.Request.RequestURI(),
			Request: h.constructReq(pack.Request),
		}

		kklogger.InfoJ("ghttp:LogHandler.Read#log_request!read", logStruct)
	}

	ctx.FireRead(obj)
}

func (h *LogHandler) constructReq(req *Request) *RequestLogStruct {
	logStruct := RequestLogStruct{
		Method:  req.Method(),
		Headers: map[string]any{},
		HOST:    req.Host(),
		URI:     req.RequestURI(),
	}

	for name, value := range req.Header() {
		valStr := ""
		if len(value) > 1 {
			for i := 0; i < len(value); i++ {
				if i == 0 {
					valStr = value[0]
				} else {
					valStr = fmt.Sprintf("%s;%s", valStr, value[i])
				}
			}
		} else {
			valStr = value[0]
		}

		logStruct.Headers[name] = valStr
	}

	bodyLength := 0
	if h.printBody {
		if len(req.Body().Bytes()) > h.MaxBodySize {
			logStruct.Body = string(req.Body().Bytes()[:h.MaxBodySize])
		} else {
			logStruct.Body = string(req.Body().Bytes())
		}

		bodyLength = req.Body().ReadableBytes()
	}

	logStruct.BodyLength = bodyLength
	return &logStruct
}

func (h *LogHandler) constructResp(resp *Response) *ResponseLogStruct {
	logStruct := ResponseLogStruct{
		StatusCode: resp.StatusCode(),
		Headers:    map[string]any{},
		URI:        resp.request.RequestURI(),
	}

	for name, value := range resp.Header() {
		valStr := ""
		if len(value) > 1 {
			for i := 0; i < len(value); i++ {
				if i == 0 {
					valStr = value[0]
				} else {
					valStr = fmt.Sprintf("%s;%s", valStr, value[i])
				}
			}
		} else {
			valStr = value[0]
		}

		logStruct.Headers[name] = valStr
	}

	if h.printBody {
		if resp.GetHeader(httpheadername.ContentEncoding) == "gzip" {
			if reader, err := gzip.NewReader(buf.NewByteBuf(resp.body.Bytes())); err == nil {
				defer reader.Close()
				bus := buf.EmptyByteBuf().WriteReader(reader)
				if len(bus.Bytes()) > h.MaxBodySize {
					logStruct.Body = string(bus.Bytes()[:h.MaxBodySize])
				} else {
					logStruct.Body = string(bus.Bytes())
				}

				logStruct.PreCompressLength = len(logStruct.Body)
			}
		} else {
			if len(resp.body.Bytes()) > h.MaxBodySize {
				logStruct.Body = string(resp.body.Bytes()[:h.MaxBodySize])
			} else {
				logStruct.Body = string(resp.body.Bytes())
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
			URI:        req.RequestURI(),
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
			URI:        req.RequestURI(),
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
