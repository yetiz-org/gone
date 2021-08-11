package http

import (
	"net/http"

	"github.com/kklab-com/gone/channel"
)

type Channel struct {
	channel.DefaultNetChannel
}

func (c *Channel) UnsafeIsAutoRead() bool {
	return false
}

func (c *Channel) UnsafeRead() (interface{}, error) {
	return nil, nil
}

func (c *Channel) UnsafeWrite(obj interface{}) error {
	pack := _UnPack(obj)
	if pack == nil {
		return channel.ErrUnknownObjectType
	}

	response := pack.Response
	for key, values := range response.header {
		for _, value := range values {
			pack.Writer.Header().Add(key, value)
		}
	}

	for _, value := range response.cookies {
		for _, cookie := range value {
			http.SetCookie(pack.Writer, &cookie)
		}
	}

	pack.Writer.WriteHeader(response.statusCode)
	_, err := pack.Writer.Write(response.Body())
	return err
}
