package example

import (
	"github.com/yetiz-org/gone/channel"
	"github.com/yetiz-org/gone/ghttp"
	"github.com/yetiz-org/gone/ghttp/httpstatus"
	buf "github.com/yetiz-org/goth-bytebuf"
)

// AutoVideoTask demonstrates automatic Range request handling
type AutoVideoTask struct {
	ghttp.DefaultHTTPHandlerTask
	videoData []byte
}

func (h *AutoVideoTask) Get(ctx channel.HandlerContext, req *ghttp.Request, resp *ghttp.Response, params map[string]any) ghttp.ErrorResponse {
	resp.SetHeader("Content-Type", "video/mp4")
	resp.SetStatusCode(httpstatus.OK)
	resp.SetBody(buf.NewByteBuf(h.videoData))
	return nil
}

// VideoTaskDisableAutoRange shows how to disable auto Range
type VideoTaskDisableAutoRange struct {
	ghttp.DefaultHTTPHandlerTask
	videoData []byte
}

func (h *VideoTaskDisableAutoRange) EnableAutoRangeSupport() bool {
	return false
}

func (h *VideoTaskDisableAutoRange) Get(ctx channel.HandlerContext, req *ghttp.Request, resp *ghttp.Response, params map[string]any) ghttp.ErrorResponse {
	resp.SetHeader("Content-Type", "video/mp4")
	resp.SetStatusCode(httpstatus.OK)
	resp.SetBody(buf.NewByteBuf(h.videoData))
	return nil
}

// CustomRangeTask shows custom Range handling with ParseRange
type CustomRangeTask struct {
	ghttp.DefaultHTTPHandlerTask
	data []byte
}

func (h *CustomRangeTask) EnableAutoRangeSupport() bool {
	return false
}

func (h *CustomRangeTask) Get(ctx channel.HandlerContext, req *ghttp.Request, resp *ghttp.Response, params map[string]any) ghttp.ErrorResponse {
	rangeHeader := req.Header().Get("Range")
	if rangeHeader != "" {
		if start, end, valid := ghttp.ParseRange(rangeHeader, int64(len(h.data))); valid {
			rangeData := h.data[start : end+1]
			resp.SetStatusCode(httpstatus.PartialContent)
			resp.SetHeader("Content-Range", "bytes ...")
			resp.SetBody(buf.NewByteBuf(rangeData))
		} else {
			resp.SetStatusCode(httpstatus.RequestedRangeNotSatisfiable)
		}
	} else {
		resp.SetStatusCode(httpstatus.OK)
		resp.SetBody(buf.NewByteBuf(h.data))
	}

	return nil
}
