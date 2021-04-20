package websocket

import (
	"errors"
	"io"
	"net"
	"os"
	"time"

	"github.com/gorilla/websocket"
	"github.com/kklab-com/gone/channel"
	"github.com/kklab-com/gone/http"
	kklogger "github.com/kklab-com/goth-kklogger"
)

type ChildChannel struct {
	*channel.DefaultNetChannel
	wsConn  *websocket.Conn
	Request *http.Request
}

func (c *ChildChannel) UnsafeWrite(obj interface{}) error {
	if !c.IsActive() {
		return net.ErrClosed
	}

	if message, ok := obj.(Message); !ok {
		kklogger.ErrorJ("websocket:ChildChannel.UnsafeWrite", channel.ErrUnknownObjectType)
		return channel.ErrUnknownObjectType
	} else {
		if err := func() error {
			if c.WriteTimeout > 0 {
				c.Conn().SetWriteDeadline(time.Now().Add(c.WriteTimeout))
			}

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
			kklogger.WarnJ("websocket:ChildChannel.UnsafeWrite", c._NewWSLog(message, err))
			return err
		}
	}

	return nil
}

func (c *ChildChannel) UnsafeRead() error {
	if c.Conn() == nil {
		return channel.ErrNilObject
	}

	if !c.IsActive() {
		return net.ErrClosed
	}

	if !c.DoRead() {
		return nil
	}

	go func() {
		for c.IsActive() {
			c.wsConn.SetReadLimit(channel.GetParamInt64Default(c, ParamWSReadLimit, 0))
			if c.ReadTimeout > 0 {
				c.Conn().SetReadDeadline(time.Now().Add(c.ReadTimeout))
			}

			typ, bs, err := c.wsConn.ReadMessage()
			if err != nil {
				if errors.Is(err, os.ErrDeadlineExceeded) && c.Conn().IsActive() {
					continue
				}

				if c.IsActive() {
					if err != io.EOF {
						kklogger.WarnJ("websocket:ChildChannel.read", err.Error())
					}

					if err == websocket.ErrReadLimit {
						c.Disconnect()
						break
					}

					if !c.Conn().IsActive() {
						c.Deregister()
						break
					}
				} else if err == io.EOF {
					break
				}
			} else {
				c.FireRead(_ParseMessage(typ, bs))
				c.FireReadCompleted()
			}
		}

		c.ReleaseRead()
	}()

	return nil
}

func (c *ChildChannel) _PingHandler(message string) error {
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

func (c *ChildChannel) _PongHandler(message string) error {
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

func (c *ChildChannel) _CloseHandler(code int, text string) error {
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

func (c *ChildChannel) _NewWSLog(message Message, err error) *WSLogStruct {
	log := &WSLogStruct{
		LogType:    WSLogType,
		ChannelID:  c.ID(),
		RequestURI: c.Request.RequestURI,
		Message:    message,
		Error:      err,
	}

	if c.wsConn != nil {
		log.RemoteAddr = c.wsConn.RemoteAddr()
		log.LocalAddr = c.wsConn.LocalAddr()
	}

	return log
}
