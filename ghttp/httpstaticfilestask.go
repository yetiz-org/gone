package ghttp

import (
	"fmt"
	"io/ioutil"
	"mime"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/tdewolff/minify/v2"
	"github.com/tdewolff/minify/v2/css"
	"github.com/tdewolff/minify/v2/html"
	"github.com/tdewolff/minify/v2/js"
	"github.com/tdewolff/minify/v2/json"
	"github.com/tdewolff/minify/v2/svg"
	"github.com/tdewolff/minify/v2/xml"
	"github.com/yetiz-org/gone/channel"
	"github.com/yetiz-org/gone/ghttp/httpheadername"
	"github.com/yetiz-org/gone/ghttp/httpstatus"
	buf "github.com/yetiz-org/goth-bytebuf"
	kklogger "github.com/yetiz-org/goth-kklogger"
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

	if entity, err := h._Load(path); entity != nil {
		resp.SetHeader(httpheadername.ContentType, entity.contentType)
		resp.SetStatusCode(httpstatus.OK)
		resp.SetBody(buf.NewByteBuf(entity.data))
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

// _ParseRange wraps ParseRange for backward compatibility
func (h *StaticFilesHandlerTask) _ParseRange(rangeHeader string, fileSize int64) (int64, int64, bool) {
	return ParseRange(rangeHeader, fileSize)
}

type staticFileCacheEntity struct {
	data        []byte
	contentType string
}
