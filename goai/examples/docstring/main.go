// Docstring example: generate OpenAPI metadata from namespaced handler
// doc-comment directives instead of goai.Register calls.
//
//	go run ./goai/examples/docstring
//
// What this example shows:
//   - Handler struct comments can provide shared operation metadata.
//   - Handler method comments provide endpoint-specific metadata.
//   - Only `@goai.*` lines flow into OpenAPI; ordinary Go comments stay
//     private to source readers.
//   - `schemaType=SomeStruct` and `schemaType=packageAlias.SomeStruct`
//     resolve Go structs into components.schemas and use $ref from operations.
package main

import (
	"fmt"
	"os"

	"github.com/yetiz-org/gone/channel"
	"github.com/yetiz-org/gone/ghttp"
	"github.com/yetiz-org/gone/goai"
)

// BookSummary is the compact representation used in collection responses.
type BookSummary struct {
	ID     string `json:"id" goai:"description=Stable book identifier;example=book_123"`
	Title  string `json:"title" goai:"description=Display title;example=Practical Go APIs"`
	Author string `json:"author" goai:"description=Primary author name;example=Ada Lovelace"`
}

// BookListResponse is the response body for GET /v1/books.
//
// @goai.schemaName BookPage
type BookListResponse struct {
	Books  []BookSummary `json:"books"`
	Count  int           `json:"count" goai:"description=Number of books in this page;minimum=0"`
	Offset int           `json:"offset" goai:"description=Zero-based result offset;minimum=0"`
	Limit  int           `json:"limit" goai:"description=Maximum number of returned books;minimum=1;maximum=100"`
}

// BookDetailResponse is the response body for GET /v1/book/{book_id}.
type BookDetailResponse struct {
	ID          string   `json:"id" goai:"description=Stable book identifier;example=book_123"`
	Title       string   `json:"title" goai:"description=Display title;example=Practical Go APIs"`
	Author      string   `json:"author" goai:"description=Primary author name;example=Ada Lovelace"`
	Description string   `json:"description,omitempty" goai:"description=Public catalog description"`
	Tags        []string `json:"tags,omitempty" goai:"description=Catalog tags;uniqueItems=true"`
}

// CreateBookRequest is the request body for POST /v1/books.
type CreateBookRequest struct {
	Title       string   `json:"title" goai:"description=Display title;minLength=1;maxLength=200;example=Practical Go APIs"`
	Author      string   `json:"author" goai:"description=Primary author name;minLength=1;example=Ada Lovelace"`
	Description string   `json:"description,omitempty" goai:"description=Public catalog description;maxLength=2000"`
	Tags        []string `json:"tags,omitempty" goai:"description=Catalog tags;uniqueItems=true"`
	CallbackURL string   `json:"callback_url,omitempty" goai:"description=Webhook URL called after creation;format=uri"`
}

// CreateBookResponse is the response body for POST /v1/books.
type CreateBookResponse struct {
	ID string `json:"id" goai:"description=Created book identifier;example=book_123"`
}

// ErrorResponse is a simple JSON error envelope.
type ErrorResponse struct {
	Code    string `json:"code" goai:"description=Stable application error code;example=not_found"`
	Message string `json:"message" goai:"description=Human-readable error message;example=Book not found."`
}

// BooksHandler keeps shared OpenAPI metadata for book collection operations.
//
// @goai.tag Books
// @goai.server https://api.example.com "Production API"
// @goai.externalDocs https://docs.example.com/books "Book API guide"
type BooksHandler struct {
	ghttp.DefaultHTTPHandlerTask
}

// Index lists books visible to the current caller.
// Cache and storage notes here remain source-only because they are not
// prefixed with @goai.
//
// @goai.endpoint GET /v1/books
// @goai.summary List books
// @goai.description Returns a filtered page of books.
// @goai.description Results are sorted by creation time descending.
// @goai.operationId books.list
// @goai.param query q string optional "Search query." example=go style=form explode=true
// @goai.param query limit integer optional "Page size." format=int32 default=25
// @goai.response 200 "Book page." schemaType=BookListResponse
// @goai.header 200 X-Request-ID string "Request trace ID." required
// @goai.example response 200 application/json success example={"summary":"One result","value":{"books":[{"id":"book_123","title":"Practical Go APIs","author":"Ada Lovelace"}],"count":1,"offset":0,"limit":25}}
// @goai.security OAuth2 books:read
func (h *BooksHandler) Index(ctx channel.HandlerContext, req *ghttp.Request, resp *ghttp.Response, params map[string]any) ghttp.ErrorResponse {
	resp.JsonResponse(BookListResponse{
		Books: []BookSummary{
			{ID: "book_123", Title: "Practical Go APIs", Author: "Ada Lovelace"},
		},
		Count:  1,
		Offset: 0,
		Limit:  25,
	})

	return nil
}

// Create adds a book to the catalog.
//
// @goai.endpoint POST /v1/books
// @goai.summary Create a book
// @goai.description Creates a catalog book record.
// @goai.requestBody required application/json object "Book payload." schemaType=CreateBookRequest
// @goai.requestBody.description The request body is validated before the record is created.
// @goai.example request application/json default "Create request" {"title":"Practical Go APIs","author":"Ada Lovelace","tags":["go","api"],"callback_url":"https://client.example.com/hooks/books"}
// @goai.response 201 "Created book." schemaType=CreateBookResponse
// @goai.response 400 "Invalid book payload." mediaType=application/json schemaType=ErrorResponse
// @goai.example response 201 application/json created example={"summary":"Created","value":{"id":"book_123"}}
// @goai.link 201 getBook books.get "Fetch created book." parameters={"book_id":"$response.body#/id"}
// @goai.callback onBookCreated {"{$request.body#/callback_url}":{"post":{"requestBody":{"content":{"application/json":{"schema":{"type":"object","properties":{"id":{"type":"string"}}}}}},"responses":{"200":{"description":"Callback accepted."}}}}}
// @goai.security OAuth2 books:write
// @goai.extension x-codegen-group "catalog"
func (h *BooksHandler) Create(ctx channel.HandlerContext, req *ghttp.Request, resp *ghttp.Response, params map[string]any) ghttp.ErrorResponse {
	resp.JsonResponse(CreateBookResponse{ID: "book_123"})

	return nil
}

// BookHandler keeps shared OpenAPI metadata for single-book operations.
//
// @goai.tag Books
// @goai.server https://api.example.com "Production API"
// @goai.externalDocs https://docs.example.com/books "Book API guide"
type BookHandler struct {
	ghttp.DefaultHTTPHandlerTask
}

// Get returns one book by ID.
//
// @goai.endpoint GET /v1/book/{book_id}
// @goai.summary Get a book
// @goai.description Returns the catalog record for one book.
// @goai.operationId books.get
// @goai.param path book_id string required "Book identifier."
// @goai.response 200 "Book detail." schemaType=BookDetailResponse
// @goai.response 404 "Book not found." mediaType=application/json schemaType=ErrorResponse
// @goai.example response 404 application/json notFound example={"summary":"Missing book","value":{"code":"not_found","message":"Book not found."}}
// @goai.security OAuth2 books:read
func (h *BookHandler) Get(ctx channel.HandlerContext, req *ghttp.Request, resp *ghttp.Response, params map[string]any) ghttp.ErrorResponse {
	resp.JsonResponse(BookDetailResponse{
		ID:          "book_123",
		Title:       "Practical Go APIs",
		Author:      "Ada Lovelace",
		Description: "A concise guide to API design in Go.",
		Tags:        []string{"go", "api"},
	})

	return nil
}

func main() {
	prev := ghttp.SetSkipHandlerRegister(true)
	defer ghttp.SetSkipHandlerRegister(prev)

	route := ghttp.NewSimpleRoute()
	route.SetEndpoint("/v1/books", &BooksHandler{})
	route.SetEndpoint("/v1/book/{book_id}", &BookHandler{})

	candidates := goai.Walk(route)
	doc := goai.Build(candidates, nil, goai.BuildOptions{
		Title:                 "Docstring Example API",
		Description:           "Demonstrates @goai.* doc-comment extraction.",
		Version:               "1.0.0",
		OperationDocExtractor: goai.DefaultOperationDocExtractor(),
		Tags: []goai.Tag{
			{Name: "Books", Description: "Book catalog operations"},
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
					"books:read":  "Read book catalog records",
					"books:write": "Create book catalog records",
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
