package gws

import (
	"net"
	"net/http"
	"reflect"
	"time"

	"github.com/yetiz-org/gone/channel"
	gtp "github.com/yetiz-org/gone/ghttp"
	kklogger "github.com/yetiz-org/goth-kklogger"

	"github.com/gorilla/websocket"
)

type UpgradeProcessor struct {
	channel.DefaultHandler
	upgrade          *websocket.Upgrader
	UpgradeCheckFunc func(req *gtp.Request, resp *gtp.Response, params map[string]any) bool
}

func (h *UpgradeProcessor) Added(ctx channel.HandlerContext) {
	h.upgrade = &websocket.Upgrader{
		CheckOrigin: func() func(r *http.Request) bool {
			if !channel.GetParamBoolDefault(ctx.Channel(), ParamCheckOrigin, true) {
				return func(r *http.Request) bool {
					return true
				}
			}

			return nil
		}(),
	}
}

func (h *UpgradeProcessor) Read(ctx channel.HandlerContext, obj any) {
	if obj == nil {
		return
	}

	if pack, cast := obj.(*gtp.Pack); cast && pack.RouteNode != nil {
		if task, ok := pack.RouteNode.HandlerTask().(ServerHandlerTask); ok {
			for _, acceptance := range pack.RouteNode.AggregatedAcceptances() {
				if err := acceptance.Do(ctx, pack.Request, pack.Response, pack.Params); err != nil {
					if err == gtp.AcceptanceInterrupt {
						return
					}

					kklogger.WarnJ("gws:UpgradeProcessor.Acceptance#acceptance!warn", gtp.ObjectLogStruct{
						ChannelID:  ctx.Channel().ID(),
						TrackID:    pack.Request.TrackID(),
						State:      "Fail",
						URI:        pack.Request.RequestURI(),
						Handler:    reflect.TypeOf(acceptance).String(),
						Message:    err.Error(),
						RemoteAddr: pack.Request.Request().RemoteAddr,
					})

					ctx.Write(obj, nil).Sync()
					return
				} else {
					if kklogger.GetLogLevel() < kklogger.TraceLevel {
						continue
					}

					kklogger.TraceJ("gws:UpgradeProcessor.Acceptance#acceptance!trace", gtp.ObjectLogStruct{
						ChannelID:  ctx.Channel().ID(),
						TrackID:    pack.Request.TrackID(),
						State:      "Pass",
						URI:        pack.Request.RequestURI(),
						Handler:    reflect.TypeOf(acceptance).String(),
						RemoteAddr: pack.Request.Request().RemoteAddr,
					})
				}
			}

			if (h.UpgradeCheckFunc != nil && !h.UpgradeCheckFunc(pack.Request, pack.Response, pack.Params)) ||
				(!task.WSUpgrade(pack.Request, pack.Response, pack.Params)) {
				ctx.Write(pack, nil).Sync()
				return
			}

			timeMark := time.Now()
			wsConn := func() *websocket.Conn {
				wsConn, err := h.upgrade.Upgrade(pack.Writer, pack.Request.Request(), pack.Response.Header())
				if err != nil {
					kklogger.WarnJ("gws:UpgradeProcessor.Read#ws_upgrade!upgrade_error", h._NewWSLog(ctx.Channel().ID(), pack.Request.TrackID(), pack.Request.RequestURI(), nil, err))
					ctx.Channel().Disconnect()
					return nil
				}

				return wsConn
			}()

			if wsConn == nil {
				return
			}

			kklogger.TraceJ("gws:UpgradeProcessor.Read#ws_upgrade!upgrade_success", h._NewWSLog(ctx.Channel().ID(), pack.Request.TrackID(), pack.Request.RequestURI(), wsConn, nil))
			pack.Params["[gone-http]ws_upgrade_time"] = time.Now().Sub(timeMark).Nanoseconds()

			// create ws channel and replace it
			ch := &Channel{
				DefaultNetChannel: &ctx.Channel().(*gtp.Channel).DefaultNetChannel,
				wsConn:            wsConn,
				Response:          pack.Response,
				Request:           pack.Request,
			}

			ch.Pipeline().(channel.PipelineSetChannel).SetChannel(ch)
			ch.Pipeline().AddBefore(ctx.Name(), "WS_INVOKER", NewInvokeHandler(task, pack.Params))
			ch.Pipeline().RemoveByName(ctx.Name())
			ch.wsConn.SetPingHandler(ch._PingHandler)
			ch.wsConn.SetPongHandler(ch._PongHandler)
			ch.wsConn.SetCloseHandler(ch._CloseHandler)
			task.WSConnected(ch, pack.Request, pack.Response, pack.Params)
			ch.Read()
			return
		}
	}

	ctx.FireRead(obj)
	return
}

func (h *UpgradeProcessor) _NewWSLog(cID string, tID string, uri string, wsConn *websocket.Conn, err error) *LogStruct {
	log := &LogStruct{
		LogType:    LogType,
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

type LogStruct struct {
	LogType    string   `json:"log_type,omitempty"`
	RemoteAddr net.Addr `json:"remote_addr,omitempty"`
	LocalAddr  net.Addr `json:"local_addr,omitempty"`
	RequestURI string   `json:"request_uri,omitempty"`
	ChannelID  string   `json:"channel_id,omitempty"`
	TrackID    string   `json:"trace_id,omitempty"`
	Message    Message  `json:"message,omitempty"`
	Error      error    `json:"error,omitempty"`
}

const LogType = "websocket"
