package http

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	"github.com/kklab-com/gone-httpheadername"
	"github.com/kklab-com/gone/channel"
	"github.com/kklab-com/goth-kklogger"
)

type LogHandler struct {
	channel.DefaultHandler
	printBody  bool
	FilterFunc func(req *Request, resp *Response, params map[string]interface{}) bool
}

var defaultFilter = func(req *Request, resp *Response, params map[string]interface{}) bool { return true }

func NewLogHandler(printBody bool) *LogHandler {
	handler := LogHandler{}
	handler.printBody = printBody
	handler.FilterFunc = defaultFilter
	return &handler
}

func (h *LogHandler) Read(ctx channel.HandlerContext, obj interface{}) {
	ctx.FireRead(obj)
}

func (h *LogHandler) constructReq(req *Request) RequestLogStruct {
	logStruct := RequestLogStruct{
		Method:  req.Method,
		Headers: map[string]interface{}{},
		HOST:    req.Host,
		URI:     req.RequestURI,
	}

	for name, value := range req.Header {
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
		logStruct.Body = string(req.Body)
		bodyLength = len(req.Body)
	}

	logStruct.BodyLength = bodyLength
	return logStruct
}

func (h *LogHandler) constructResp(resp *Response) ResponseLogStruct {
	logStruct := ResponseLogStruct{
		StatusCode: resp.StatusCode(),
		Headers:    map[string]interface{}{},
		URI:        resp.request.RequestURI,
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
			gzBuffer := bytes.NewBuffer(resp.body.Bytes())
			if reader, err := gzip.NewReader(gzBuffer); err == nil {
				defer reader.Close()
				bs, _ := ioutil.ReadAll(reader)
				logStruct.Body = string(bs)
			}
		} else {
			logStruct.Body = resp.body.String()
		}
	}

	logStruct.BodyLength = resp.body.Len()
	return logStruct
}

func (h *LogHandler) Write(ctx channel.HandlerContext, obj interface{}) {
	pack := _UnPack(obj)
	if pack == nil {
		ctx.FireWrite(obj)
		return
	}

	req, resp, params := pack.Req, pack.Resp, pack.Params
	go func(cid string, req *Request, resp *Response, params map[string]interface{}) {
		if !h.FilterFunc(req, resp, params) {
			return
		}

		defer deferError()
		logStruct := LogStruct{
			ChannelID:  cid,
			TrackID:    req.TrackID(),
			Method:     req.Method,
			URI:        req.RequestURI,
			StatusCode: resp.StatusCode(),
			RemoteAddr: req.Request.RemoteAddr,
			RemoteAddrs: func(addrs []string) string {
				sb := strings.Builder{}
				for _, addr := range addrs {
					sb.WriteString(addr + ", ")
				}

				r := sb.String()
				return r[:len(r)-2]
			}(req.RemoteAddrs),
			Request:     h.constructReq(req),
			Response:    h.constructResp(resp),
			AcceptTime:  req.CreatedAt.UnixNano(),
			ProcessTime: time.Now().UnixNano() - req.CreatedAt.UnixNano(),
		}

		if v := params["[gone]h_locate_time"]; v != nil {
			logStruct.HLocateTime = v.(int64)
		}

		if v := params["[gone]h_acceptance_time"]; v != nil {
			logStruct.HAcceptanceTime = v.(int64)
		}

		if v := params["[gone]handler_time"]; v != nil {
			logStruct.HandlerTime = v.(int64)
		}

		if v := params["[gone]h_error_time"]; v != nil {
			logStruct.HErrorTime = v.(int64)
		}

		if v := params["[gone]compress_time"]; v != nil {
			logStruct.CompressTime = v.(int64)
		}

		if v := params["[gone]extend"]; v != nil {
			logStruct.Extend = v
		}

		kklogger.InfoJ("HTTPLog", logStruct)
	}(ctx.Channel().ID(), req, resp, params)

	ctx.FireWrite(obj)
}

func deferError() {
	if err := recover(); err != nil {
		kklogger.ErrorJ("HTTPLog", err)
	}
}

type LogStruct struct {
	ChannelID       string            `json:"cid,omitempty"`
	TrackID         string            `json:"tid,omitempty"`
	Method          string            `json:"method,omitempty"`
	URI             string            `json:"uri,omitempty"`
	StatusCode      int               `json:"status_code,omitempty"`
	RemoteAddr      string            `json:"remote_addr,omitempty"`
	RemoteAddrs     string            `json:"remote_addrs,omitempty"`
	Request         RequestLogStruct  `json:"request,omitempty"`
	Response        ResponseLogStruct `json:"response,omitempty"`
	AcceptTime      int64             `json:"accept_time,omitempty"`
	HLocateTime     int64             `json:"h_locate_time,omitempty"`
	HAcceptanceTime int64             `json:"h_acceptance_time,omitempty"`
	HandlerTime     int64             `json:"handler_time,omitempty"`
	HErrorTime      int64             `json:"h_error_time,omitempty"`
	CompressTime    int64             `json:"compress_time,omitempty"`
	ProcessTime     int64             `json:"process_time,omitempty"`
	Extend          interface{}       `json:"extend,omitempty"`
}

type RequestLogStruct struct {
	URI        string                 `json:"uri,omitempty"`
	Method     string                 `json:"method,omitempty"`
	Headers    map[string]interface{} `json:"headers,omitempty"`
	HOST       string                 `json:"host,omitempty"`
	Body       string                 `json:"body,omitempty"`
	BodyLength int                    `json:"body_length,omitempty"`
}

type ResponseLogStruct struct {
	URI        string                 `json:"uri,omitempty"`
	StatusCode int                    `json:"status_code,omitempty"`
	Headers    map[string]interface{} `json:"headers,omitempty"`
	Body       string                 `json:"body,omitempty"`
	BodyLength int                    `json:"body_length,omitempty"`
}
