package websocket

import (
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/kklab-com/gone/channel"
	gtp "github.com/kklab-com/gone/http"
	kklogger "github.com/kklab-com/goth-kklogger"
)

var ErrWrongObjectType = fmt.Errorf("wrong object type")

type Channel struct {
	*channel.DefaultNetChannel
	wsConn   *websocket.Conn
	Response *gtp.Response
	Request  *gtp.Request
}

func (c *Channel) BootstrapPreInit() {
	c.DefaultNetChannel = &channel.DefaultNetChannel{}
}

func (c *Channel) Init() channel.Channel {
	return c.DefaultNetChannel.Init()
}

func (c *Channel) UnsafeWrite(obj interface{}) error {
	if !c.IsActive() {
		return net.ErrClosed
	}

	if message, ok := obj.(Message); !ok {
		kklogger.ErrorJ("websocket:Channel.UnsafeWrite", channel.ErrUnknownObjectType)
		return channel.ErrUnknownObjectType
	} else {
		if err := func() error {
			switch message.(type) {
			case *CloseMessage, *PingMessage, *PongMessage:
				dead := func() time.Time {
					if message.Deadline() == nil {
						return time.Now().Add(time.Second * 3)
					}

					return *message.Deadline()
				}()

				return c.wsConn.WriteControl(message.Type().wsLibType(), message.Encoded(), dead)
			case *DefaultMessage:
				return c.wsConn.WriteMessage(message.Type().wsLibType(), message.Encoded())
			default:
				return ErrWrongObjectType
			}
		}(); err != nil {
			kklogger.WarnJ("websocket:Channel.UnsafeWrite", c._NewWSLog(message, err))
			return err
		}
	}

	return nil
}

func (c *Channel) UnsafeRead() (interface{}, error) {
	if c.Conn() == nil {
		return nil, channel.ErrNilObject
	}

	if !c.IsActive() {
		return nil, net.ErrClosed
	}

	c.wsConn.SetReadLimit(channel.GetParamInt64Default(c, ParamWSReadLimit, 0))
	typ, bs, err := c.wsConn.ReadMessage()
	if err != nil {
		if c.IsActive() {
			if wsErr, ok := err.(*websocket.CloseError); !(ok && wsErr.Code == 1000) {
				kklogger.WarnJ("websocket:Channel.read", err.Error())
			}

			if c.Conn().IsActive() {
				c.Disconnect()
			} else {
				c.Deregister()
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

func (c *Channel) _NewWSLog(message Message, err error) *WSLogStruct {
	log := &WSLogStruct{
		LogType:    WSLogType,
		ChannelID:  c.ID(),
		RequestURI: c.Response.Request().RequestURI(),
		Message:    message,
		Error:      err,
	}

	if c.wsConn != nil {
		log.RemoteAddr = c.wsConn.RemoteAddr()
		log.LocalAddr = c.wsConn.LocalAddr()
	}

	return log
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
