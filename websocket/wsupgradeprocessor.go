package websocket

import (
	"net"
	"net/http"
	"reflect"
	"time"

	"github.com/gorilla/websocket"
	"github.com/kklab-com/gone/channel"
	gtp "github.com/kklab-com/gone/http"
	"github.com/kklab-com/goth-kklogger"
)

type WSUpgradeProcessor struct {
	channel.DefaultHandler
	upgrade          *websocket.Upgrader
	UpgradeCheckFunc func(req *gtp.Request, resp *gtp.Response, params map[string]interface{}) bool
}

func (h *WSUpgradeProcessor) Added(ctx channel.HandlerContext) {
	h.upgrade = &websocket.Upgrader{
		CheckOrigin: func() func(r *http.Request) bool {
			if channel.GetParamBoolDefault(ctx.Channel(), ParamCheckOrigin, true) {
				return nil
			}

			return func(r *http.Request) bool {
				return true
			}
		}(),
	}
}

func (h *WSUpgradeProcessor) Read(ctx channel.HandlerContext, obj interface{}) {
	if obj == nil {
		return
	}

	if pack, cast := obj.(*gtp.Pack); cast && pack.RouteNode != nil {
		if task, ok := pack.RouteNode.HandlerTask().(ServerHandlerTask); ok {
			for _, acceptance := range pack.RouteNode.AggregatedAcceptances() {
				if err := acceptance.Do(pack.Request, pack.Response, pack.Params); err != nil {
					if err == gtp.AcceptanceInterrupt {
						return
					}

					kklogger.WarnJ("Acceptance", gtp.ObjectLogStruct{
						ChannelID:  ctx.Channel().ID(),
						TrackID:    pack.Request.TrackID(),
						State:      "Fail",
						URI:        pack.Request.RequestURI,
						Handler:    reflect.TypeOf(acceptance).String(),
						Message:    err.Error(),
						RemoteAddr: pack.Request.Request.RemoteAddr,
					})

					ctx.Write(obj, nil)
					return
				} else {
					if kklogger.GetLogLevel() < kklogger.TraceLevel {
						continue
					}

					kklogger.TraceJ("Acceptance", gtp.ObjectLogStruct{
						ChannelID:  ctx.Channel().ID(),
						TrackID:    pack.Request.TrackID(),
						State:      "Pass",
						URI:        pack.Request.RequestURI,
						Handler:    reflect.TypeOf(acceptance).String(),
						RemoteAddr: pack.Request.Request.RemoteAddr,
					})
				}
			}

			if (h.UpgradeCheckFunc != nil && !h.UpgradeCheckFunc(pack.Request, pack.Response, pack.Params)) ||
				(!task.WSUpgrade(pack.Request, pack.Response, pack.Params)) {
				ctx.Write(pack, nil)
				return
			}

			timeMark := time.Now()
			wsConn := func() *websocket.Conn {
				wsConn, err := h.upgrade.Upgrade(pack.Writer, &pack.Request.Request, pack.Response.Header())
				if err != nil {
					kklogger.WarnJ("WSUpgradeProcessor.Read#WSUpgrade", h._NewWSLog(ctx.Channel().ID(), pack.Request.TrackID(), pack.Request.RequestURI, nil, err))
					ctx.Channel().Disconnect()
					return nil
				}

				return wsConn
			}()

			if wsConn == nil {
				return
			}

			kklogger.DebugJ("WSUpgradeProcessor.Read#WSUpgrade", h._NewWSLog(ctx.Channel().ID(), pack.Request.TrackID(), pack.Request.RequestURI, wsConn, nil))
			pack.Params["[gone-http]ws_upgrade_time"] = time.Now().Sub(timeMark).Nanoseconds()

			// create ws channel and replace it
			ch := &Channel{
				DefaultNetChannel: &ctx.Channel().(*gtp.Channel).DefaultNetChannel,
				wsConn:            wsConn,
				Response:          pack.Response,
				Request:           pack.Request,
			}

			ch.Pipeline().(channel.PipelineSetChannel).SetChannel(ch)
			ch.Pipeline().Clear()
			ch.Pipeline().AddLast("WS_INVOKER", NewInvokeHandler(task, pack.Params))
			task.WSConnected(pack.Request, pack.Response, pack.Params)
			ch.Read()
			return
		}
	}

	ctx.FireRead(obj)
	return
}

func (h *WSUpgradeProcessor) _NewWSLog(cID string, tID string, uri string, wsConn *websocket.Conn, err error) *WSLogStruct {
	log := &WSLogStruct{
		LogType:    WSLogType,
		ChannelID:  cID,
		TrackID:    tID,
		RequestURI: uri,
		Error:      err,
	}

	if wsConn != nil {
		log.RemoteAddr = wsConn.RemoteAddr()
		log.LocalAddr = wsConn.LocalAddr()
	}

	return log
}

type WSLogStruct struct {
	LogType    string   `json:"log_type,omitempty"`
	RemoteAddr net.Addr `json:"remote_addr,omitempty"`
	LocalAddr  net.Addr `json:"local_addr,omitempty"`
	RequestURI string   `json:"request_uri,omitempty"`
	ChannelID  string   `json:"channel_id,omitempty"`
	TrackID    string   `json:"trace_id,omitempty"`
	Message    Message  `json:"message,omitempty"`
	Error      error    `json:"error,omitempty"`
}

const WSLogType = "websocket"
