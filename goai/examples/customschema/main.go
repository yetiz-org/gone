// Customschema example: register Go types with goai.Register so that the
// generated spec carries fully fleshed-out request and response schemas,
// then drive validation constraints via the `goai:"..."` struct tag.
//
//	go run ./goai/examples/customschema
//
// What this example shows:
//   - goai.Register links a (handler, method) pair to Go types that
//     describe the request / response payloads.
//   - The `goai:"..."` struct tag adds OpenAPI metadata (description,
//     example, format, validation constraints) to fields without forking
//     the Go type definition.
//   - Functional options on goai.Register (WithSummary, WithTag,
//     WithExample, WithExternalDocs, ...) shape the resulting Operation.
package main

import (
	"fmt"
	"os"

	"github.com/yetiz-org/gone/channel"
	"github.com/yetiz-org/gone/ghttp"
	"github.com/yetiz-org/gone/goai"
)

// CreateAlbumRequest is the request payload for POST /albums.
type CreateAlbumRequest struct {
	Title       string   `json:"title" goai:"description=The album title;example=Greatest Hits;minLength=1;maxLength=200"`
	Artist      string   `json:"artist" goai:"description=Primary credited artist;example=The Beatles"`
	ReleaseYear int      `json:"release_year" goai:"description=Year of release;minimum=1900;maximum=2100;example=2026"`
	Genres      []string `json:"genres,omitempty" goai:"description=Free-form genre tags;minItems=1;uniqueItems=true"`
	Explicit    bool     `json:"explicit" goai:"description=Whether the album carries an explicit content advisory"`
}

// CreateAlbumResponse is the response payload for POST /albums.
type CreateAlbumResponse struct {
	ID        string `json:"id" goai:"description=Server-assigned album identifier;example=alb_01J0..."`
	Title     string `json:"title"`
	Artist    string `json:"artist"`
	CreatedAt string `json:"created_at" goai:"format=date-time;example=2026-04-26T12:00:00Z"`
}

// AlbumHandler implements POST /albums.
type AlbumHandler struct {
	ghttp.DefaultHTTPHandlerTask
}

func (h *AlbumHandler) Post(ctx channel.HandlerContext, req *ghttp.Request, resp *ghttp.Response, params map[string]any) ghttp.ErrorResponse {
	resp.JsonResponse(CreateAlbumResponse{
		ID:        "alb_demo",
		Title:     "Demo",
		Artist:    "Anonymous",
		CreatedAt: "2026-04-26T12:00:00Z",
	})

	return nil
}

func main() {
	prev := ghttp.SetSkipHandlerRegister(true)
	defer ghttp.SetSkipHandlerRegister(prev)

	albumHandler := &AlbumHandler{}

	// Register links the handler-method pair to typed request/response.
	// Use typed-nil pointers so reflect can extract the element type even
	// when callers do not have a concrete value handy.
	goai.Register(albumHandler, "Post",
		(*CreateAlbumRequest)(nil),
		(*CreateAlbumResponse)(nil),
		goai.WithSummary("Create a new album"),
		goai.WithDescription("Creates a new album record. The server assigns an immutable identifier."),
		goai.WithTag("Album"),
		goai.WithOperationID("albums.create"),
		goai.WithExample("application/json", map[string]any{
			"title":        "Greatest Hits",
			"artist":       "The Beatles",
			"release_year": 2026,
			"genres":       []string{"rock", "pop"},
			"explicit":     false,
		}),
		goai.WithExternalDocs(
			"https://example.com/docs/albums",
			"Detailed album lifecycle documentation",
		),
		goai.WithSecurity("OAuth2", "albums:write"),
	)

	route := ghttp.NewSimpleRoute()
	route.SetEndpoint("/albums", albumHandler)

	candidates := goai.Walk(route)
	doc := goai.Build(candidates, nil, goai.BuildOptions{
		Title:       "Album Service API",
		Description: "Demonstrates goai.Register + struct tags.",
		Version:     "1.0.0",
		Servers: []goai.Server{
			{URL: "https://api.example.com", Description: "Production"},
		},
		Tags: []goai.Tag{
			{Name: "Album", Description: "Album catalog operations"},
		},
	})

	doc.Components.SecuritySchemes["OAuth2"] = &goai.SecurityScheme{
		Type:        "oauth2",
		Description: "OAuth 2.0 Authorization Code with PKCE",
		Flows: &goai.OAuthFlows{
			AuthorizationCode: &goai.OAuthFlow{
				AuthorizationURL: "https://api.example.com/authorize",
				TokenURL:         "https://api.example.com/oauth/token",
				Scopes: map[string]string{
					"albums:read":  "Read album metadata",
					"albums:write": "Create and modify albums",
				},
			},
		},
	}

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
