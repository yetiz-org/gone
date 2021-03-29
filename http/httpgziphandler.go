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

func (h *GZipHandler) Write(ctx channel.HandlerContext, obj interface{}) {
	pack := _UnPack(obj)
	if pack == nil {
		ctx.FireWrite(obj)
		return
	}

	response := pack.Resp
	params := pack.Params
	if response == nil {
		ctx.FireWrite(obj)
		return
	}

	if response.body.Len() > 0 && strings.Contains(response.request.Header.Get(httpheadername.AcceptEncoding), "gzip") {
		st := time.Now()
		response.SetHeader(httpheadername.ContentEncoding, "gzip")
		response.SetBody(h.gzipWrite(response.body))
		params["[gone]compress_time"] = time.Now().Sub(st).Nanoseconds()
	}

	ctx.FireWrite(obj)
}

func (h *GZipHandler) gzipWrite(buffer *bytes.Buffer) *bytes.Buffer {
	gzBuffer := bytes.NewBuffer([]byte{})
	writer, _ := gzip.NewWriterLevel(gzBuffer, gzip.BestSpeed)
	defer writer.Close()
	writer.Write(buffer.Bytes())
	writer.Flush()
	return gzBuffer
}
