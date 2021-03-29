package http

import (
	"fmt"
	"net/http"

	"github.com/kklab-com/gone/channel"
)

type DefaultClientChannel struct {
	channel.DefaultNetClientChannel
	writer ResponseWriter
}

var UnknownObjectType = fmt.Errorf("unknown object type")

func (c *DefaultClientChannel) Init() channel.Channel {
	c.ChannelPipeline = channel.NewDefaultPipeline(c)
	c.Unsafe.WriteFunc = c.write
	return c
}

func (c *DefaultClientChannel) write(obj interface{}) error {
	pack := _UnPack(obj)
	if pack == nil {
		return UnknownObjectType
	}

	response := pack.Resp
	for key, values := range response.header {
		for _, value := range values {
			c.writer.Header().Add(key, value)
		}
	}

	for _, value := range response.cookies {
		for _, cookie := range value {
			http.SetCookie(c.writer, &cookie)
		}
	}

	c.writer.WriteHeader(response.statusCode)
	_, err := c.writer.Write(response.Body())
	return err
}
