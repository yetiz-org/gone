package ghttp

import (
	"encoding/xml"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"
)

type sitemapPlainTask struct {
	DefaultHTTPHandlerTask
}

type sitemapHTMLTask struct {
	DefaultHTTPHandlerTask
}

func (h *sitemapHTMLTask) RenderHtml(templateName string, config any, resp *Response) {}

type sitemapOverrideTask struct {
	DefaultHTTPHandlerTask
	config SitemapConfig
}

func (h *sitemapOverrideTask) RenderHtml(templateName string, config any, resp *Response) {}

func (h *sitemapOverrideTask) SitemapConfig(req *Request, params map[string]any) SitemapConfig {
	return h.config
}

type sitemapForcedTask struct {
	DefaultHTTPHandlerTask
	config SitemapConfig
}

func (h *sitemapForcedTask) SitemapConfig(req *Request, params map[string]any) SitemapConfig {
	return h.config
}

type parsedURLSet struct {
	XMLName xml.Name        `xml:"http://www.sitemaps.org/schemas/sitemap/0.9 urlset"`
	URLs    []parsedSitemap `xml:"url"`
}

type parsedSitemap struct {
	Loc        string `xml:"loc"`
	LastMod    string `xml:"lastmod"`
	ChangeFreq string `xml:"changefreq"`
	Priority   string `xml:"priority"`
}

func boolPtr(v bool) *bool {
	return &v
}

func floatPtr(v float64) *float64 {
	return &v
}

func TestSimpleRouteRouteEntriesListsEndpointPaths(t *testing.T) {
	route := NewSimpleRoute()
	route.SetRoot(&sitemapHTMLTask{})
	route.SetGroup("/api")
	route.SetEndpoint("/about", &sitemapHTMLTask{})
	route.SetEndpoint("/api/v1/health", &sitemapPlainTask{})
	route.SetEndpoint("/static/*", &sitemapHTMLTask{})
	route.SetEndpoint("/articles/{slug}", &sitemapHTMLTask{})

	entries := route.RouteEntries()
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		paths = append(paths, entry.Path)
	}

	slices.Sort(paths)
	expected := []string{"/", "/about", "/api/v1/health", "/articles/{slug}", "/static/*"}
	if !slices.Equal(paths, expected) {
		t.Fatalf("unexpected route entries: got %v want %v", paths, expected)
	}
}

func TestDefaultRouteRouteEntriesListsEndpointPaths(t *testing.T) {
	route := NewRoute()
	route.
		SetRoot(NewEndPoint("", &sitemapHTMLTask{}, nil)).
		AddEndPoint(NewEndPoint("about", &sitemapHTMLTask{}, nil)).
		AddRecursivePoint(NewEndPoint("static", &sitemapHTMLTask{}, nil)).
		AddGroup(NewGroup("v1", nil).
			AddEndPoint(NewEndPoint("home", &sitemapHTMLTask{}, nil)),
		)

	entries := route.RouteEntries()
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		paths = append(paths, entry.Path)
	}

	slices.Sort(paths)
	expected := []string{"/", "/about", "/static/*", "/v1/home"}
	if !slices.Equal(paths, expected) {
		t.Fatalf("unexpected route entries: got %v want %v", paths, expected)
	}
}

func TestSitemapHandlerIncludesHTMLHandlersWithDefaultsAndOverrides(t *testing.T) {
	lastMod := time.Date(2026, 4, 24, 12, 30, 0, 0, time.UTC)
	route := NewSimpleRoute()
	route.SetEndpoint("/api/v1/health", &sitemapPlainTask{})
	route.SetEndpoint("/login", &sitemapHTMLTask{})
	route.SetEndpoint("/custom", &sitemapOverrideTask{config: SitemapConfig{
		Loc:        "https://canonical.example/custom",
		LastMod:    &lastMod,
		ChangeFreq: "daily",
		Priority:   floatPtr(0.9),
	}})
	route.SetEndpoint("/hidden", &sitemapOverrideTask{config: SitemapConfig{
		Include: boolPtr(false),
	}})
	route.SetEndpoint("/forced-api", &sitemapForcedTask{config: SitemapConfig{
		Include:  boolPtr(true),
		Priority: floatPtr(0.3),
	}})
	route.SetEndpoint("/articles/{slug}", &sitemapHTMLTask{})
	route.SetEndpoint("/posts/{slug}", &sitemapOverrideTask{config: SitemapConfig{
		Loc:     "https://example.com/blog/hello",
		Include: boolPtr(true),
	}})
	route.SetEndpoint("/static/*", &sitemapHTMLTask{})

	handler := NewSitemapHandlerTask(route,
		SitemapBaseURL("https://example.com/base/"),
		SitemapDefaults(SitemapConfig{
			ChangeFreq: "weekly",
			Priority:   floatPtr(0.5),
		}),
	)

	req := &Request{request: httptest.NewRequest("GET", "https://ignored.test/sitemap.xml", nil)}
	resp := EmptyResponse()

	if err := handler.Get(nil, req, resp, map[string]any{}); err != nil {
		t.Fatalf("sitemap handler returned error: %v", err)
	}

	if got := resp.GetHeader("Content-Type"); got != "application/xml; charset=utf-8" {
		t.Fatalf("unexpected content type: %s", got)
	}

	var parsed parsedURLSet
	if err := xml.Unmarshal(resp.Body().Bytes(), &parsed); err != nil {
		t.Fatalf("invalid sitemap xml: %v\n%s", err, string(resp.Body().Bytes()))
	}
	if parsed.XMLName.Space != sitemapNamespace {
		t.Fatalf("unexpected sitemap namespace: %s", parsed.XMLName.Space)
	}

	got := map[string]parsedSitemap{}
	for _, entry := range parsed.URLs {
		got[entry.Loc] = entry
	}

	if _, ok := got["https://example.com/base/login"]; !ok {
		t.Fatalf("expected html handler URL in sitemap, got %#v", got)
	}
	if _, ok := got["https://example.com/base/api/v1/health"]; ok {
		t.Fatalf("plain handler must not be included by default")
	}
	if _, ok := got["https://example.com/base/hidden"]; ok {
		t.Fatalf("handler override Include=false must exclude URL")
	}
	if _, ok := got["https://example.com/base/static/*"]; ok {
		t.Fatalf("wildcard route must be excluded by default")
	}
	if _, ok := got["https://example.com/base/articles/{slug}"]; ok {
		t.Fatalf("dynamic placeholder route must be excluded without concrete loc")
	}
	if _, ok := got["https://example.com/base/articles"]; ok {
		t.Fatalf("dynamic placeholder route must not be collapsed into a concrete-looking URL")
	}
	if _, ok := got["https://example.com/blog/hello"]; !ok {
		t.Fatalf("dynamic placeholder route with loc override should be included")
	}

	login := got["https://example.com/base/login"]
	if login.ChangeFreq != "weekly" || login.Priority != "0.5" {
		t.Fatalf("missing default metadata: %#v", login)
	}

	custom := got["https://canonical.example/custom"]
	if custom.LastMod != "2026-04-24" || custom.ChangeFreq != "daily" || custom.Priority != "0.9" {
		t.Fatalf("missing override metadata: %#v", custom)
	}

	forced := got["https://example.com/base/forced-api"]
	if forced.Priority != "0.3" || forced.ChangeFreq != "weekly" {
		t.Fatalf("forced non-html handler should merge defaults and overrides: %#v", forced)
	}
}

func TestSitemapHandlerOmitsInvalidOptionalMetadata(t *testing.T) {
	route := NewSimpleRoute()
	route.SetEndpoint("/invalid", &sitemapOverrideTask{config: SitemapConfig{
		ChangeFreq: "sometimes",
		Priority:   floatPtr(1.5),
	}})

	handler := NewSitemapHandlerTask(route, SitemapBaseURL("https://example.com"))
	req := &Request{request: httptest.NewRequest("GET", "https://example.test/sitemap.xml", nil)}
	resp := EmptyResponse()

	if err := handler.Get(nil, req, resp, map[string]any{}); err != nil {
		t.Fatalf("sitemap handler returned error: %v", err)
	}

	var parsed parsedURLSet
	if err := xml.Unmarshal(resp.Body().Bytes(), &parsed); err != nil {
		t.Fatalf("invalid sitemap xml: %v", err)
	}
	if len(parsed.URLs) != 1 {
		t.Fatalf("expected one URL, got %d", len(parsed.URLs))
	}
	if parsed.URLs[0].ChangeFreq != "" || parsed.URLs[0].Priority != "" {
		t.Fatalf("invalid metadata should be omitted: %#v", parsed.URLs[0])
	}
}

func TestSitemapHandlerSkipsItselfAndSupportsCustomPrefixFilter(t *testing.T) {
	route := NewSimpleRoute()
	route.SetEndpoint("/public", &sitemapHTMLTask{})
	route.SetEndpoint("/private/page", &sitemapHTMLTask{})

	handler := NewSitemapHandlerTask(route,
		SitemapBaseURL("https://example.com"),
		SitemapExcludePrefix("/private"),
	)
	route.SetEndpoint("/sitemap.xml", handler)

	req := &Request{request: httptest.NewRequest("GET", "https://example.test/sitemap.xml", nil)}
	resp := EmptyResponse()

	if err := handler.Get(nil, req, resp, map[string]any{}); err != nil {
		t.Fatalf("sitemap handler returned error: %v", err)
	}

	body := string(resp.Body().Bytes())
	if !strings.Contains(body, "https://example.com/public") {
		t.Fatalf("expected public URL in sitemap: %s", body)
	}
	if strings.Contains(body, "private") || strings.Contains(body, "sitemap.xml") {
		t.Fatalf("sitemap should skip excluded prefix and itself: %s", body)
	}
}

func TestSitemapHandlerOnlyExcludesStaticByDefault(t *testing.T) {
	route := NewSimpleRoute()
	route.SetEndpoint("/api/page", &sitemapHTMLTask{})
	route.SetEndpoint("/static/app", &sitemapHTMLTask{})

	handler := NewSitemapHandlerTask(route, SitemapBaseURL("https://example.com"))
	req := &Request{request: httptest.NewRequest("GET", "https://example.test/sitemap.xml", nil)}
	resp := EmptyResponse()

	if err := handler.Get(nil, req, resp, map[string]any{}); err != nil {
		t.Fatalf("sitemap handler returned error: %v", err)
	}

	body := string(resp.Body().Bytes())
	if !strings.Contains(body, "https://example.com/api/page") {
		t.Fatalf("non-static application paths must not be excluded by default: %s", body)
	}
	if strings.Contains(body, "https://example.com/static/app") {
		t.Fatalf("static paths should stay excluded by default: %s", body)
	}
}

func TestSitemapHandlerBuildsBaseURLFromRequest(t *testing.T) {
	route := NewSimpleRoute()
	route.SetEndpoint("/login", &sitemapHTMLTask{})

	handler := NewSitemapHandlerTask(route)
	req := &Request{request: httptest.NewRequest("GET", "http://internal.test/sitemap.xml", nil)}
	req.Header().Set("X-Forwarded-Proto", "https")
	req.request.Host = "public.example"
	resp := EmptyResponse()

	if err := handler.Get(nil, req, resp, map[string]any{}); err != nil {
		t.Fatalf("sitemap handler returned error: %v", err)
	}

	if body := string(resp.Body().Bytes()); !containsAll(body, "https://public.example/login") {
		t.Fatalf("sitemap did not use request base URL: %s", body)
	}
}

func TestSitemapHandlerRequiresEnumerableRoute(t *testing.T) {
	handler := NewSitemapHandlerTask(&stubNonEnumerableRoute{})
	req := &Request{request: httptest.NewRequest("GET", "http://example.test/sitemap.xml", nil)}
	resp := EmptyResponse()

	if err := handler.Get(nil, req, resp, map[string]any{}); err != NotImplemented {
		t.Fatalf("expected NotImplemented for non-enumerable route, got %v", err)
	}
}

type stubNonEnumerableRoute struct{}

func (r *stubNonEnumerableRoute) RouteNode(path string) (RouteNode, map[string]any, bool) {
	return nil, nil, false
}

func containsAll(text string, values ...string) bool {
	for _, value := range values {
		if !strings.Contains(text, value) {
			return false
		}
	}
	return true
}
