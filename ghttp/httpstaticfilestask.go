package ghttp

import (
	"fmt"
	"io/ioutil"
	"mime"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/yetiz-org/gone/ghttp/httpheadername"
	"github.com/yetiz-org/gone/ghttp/httpstatus"

	"github.com/tdewolff/minify/v2"
	"github.com/tdewolff/minify/v2/css"
	"github.com/tdewolff/minify/v2/html"
	"github.com/tdewolff/minify/v2/js"
	"github.com/tdewolff/minify/v2/json"
	"github.com/tdewolff/minify/v2/svg"
	"github.com/tdewolff/minify/v2/xml"
	"github.com/yetiz-org/gone/channel"
	buf "github.com/yetiz-org/goth-bytebuf"
	"github.com/yetiz-org/goth-kklogger"
)

type StaticFilesHandlerTask struct {
	DefaultHTTPHandlerTask
	FolderPath string
	DoMinify   bool
	DoCache    bool
	cacheMap   map[string]*staticFileCacheEntity
	m          *minify.M
}

func NewStaticFilesHandlerTask(folderPath string) *StaticFilesHandlerTask {
	if folderPath == "" {
		folderPath = "./resources/static"
	}

	s := &StaticFilesHandlerTask{
		FolderPath: folderPath,
		DoMinify:   true,
		DoCache:    true,
		cacheMap:   map[string]*staticFileCacheEntity{},
		m:          minify.New(),
	}

	s.m.AddFunc("text/css", css.Minify)
	s.m.AddFunc("text/html", html.Minify)
	s.m.AddFunc("image/svg+xml", svg.Minify)
	s.m.AddFuncRegexp(regexp.MustCompile("^(application|text)/(x-)?(java|ecma)script$"), js.Minify)
	s.m.AddFuncRegexp(regexp.MustCompile("[/+]json$"), json.Minify)
	s.m.AddFuncRegexp(regexp.MustCompile("[/+]xml$"), xml.Minify)
	s.m.AddFuncRegexp(regexp.MustCompile("^(application|text)/(x-)?(java|ecma)script$"), js.Minify)

	return s
}

func (h *StaticFilesHandlerTask) Get(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) ErrorResponse {
	path := fmt.Sprintf("%s/%s", h.FolderPath, strings.ReplaceAll(req.Url().Path, "../", "/"))

	// Parse Range header
	rangeHeader := req.Header().Get(httpheadername.Range)

	if entity, err := h._Load(path); entity != nil {
		// Set Accept-Ranges header to indicate range request support
		resp.SetHeader(httpheadername.AcceptRanges, "bytes")
		resp.SetHeader(httpheadername.ContentType, entity.contentType)

		// Handle Range request
		if rangeHeader != "" {
			if start, end, valid := h._ParseRange(rangeHeader, int64(len(entity.data))); valid {
				// Valid Range request
				rangeData := entity.data[start : end+1]
				resp.SetStatusCode(httpstatus.PartialContent)
				resp.SetHeader(httpheadername.ContentRange, fmt.Sprintf("bytes %d-%d/%d", start, end, len(entity.data)))
				resp.SetHeader(httpheadername.ContentLength, strconv.Itoa(len(rangeData)))
				resp.SetBody(buf.NewByteBuf(rangeData))
				kklogger.DebugJ("ghttp:StaticFilesHandlerTask.Get#range_request", fmt.Sprintf("path=%s, range=%d-%d/%d", path, start, end, len(entity.data)))
			} else {
				// Invalid Range request
				resp.SetStatusCode(httpstatus.RequestedRangeNotSatisfiable)
				resp.SetHeader(httpheadername.ContentRange, fmt.Sprintf("bytes */%d", len(entity.data)))
				kklogger.WarnJ("ghttp:StaticFilesHandlerTask.Get#range_request!invalid_range", fmt.Sprintf("path=%s, range=%s", path, rangeHeader))
			}
		} else {
			// Normal request (no Range header)
			resp.SetStatusCode(httpstatus.OK)
			resp.SetHeader(httpheadername.ContentLength, strconv.Itoa(len(entity.data)))
			resp.SetBody(buf.NewByteBuf(entity.data))
		}
	} else if err == nil {
		resp.SetStatusCode(httpstatus.NotFound)
	} else {
		resp.SetStatusCode(httpstatus.InternalServerError)
	}

	return nil
}

func (h *StaticFilesHandlerTask) _Load(path string) (*staticFileCacheEntity, error) {
	if h.DoCache {
		if entity, f := h.cacheMap[path]; f {
			return entity, nil
		}
	}

	if file, e := os.Open(path); e == nil {
		defer file.Close()
		if data, e := ioutil.ReadAll(file); e == nil {
			entity := staticFileCacheEntity{}
			entity.contentType = mime.TypeByExtension(filepath.Ext(path))
			if h.DoMinify {
				mini, err := h.m.Bytes(entity.contentType, data)
				if err == nil {
					entity.data = mini
				}
			}

			if len(entity.data) == 0 {
				entity.data = data
			}

			if h.DoCache {
				h.cacheMap[path] = &entity
			}

			return &entity, nil
		} else {
			kklogger.ErrorJ("ghttp:StaticFilesHandlerTask.Get#get!file_error", e.Error())
			return nil, e
		}
	} else if os.IsNotExist(e) {
		return nil, nil
	} else {
		kklogger.WarnJ("ghttp:StaticFilesHandlerTask.Get#get!file_warn", e.Error())
		return nil, e
	}
}

// _ParseRange parses HTTP Range header
// Returns start, end, valid
// Supports formats: bytes=start-end, bytes=start-, bytes=-suffix
func (h *StaticFilesHandlerTask) _ParseRange(rangeHeader string, fileSize int64) (int64, int64, bool) {
	// Check if unit is bytes
	if !strings.HasPrefix(rangeHeader, "bytes=") {
		return 0, 0, false
	}

	rangeSpec := strings.TrimPrefix(rangeHeader, "bytes=")

	// Currently only supports single range (multi-range like bytes=0-100,200-300 not supported)
	if strings.Contains(rangeSpec, ",") {
		return 0, 0, false
	}

	parts := strings.Split(rangeSpec, "-")
	if len(parts) != 2 {
		return 0, 0, false
	}

	var start, end int64
	var err error

	if parts[0] == "" {
		// Format: bytes=-500 (last N bytes)
		if parts[1] == "" {
			return 0, 0, false
		}
		suffixLength, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil || suffixLength <= 0 {
			return 0, 0, false
		}
		if suffixLength > fileSize {
			suffixLength = fileSize
		}
		start = fileSize - suffixLength
		end = fileSize - 1
	} else if parts[1] == "" {
		// Format: bytes=100- (from position to end)
		start, err = strconv.ParseInt(parts[0], 10, 64)
		if err != nil || start < 0 {
			return 0, 0, false
		}
		if start >= fileSize {
			return 0, 0, false
		}
		end = fileSize - 1
	} else {
		// Format: bytes=100-200 (specified range)
		start, err = strconv.ParseInt(parts[0], 10, 64)
		if err != nil || start < 0 {
			return 0, 0, false
		}
		end, err = strconv.ParseInt(parts[1], 10, 64)
		if err != nil || end < start || start >= fileSize {
			return 0, 0, false
		}
		// If end exceeds file size, adjust to file end
		if end >= fileSize {
			end = fileSize - 1
		}
	}

	return start, end, true
}

type staticFileCacheEntity struct {
	data        []byte
	contentType string
}
