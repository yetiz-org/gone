package http

import (
	"bytes"
	"fmt"
	"net/http"
	"reflect"
	"runtime/debug"
	"runtime/pprof"
	"time"

	"github.com/kklab-com/gone-httpheadername"
	"github.com/kklab-com/gone-httpstatus"
	"github.com/kklab-com/gone/channel"
	"github.com/kklab-com/gone/http/httpmethod"
	"github.com/kklab-com/goth-erresponse"
	"github.com/kklab-com/goth-kklogger"
	"github.com/kklab-com/goth-kkutil/hash"
	"github.com/kklab-com/goth-kkutil/value"
)

type DispatchHandler struct {
	channel.DefaultHandler
	route             Route
	DefaultStatusCode int
	NotFound404       *bytes.Buffer
}

func NewDispatchHandler(route Route) *DispatchHandler {
	return &DispatchHandler{route: route,
		DefaultStatusCode: 200,
		NotFound404:       bytes.NewBufferString("<html><img src='https://http.cat/404' /></html>")}
}

func (h *DispatchHandler) Read(ctx channel.HandlerContext, obj interface{}) {
	pack := _UnPack(obj)
	if pack == nil {
		ctx.FireRead(obj)
		return
	}

	request, response, params := pack.Req, pack.Resp, pack.Params
	response.SetStatusCode(h.DefaultStatusCode)
	timeMark := time.Now()
	if node, nodeParams, isLast := h.route.RouteEndPoint(request); node != nil {
		params["[gone]h_locate_time"] = time.Now().Sub(timeMark).Nanoseconds()
		params["[gone]node"] = node
		params["[gone]node_name"] = node.Name()
		params["[gone]is_index"] = isLast
		if nodeParams != nil {
			for k, v := range nodeParams {
				params[k] = v
			}
		}

		task, ok := node.HandlerTask().(HandlerTask)
		if !ok {
			ctx.FireRead(obj)
			return
		}

		var rtnCatch ReturnCatch
		defer ctx.FireWrite(obj)
		defer h._UpdateSessionCookie(response)
		defer h._PanicCatch(ctx, request, response, task, params, &rtnCatch)
		timeMark = time.Now()
		var acceptances []Acceptance
		for n := node; n != nil; n = n.Parent() {
			if n.Acceptances() != nil && len(n.Acceptances()) > 0 {
				acceptances = append(n.Acceptances(), acceptances...)
			}
		}

		for _, acceptance := range acceptances {
			if err := acceptance.Do(request, response, params); err != nil {
				if err == AcceptanceInterrupt {
					return
				}

				params["[gone]h_acceptance_time"] = time.Now().Sub(timeMark).Nanoseconds()
				kklogger.WarnJ("Acceptance", ObjectLogStruct{
					ChannelID:  ctx.Channel().ID(),
					TrackID:    request.TrackID(),
					State:      "Fail",
					URI:        request.RequestURI,
					Handler:    reflect.TypeOf(acceptance).String(),
					Message:    err.Error(),
					RemoteAddr: request.Request.RemoteAddr,
				})

				return
			} else {
				if kklogger.GetLogLevel() < kklogger.TraceLevel {
					continue
				}

				kklogger.TraceJ("Acceptance", ObjectLogStruct{
					ChannelID:  ctx.Channel().ID(),
					TrackID:    request.TrackID(),
					State:      "Pass",
					URI:        request.RequestURI,
					Handler:    reflect.TypeOf(acceptance).String(),
					RemoteAddr: request.Request.RemoteAddr,
				})
			}
		}

		params["[gone]h_acceptance_time"] = time.Now().Sub(timeMark).Nanoseconds()
		timeMark = time.Now()
		rtnCatch.err = h.invokeMethod(task, request, response, params, isLast)
		params["[gone]handler_time"] = time.Now().Sub(timeMark).Nanoseconds()
	} else {
		defer ctx.FireWrite(obj)
		defer h._UpdateSessionCookie(response)
		params["[gone]h_locate_time"] = time.Now().Sub(timeMark).Nanoseconds()
		if upgrade := request.Header.Get(httpheadername.Upgrade); upgrade != "" {
			response.Header().Set(httpheadername.Upgrade, upgrade)
		}

		if connection := request.Header.Get(httpheadername.Connection); connection != "" {
			response.Header().Set(httpheadername.Connection, connection)
		}

		response.statusCode = httpstatus.NotFound
		response.body = h.NotFound404
		kklogger.WarnJ("DispatchHandler.Read#EndpointNotExist", ObjectLogStruct{
			ChannelID:  ctx.Channel().ID(),
			TrackID:    request.TrackID(),
			URI:        request.RequestURI,
			RemoteAddr: request.Request.RemoteAddr,
		})
	}
}

func (h *DispatchHandler) _PanicCatch(ctx channel.HandlerContext, request *Request, response *Response, task HandlerTask, params map[string]interface{}, rtnCatch *ReturnCatch) {
	erErr := rtnCatch.err
	timeMark := time.Now()
	var err error
	if r := recover(); r != nil {
		switch er := r.(type) {
		case ErrorResponse:
			erErr = er
		case error:
			err = er
		case *channel.HandlerCaughtError:
			err = er
		case string:
			err = &channel.StringError{Message: er}
		default:
			err = &channel.StringError{Message: value.JsonMarshal(er)}
		}

		kklogger.ErrorJ("DispatchHandler.Read#ErrorCaught", ObjectLogStruct{
			ChannelID:  ctx.Channel().ID(),
			TrackID:    request.TrackID(),
			URI:        request.RequestURI,
			Handler:    reflect.TypeOf(task).String(),
			RemoteAddr: request.Request.RemoteAddr,
		})

		if err != nil {
			if _, ok := err.(*channel.HandlerCaughtError); !ok {
				buffer := &bytes.Buffer{}
				pprof.Lookup("goroutine").WriteTo(buffer, 1)
				err = &channel.HandlerCaughtError{
					Err:             err.Error(),
					PanicCallStack:  string(debug.Stack()),
					GoRoutineStacks: buffer.String(),
				}
			}

			h.ErrorCaught(ctx, err)
			erErr = erresponse.ServerErrorPanic
		}
	}

	if erErr != nil {
		erErr = erErr.Clone()
		erErr.ErrorData()["cid"] = request.Channel().ID()
		erErr.ErrorData()["tid"] = request.TrackID()
		timeMark = time.Now()
		err := task.ErrorCaught(request, response, params, erErr)
		params["[gone]h_error_time"] = time.Now().Sub(timeMark).Nanoseconds()
		if err != nil {
			h.ErrorCaught(ctx, err)
		}
	}
}

type ReturnCatch struct {
	err ErrorResponse
}

func (h *DispatchHandler) invokeMethod(task HandlerTask, request *Request, response *Response, params map[string]interface{}, isLast bool) ErrorResponse {
	if err := task.PreCheck(request, response, params); err != nil {
		return err
	}

	if err := task.Before(request, response, params); err != nil {
		return err
	}

	if invokeErr := func() ErrorResponse {
		switch {
		case request.Method == httpmethod.GET:
			if isLast {
				if err := task.Index(request, response, params); err != nil {
					if err == NotImplemented {
						return task.Get(request, response, params)
					}

					return err
				}

				return nil
			} else {
				return task.Get(request, response, params)
			}
		case request.Method == httpmethod.POST:
			return task.Post(request, response, params)
		case request.Method == httpmethod.PUT:
			return task.Put(request, response, params)
		case request.Method == httpmethod.DELETE:
			return task.Delete(request, response, params)
		case request.Method == httpmethod.OPTIONS:
			return task.Options(request, response, params)
		case request.Method == httpmethod.PATCH:
			return task.Patch(request, response, params)
		case request.Method == httpmethod.TRACE:
			return task.Trace(request, response, params)
		case request.Method == httpmethod.CONNECT:
			return task.Connect(request, response, params)
		}

		kklogger.WarnJ("DispatchHandler", fmt.Sprintf("no match method %s", request.Method))
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
	var ce *channel.HandlerCaughtError
	if e, ok := err.(*channel.HandlerCaughtError); ok {
		ce = e
	} else {
		buffer := &bytes.Buffer{}
		pprof.Lookup("goroutine").WriteTo(buffer, 1)
		ce = &channel.HandlerCaughtError{
			Err:             err.Error(),
			PanicCallStack:  string(debug.Stack()),
			GoRoutineStacks: buffer.String(),
		}
	}

	kklogger.ErrorJ("DispatchHandler.ErrorCaught", ce)
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
				Value:    hash.TimeHash([]byte(resp.request.session.ID()), time.Now().Add(time.Second*time.Duration(SessionExpireTime)).Unix()),
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
			Value:    hash.TimeHash([]byte(resp.request.session.ID()), time.Now().Add(time.Second*time.Duration(SessionExpireTime)).Unix()),
			Path:     "/",
			MaxAge:   SessionExpireTime,
			Domain:   SessionDomain,
			HttpOnly: SessionHttpOnly,
			Secure:   SessionSecure,
		})
	} else {
		kklogger.WarnJ("UpdateSessionCookie", fmt.Sprintf("get req cookie error [%s]", err))
	}

	resp.request.session.Save()
}

type ObjectLogStruct struct {
	ChannelID  string `json:"cid,omitempty"`
	TrackID    string `json:"tid,omitempty"`
	State      string `json:"state,omitempty"`
	Handler    string `json:"handler,omitempty"`
	URI        string `json:"uri,omitempty"`
	Message    string `json:"message,omitempty"`
	RemoteAddr string `json:"remote_addr,omitempty"`
}
