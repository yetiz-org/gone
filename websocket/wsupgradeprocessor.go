package websocket

import (
	"fmt"
	"net"
	"net/http"
	"reflect"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/kklab-com/gone/channel"
	gtp "github.com/kklab-com/gone/http"
	"github.com/kklab-com/goth-kklogger"
)

type WSUpgradeProcessor struct {
	channel.DefaultHandler
	ch               channel.Channel
	wsConnClosed     bool
	wsConn           *websocket.Conn
	upgrader         *websocket.Upgrader
	pack             *gtp.Pack
	task             ServerHandlerTask
	writeLock        sync.Mutex
	UpgradeCheckFunc func(req *gtp.Request, resp *gtp.Response, params map[string]interface{}) bool
}

func (h *WSUpgradeProcessor) Disconnect(ctx channel.HandlerContext, future channel.Future) {
	if !h.wsConnClosed {
		if h.task != nil {
			h.task.WSDisconnected(nil, h.pack.Req, h.pack.Params)
		}

		h.wsConnClosed = true
	}

	ctx.Disconnect(future)
}

func (h *WSUpgradeProcessor) slowDisconnect(ctx channel.HandlerContext) {
	go func() {
		time.Sleep(time.Second)
		ctx.Channel().Disconnect()
	}()
}

func (h *WSUpgradeProcessor) Added(ctx channel.HandlerContext) {
	h.ch = ctx.Channel()
	h.upgrader = &websocket.Upgrader{
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

func (h *WSUpgradeProcessor) Active(ctx channel.HandlerContext) {
	kklogger.DebugJ("WSUpgradeProcessor.Active", fmt.Sprintf("connection %s active", ctx.Channel().ID()))
	ctx.FireActive()
}

func (h *WSUpgradeProcessor) Inactive(ctx channel.HandlerContext) {
	kklogger.DebugJ("WSUpgradeProcessor.Inactive", fmt.Sprintf("connection %s inactive", ctx.Channel().ID()))
	ctx.FireInactive()
}

func (h *WSUpgradeProcessor) Read(ctx channel.HandlerContext, obj interface{}) {
	if obj == nil {
		return
	}

	if pkg, cast := obj.(*gtp.Pack); cast {
		h.pack = pkg
	} else if _, cast := obj.(*HttpWebsocketPack); cast {
		ctx.FireRead(obj)
		return
	}

	var node gtp.RouteNode
	if v, f := h.pack.Params["[gone]node"]; f {
		node = v.(gtp.RouteNode)
	} else {
		kklogger.ErrorJ("WSUpgradeProcessor.Read#NotFound", "node is not in [gone]node")
		return
	}

	task := func() ServerHandlerTask {
		if task, ok := node.HandlerTask().(ServerHandlerTask); ok {
			return task
		}

		return nil
	}()

	if task == nil {
		return
	}

	var acceptances []gtp.Acceptance
	for n := node; n != nil; n = n.Parent() {
		if n.Acceptances() != nil && len(n.Acceptances()) > 0 {
			acceptances = append(n.Acceptances(), acceptances...)
		}
	}

	for _, acceptance := range acceptances {
		if err := acceptance.Do(h.pack.Req, h.pack.Resp, h.pack.Params); err != nil {
			if err == gtp.AcceptanceInterrupt {
				return
			}

			kklogger.WarnJ("Acceptance", gtp.ObjectLogStruct{
				ChannelID:  ctx.Channel().ID(),
				TrackID:    h.pack.Req.TrackID(),
				State:      "Fail",
				URI:        h.pack.Req.RequestURI,
				Handler:    reflect.TypeOf(acceptance).String(),
				Message:    err.Error(),
				RemoteAddr: h.pack.Req.Request.RemoteAddr,
			})

			ctx.Write(obj, nil)
			return
		} else {
			if kklogger.GetLogLevel() < kklogger.TraceLevel {
				continue
			}

			kklogger.TraceJ("Acceptance", gtp.ObjectLogStruct{
				ChannelID:  ctx.Channel().ID(),
				TrackID:    h.pack.Req.TrackID(),
				State:      "Pass",
				URI:        h.pack.Req.RequestURI,
				Handler:    reflect.TypeOf(acceptance).String(),
				RemoteAddr: h.pack.Req.Request.RemoteAddr,
			})
		}
	}

	h.task = task
	if (h.UpgradeCheckFunc != nil && !h.UpgradeCheckFunc(h.pack.Req, h.pack.Resp, h.pack.Params)) ||
		(!h.task.WSUpgrade(h.pack.Req, h.pack.Resp, h.pack.Params)) {
		ctx.Write(h.pack, nil)
		return
	}

	timeMark := time.Now()
	wsConn := func() *websocket.Conn {
		wsConn, err := h.upgrader.Upgrade(h.pack.Writer, &h.pack.Req.Request, h.pack.Resp.Header())
		if err != nil {
			kklogger.ErrorJ("WSUpgradeProcessor.Read#WSUpgrade", h._NewWSLog(nil, err))
			h.task.WSDisconnected(nil, h.pack.Req, h.pack.Params)
			h.wsConnClosed = true
			h.slowDisconnect(ctx)
			return nil
		}

		return wsConn
	}()

	if wsConn == nil {
		return
	}

	h.wsConn = wsConn
	kklogger.DebugJ("WSUpgradeProcessor.Read#WSUpgrade", h._NewWSLog(nil, nil))
	ctx.Channel().SetParam(ParamWSUpgrader, h)
	ctx.Channel().SetParam(ParamWSDisconnectOnce, &sync.Once{})
	h.pack.Params["[gone]ws_upgrade_time"] = time.Now().Sub(timeMark).Nanoseconds()
	wsConn.SetCloseHandler(h._CloseHandler)
	wsConn.SetPingHandler(h._PingHandler)
	wsConn.SetPongHandler(h._PongHandler)
	h.task.WSConnected(nil, h.pack.Req, h.pack.Params)
	for ctx.Channel().IsActive() {
		timeMark = time.Now()
		messageType, message, err := wsConn.ReadMessage()
		if err != nil {
			if _, ok := err.(*websocket.CloseError); ok {
				kklogger.DebugJ("WSUpgradeProcessor.Read#Close", h._NewWSLog(nil, err))
			} else {
				kklogger.WarnJ("WSUpgradeProcessor.Read#ReadMessage", h._NewWSLog(nil, err))
			}

			h.task.WSDisconnected(nil, h.pack.Req, h.pack.Params)
			h.wsConnClosed = true
			ctx.Channel().Disconnect()
			return
		}

		msg := _ParseMessage(messageType, message)
		if msg != nil {
			func() {
				params := map[string]interface{}{"[gone]ws_read_time": time.Now().Sub(timeMark).Nanoseconds()}
				for k, v := range h.pack.Params {
					params[k] = v
				}

				timeMark = time.Now()
				var obj interface{} = &HttpWebsocketPack{
					Request:     h.pack.Req,
					HandlerTask: h.task,
					Message:     msg,
					Params:      params,
				}

				kklogger.TraceJ("WSUpgradeProcessor.Read#Read", h._NewWSLog(msg, nil))
				ctx.FireRead(obj)
				params["[gone]handler_time"] = time.Now().Sub(timeMark).Nanoseconds()
			}()
		}
	}
}

func (h *WSUpgradeProcessor) Write(ctx channel.HandlerContext, obj interface{}, future channel.Future) {
	if !ctx.Channel().IsActive() {
		return
	}

	var message = func() Message {
		if msg, ok := obj.(Message); ok {
			return msg
		}

		return nil
	}()

	if message == nil {
		kklogger.ErrorJ("WSUpgradeProcessor.Write#Cast", h._NewWSLog(message, ErrWrongObjectType))
		return
	}

	wsConn := func() *websocket.Conn {
		if obj := ctx.Channel().Param(ParamWSUpgrader); obj != nil {
			if v, ok := obj.(*WSUpgradeProcessor); ok {
				return v.wsConn
			}
		}

		return nil
	}()

	if wsConn == nil {
		kklogger.ErrorJ("WSUpgradeProcessor.Write#WSConn", h._NewWSLog(message, WSConnNotExist))
		return
	}

	var err error
	switch obj.(type) {
	case *CloseMessage, *PingMessage, *PongMessage:
		dead := func() time.Time {
			if message.Deadline() == nil {
				return time.Now().Add(time.Second)
			}

			return *message.Deadline()
		}()

		h.writeLock.Lock()
		err = func(message Message, dead time.Time) error {
			defer h.writeLock.Unlock()
			return wsConn.WriteControl(message.Type().wsLibType(), message.Encoded(), dead)
		}(message, dead)

		if err == websocket.ErrCloseSent {
		} else if e, ok := err.(net.Error); ok && e.Temporary() {
			err = nil
		}
	case *DefaultMessage:
		h.writeLock.Lock()
		err = func(message Message) error {
			defer h.writeLock.Unlock()
			return wsConn.WriteMessage(message.Type().wsLibType(), message.Encoded())
		}(message)
	default:
		err = ErrWrongObjectType
	}

	if err != nil {
		ctx.Channel().Param(ParamWSDisconnectOnce).(*sync.Once).Do(func() {
			ctx.Channel().Disconnect()
			kklogger.WarnJ("WSUpgradeProcessor.Write#Write", h._NewWSLog(message, err))
		})
	}
}

func (h *WSUpgradeProcessor) _PingHandler(message string) error {
	msg := &PingMessage{
		DefaultMessage: DefaultMessage{
			MessageType: PingMessageType,
			Message:     []byte(message),
		},
	}

	params := map[string]interface{}{}
	for k, v := range h.pack.Params {
		params[k] = v
	}

	var obj interface{} = &HttpWebsocketPack{
		Request:     h.pack.Req,
		HandlerTask: h.task,
		Message:     msg,
		Params:      params,
	}

	kklogger.TraceJ("WSUpgradeProcessor._PingHandler#Read", h._NewWSLog(msg, nil))
	h.ch.FireRead(obj)
	return nil
}

func (h *WSUpgradeProcessor) _PongHandler(message string) error {
	msg := &PongMessage{
		DefaultMessage: DefaultMessage{
			MessageType: PongMessageType,
			Message:     []byte(message),
		},
	}

	params := map[string]interface{}{}
	for k, v := range h.pack.Params {
		params[k] = v
	}

	var obj interface{} = &HttpWebsocketPack{
		Request:     h.pack.Req,
		HandlerTask: h.task,
		Message:     msg,
		Params:      params,
	}

	kklogger.TraceJ("WSUpgradeProcessor._PongHandler#Read", h._NewWSLog(msg, nil))
	h.ch.FireRead(obj)
	return nil
}

func (h *WSUpgradeProcessor) _CloseHandler(code int, text string) error {
	msg := &CloseMessage{
		DefaultMessage: DefaultMessage{
			MessageType: CloseMessageType,
			Message:     []byte(text),
		},
		CloseCode: CloseCode(code),
	}

	params := map[string]interface{}{}
	for k, v := range h.pack.Params {
		params[k] = v
	}

	var obj interface{} = &HttpWebsocketPack{
		Request:     h.pack.Req,
		HandlerTask: h.task,
		Message:     msg,
		Params:      params,
	}

	kklogger.TraceJ("WSUpgradeProcessor._CloseHandler#Read", h._NewWSLog(msg, nil))
	h.ch.FireRead(obj)
	return nil
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

func (h *WSUpgradeProcessor) _NewWSLog(message Message, err error) *WSLogStruct {
	log := &WSLogStruct{
		LogType:    WSLogType,
		ChannelID:  h.ch.ID(),
		TrackID:    h.pack.Req.TrackID(),
		RequestURI: h.pack.Req.RequestURI,
		Message:    message,
		Error:      err,
	}

	if h.wsConn != nil {
		log.RemoteAddr = h.wsConn.RemoteAddr()
		log.LocalAddr = h.wsConn.LocalAddr()
	}

	return log
}
