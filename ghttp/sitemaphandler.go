package ghttp

import (
	"encoding/xml"
	"fmt"
	"net/url"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/yetiz-org/gone/channel"
	"github.com/yetiz-org/gone/ghttp/httpheadername"
	"github.com/yetiz-org/gone/ghttp/httpstatus"
	buf "github.com/yetiz-org/goth-bytebuf"
)

const sitemapNamespace = "http://www.sitemaps.org/schemas/sitemap/0.9"

// SitemapConfig controls sitemap inclusion and metadata for a route.
type SitemapConfig struct {
	Include    *bool
	Loc        string
	LastMod    *time.Time
	ChangeFreq string
	Priority   *float64
}

// SitemapConfigProvider lets a handler override sitemap inclusion or metadata.
type SitemapConfigProvider interface {
	SitemapConfig(req *Request, params map[string]any) SitemapConfig
}

// SitemapOption configures a SitemapHandlerTask.
type SitemapOption func(*SitemapHandlerTask)

// SitemapHandlerTask serves sitemap.xml from an enumerable route tree.
type SitemapHandlerTask struct {
	DefaultHTTPHandlerTask
	route           Route
	baseURL         string
	defaults        SitemapConfig
	include         func(RouteEntry) bool
	excludePrefixes []string
}

// NewSitemapHandlerTask returns an HTTP handler task for sitemap.xml.
func NewSitemapHandlerTask(route Route, options ...SitemapOption) *SitemapHandlerTask {
	h := &SitemapHandlerTask{
		route: route,
		excludePrefixes: []string{
			"/static",
		},
	}
	for _, option := range options {
		if option != nil {
			option(h)
		}
	}
	return h
}

// SitemapBaseURL sets the absolute base URL used for relative route paths.
func SitemapBaseURL(baseURL string) SitemapOption {
	return func(h *SitemapHandlerTask) {
		h.baseURL = strings.TrimRight(baseURL, "/")
	}
}

// SitemapDefaults sets default sitemap metadata.
func SitemapDefaults(config SitemapConfig) SitemapOption {
	return func(h *SitemapHandlerTask) {
		h.defaults = config
	}
}

// SitemapInclude sets a final route filter for sitemap entries.
func SitemapInclude(include func(RouteEntry) bool) SitemapOption {
	return func(h *SitemapHandlerTask) {
		h.include = include
	}
}

// SitemapExcludePrefix sets route path prefixes excluded from the sitemap.
func SitemapExcludePrefix(prefixes ...string) SitemapOption {
	return func(h *SitemapHandlerTask) {
		h.excludePrefixes = append([]string{}, prefixes...)
	}
}

// Index writes the sitemap for an exact /sitemap.xml route match.
func (h *SitemapHandlerTask) Index(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) ErrorResponse {
	return h.Get(ctx, req, resp, params)
}

// Get writes the sitemap.
func (h *SitemapHandlerTask) Get(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) ErrorResponse {
	provider, ok := h.route.(RouteEntriesProvider)
	if !ok {
		return NotImplemented
	}

	urls := h.sitemapURLs(req, params, provider.RouteEntries())
	body, err := xml.Marshal(sitemapURLSet{
		XMLName: xml.Name{Space: sitemapNamespace, Local: "urlset"},
		URLs:    urls,
	})
	if err != nil {
		return nil
	}

	resp.SetStatusCode(httpstatus.OK)
	resp.SetHeader(httpheadername.ContentType, "application/xml; charset=utf-8")
	resp.SetBody(buf.NewByteBuf(append([]byte(xml.Header), body...)))
	return nil
}

func (h *SitemapHandlerTask) sitemapURLs(req *Request, params map[string]any, entries []RouteEntry) []sitemapURL {
	baseURL := h.resolveBaseURL(req)
	urls := make([]sitemapURL, 0, len(entries))
	for _, entry := range entries {
		config, ok := h.configForEntry(req, params, entry)
		if !ok {
			continue
		}

		loc := config.Loc
		if loc == "" {
			loc = joinBaseURL(baseURL, entry.Path)
		}
		if loc == "" {
			continue
		}

		urls = append(urls, sitemapURL{
			Loc:        loc,
			LastMod:    formatSitemapLastMod(config.LastMod),
			ChangeFreq: validSitemapChangeFreq(config.ChangeFreq),
			Priority:   formatSitemapPriority(config.Priority),
		})
	}

	slices.SortFunc(urls, func(a sitemapURL, b sitemapURL) int {
		return strings.Compare(a.Loc, b.Loc)
	})
	return urls
}

func (h *SitemapHandlerTask) configForEntry(req *Request, params map[string]any, entry RouteEntry) (SitemapConfig, bool) {
	if entry.Node == nil || entry.Path == "" || entry.Node.RouteType() == RouteTypeGroup {
		return SitemapConfig{}, false
	}
	if entry.Node.RouteType() == RouteTypeRecursiveEndPoint || strings.Contains(entry.Path, "*") {
		return SitemapConfig{}, false
	}
	if h.isExcludedPath(entry.Path) {
		return SitemapConfig{}, false
	}
	if h.include != nil && !h.include(entry) {
		return SitemapConfig{}, false
	}

	task := entry.Node.HandlerTask()
	if task == nil || task == h {
		return SitemapConfig{}, false
	}

	config := h.defaults
	if provider, ok := task.(SitemapConfigProvider); ok {
		config = mergeSitemapConfig(config, provider.SitemapConfig(req, params))
	}
	if config.Loc == "" && hasSitemapPlaceholder(entry.Path) {
		return SitemapConfig{}, false
	}

	include := hasRenderHTMLMethod(task)
	if config.Include != nil {
		include = *config.Include
	}

	if !include {
		return SitemapConfig{}, false
	}
	return config, true
}

func hasSitemapPlaceholder(path string) bool {
	return strings.Contains(path, ":") || strings.Contains(path, "{") || strings.Contains(path, "}")
}

func (h *SitemapHandlerTask) isExcludedPath(path string) bool {
	for _, prefix := range h.excludePrefixes {
		if prefix == "" {
			continue
		}
		if path == prefix || strings.HasPrefix(path, strings.TrimRight(prefix, "/")+"/") {
			return true
		}
	}
	return false
}

func (h *SitemapHandlerTask) resolveBaseURL(req *Request) string {
	if h.baseURL != "" {
		return h.baseURL
	}
	if req == nil || req.Request() == nil {
		return ""
	}

	scheme := req.Header().Get("X-Forwarded-Proto")
	if scheme == "" {
		if req.Request().TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}
	return fmt.Sprintf("%s://%s", scheme, req.Host())
}

func mergeSitemapConfig(base SitemapConfig, override SitemapConfig) SitemapConfig {
	if override.Include != nil {
		base.Include = override.Include
	}
	if override.Loc != "" {
		base.Loc = override.Loc
	}
	if override.LastMod != nil {
		base.LastMod = override.LastMod
	}
	if override.ChangeFreq != "" {
		base.ChangeFreq = override.ChangeFreq
	}
	if override.Priority != nil {
		base.Priority = override.Priority
	}
	return base
}

func hasRenderHTMLMethod(task HandlerTask) bool {
	value := reflect.ValueOf(task)
	if !value.IsValid() {
		return false
	}
	return value.MethodByName("RenderHTML").IsValid() || value.MethodByName("RenderHtml").IsValid()
}

func joinBaseURL(baseURL string, path string) string {
	if path == "" {
		return ""
	}
	if parsed, err := url.Parse(path); err == nil && parsed.IsAbs() {
		return path
	}
	if baseURL == "" {
		return path
	}
	if path == "/" {
		return baseURL + "/"
	}
	return strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(path, "/")
}

func formatSitemapLastMod(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format("2006-01-02")
}

func validSitemapChangeFreq(changeFreq string) string {
	switch changeFreq {
	case "always", "hourly", "daily", "weekly", "monthly", "yearly", "never":
		return changeFreq
	default:
		return ""
	}
}

func formatSitemapPriority(priority *float64) string {
	if priority == nil || *priority < 0 || *priority > 1 {
		return ""
	}
	return strconv.FormatFloat(*priority, 'f', -1, 64)
}

type sitemapURLSet struct {
	XMLName xml.Name     `xml:"http://www.sitemaps.org/schemas/sitemap/0.9 urlset"`
	URLs    []sitemapURL `xml:"url"`
}

type sitemapURL struct {
	Loc        string `xml:"loc"`
	LastMod    string `xml:"lastmod,omitempty"`
	ChangeFreq string `xml:"changefreq,omitempty"`
	Priority   string `xml:"priority,omitempty"`
}
