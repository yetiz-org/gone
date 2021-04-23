package http

import (
	"compress/gzip"
	"strings"
	"time"

	"github.com/kklab-com/gone-httpheadername"
	"github.com/kklab-com/gone/channel"
	"github.com/kklab-com/goth-kkutil/buf"
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

	if response.body.ReadableBytes() > 0 && strings.Contains(response.request.Header().Get(httpheadername.AcceptEncoding), "gzip") {
		st := time.Now()
		response.SetHeader(httpheadername.ContentEncoding, "gzip")
		response.SetBody(h.gzipWrite(response.body))
		params["[gone-http]compress_time"] = time.Now().Sub(st).Nanoseconds()
	}

	ctx.Write(obj, future)
}

func (h *GZipHandler) gzipWrite(buffer buf.ByteBuf) buf.ByteBuf {
	gzBuffer := buf.EmptyByteBuf()
	writer, _ := gzip.NewWriterLevel(gzBuffer, gzip.BestSpeed)
	defer writer.Close()
	writer.Write(buffer.Bytes())
	writer.Flush()
	return gzBuffer
}
