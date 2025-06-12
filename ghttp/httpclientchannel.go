package ghttp

import (
	"github.com/yetiz-org/gone/channel"
	"net/http"
)

type Channel struct {
	channel.DefaultNetChannel
}

func (c *Channel) UnsafeIsAutoRead() bool {
	return false
}

func (c *Channel) UnsafeRead() (any, error) {
	return nil, nil
}

func (c *Channel) UnsafeWrite(obj any) error {
	pack := _UnPack(obj)
	if pack == nil {
		return channel.ErrUnknownObjectType
	}

	if pack.Response == nil {
		return nil
	}

	response := pack.Response
	if pack.writeSeparateMode {
		if !response.headerWritten {
			for key, values := range response.header {
				pack.Writer.Header().Del(key)
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
			response.headerWritten = true
		} else {
			_, err := pack.Writer.Write(response.Body().Bytes())
			if flusher, ok := pack.Writer.(http.Flusher); ok {
				flusher.Flush()
			}

			return err
		}
	} else {
		for key, values := range response.header {
			pack.Writer.Header().Del(key)
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
		_, err := pack.Writer.Write(response.Body().Bytes())
		return err
	}

	return nil
}
