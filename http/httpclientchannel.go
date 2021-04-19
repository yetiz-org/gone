package http

import (
	"net/http"

	"github.com/kklab-com/gone/channel"
)

type Channel struct {
	channel.DefaultNetChannel
	writer ResponseWriter
}

func (c *Channel) UnsafeRead() error {
	return nil
}

func (c *Channel) UnsafeWrite(obj interface{}) error {
	pack := _UnPack(obj)
	if pack == nil {
		return channel.ErrUnknownObjectType
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
