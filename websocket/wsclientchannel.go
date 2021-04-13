package websocket

import (
	"fmt"
	"io"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/kklab-com/gone/channel"
	kklogger "github.com/kklab-com/goth-kklogger"
	"github.com/kklab-com/goth-kkutil/buf"
	kkpanic "github.com/kklab-com/goth-panic"
)

type DefaultWSClientChannel struct {
	*channel.DefaultNetClientChannel
	netClientCustomConnectFunc func(v interface{}) error
	conn                       *websocket.Conn
}

var ErrUnknownObjectType = fmt.Errorf("unknown object type")

func (c *DefaultWSClientChannel) Init() channel.Channel {
	if c.DefaultNetClientChannel == nil {
		c.DefaultNetClientChannel = channel.NewDefaultNetClientChannel()
		c.netClientCustomConnectFunc = c.Unsafe.CustomConnectFunc
	}

	c.ChannelPipeline = channel.NewDefaultPipeline(c)
	c.Unsafe.CustomConnectFunc = c.customConnect
	c.Unsafe.WriteFunc = c.write
	return c
}

func (c *DefaultWSClientChannel) write(obj interface{}) error {
	var bs []byte
	switch v := obj.(type) {
	case Message:
		bs = v.Encoded()
	default:
		kklogger.ErrorJ("DefaultWSClientChannel.write", ErrUnknownObjectType)
		return ErrUnknownObjectType
	}

	if _, err := c.Conn().Write(bs); err != nil {
		kklogger.ErrorJ("DefaultWSClientChannel.write", err.Error())
		return err
	}

	return nil
}

func (c *DefaultWSClientChannel) read() {
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
				c.FireRead(_ParseMessage())
				c.FireReadCompleted()
			}, func(r *kkpanic.Caught) {
				kklogger.ErrorJ("DefaultWSClientChannel.read", r.String())
			})
		}
	}
}

func (c *DefaultWSClientChannel) customConnect(v interface{}) error {
	if conf, ok := v.(*WSCustomConnectConfig); !ok {
		return ErrUnknownObjectType
	} else {
		wsConn, resp, err := websocket.DefaultDialer.Dial(conf.Url, conf.Header)
		if err != nil {
			return err
		}

		c.SetParam(ParamWSHttpResponse, resp)
		c.conn = wsConn
		if err := c.netClientCustomConnectFunc(wsConn); err != nil {
			c.Disconnect()
			return err
		}
	}

	go c.read()
	return nil
}

type WSCustomConnectConfig struct {
	Url    string
	Header http.Header
}
