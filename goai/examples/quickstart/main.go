// Quickstart example: build a minimal route tree, walk it, and print the
// generated OpenAPI 3.0.3 YAML to stdout.
//
//	go run ./goai/examples/quickstart
//
// The example demonstrates the smallest possible goai pipeline:
// route → Walk → Build → EmitYAML. No registry entries, no struct tags,
// no merge — just the auto-discovered shape of two endpoints.
package main

import (
	"fmt"
	"os"

	"github.com/yetiz-org/gone/channel"
	"github.com/yetiz-org/gone/ghttp"
	"github.com/yetiz-org/gone/goai"
)

// HelloHandler is a trivial GET endpoint that returns "Hello, world".
type HelloHandler struct {
	ghttp.DefaultHTTPHandlerTask
}

func (h *HelloHandler) Get(ctx channel.HandlerContext, req *ghttp.Request, resp *ghttp.Response, params map[string]any) ghttp.ErrorResponse {
	resp.JsonResponse(map[string]string{"message": "Hello, world"})

	return nil
}

// PingHandler answers POST /ping with a static body.
type PingHandler struct {
	ghttp.DefaultHTTPHandlerTask
}

func (h *PingHandler) Post(ctx channel.HandlerContext, req *ghttp.Request, resp *ghttp.Response, params map[string]any) ghttp.ErrorResponse {
	resp.JsonResponse(map[string]string{"message": "pong"})

	return nil
}

func main() {
	prev := ghttp.SetSkipHandlerRegister(true)
	defer ghttp.SetSkipHandlerRegister(prev)

	route := ghttp.NewSimpleRoute()
	route.SetEndpoint("/hello", &HelloHandler{})
	route.SetEndpoint("/ping", &PingHandler{})

	candidates := goai.Walk(route)
	doc := goai.Build(candidates, nil, goai.BuildOptions{
		Title:   "Quickstart API",
		Version: "1.0.0",
	})

	body, err := goai.EmitYAML(doc)
	if err != nil {
		fmt.Fprintln(os.Stderr, "emit failed:", err)
		os.Exit(1)
	}

	if _, err := os.Stdout.Write(body); err != nil {
		fmt.Fprintln(os.Stderr, "write failed:", err)
		os.Exit(1)
	}
}
