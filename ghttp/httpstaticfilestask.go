package ghttp

import (
	"bytes"
	"container/list"
	"errors"
	"io"
	"mime"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

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
	"golang.org/x/sync/singleflight"
)

const (
	defaultStaticMaxFileBytes       = 16 << 20
	defaultStaticMaxCacheBytes      = 64 << 20
	defaultStaticMaxCacheEntries    = 1024
	defaultStaticMaxConcurrentLoads = 8
)

var (
	errStaticFileTooLarge = errors.New("static file exceeds byte limit")
	errStaticLoadLimit    = errors.New("static file load limit reached")
)

// StaticFilesHandlerTask serves bounded assets. Configure fields before serving requests.
type StaticFilesHandlerTask struct {
	DefaultHTTPHandlerTask
	FolderPath string
	DoMinify   bool
	DoCache    bool
	// AllowExternalSymlinks opts into legacy access outside FolderPath.
	AllowExternalSymlinks bool
	// Nonpositive limits use the constructor defaults: 16 MiB per file,
	// 64 MiB cached payload, 1024 cache entries, and 8 concurrent loads.
	MaxFileBytes       int64
	MaxCacheBytes      int64
	MaxCacheEntries    int
	MaxConcurrentLoads int
	cacheMap           map[string]*staticFileCacheEntity
	cacheMu            sync.Mutex
	cacheOrder         list.List
	cacheBytes         int64
	loads              singleflight.Group
	loadOnce           sync.Once
	loadSlots          chan struct{}
	m                  *minify.M
}

func NewStaticFilesHandlerTask(folderPath string) (handler *StaticFilesHandlerTask) {
	if folderPath == "" {
		folderPath = "./resources/static"
	}

	s := &StaticFilesHandlerTask{
		FolderPath:         folderPath,
		DoMinify:           true,
		DoCache:            true,
		MaxFileBytes:       defaultStaticMaxFileBytes,
		MaxCacheBytes:      defaultStaticMaxCacheBytes,
		MaxCacheEntries:    defaultStaticMaxCacheEntries,
		MaxConcurrentLoads: defaultStaticMaxConcurrentLoads,
		cacheMap:           map[string]*staticFileCacheEntity{},
		m:                  minify.New(),
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

func (h *StaticFilesHandlerTask) Get(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) (rtErr ErrorResponse) {
	reqPath := req.Url().Path
	if reqPath == "" {
		reqPath = "/"
	}

	clean := path.Clean("/" + reqPath)
	if strings.HasPrefix(clean, "/../") || clean == "/.." {
		resp.SetStatusCode(httpstatus.NotFound)
		return nil
	}

	root := filepath.Clean(h.FolderPath)
	cleanRel := strings.TrimPrefix(clean, "/")
	fsPath := filepath.Clean(filepath.Join(root, filepath.FromSlash(cleanRel)))
	if entity, err := h._Load(fsPath); entity != nil {
		resp.SetHeader(httpheadername.ContentType, entity.contentType)
		resp.SetStatusCode(httpstatus.OK)
		resp.SetBody(buf.NewByteBuf(entity.data))
	} else if err == nil {
		resp.SetStatusCode(httpstatus.NotFound)
	} else if errors.Is(err, errStaticFileTooLarge) || errors.Is(err, errStaticLoadLimit) {
		resp.SetStatusCode(httpstatus.ServiceUnavailable)
	} else {
		resp.SetStatusCode(httpstatus.InternalServerError)
	}

	return nil
}

func (h *StaticFilesHandlerTask) _Load(filePath string) (entity *staticFileCacheEntity, rtErr error) {
	if cached := h._Cached(filePath); cached != nil {
		return cached, nil
	}
	h.loadOnce.Do(func() {
		limit := h.MaxConcurrentLoads
		if limit <= 0 {
			limit = defaultStaticMaxConcurrentLoads
		}
		h.loadSlots = make(chan struct{}, limit)
	})
	loaded, err, _ := h.loads.Do(filePath, func() (any, error) {
		if cached := h._Cached(filePath); cached != nil {
			return cached, nil
		}
		select {
		case h.loadSlots <- struct{}{}:
			defer func() { <-h.loadSlots }()
		default:
			return nil, errStaticLoadLimit
		}
		result, readErr := h._ReadFile(filePath)
		if result != nil && h.DoCache {
			h._Cache(filePath, result)
		}
		return result, readErr
	})
	if err != nil || loaded == nil {
		return nil, err
	}
	return loaded.(*staticFileCacheEntity), nil
}

func (h *StaticFilesHandlerTask) _Cached(filePath string) (entity *staticFileCacheEntity) {
	if !h.DoCache {
		return nil
	}
	h.cacheMu.Lock()
	defer h.cacheMu.Unlock()
	if cached := h.cacheMap[filePath]; cached != nil {
		h.cacheOrder.MoveToFront(cached.cacheElement)
		return cached
	}
	return nil
}

func (h *StaticFilesHandlerTask) _Cache(filePath string, entity *staticFileCacheEntity) {
	byteLimit, entryLimit := h.MaxCacheBytes, h.MaxCacheEntries
	if byteLimit <= 0 {
		byteLimit = defaultStaticMaxCacheBytes
	}
	if entryLimit <= 0 {
		entryLimit = defaultStaticMaxCacheEntries
	}
	if int64(len(entity.data)) > byteLimit {
		return
	}
	if cap(entity.data) > len(entity.data) {
		compact := make([]byte, len(entity.data))
		copy(compact, entity.data)
		entity.data = compact
	}
	h.cacheMu.Lock()
	defer h.cacheMu.Unlock()
	for len(h.cacheMap) >= entryLimit || h.cacheBytes > byteLimit-int64(len(entity.data)) {
		oldest := h.cacheOrder.Back()
		key := oldest.Value.(string)
		h.cacheBytes -= int64(len(h.cacheMap[key].data))
		delete(h.cacheMap, key)
		h.cacheOrder.Remove(oldest)
	}
	if h.cacheMap == nil {
		h.cacheMap = make(map[string]*staticFileCacheEntity)
	}
	entity.cacheElement = h.cacheOrder.PushFront(filePath)
	h.cacheMap[filePath] = entity
	h.cacheBytes += int64(len(entity.data))
}

func (h *StaticFilesHandlerTask) _ReadFile(filePath string) (entity *staticFileCacheEntity, rtErr error) {
	root, err := filepath.Abs(h.FolderPath)
	if err != nil {
		return nil, err
	}
	fullPath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, err
	}
	relative, err := filepath.Rel(root, fullPath)
	if err != nil || !filepath.IsLocal(relative) {
		return nil, os.ErrPermission
	}
	var file *os.File
	if h.AllowExternalSymlinks {
		file, err = os.OpenFile(fullPath, staticOpenFlags, 0)
	} else {
		var dir *os.Root
		if dir, err = os.OpenRoot(root); err == nil {
			file, err = dir.OpenFile(relative, staticOpenFlags, 0)
			dir.Close()
		}
	}
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		kklogger.WarnJ("ghttp:StaticFilesHandlerTask.Get#get!file_warn", err.Error())
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, os.ErrPermission
	}
	limit := h.MaxFileBytes
	if limit <= 0 {
		limit = defaultStaticMaxFileBytes
	}
	if info.Size() > limit {
		return nil, errStaticFileTooLarge
	}
	data, err := io.ReadAll(io.LimitReader(file, limit))
	if err == nil && int64(len(data)) == limit {
		var extra [1]byte
		if n, readErr := file.Read(extra[:]); n != 0 {
			return nil, errStaticFileTooLarge
		} else if readErr != nil && !errors.Is(readErr, io.EOF) {
			err = readErr
		}
	}
	if err != nil {
		kklogger.ErrorJ("ghttp:StaticFilesHandlerTask.Get#get!file_error", err.Error())
		return nil, err
	}
	result := &staticFileCacheEntity{data: data, contentType: mime.TypeByExtension(filepath.Ext(filePath))}
	if h.DoMinify {
		mini := &staticFileWriter{limit: limit}
		minifyErr := h.m.Minify(result.contentType, mini, bytes.NewReader(data))
		if mini.exceeded {
			return nil, errStaticFileTooLarge
		}
		if minifyErr == nil && mini.buffer.Len() != 0 {
			result.data = mini.buffer.Bytes()
		}
	}
	return result, nil
}

// _ParseRange wraps ParseRange for backward compatibility
func (h *StaticFilesHandlerTask) _ParseRange(rangeHeader string, fileSize int64) (int64, int64, bool) {
	return ParseRange(rangeHeader, fileSize)
}

type staticFileCacheEntity struct {
	data         []byte
	contentType  string
	cacheElement *list.Element
}

type staticFileWriter struct {
	buffer   bytes.Buffer
	limit    int64
	exceeded bool
}

func (w *staticFileWriter) Write(data []byte) (n int, rtErr error) {
	if w.exceeded || int64(len(data)) > w.limit-int64(w.buffer.Len()) {
		w.exceeded = true
		return 0, errStaticFileTooLarge
	}
	return w.buffer.Write(data)
}
