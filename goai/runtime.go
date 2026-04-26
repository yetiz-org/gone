package goai

import (
	"sync"
	"time"

	"github.com/yetiz-org/gone/channel"
	"github.com/yetiz-org/gone/erresponse"
	"github.com/yetiz-org/gone/ghttp"
	buf "github.com/yetiz-org/goth-bytebuf"
	kklogger "github.com/yetiz-org/goth-kklogger"
)

// RuntimeOption configures the http handler returned by Handler.
type RuntimeOption func(*runtimeConfig)

type runtimeConfig struct {
	profileName string
	build       BuildOptions
	cacheTTL    time.Duration
}

// WithRuntimeProfile names the profile to emit. Defaults to "all".
func WithRuntimeProfile(name string) RuntimeOption {
	return func(c *runtimeConfig) { c.profileName = name }
}

// WithRuntimeBuildOptions overrides the BuildOptions used at runtime
// (Title, Servers, Tags, etc.).
func WithRuntimeBuildOptions(opts BuildOptions) RuntimeOption {
	return func(c *runtimeConfig) { c.build = opts }
}

// WithRuntimeCacheTTL sets how long the emitted yaml is cached before
// re-walking the route tree. The default is 0, which disables caching
// (every request rebuilds — fine for local dev, tune for production).
func WithRuntimeCacheTTL(d time.Duration) RuntimeOption {
	return func(c *runtimeConfig) { c.cacheTTL = d }
}

// Handler returns a ghttp.HandlerTask that responds to GET with an
// OpenAPI YAML document derived from the supplied route tree. POST/PATCH/
// other verbs return NotImplemented.
//
// The handler caches the emitted bytes when WithRuntimeCacheTTL is > 0.
// Caching is invalidated lazily on TTL expiry.
func Handler(route ghttp.RouteEntriesProvider, opts ...RuntimeOption) ghttp.HandlerTask {
	cfg := &runtimeConfig{profileName: "all"}
	for _, opt := range opts {
		opt(cfg)
	}

	h := &runtimeHandler{
		route: route,
		cfg:   cfg,
	}

	return h
}

type runtimeHandler struct {
	ghttp.DefaultHTTPHandlerTask
	route ghttp.RouteEntriesProvider
	cfg   *runtimeConfig

	mu       sync.Mutex
	cached   []byte
	cachedAt time.Time
}

// Get serves the generated yaml document.
func (h *runtimeHandler) Get(ctx channel.HandlerContext, req *ghttp.Request, resp *ghttp.Response, params map[string]any) ghttp.ErrorResponse {
	body, err := h.materialise()
	if err != nil {
		kklogger.ErrorJ("goai:runtimeHandler.Get#emit!fail", err.Error())

		return erresponse.ServerError
	}

	resp.SetHeader("Content-Type", "application/yaml; charset=utf-8")
	resp.SetBody(buf.NewByteBuf(body))

	return nil
}

// materialise returns the cached yaml bytes when fresh, otherwise rebuilds.
func (h *runtimeHandler) materialise() ([]byte, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.cfg.cacheTTL > 0 && len(h.cached) > 0 && time.Since(h.cachedAt) < h.cfg.cacheTTL {
		return h.cached, nil
	}

	candidates := Walk(h.route)
	cfg := DefaultConfig()
	profile := cfg.BuildProfile(h.cfg.profileName)

	doc := Build(candidates, profile, h.cfg.build)
	bytes, err := EmitYAML(doc)
	if err != nil {
		return nil, err
	}

	h.cached = bytes
	h.cachedAt = time.Now()

	return bytes, nil
}
