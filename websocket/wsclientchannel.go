package websocket

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/kklab-com/gone/channel"
	kklogger "github.com/kklab-com/goth-kklogger"
	kkpanic "github.com/kklab-com/goth-panic"
)

type DefaultWSClientChannel struct {
	*channel.DefaultNetClientChannel
	netClientConnectFunc func(remoteAddr net.Addr) error
	conn                 *websocket.Conn
	response             *http.Response
}

var ErrUnknownObjectType = fmt.Errorf("unknown object type")

func (c *DefaultWSClientChannel) Init() channel.Channel {
	if c.DefaultNetClientChannel == nil {
		c.DefaultNetClientChannel = channel.NewDefaultNetClientChannel()
	}

	c.ChannelPipeline = channel.NewDefaultPipeline(c)
	c.Unsafe.ConnectFunc = c.connect
	c.Unsafe.WriteFunc = c.write
	return c
}

func (c *DefaultWSClientChannel) write(obj interface{}) error {
	if !c.IsActive() {
		return net.ErrClosed
	}

	c.WriteLock.Lock()
	defer c.WriteLock.Unlock()
	if message, ok := obj.(Message); !ok {
		kklogger.ErrorJ("DefaultWSClientChannel.write", ErrUnknownObjectType)
		return ErrUnknownObjectType
	} else {
		var err error
		switch message.(type) {
		case *CloseMessage, *PingMessage, *PongMessage:
			dead := func() time.Time {
				if message.Deadline() == nil {
					return time.Now().Add(time.Second * 3)
				}

				return *message.Deadline()
			}()

			err = c.conn.WriteControl(message.Type().wsLibType(), message.Encoded(), dead)
		case *DefaultMessage:
			err = c.conn.WriteMessage(message.Type().wsLibType(), message.Encoded())
		default:
			err = WrongObjectType
		}

		if err != nil {
			c.Disconnect()
			kklogger.WarnJ("DefaultWSClientChannel.Write#Write", c._NewWSLog(message, err))
			return err
		}
	}

	return nil
}

func (c *DefaultWSClientChannel) read() {
	defer kkpanic.Call(func(r *kkpanic.Caught) {
		c.Disconnect()
	})

	for c.IsActive() {
		if c.conn == nil {
			c.Disconnect()
			return
		}

		if typ, bs, err := c.conn.ReadMessage(); err != nil {
			if c.IsActive() {
				if err != io.EOF {
					kklogger.WarnJ("DefaultWSClientChannel.read", err.Error())
				}

				c.Disconnect()
			}
		} else {
			kkpanic.Catch(func() {
				c.FireRead(_ParseMessage(typ, bs))
				c.FireReadCompleted()
			}, func(r *kkpanic.Caught) {
				kklogger.ErrorJ("DefaultWSClientChannel.read", r.String())
			})
		}
	}
}

func (c *DefaultWSClientChannel) connect(remoteAddr net.Addr) error {
	if conf, ok := remoteAddr.(*WSCustomConnectConfig); !ok {
		return ErrUnknownObjectType
	} else {
		wsConn, resp, err := websocket.DefaultDialer.Dial(conf.Url, conf.Header)
		if err != nil {
			return err
		}

		c.response = resp
		c.conn = wsConn
		c.conn.SetPingHandler(c._PingHandler)
		c.conn.SetPongHandler(c._PongHandler)
		c.conn.SetCloseHandler(c._CloseHandler)
		c.PostActive(wsConn.UnderlyingConn())
	}

	go c.read()
	return nil
}

func (c *DefaultWSClientChannel) _PingHandler(message string) error {
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

func (c *DefaultWSClientChannel) _PongHandler(message string) error {
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

func (c *DefaultWSClientChannel) _CloseHandler(code int, text string) error {
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

func (c *DefaultWSClientChannel) _NewWSLog(message Message, err error) *WSLogStruct {
	log := &WSLogStruct{
		LogType:    WSLogType,
		ChannelID:  c.ID(),
		RequestURI: c.response.Request.RequestURI,
		Message:    message,
		Error:      err,
	}

	if c.conn != nil {
		log.RemoteAddr = c.conn.RemoteAddr()
		log.LocalAddr = c.conn.LocalAddr()
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
