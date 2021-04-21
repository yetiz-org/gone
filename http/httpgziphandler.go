package http

import (
	"bytes"
	"compress/gzip"
	"strings"
	"time"

	"github.com/kklab-com/gone-httpheadername"
	"github.com/kklab-com/gone/channel"
)

type GZipHandler struct {
	channel.DefaultHandler
}

func (h *GZipHandler) Write(ctx channel.HandlerContext, obj interface{}, future channel.Future) {
	pack := _UnPack(obj)
	if pack == nil {
		ctx.Write(obj, future)
		return
	}

	response := pack.Response
	params := pack.Params
	if response == nil {
		ctx.Write(obj, future)
		return
	}

	if response.body.Len() > 0 && strings.Contains(response.request.Header.Get(httpheadername.AcceptEncoding), "gzip") {
		st := time.Now()
		response.SetHeader(httpheadername.ContentEncoding, "gzip")
		response.SetBody(h.gzipWrite(response.body))
		params["[gone]compress_time"] = time.Now().Sub(st).Nanoseconds()
	}

	ctx.Write(obj, future)
}

func (h *GZipHandler) gzipWrite(buffer *bytes.Buffer) *bytes.Buffer {
	gzBuffer := bytes.NewBuffer([]byte{})
	writer, _ := gzip.NewWriterLevel(gzBuffer, gzip.BestSpeed)
	defer writer.Close()
	writer.Write(buffer.Bytes())
	writer.Flush()
	return gzBuffer
}
