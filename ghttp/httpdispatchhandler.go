package ghttp

import (
	"errors"
	"fmt"
	"maps"
	"net"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/yetiz-org/gone/channel"
	"github.com/yetiz-org/gone/erresponse"
	"github.com/yetiz-org/gone/ghttp/httpheadername"
	"github.com/yetiz-org/gone/ghttp/httpsession"
	"github.com/yetiz-org/gone/ghttp/httpsession/memory"
	"github.com/yetiz-org/gone/ghttp/httpstatus"
	buf "github.com/yetiz-org/goth-bytebuf"
	kklogger "github.com/yetiz-org/goth-kklogger"
	kkpanic "github.com/yetiz-org/goth-panic"
	"github.com/yetiz-org/goth-util/hash"
)

type DispatchHandler struct {
	channel.DefaultHandler
	route                 Route
	DefaultStatusCode     int
	DefaultStatusResponse map[int]func(req *Request, resp *Response, params map[string]any)
}

func NewDispatchHandler(route Route) *DispatchHandler {
	return &DispatchHandler{route: route, DefaultStatusCode: 200, DefaultStatusResponse: map[int]func(req *Request, resp *Response, params map[string]any){}}
}

func (h *DispatchHandler) defaultNotFound404(req *Request, resp *Response, params map[string]any) {
	resp.SetStatusCode(httpstatus.NotFound)
	resp.SetBody(buf.NewByteBuf([]byte("<html><img src='https://http.cat/404' /></html>")))
}

func (h *DispatchHandler) Read(ctx channel.HandlerContext, obj any) {
	pack := _UnPack(obj)
	if pack == nil {
		ctx.FireRead(obj)
		return
	}

	request, response := pack.Request, pack.Response
	node, isLast, found := h._Locate(pack)
	if !found {
		defer h.callWrite(ctx, obj)
		defer h._UpdateSessionCookie(response)
		if upgrade := request.Header().Get(httpheadername.Upgrade); upgrade != "" {
			response.Header().Set(httpheadername.Upgrade, upgrade)
		}

		if connection := request.Header().Get(httpheadername.Connection); connection != "" {
			response.Header().Set(httpheadername.Connection, connection)
		}

		response.SetStatusCode(404)
		kklogger.WarnJ("ghttp:DispatchHandler.Read#endpoint_not_exist!not_found", ObjectLogStruct{
			ChannelID:  ctx.Channel().ID(),
			TrackID:    request.TrackID(),
			URI:        request.RequestURI(),
			RemoteAddr: request.Request().RemoteAddr,
		})

		return
	}

	task, ok := node.HandlerTask().(HttpHandlerTask)
	if !ok {
		ctx.FireRead(obj)
		return
	}

	defer h.callWrite(ctx, obj)
	defer h._UpdateSessionCookie(response)
	h._Run(ctx, task, node, pack, isLast)
}

// Dispatch serves r in process through the route, acceptances and handler task and returns its pack.
// seed is copied into the params before acceptances run. The pack has no writer, so nothing is written
// to the network, RawMode and SSEMode are refused, CORSHelper and DefaultStatusResponse are skipped, and
// the session lives in a private store that is never shared. The caller bounds r's body; Dispatch
// returns nil only when that body exceeds an http.MaxBytesReader limit.
func (h *DispatchHandler) Dispatch(ctx channel.HandlerContext, r *http.Request, seed map[string]any) (pack *Pack) {
	request := WrapRequest(ctx.Channel(), r)
	if request == nil {
		return nil
	}

	request.session = httpsession.NewDefaultSession(memory.NewSessionProvider())
	pack = &Pack{Request: request, Response: NewResponse(request), Params: map[string]any{}}
	maps.Copy(pack.Params, seed)
	if request.BodyReadError() != nil {
		pack.Response.SetStatusCode(httpstatus.BadRequest)
		return pack
	}

	node, isLast, found := h._Locate(pack)
	if !found {
		pack.Response.SetStatusCode(httpstatus.NotFound)
		return pack
	}

	task, ok := node.HandlerTask().(HttpHandlerTask)
	if !ok {
		pack.Response.SetStatusCode(httpstatus.NotFound)
		return pack
	}

	h._Run(ctx, task, node, pack, isLast)
	if pack.Response.StatusCode() == 0 {
		pack.Response.SetStatusCode(h.DefaultStatusCode)
	}

	return pack
}

// _Locate resolves the endpoint node for the pack path and records the locate time. A matched node
// and its route parameters are bound to the pack; found is false for an unknown path or a group node.
func (h *DispatchHandler) _Locate(pack *Pack) (node RouteNode, isLast bool, found bool) {
	timeMark := time.Now()
	node, nodeParams, isLast := h.route.RouteNode(pack.Request.Url().Path)
	pack.Params["[gone-http]h_locate_time"] = time.Since(timeMark).Nanoseconds()
	if node == nil || node.RouteType() == RouteTypeGroup {
		return nil, false, false
	}

	pack.RouteNode = node
	pack.Params["[gone-http]node"] = node
	pack.Params["[gone-http]node_name"] = node.Name()
	pack.Params["[gone-http]is_index"] = isLast
	pack.Params["[gone-http]dispatcher"] = h
	pack.Params["[gone-http]context_pack"] = pack
	if nodeParams != nil {
		maps.Copy(pack.Params, nodeParams)
	}

	return node, isLast, true
}

// _Run runs the node acceptances and then the task method with panic recovery. A rejecting
// acceptance leaves its error on the response. CORSHelper runs only for packs with a network writer.
func (h *DispatchHandler) _Run(ctx channel.HandlerContext, task HttpHandlerTask, node RouteNode, pack *Pack, isLast bool) {
	request, response, params := pack.Request, pack.Response, pack.Params
	var rtnCatch ReturnCatch
	defer h._PanicCatch(ctx, request, response, task, params, &rtnCatch)
	if pack.Writer != nil {
		defer task.CORSHelper(request, response, params)
	}

	timeMark := time.Now()
	for _, acceptance := range node.AggregatedAcceptances() {
		if request.Method() == MethodOptions && acceptance.SkipMethodOptions() {
			continue
		}

		if err := acceptance.Do(ctx, request, response, params); err != nil {
			if err == AcceptanceInterrupt {
				if kklogger.GetLogLevel() >= kklogger.TraceLevel {
					kklogger.TraceJ("ghttp:DispatchHandler.Acceptance#acceptance!trace", ObjectLogStruct{
						ChannelID:  ctx.Channel().ID(),
						TrackID:    request.TrackID(),
						State:      "Skip",
						URI:        request.RequestURI(),
						Handler:    reflect.TypeOf(acceptance).String(),
						RemoteAddr: request.Request().RemoteAddr,
					})
				}

				return
			}

			params["[gone-http]h_acceptance_time"] = time.Since(timeMark).Nanoseconds()
			kklogger.WarnJ("ghttp:DispatchHandler.Acceptance#acceptance!warn", ObjectLogStruct{
				ChannelID:  ctx.Channel().ID(),
				TrackID:    request.TrackID(),
				State:      "Fail",
				URI:        request.RequestURI(),
				Handler:    reflect.TypeOf(acceptance).String(),
				Message:    err.Error(),
				RemoteAddr: request.Request().RemoteAddr,
			})

			if cast, ok := err.(ErrorResponse); ok {
				if response.statusCode == 0 {
					response.ResponseError(cast)
				}
			} else if response.statusCode == 0 {
				response.SetStatusCode(httpstatus.BadRequest)
			}

			return
		} else {
			if kklogger.GetLogLevel() >= kklogger.TraceLevel {
				kklogger.TraceJ("ghttp:DispatchHandler.Acceptance#acceptance!trace", ObjectLogStruct{
					ChannelID:  ctx.Channel().ID(),
					TrackID:    request.TrackID(),
					State:      "Pass",
					URI:        request.RequestURI(),
					Handler:    reflect.TypeOf(acceptance).String(),
					RemoteAddr: request.Request().RemoteAddr,
				})
			}
		}
	}

	params["[gone-http]h_acceptance_time"] = time.Since(timeMark).Nanoseconds()
	timeMark = time.Now()
	rtnCatch.err = h.invokeMethod(ctx, task, request, response, params, isLast)
	params["[gone-http]handler_time"] = time.Since(timeMark).Nanoseconds()
}

func (h *DispatchHandler) callWrite(ctx channel.HandlerContext, obj any) channel.Future {
	pack := _UnPack(obj)
	if pack.writeSeparateMode {
		chCtx := channel.NewFuture(ctx.Channel())
		chCtx.Completable().Complete(obj)
		return chCtx
	}

	if ff, f := h.DefaultStatusResponse[pack.Response.StatusCode()]; f {
		if pack.Response.body.ReadableBytes() == 0 {
			ff(pack.Request, pack.Response, pack.Params)
		}
	} else if pack.Response.StatusCode() == 404 {
		if pack.Response.body.ReadableBytes() == 0 {
			h.defaultNotFound404(pack.Request, pack.Response, pack.Params)
		}
	}

	if pack.Response.StatusCode() == 0 {
		pack.Response.SetStatusCode(h.DefaultStatusCode)
	}

	return ctx.Write(obj, pack.Response.done).Sync()
}

func (h *DispatchHandler) callWriteHeader(ctx channel.HandlerContext, obj any) channel.Future {
	pack := _UnPack(obj)
	chCtx := channel.NewFuture(ctx.Channel())
	if pack == nil {
		chCtx.Completable().Fail(fmt.Errorf("not found pack"))
		return chCtx
	}

	if pack.Response.headerWritten {
		chCtx.Completable().Complete(obj)
		return chCtx
	}

	if ff, f := h.DefaultStatusResponse[pack.Response.StatusCode()]; f {
		if pack.Response.body.ReadableBytes() == 0 {
			ff(pack.Request, pack.Response, pack.Params)
		}
	} else if pack.Response.StatusCode() == 404 {
		if pack.Response.body.ReadableBytes() == 0 {
			h.defaultNotFound404(pack.Request, pack.Response, pack.Params)
		}
	}

	if pack.Response.StatusCode() == 0 {
		pack.Response.SetStatusCode(h.DefaultStatusCode)
	}

	return ctx.Write(obj, chCtx)
}

func (h *DispatchHandler) _PanicCatch(ctx channel.HandlerContext, request *Request, response *Response, task HttpHandlerTask, params map[string]any, rtnCatch *ReturnCatch) {
	erErr := rtnCatch.err
	timeMark := time.Now()
	var err error
	if r := recover(); r != nil {
		erErr = erresponse.ServerErrorPanic
		switch er := r.(type) {
		case ErrorResponse:
			erErr = er
			err = er
		case *kkpanic.CaughtImpl:
			err = er
		default:
			err = kkpanic.Convert(er)
		}

		h.ErrorCaught(ctx, err)
		kklogger.ErrorJ("ghttp:DispatchHandler.Read#error_caught!error", ObjectLogStruct{
			ChannelID:  ctx.Channel().ID(),
			TrackID:    request.TrackID(),
			URI:        request.RequestURI(),
			Handler:    reflect.TypeOf(task).String(),
			RemoteAddr: request.Request().RemoteAddr,
			Message:    err,
		})
	}

	if erErr != nil {
		erErr = &ErrorResponseImpl{
			ErrorResponse: erErr.Clone(),
		}

		if err != nil {
			if erc, ok := err.(*kkpanic.CaughtImpl); ok {
				erErr.(*ErrorResponseImpl).Caught = erc
			} else {
				erErr.(*ErrorResponseImpl).Caught = kkpanic.Convert(err)
			}
		}

		erErr.ErrorData()["cid"] = request.Channel().ID()
		erErr.ErrorData()["tid"] = request.TrackID()
		timeMark = time.Now()
		err := task.ErrorCaught(request, response, params, erErr)
		params["[gone-http]h_error_time"] = time.Now().Sub(timeMark).Nanoseconds()
		if err != nil {
			h.ErrorCaught(ctx, err)
		}
	}
}

type ReturnCatch struct {
	err ErrorResponse
}

func (h *DispatchHandler) invokeMethod(ctx channel.HandlerContext, task HttpHandlerTask, request *Request, response *Response, params map[string]any, isLast bool) ErrorResponse {
	// Check if we should skip PreCheck for OPTIONS method
	// Default behavior: if task doesn't implement PreCheckSkipOptions, PreCheck will be executed
	shouldSkipPreCheck := false
	if skipper, ok := task.(PreCheckSkipOptions); ok {
		shouldSkipPreCheck = skipper.SkipPreCheckForOptions() && request.Method() == MethodOptions
	}

	if !shouldSkipPreCheck {
		if err := task.PreCheck(request, response, params); err != nil {
			return err
		}
	}

	if err := task.Before(request, response, params); err != nil {
		return err
	}

	if invokeErr := func() ErrorResponse {
		switch {
		case request.Method() == MethodGet:
			if isLast {
				if err := task.Index(ctx, request, response, params); err == nil {
					return nil
				} else if !errors.Is(err, NotImplemented) {
					return err
				}
			}

			return task.Get(ctx, request, response, params)
		case request.Method() == MethodHead:
			return task.Head(ctx, request, response, params)
		case request.Method() == MethodPost:
			if isLast {
				if err := task.Create(ctx, request, response, params); err == nil {
					return nil
				} else if !errors.Is(err, NotImplemented) {
					return err
				}
			}

			return task.Post(ctx, request, response, params)
		case request.Method() == MethodPut:
			return task.Put(ctx, request, response, params)
		case request.Method() == MethodDelete:
			return task.Delete(ctx, request, response, params)
		case request.Method() == MethodOptions:
			return task.Options(ctx, request, response, params)
		case request.Method() == MethodPatch:
			return task.Patch(ctx, request, response, params)
		case request.Method() == MethodTrace:
			return task.Trace(ctx, request, response, params)
		case request.Method() == MethodConnect:
			return task.Connect(ctx, request, response, params)
		}

		kklogger.WarnJ("ghttp:DispatchHandler.matchMethod#match_method!no_match", fmt.Sprintf("no match method %s", request.Method()))
		return nil
	}(); invokeErr != nil {
		return invokeErr
	}

	h.handleAutoRange(task, request, response)
	if err := task.After(request, response, params); err != nil {
		return err
	}

	return nil
}

func (h *DispatchHandler) handleAutoRange(task HttpHandlerTask, request *Request, response *Response) {
	supporter, ok := task.(AutoRangeSupporter)
	if !ok || !supporter.EnableAutoRangeSupport() {
		return
	}

	if response.body == nil || response.body.ReadableBytes() == 0 {
		return
	}

	response.SetHeader(httpheadername.AcceptRanges, "bytes")

	rangeHeader := request.Header().Get(httpheadername.Range)
	if rangeHeader == "" {
		return
	}

	content := response.body.Bytes()
	contentSize := int64(len(content))

	if start, end, valid := ParseRange(rangeHeader, contentSize); valid {
		rangeData := content[start : end+1]
		response.SetStatusCode(httpstatus.PartialContent)
		response.SetHeader(httpheadername.ContentRange, "bytes "+strconv.FormatInt(start, 10)+"-"+strconv.FormatInt(end, 10)+"/"+strconv.FormatInt(contentSize, 10))
		response.SetHeader(httpheadername.ContentLength, strconv.Itoa(len(rangeData)))
		response.SetBody(buf.NewSharedByteBuf(rangeData))
		if kklogger.GetLogLevel() >= kklogger.DebugLevel {
			kklogger.DebugJ("ghttp:DispatchHandler.handleAutoRange#range_request", fmt.Sprintf("range=%d-%d/%d", start, end, contentSize))
		}
	} else {
		response.SetStatusCode(httpstatus.RequestedRangeNotSatisfiable)
		response.SetHeader(httpheadername.ContentRange, "bytes */"+strconv.FormatInt(contentSize, 10))
		kklogger.WarnJ("ghttp:DispatchHandler.handleAutoRange#range_request!invalid_range", "range="+rangeHeader)
	}
}

func (h *DispatchHandler) ErrorCaught(ctx channel.HandlerContext, err error) {
	kklogger.ErrorJ("ghttp:DispatchHandler.ErrorCaught#error_caught!error", err.Error())
}

func (h *DispatchHandler) _UpdateSessionCookie(resp *Response) {
	if resp.request.session == nil {
		return
	}

	domain := h._SessionCookieDomain(resp.Request())
	cke, err := resp.Request().Cookie(SessionKey)
	if err == nil {
		if timestamp := hash.TimestampOfTimeHash(cke.Value); timestamp < time.Now().Add(time.Second*time.Duration(SessionExpireTime/10)).Unix() {
			resp.SetCookie(&http.Cookie{
				Name:     SessionKey,
				Value:    hash.TimeHash([]byte(resp.request.session.Id()), time.Now().Add(time.Second*time.Duration(SessionExpireTime)).Unix()),
				Path:     "/",
				MaxAge:   SessionExpireTime,
				Domain:   domain,
				HttpOnly: SessionHttpOnly,
				Secure:   SessionSecure,
			})
		}
	} else if err == http.ErrNoCookie {
		resp.SetCookie(&http.Cookie{
			Name:     SessionKey,
			Value:    hash.TimeHash([]byte(resp.request.session.Id()), time.Now().Add(time.Second*time.Duration(SessionExpireTime)).Unix()),
			Path:     "/",
			MaxAge:   SessionExpireTime,
			Domain:   domain,
			HttpOnly: SessionHttpOnly,
			Secure:   SessionSecure,
		})
	} else {
		kklogger.WarnJ("ghttp:DispatchHandler.UpdateSessionCookie#update_session!cookie_error", fmt.Sprintf("get req cookie error [%s]", err))
	}

	resp.request.session.Save()
}

func (h *DispatchHandler) _SessionCookieDomain(req *Request) string {
	if SessionDomain != "" || req == nil {
		return SessionDomain
	}

	host := req.Host()
	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		host = parsedHost
	}

	host = strings.TrimPrefix(strings.TrimSuffix(host, "]"), "[")
	if strings.Contains(host, ":") {
		return ""
	}

	return host
}

type ObjectLogStruct struct {
	ChannelID  string `json:"cid,omitempty"`
	TrackID    string `json:"tid,omitempty"`
	State      string `json:"state,omitempty"`
	Handler    string `json:"handler,omitempty"`
	URI        string `json:"uri,omitempty"`
	Message    any    `json:"message,omitempty"`
	RemoteAddr string `json:"remote_addr,omitempty"`
}
