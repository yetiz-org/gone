package ghttp

import (
	"compress/gzip"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/yetiz-org/gone/channel"
	"github.com/yetiz-org/gone/ghttp/httpheadername"
	buf "github.com/yetiz-org/goth-bytebuf"
)

var gzipBestSpeedWriterPool = sync.Pool{
	New: func() any {
		writer, _ := gzip.NewWriterLevel(io.Discard, gzip.BestSpeed)
		return writer
	},
}

type GZipHandler struct {
	channel.DefaultHandler
	CompressThreshold int
}

const defaultGZipCompressThreshold = 128

func (h *GZipHandler) Added(ctx channel.HandlerContext) {
	if h.CompressThreshold == 0 {
		h.CompressThreshold = defaultGZipCompressThreshold
	}
}

func (h *GZipHandler) Write(ctx channel.HandlerContext, obj any, future channel.Future) {
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

	if !h.shouldCompress(pack) {
		ctx.Write(obj, future)
		return
	}

	if h.acceptsGzip(response.request.Header().Get(httpheadername.AcceptEncoding)) {
		st := time.Now()
		if gzBody, err := h.gzipWrite(response.body); err == nil {
			response.SetHeader(httpheadername.ContentEncoding, "gzip")
			response.SetHeader(httpheadername.ContentLength, strconv.Itoa(gzBody.ReadableBytes()))
			response.SetBody(gzBody)
			if params != nil {
				params["[gone-http]compress_time"] = time.Now().Sub(st).Nanoseconds()
			}
		}
	}

	ctx.Write(obj, future)
}

func (h *GZipHandler) shouldCompress(pack *Pack) bool {
	if pack.writeSeparateMode {
		return false
	}

	response := pack.Response
	compressThreshold := h.CompressThreshold
	if compressThreshold == 0 {
		compressThreshold = defaultGZipCompressThreshold
	}
	if response.body == nil || response.body.ReadableBytes() < compressThreshold {
		return false
	}

	if response.GetHeader(httpheadername.ContentEncoding) != "" {
		return false
	}

	if response.StatusCode() == 206 || response.GetHeader(httpheadername.ContentRange) != "" {
		return false
	}

	contentType := strings.ToLower(response.GetHeader(httpheadername.ContentType))
	if idx := strings.IndexByte(contentType, ';'); idx >= 0 {
		contentType = strings.TrimSpace(contentType[:idx])
	}

	switch {
	case contentType == "":
		return true
	case strings.HasPrefix(contentType, "text/"):
		return true
	case contentType == "application/json":
		return true
	case strings.HasSuffix(contentType, "+json"):
		return true
	case contentType == "application/javascript":
		return true
	case contentType == "application/xml":
		return true
	case strings.HasSuffix(contentType, "+xml"):
		return true
	default:
		return false
	}
}

func (h *GZipHandler) acceptsGzip(header string) bool {
	var wildcardQ *float64
	for entity := range strings.SplitSeq(header, ",") {
		entity = strings.TrimSpace(entity)
		if entity == "" {
			continue
		}

		value, params, hasParams := strings.Cut(entity, ";")
		value = strings.ToLower(strings.TrimSpace(value))
		if value != "gzip" && value != "*" {
			continue
		}

		q := 1.0
		if hasParams {
			for param := range strings.SplitSeq(params, ";") {
				key, rawValue, found := strings.Cut(strings.TrimSpace(param), "=")
				if !found || strings.ToLower(strings.TrimSpace(key)) != "q" {
					continue
				}
				if parsed, err := strconv.ParseFloat(strings.TrimSpace(rawValue), 64); err == nil {
					q = parsed
				}
			}
		}

		if value == "gzip" {
			return q > 0
		}
		wildcardQ = &q
	}

	return wildcardQ != nil && *wildcardQ > 0
}

func (h *GZipHandler) gzipWrite(buffer buf.ByteBuf) (buf.ByteBuf, error) {
	// Pre-size the gzip destination to ~1/3 of the input size (typical text
	// compression ratio) with a 128-byte floor. This skips the first few
	// doublings the buffer would perform during the gzip writer's appends.
	est := buffer.ReadableBytes() / 3
	if est < 128 {
		est = 128
	}
	gzBuffer := buf.EmptyByteBuf().EnsureCapacity(est)
	writer := gzipBestSpeedWriterPool.Get().(*gzip.Writer)
	writer.Reset(gzBuffer)

	_, writeErr := writer.Write(buffer.Bytes())
	closeErr := writer.Close()
	writer.Reset(io.Discard)
	gzipBestSpeedWriterPool.Put(writer)

	if writeErr != nil {
		return nil, writeErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	return gzBuffer, nil
}
