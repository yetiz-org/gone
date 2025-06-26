package ghttp

import (
	"errors"
	"fmt"
	"github.com/yetiz-org/gone/channel"
	"github.com/yetiz-org/gone/erresponse"
	"github.com/yetiz-org/gone/ghttp/httpheadername"
	"github.com/yetiz-org/gone/ghttp/httpstatus"
	buf "github.com/yetiz-org/goth-bytebuf"
	kklogger "github.com/yetiz-org/goth-kklogger"
	kkpanic "github.com/yetiz-org/goth-panic"
	"github.com/yetiz-org/goth-util/hash"
	"net/http"
	"reflect"
	"time"
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

	request, response, params := pack.Request, pack.Response, pack.Params
	timeMark := time.Now()
	if node, nodeParams, isLast := h.route.RouteNode(request.Url().Path); node != nil && node.RouteType() != RouteTypeGroup {
		pack.RouteNode = node
		params["[gone-http]h_locate_time"] = time.Now().Sub(timeMark).Nanoseconds()
		params["[gone-http]node"] = node
		params["[gone-http]node_name"] = node.Name()
		params["[gone-http]is_index"] = isLast
		params["[gone-http]dispatcher"] = h
		params["[gone-http]context_pack"] = obj
		if nodeParams != nil {
			for k, v := range nodeParams {
				params[k] = v
			}
		}

		task, ok := node.HandlerTask().(HttpHandlerTask)
		if !ok {
			ctx.FireRead(obj)
			return
		}

		var rtnCatch ReturnCatch
		defer h.callWrite(ctx, obj)
		defer h._UpdateSessionCookie(response)
		defer h._PanicCatch(ctx, request, response, task, params, &rtnCatch)
		defer task.CORSHelper(request, response, params)
		timeMark = time.Now()
		for _, acceptance := range node.AggregatedAcceptances() {
			if request.Method() == OPTIONS && acceptance.SkipMethodOptions() {
				continue
			}

			if err := acceptance.Do(ctx, request, response, params); err != nil {
				if err == AcceptanceInterrupt {
					kklogger.TraceJ("ghttp:Acceptance", ObjectLogStruct{
						ChannelID:  ctx.Channel().ID(),
						TrackID:    request.TrackID(),
						State:      "Skip",
						URI:        request.RequestURI(),
						Handler:    reflect.TypeOf(acceptance).String(),
						RemoteAddr: request.Request().RemoteAddr,
					})

					return
				}

				params["[gone-http]h_acceptance_time"] = time.Now().Sub(timeMark).Nanoseconds()
				kklogger.WarnJ("ghttp:Acceptance", ObjectLogStruct{
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
				}

				return
			} else {
				if kklogger.GetLogLevel() < kklogger.TraceLevel {
					continue
				}

				kklogger.TraceJ("ghttp:Acceptance", ObjectLogStruct{
					ChannelID:  ctx.Channel().ID(),
					TrackID:    request.TrackID(),
					State:      "Pass",
					URI:        request.RequestURI(),
					Handler:    reflect.TypeOf(acceptance).String(),
					RemoteAddr: request.Request().RemoteAddr,
				})
			}
		}

		params["[gone-http]h_acceptance_time"] = time.Now().Sub(timeMark).Nanoseconds()
		timeMark = time.Now()
		rtnCatch.err = h.invokeMethod(ctx, task, request, response, params, isLast)
		params["[gone-http]handler_time"] = time.Now().Sub(timeMark).Nanoseconds()
	} else {
		defer h.callWrite(ctx, obj)
		defer h._UpdateSessionCookie(response)
		params["[gone-http]h_locate_time"] = time.Now().Sub(timeMark).Nanoseconds()
		if upgrade := request.Header().Get(httpheadername.Upgrade); upgrade != "" {
			response.Header().Set(httpheadername.Upgrade, upgrade)
		}

		if connection := request.Header().Get(httpheadername.Connection); connection != "" {
			response.Header().Set(httpheadername.Connection, connection)
		}

		response.SetStatusCode(404)
		kklogger.WarnJ("ghttp:DispatchHandler.Read#EndpointNotExist", ObjectLogStruct{
			ChannelID:  ctx.Channel().ID(),
			TrackID:    request.TrackID(),
			URI:        request.RequestURI(),
			RemoteAddr: request.Request().RemoteAddr,
		})
	}
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
		kklogger.ErrorJ("ghttp:DispatchHandler.Read#ErrorCaught", ObjectLogStruct{
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
	if err := task.PreCheck(request, response, params); err != nil {
		return err
	}

	if err := task.Before(request, response, params); err != nil {
		return err
	}

	if invokeErr := func() ErrorResponse {
		switch {
		case request.Method() == GET:
			if isLast {
				if err := task.Index(ctx, request, response, params); err == nil {
					return nil
				} else if !errors.Is(err, NotImplemented) {
					return err
				}
			}

			return task.Get(ctx, request, response, params)
		case request.Method() == POST:
			if isLast {
				if err := task.Create(ctx, request, response, params); err == nil {
					return nil
				} else if !errors.Is(err, NotImplemented) {
					return err
				}
			}

			return task.Post(ctx, request, response, params)
		case request.Method() == PUT:
			return task.Put(ctx, request, response, params)
		case request.Method() == DELETE:
			return task.Delete(ctx, request, response, params)
		case request.Method() == OPTIONS:
			return task.Options(ctx, request, response, params)
		case request.Method() == PATCH:
			return task.Patch(ctx, request, response, params)
		case request.Method() == TRACE:
			return task.Trace(ctx, request, response, params)
		case request.Method() == CONNECT:
			return task.Connect(ctx, request, response, params)
		}

		kklogger.WarnJ("ghttp:DispatchHandler", fmt.Sprintf("no match method %s", request.Method()))
		return nil
	}(); invokeErr != nil {
		return invokeErr
	}

	if err := task.After(request, response, params); err != nil {
		return err
	}

	return nil
}

func (h *DispatchHandler) ErrorCaught(ctx channel.HandlerContext, err error) {
	kklogger.ErrorJ("ghttp:DispatchHandler.ErrorCaught", err.Error())
}

func (h *DispatchHandler) _UpdateSessionCookie(resp *Response) {
	if resp.request.session == nil {
		return
	}

	cke, err := resp.Request().Cookie(SessionKey)
	if err == nil {
		if timestamp := hash.TimestampOfTimeHash(cke.Value); timestamp < time.Now().Add(time.Second*time.Duration(SessionExpireTime/10)).Unix() {
			resp.SetCookie(&http.Cookie{
				Name:     SessionKey,
				Value:    hash.TimeHash([]byte(resp.request.session.Id()), time.Now().Add(time.Second*time.Duration(SessionExpireTime)).Unix()),
				Path:     "/",
				MaxAge:   SessionExpireTime,
				Domain:   SessionDomain,
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
			Domain:   SessionDomain,
			HttpOnly: SessionHttpOnly,
			Secure:   SessionSecure,
		})
	} else {
		kklogger.WarnJ("ghttp:UpdateSessionCookie", fmt.Sprintf("get req cookie error [%s]", err))
	}

	resp.request.session.Save()
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
