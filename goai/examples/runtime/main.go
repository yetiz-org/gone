// Runtime example: serve the generated OpenAPI YAML at /openapi.yaml from
// inside a running gone HTTP server, instead of generating it ahead of time
// from a CLI.
//
//	go run ./goai/examples/runtime
//	curl http://localhost:8080/openapi.yaml
//
// The runtime handler caches the emitted bytes for `cacheTTL`. Set TTL > 0
// in production to keep the spec request cheap; leave it at zero in local
// development so route changes show up on every refresh.
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/yetiz-org/gone/channel"
	"github.com/yetiz-org/gone/ghttp"
	"github.com/yetiz-org/gone/goai"
)

// HelloHandler answers GET /hello.
type HelloHandler struct {
	ghttp.DefaultHTTPHandlerTask
}

func (h *HelloHandler) Get(ctx channel.HandlerContext, req *ghttp.Request, resp *ghttp.Response, params map[string]any) ghttp.ErrorResponse {
	resp.JsonResponse(map[string]string{"message": "Hello, world"})

	return nil
}

func main() {
	prev := ghttp.SetSkipHandlerRegister(true)
	defer ghttp.SetSkipHandlerRegister(prev)

	route := ghttp.NewSimpleRoute()
	route.SetEndpoint("/hello", &HelloHandler{})

	// Wire goai's runtime handler at /openapi.yaml. WithRuntimeBuildOptions
	// supplies the same metadata as RunOptions; WithRuntimeCacheTTL caps how
	// often the spec is re-walked under load.
	route.SetEndpoint("/openapi.yaml", goai.Handler(route,
		goai.WithRuntimeBuildOptions(goai.BuildOptions{
			Title:   "Runtime Example API",
			Version: "1.0.0",
			Servers: []goai.Server{
				{URL: "http://localhost:8080", Description: "Local"},
			},
			Tags: []goai.Tag{
				{Name: "Public", Description: "Public endpoints"},
			},
		}),
		goai.WithRuntimeCacheTTL(30*time.Second),
	))

	// In a real binary you would now call ghttp.Listen(...) or boot the gone
	// daemon stack. For this example we just demonstrate that the spec
	// handler materialises a valid document.
	candidates := goai.Walk(route)
	fmt.Fprintf(os.Stderr, "discovered %d operations\n", len(candidates))

	doc := goai.Build(candidates, nil, goai.BuildOptions{
		Title:   "Runtime Example API",
		Version: "1.0.0",
	})

	body, err := goai.EmitYAML(doc)
	if err != nil {
		fmt.Fprintln(os.Stderr, "emit failed:", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "spec is %d bytes; mount goai.Handler at /openapi.yaml in your real server.\n",
		len(body))

	if _, err := os.Stdout.Write(body); err != nil {
		fmt.Fprintln(os.Stderr, "write failed:", err)
		os.Exit(1)
	}
}
