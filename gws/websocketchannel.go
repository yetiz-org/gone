package gws

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	kklogger "github.com/yetiz-org/goth-kklogger"

	"github.com/gorilla/websocket"
	"github.com/yetiz-org/gone/channel"
	gtp "github.com/yetiz-org/gone/ghttp"
)

var ErrWrongObjectType = fmt.Errorf("wrong object type")
var ErrEmpty = fmt.Errorf("wrong object type")

type Channel struct {
	*channel.DefaultNetChannel
	wsConn           *websocket.Conn
	Response         *gtp.Response
	Request          *gtp.Request
	assemblyDeadline time.Time
	assemblyTimeout  time.Duration
	readLimit        int64
	inboundBudget    *channel.InboundBudget
}

func (c *Channel) BootstrapPreInit() {
	c.DefaultNetChannel = &channel.DefaultNetChannel{}
}

func (c *Channel) Init() channel.Channel {
	return c.DefaultNetChannel.Init()
}

func (c *Channel) UnsafeWrite(obj any) (rtErr error) {
	if !c.IsActive() {
		return net.ErrClosed
	}

	if message, ok := obj.(Message); !ok {
		kklogger.ErrorJ("gws:WebSocketChannel.UnsafeWrite#unsafe_write!type_error", channel.ErrUnknownObjectType)
		return channel.ErrUnknownObjectType
	} else {
		if err := func() error {
			bs := message.Encoded()
			switch message.(type) {
			case *CloseMessage, *PingMessage, *PongMessage:
				dead := func() time.Time {
					if message.Deadline() == nil {
						return time.Now().Add(time.Second * 3)
					}

					return *message.Deadline()
				}()

				return c.wsConn.WriteControl(message.Type().wsLibType(), bs, dead)
			case *DefaultMessage:
				deadline := time.Time{}
				if c.WriteTimeout > 0 {
					deadline = time.Now().Add(c.WriteTimeout)
				}
				if err := c.wsConn.SetWriteDeadline(deadline); err != nil {
					return err
				}
				return c.wsConn.WriteMessage(message.Type().wsLibType(), bs)
			default:
				return ErrWrongObjectType
			}
		}(); err != nil {
			kklogger.WarnJ("gws:WebSocketChannel.UnsafeWrite#unsafe_write!write_error", c._NewWSLog(message, err))
			return err
		}
	}

	return nil
}

func (c *Channel) UnsafeRead() (obj any, rtErr error) {
	if c.Conn() == nil {
		return nil, channel.ErrNilObject
	}

	if !c.IsActive() {
		return nil, net.ErrClosed
	}

	c._ExtendReadDeadline()
	typ, bs, err := c._ReadMessage()
	if err != nil {
		if c.IsActive() {
			if wsErr, ok := err.(*websocket.CloseError); !(ok && wsErr.Code == 1000) {
				kklogger.WarnJ("gws:WebSocketChannel.read#read!read_error", err.Error())
			}
		}

		return nil, err
	} else {
		return _ParseMessage(typ, bs), nil
	}
}

func (c *Channel) UnsafeConnect(localAddr net.Addr, remoteAddr net.Addr) error {
	if conf, ok := remoteAddr.(*WSCustomConnectConfig); !ok {
		return channel.ErrUnknownObjectType
	} else {
		wsConn, resp, err := websocket.DefaultDialer.Dial(conf.Url, conf.Header)
		if err != nil {
			return err
		}

		c.Response = gtp.WrapResponse(c, resp)
		c.wsConn = wsConn
		c.wsConn.SetPingHandler(c._PingHandler)
		c.wsConn.SetPongHandler(c._PongHandler)
		c.wsConn.SetCloseHandler(c._CloseHandler)
		c.SetConn(wsConn.UnderlyingConn())
	}

	return nil
}

func (c *Channel) _PingHandler(message string) error {
	c._ExtendReadDeadline()
	msg := &PingMessage{
		DefaultMessage: DefaultMessage{
			MessageType: PingMessageType,
			Message:     []byte(message),
		},
	}

	c.FireRead(msg)
	c.FireReadCompleted()
	return nil
}

func (c *Channel) _PongHandler(message string) error {
	c._ExtendReadDeadline()
	msg := &PongMessage{
		DefaultMessage: DefaultMessage{
			MessageType: PongMessageType,
			Message:     []byte(message),
		},
	}

	c.FireRead(msg)
	c.FireReadCompleted()
	return nil
}

func (c *Channel) _CloseHandler(code int, text string) error {
	msg := &CloseMessage{
		DefaultMessage: DefaultMessage{
			MessageType: CloseMessageType,
			Message:     []byte(text),
		},
		CloseCode: CloseCode(code),
	}

	c.FireRead(msg)
	c.FireReadCompleted()
	return nil
}

// Control frames renew idle time but never extend the current message's assembly deadline.
func (c *Channel) _ExtendReadDeadline() {
	deadline := c.assemblyDeadline
	if c.ReadIdleTimeout > 0 {
		idle := time.Now().Add(c.ReadIdleTimeout)
		if deadline.IsZero() || idle.Before(deadline) {
			deadline = idle
		}
	}
	c.wsConn.SetReadDeadline(deadline)
}

func (c *Channel) _NewWSLog(message Message, err error) (entry *LogStruct) {
	uri := "/"
	if c.Request != nil {
		uri = c.Request.LogURI()
	} else if c.Response != nil && c.Response.Request() != nil {
		uri = c.Response.Request().LogURI()
	}
	log := &LogStruct{
		LogType:    LogType,
		ChannelID:  c.ID(),
		RequestURI: uri,
		Message:    message,
		Error:      err,
	}

	if c.wsConn != nil {
		log.RemoteAddr = c.wsConn.RemoteAddr()
		log.LocalAddr = c.wsConn.LocalAddr()
	}

	return log
}

func (c *Channel) _ReadMessage() (typ int, data []byte, rtErr error) {
	if c.inboundBudget == nil {
		c.inboundBudget = channel.InboundBudgetFor(c)
		c.readLimit = channel.GetParamInt64Default(c, ParamWSReadLimit, DefaultWSReadLimit)
		timeout := channel.GetParamInt64Default(c, channel.ParamFrameAssemblyTimeout, channel.DefaultFrameAssemblyTimeout)
		if timeout <= 0 {
			timeout = channel.DefaultFrameAssemblyTimeout
		}
		c.assemblyTimeout = time.Duration(timeout) * time.Millisecond
		c.wsConn.SetReadLimit(c.readLimit)
	}
	typ, reader, err := c.wsConn.NextReader()
	if err != nil {
		return 0, nil, err
	}
	c.assemblyDeadline = time.Now().Add(c.assemblyTimeout)
	c._ExtendReadDeadline()

	// Socket deadlines cannot interrupt a synchronous Ping/Pong callback.
	var finished atomic.Bool
	timer := time.AfterFunc(c.assemblyTimeout, func() {
		if !finished.Swap(true) {
			c.UnsafeDisconnect()
		}
	})
	defer func() { finished.Store(true); timer.Stop() }()
	reserved := int64(0)
	defer func() { c.inboundBudget.Release(reserved); c.assemblyDeadline = time.Time{} }()
	for {
		if len(data) == cap(data) {
			if len(data) > 0 {
				// At a capacity boundary, check for message completion before growing.
				_, readErr := reader.Read(nil)
				if readErr == io.EOF {
					return typ, data, nil
				}
				if readErr != nil {
					return 0, nil, readErr
				}
			}
			if c.readLimit > 0 && int64(len(data)) == c.readLimit {
				return 0, nil, channel.ErrFrameTooLarge
			}
			nextCapacity := max(512, 2*cap(data))
			if c.readLimit > 0 && int64(nextCapacity) > c.readLimit {
				nextCapacity = int(c.readLimit)
			}
			if !c.inboundBudget.Reserve(int64(nextCapacity)) {
				return 0, nil, channel.ErrInboundBudget
			}
			next := make([]byte, len(data), nextCapacity)
			copy(next, data)
			c.inboundBudget.Release(reserved)
			reserved = int64(nextCapacity)
			data = next
		}
		n, readErr := reader.Read(data[len(data):cap(data)])
		data = data[:len(data)+n]
		if readErr != nil {
			if readErr == io.EOF {
				return typ, data, nil
			}
			return 0, nil, readErr
		}
	}
}

type WSCustomConnectConfig struct {
	Url    string
	Header http.Header
}

func (c *WSCustomConnectConfig) Network() string {
	return "ws"
}

func (c *WSCustomConnectConfig) String() string {
	return c.Url
}
