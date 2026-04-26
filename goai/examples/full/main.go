// Full example: drive RunCLI with the complete RunOptions surface so the
// generated YAML exercises every Document-level OpenAPI 3.0.3 feature
// goai supports — License, Contact, TermsOfService, ExternalDocs,
// multiple Servers with ServerVariables, multiple Tags, GlobalSecurity,
// SecuritySchemes, and a custom DefaultOutput path.
//
//	go run ./goai/examples/full -o /tmp/full.yaml
//	go run ./goai/examples/full -o -        # write to stdout
//
// Use this file as a copy-paste template when bootstrapping a new project's
// `cmd/goaispec` (or equivalent) binary.
package main

import (
	"github.com/yetiz-org/gone/channel"
	"github.com/yetiz-org/gone/ghttp"
	"github.com/yetiz-org/gone/goai"
)

// RootHandler answers GET / with a status object.
type RootHandler struct {
	ghttp.DefaultHTTPHandlerTask
}

func (h *RootHandler) Get(ctx channel.HandlerContext, req *ghttp.Request, resp *ghttp.Response, params map[string]any) ghttp.ErrorResponse {
	resp.JsonResponse(map[string]string{"status": "ok"})

	return nil
}

// HealthHandler answers GET /health.
type HealthHandler struct {
	ghttp.DefaultHTTPHandlerTask
}

func (h *HealthHandler) Get(ctx channel.HandlerContext, req *ghttp.Request, resp *ghttp.Response, params map[string]any) ghttp.ErrorResponse {
	resp.JsonResponse(map[string]string{"status": "healthy"})

	return nil
}

// MeHandler answers GET /v1/me. It is documented as requiring OAuth2.
type MeHandler struct {
	ghttp.DefaultHTTPHandlerTask
}

// GOAIGetSpec surfaces this handler's Get-method spec at walk time without
// requiring an explicit goai.Register call. The method name mirrors the
// handler's Get() method on the struct.
func (h *MeHandler) GOAIGetSpec() goai.Spec {
	return goai.NewSpec(
		goai.WithSummary("Get the current user"),
		goai.WithDescription("Returns the profile of the user authenticated by the bearer token."),
		goai.WithTag("User"),
		goai.WithOperationID("users.me"),
		goai.WithSecurity("OAuth2", "openid", "profile"),
	)
}

func (h *MeHandler) Get(ctx channel.HandlerContext, req *ghttp.Request, resp *ghttp.Response, params map[string]any) ghttp.ErrorResponse {
	resp.JsonResponse(map[string]string{"id": "user_demo", "email": "demo@example.com"})

	return nil
}

func main() {
	goai.RunCLI(
		func() ghttp.RouteEntriesProvider {
			route := ghttp.NewSimpleRoute()
			route.SetEndpoint("/", &RootHandler{})
			route.SetEndpoint("/health", &HealthHandler{})
			route.SetEndpoint("/v1/me", &MeHandler{})

			return route
		},
		goai.RunOptions{
			Title:          "Full Example API",
			Description:    "An exhaustive RunOptions configuration showing every Document-level field goai supports.",
			Version:        "2.0.0",
			TermsOfService: "https://example.com/terms",
			Contact: &goai.Contact{
				Name:  "API Support",
				URL:   "https://example.com/support",
				Email: "api@example.com",
			},
			License: &goai.License{
				Name: "Apache 2.0",
				URL:  "https://www.apache.org/licenses/LICENSE-2.0",
			},
			ExternalDocs: &goai.ExternalDocumentation{
				Description: "Find more info here",
				URL:         "https://example.com/docs",
			},
			Servers: []goai.Server{
				{
					URL:         "https://{environment}.api.example.com/{basePath}",
					Description: "Templated production server",
					Variables: map[string]*goai.ServerVariable{
						"environment": {
							Default:     "prod",
							Enum:        []string{"prod", "staging"},
							Description: "Deployment environment",
						},
						"basePath": {
							Default:     "v1",
							Description: "Base API path",
						},
					},
				},
				{URL: "http://localhost:8080", Description: "Local development"},
			},
			Tags: []goai.Tag{
				{Name: "Public", Description: "Endpoints requiring no authentication"},
				{Name: "User", Description: "Authenticated user endpoints"},
			},
			GlobalSecurity: []map[string][]string{
				{"OAuth2": {}},
			},
			SecuritySchemes: map[string]*goai.SecurityScheme{
				"OAuth2": {
					Type:        "oauth2",
					Description: "OAuth 2.0 Authorization Code Flow with PKCE",
					Flows: &goai.OAuthFlows{
						AuthorizationCode: &goai.OAuthFlow{
							AuthorizationURL: "https://api.example.com/authorize",
							TokenURL:         "https://api.example.com/oauth/token",
							RefreshURL:       "https://api.example.com/oauth/token",
							Scopes: map[string]string{
								"openid":  "OpenID Connect identity claim",
								"profile": "Read user profile",
								"email":   "Read user email",
							},
						},
					},
				},
				"ApiKey": {
					Type:        "apiKey",
					In:          "header",
					Name:        "X-API-Key",
					Description: "Server-to-server API key",
				},
			},
			DefaultOutput: "openapi.full.yaml",
		},
	)
}
