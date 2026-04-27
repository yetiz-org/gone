package goai

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yetiz-org/gone/channel"
	"github.com/yetiz-org/gone/ghttp"
	firstmodels "github.com/yetiz-org/gone/goai/internal/testfixtures/collision/first/models"
	secondmodels "github.com/yetiz-org/gone/goai/internal/testfixtures/collision/second/models"
	aliasschema "github.com/yetiz-org/gone/goai/internal/testfixtures/docaliasschema"
	"github.com/yetiz-org/gone/goai/internal/testfixtures/docschemafixture"
)

type _DocstringTestHandler struct {
	ghttp.DefaultHTTPHandlerTask
}

type _DocMergeRequest struct {
	Name string `json:"name"`
}

type _DocMergeResponse struct {
	ID string `json:"id"`
}

type _ResponseSpecError struct {
	Message string `json:"message"`
}

type _CustomSchemaNameResponse struct {
	ID string `json:"id"`
}

func (_CustomSchemaNameResponse) GOAISchemaName() string {
	return "PublicCustomResponse"
}

func TestImportPackageNamePrefersModuleDirWithoutGoList(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/mod\n\ngo 1.26\n"), 0o644))

	pkgDir := filepath.Join(root, "pkg", "foo")
	require.NoError(t, os.MkdirAll(pkgDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(pkgDir, "foo.go"), []byte("package custompkg\n\ntype T struct{}\n"), 0o644))

	fakeBin := filepath.Join(root, "bin")
	require.NoError(t, os.MkdirAll(fakeBin, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(fakeBin, "go"), []byte("#!/bin/sh\nsleep 2\nexit 1\n"), 0o755))

	t.Setenv("GO111MODULE", "on")
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	done := make(chan string, 1)
	go func() {
		done <- _ImportPackageName("example.com/mod/pkg/foo", root, nil, &sync.Map{})
	}()

	select {
	case got := <-done:
		assert.Equal(t, "custompkg", got)
	case <-time.After(200 * time.Millisecond):
		t.Fatal("_ImportPackageName invoked go list before checking the local module directory")
	}
}

func TestImportPackageNameUsesBaseForExternalImportWithoutGoList(t *testing.T) {
	root := t.TempDir()
	fakeBin := filepath.Join(root, "bin")
	require.NoError(t, os.MkdirAll(fakeBin, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(fakeBin, "go"), []byte("#!/bin/sh\nsleep 2\nexit 1\n"), 0o755))

	t.Setenv("GO111MODULE", "on")
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	done := make(chan string, 1)
	go func() {
		done <- _ImportPackageName("github.com/aws/aws-sdk-go-v2/config", root, nil, &sync.Map{})
	}()

	select {
	case got := <-done:
		assert.Equal(t, "config", got)
	case <-time.After(200 * time.Millisecond):
		t.Fatal("_ImportPackageName invoked go list for an external package alias")
	}
}

func TestImportPackageNameCachesParsedPackage(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/mod\n\ngo 1.26\n"), 0o644))

	pkgDir := filepath.Join(root, "pkg", "large")
	require.NoError(t, os.MkdirAll(pkgDir, 0o755))
	for fileIdx := 0; fileIdx < 60; fileIdx++ {
		var src strings.Builder
		src.WriteString("package custompkg\n\n")
		for typeIdx := 0; typeIdx < 40; typeIdx++ {
			src.WriteString("type T")
			src.WriteString(strconv.Itoa(fileIdx))
			src.WriteString("_")
			src.WriteString(strconv.Itoa(typeIdx))
			src.WriteString(" struct {\n")
			for fieldIdx := 0; fieldIdx < 30; fieldIdx++ {
				src.WriteString("F")
				src.WriteString(strconv.Itoa(fieldIdx))
				src.WriteString(" string\n")
			}
			src.WriteString("}\n")
		}

		require.NoError(t, os.WriteFile(filepath.Join(pkgDir, "file"+strconv.Itoa(fileIdx)+".go"), []byte(src.String()), 0o644))
	}

	start := time.Now()
	cache := &sync.Map{}
	for i := 0; i < 20; i++ {
		assert.Equal(t, "custompkg", _ImportPackageName("example.com/mod/pkg/large", root, nil, cache))
	}

	if elapsed := time.Since(start); elapsed > 300*time.Millisecond {
		t.Fatalf("_ImportPackageName reparsed the same package too often: %s", elapsed)
	}
}

// _DocstringSpecHandler keeps common OpenAPI metadata on the handler type.
//
// @goai.tag Organization
// @goai.security OrganizationToken read
type _DocstringSpecHandler struct {
	ghttp.DefaultHTTPHandlerTask
}

// Get returns a handler-level tagged operation.
//
// @goai.endpoint GET /orgs
// @goai.summary List organizations
func (h *_DocstringSpecHandler) Get(ctx channel.HandlerContext, req *ghttp.Request, resp *ghttp.Response, params map[string]any) ghttp.ErrorResponse {
	return nil
}

type _DocstringSchemaResponse struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty" goai:"description=Organization name"`
}

// _DocstringCustomNameResponse has a stable public component name.
//
// @goai.schemaName PublicDocstringResponse
type _DocstringCustomNameResponse struct {
	ID string `json:"id"`
}

type _DocstringPage[T any] struct {
	Data  T   `json:"data"`
	Items []T `json:"items,omitempty"`
}

type _DocstringEnvelope[T any] struct {
	Page _DocstringPage[T] `json:"page"`
}

type _DocstringFirstCollisionUser = firstmodels.User

type _DocstringSecondCollisionUser = secondmodels.User

type _DocstringAliasPackageResponse = aliasschema.SharedResponse

type _DocstringAliasResponse = docschemafixture.SharedResponse

type _DocstringSchemaHandler struct {
	ghttp.DefaultHTTPHandlerTask
}

// _DocstringTagOverrideHandler keeps a default tag on the handler type.
//
// @goai.tag Organization
type _DocstringTagOverrideHandler struct {
	ghttp.DefaultHTTPHandlerTask
}

// _DocstringEndpointOnStructHandler keeps a misleading endpoint directive on
// the handler type; endpoint directives must stay method-local.
//
// @goai.endpoint GET /wrong
// @goai.tag StructOnly
type _DocstringEndpointOnStructHandler struct {
	ghttp.DefaultHTTPHandlerTask
}

// Get returns a response documented with a Go struct schema.
//
// @goai.endpoint GET /schema
// @goai.response 200 "Schema response." schemaType=_DocstringSchemaResponse
func (h *_DocstringSchemaHandler) Get(ctx channel.HandlerContext, req *ghttp.Request, resp *ghttp.Response, params map[string]any) ghttp.ErrorResponse {
	return nil
}

// Get repeats the handler-level tag.
//
// @goai.endpoint GET /tag-override
// @goai.tag Organization
func (h *_DocstringTagOverrideHandler) Get(ctx channel.HandlerContext, req *ghttp.Request, resp *ghttp.Response, params map[string]any) ghttp.ErrorResponse {
	return nil
}

// Post overrides the handler-level tag.
//
// @goai.endpoint POST /tag-override
// @goai.tag Management
func (h *_DocstringTagOverrideHandler) Post(ctx channel.HandlerContext, req *ghttp.Request, resp *ghttp.Response, params map[string]any) ghttp.ErrorResponse {
	return nil
}

// Get has no endpoint directive, so the struct-level endpoint must not leak.
//
// @goai.summary Missing endpoint
func (h *_DocstringEndpointOnStructHandler) Get(ctx channel.HandlerContext, req *ghttp.Request, resp *ghttp.Response, params map[string]any) ghttp.ErrorResponse {
	return nil
}

// Get keeps runtime implementation notes out of generated docs.
// It can mention cache warming, retries, or debug behavior privately.
//
// @goai.endpoint GET /items/{id}
// @goai.summary Runtime extracted user
// @goai.description Loaded through the default AST extractor.
// @goai.param query q string optional "Runtime query."
func (h *_DocstringTestHandler) Get(ctx channel.HandlerContext, req *ghttp.Request, resp *ghttp.Response, params map[string]any) ghttp.ErrorResponse {
	return nil
}

func TestParseOpenAPIDocCommentExtractsNamespacedDirectives(t *testing.T) {
	text := `Get keeps internal implementation notes out of generated docs.

@goai.endpoint GET /api/v1/me
@goai.summary Fetch current user
@goai.description Returns the profile visible to the current token.
@goai.description Internal implementation notes stay outside directive lines.
@goai.operationId me.get
@goai.tag User
@goai.tag Internal
@goai.deprecated
@goai.param path organizations_id string required "Encrypted organization ID."
@goai.param query l integer optional "Page size." format=int32 example=50 style=form explode=true allowReserved x-param-audience=partner
@goai.example param query l default "Default page size" 50
@goai.requestBody required application/json object "Update payload." schema={"type":"object","required":["name"],"properties":{"name":{"type":"string"}}} example={"name":"Alice"}
@goai.response 200 "Current user profile." mediaType=application/json schema={"type":"object","properties":{"id":{"type":"string"}}}
@goai.example response 200 application/json success "Successful response" {"id":"usr_1"}
@goai.header 200 X-Request-ID string "Request trace ID." required deprecated allowEmptyValue allowReserved style=simple explode=false example=req_123 examples={"default":{"summary":"Request ID","value":"req_123"}} content=application/json
@goai.response 400 "Invalid request."
@goai.example response 400 application/json rich example={"summary":"Bad request","description":"Detailed error","value":{"message":"bad request"},"x-origin":"goai"}
@goai.link 200 nextOperation users.list "List users." parameters={"id":"$response.body#/id"} requestBody={"cursor":"$response.body#/nextCursor"} server={"url":"https://api.example.com"} x-link-tier=public
@goai.security OAuth2 profile email
@goai.security ApiKey
@goai.server https://api.example.com "Production API"
@goai.externalDocs https://example.com/docs/me "User docs"
@goai.callback onEvent {"{$request.body#/callbackUrl}":{"post":{"responses":{"200":{"description":"Callback accepted."}}}}}
@goai.extension x-operation-tier "internal"
`

	doc, ok := _ParseOpenAPIDocComment(text, "Get")
	require.True(t, ok)
	require.NotNil(t, doc)

	assert.Equal(t, "GET", doc.Endpoint.Method)
	assert.Equal(t, "/api/v1/me", doc.Endpoint.Path)

	op := &doc.Operation
	assert.Equal(t, "Fetch current user", op.Summary)
	assert.Equal(t, "Returns the profile visible to the current token.\nInternal implementation notes stay outside directive lines.", op.Description)
	assert.Equal(t, "me.get", op.OperationID)
	assert.Equal(t, []string{"User", "Internal"}, op.Tags)
	assert.True(t, op.Deprecated)

	require.Len(t, op.Parameters, 2)
	pathParam := op.Parameters[0]
	assert.Equal(t, "organizations_id", pathParam.Name)
	assert.Equal(t, "path", pathParam.In)
	assert.Equal(t, "Encrypted organization ID.", pathParam.Description)
	assert.True(t, pathParam.Required)
	require.NotNil(t, pathParam.Schema)
	assert.Equal(t, "string", pathParam.Schema.Type)

	queryParam := op.Parameters[1]
	assert.Equal(t, "l", queryParam.Name)
	assert.Equal(t, "query", queryParam.In)
	assert.Equal(t, "Page size.", queryParam.Description)
	require.NotNil(t, queryParam.Schema)
	assert.Equal(t, "integer", queryParam.Schema.Type)
	assert.Equal(t, "int32", queryParam.Schema.Format)
	assert.Equal(t, float64(50), queryParam.Example)
	require.NotNil(t, queryParam.Explode)
	assert.True(t, *queryParam.Explode)
	assert.True(t, queryParam.AllowReserved)
	assert.Equal(t, "partner", queryParam.Extensions["x-param-audience"])
	require.Contains(t, queryParam.Examples, "default")
	assert.Equal(t, "Default page size", queryParam.Examples["default"].Summary)
	assert.Equal(t, float64(50), queryParam.Examples["default"].Value)

	require.NotNil(t, op.RequestBody)
	assert.Equal(t, "Update payload.", op.RequestBody.Description)
	assert.True(t, op.RequestBody.Required)
	reqMedia := op.RequestBody.Content["application/json"]
	require.NotNil(t, reqMedia)
	require.NotNil(t, reqMedia.Schema)
	assert.Equal(t, "object", reqMedia.Schema.Type)
	assert.Equal(t, []string{"name"}, reqMedia.Schema.Required)
	assert.Equal(t, "string", reqMedia.Schema.Properties["name"].Type)
	assert.Equal(t, map[string]any{"name": "Alice"}, reqMedia.Example)

	require.Contains(t, op.Responses, "200")
	resp200 := op.Responses["200"]
	assert.Equal(t, "Current user profile.", resp200.Description)
	require.Contains(t, resp200.Content, "application/json")
	assert.Equal(t, "object", resp200.Content["application/json"].Schema.Type)
	require.Contains(t, resp200.Content["application/json"].Examples, "success")
	assert.Equal(t, "Successful response", resp200.Content["application/json"].Examples["success"].Summary)
	assert.Equal(t, map[string]any{"id": "usr_1"}, resp200.Content["application/json"].Examples["success"].Value)
	require.Contains(t, resp200.Headers, "X-Request-ID")
	assert.Equal(t, "Request trace ID.", resp200.Headers["X-Request-ID"].Description)
	assert.True(t, resp200.Headers["X-Request-ID"].Required)
	assert.True(t, resp200.Headers["X-Request-ID"].Deprecated)
	assert.True(t, resp200.Headers["X-Request-ID"].AllowEmptyValue)
	assert.True(t, resp200.Headers["X-Request-ID"].AllowReserved)
	assert.Equal(t, "simple", resp200.Headers["X-Request-ID"].Style)
	require.NotNil(t, resp200.Headers["X-Request-ID"].Explode)
	assert.False(t, *resp200.Headers["X-Request-ID"].Explode)
	assert.Equal(t, "req_123", resp200.Headers["X-Request-ID"].Example)
	require.Contains(t, resp200.Headers["X-Request-ID"].Examples, "default")
	assert.Equal(t, "Request ID", resp200.Headers["X-Request-ID"].Examples["default"].Summary)
	require.Contains(t, resp200.Headers["X-Request-ID"].Content, "application/json")
	require.Contains(t, resp200.Links, "nextOperation")
	link := resp200.Links["nextOperation"]
	assert.Equal(t, "users.list", link.OperationID)
	assert.Equal(t, map[string]any{"id": "$response.body#/id"}, link.Parameters)
	assert.Equal(t, map[string]any{"cursor": "$response.body#/nextCursor"}, link.RequestBody)
	require.NotNil(t, link.Server)
	assert.Equal(t, "https://api.example.com", link.Server.URL)
	assert.Equal(t, "public", link.Extensions["x-link-tier"])

	require.Contains(t, op.Responses, "400")
	assert.Equal(t, "Invalid request.", op.Responses["400"].Description)
	require.Contains(t, op.Responses["400"].Content, "application/json")
	require.Contains(t, op.Responses["400"].Content["application/json"].Examples, "rich")
	richExample := op.Responses["400"].Content["application/json"].Examples["rich"]
	assert.Equal(t, "Bad request", richExample.Summary)
	assert.Equal(t, "Detailed error", richExample.Description)
	assert.Equal(t, map[string]any{"message": "bad request"}, richExample.Value)
	assert.Equal(t, "goai", richExample.Extensions["x-origin"])

	require.NotNil(t, op.Security)
	require.Len(t, *op.Security, 2)
	assert.Equal(t, []string{"profile", "email"}, (*op.Security)[0]["OAuth2"])
	assert.Contains(t, (*op.Security)[1], "ApiKey")
	assert.Empty(t, (*op.Security)[1]["ApiKey"])
	require.Len(t, op.Servers, 1)
	assert.Equal(t, "https://api.example.com", op.Servers[0].URL)
	assert.Equal(t, "Production API", op.Servers[0].Description)
	require.NotNil(t, op.ExternalDocs)
	assert.Equal(t, "https://example.com/docs/me", op.ExternalDocs.URL)
	assert.Equal(t, "User docs", op.ExternalDocs.Description)
	require.Contains(t, op.Callbacks, "onEvent")
	assert.Equal(t, "Callback accepted.", op.Callbacks["onEvent"]["{$request.body#/callbackUrl}"].Post.Responses["200"].Description)
	assert.Equal(t, "internal", op.Extensions["x-operation-tier"])
}

func TestDefaultOperationDocExtractorReadsNamespacedDirectives(t *testing.T) {
	extractor := DefaultOperationDocExtractor()

	doc, ok := extractor(&_DocstringTestHandler{}, "Get")
	require.True(t, ok)
	require.NotNil(t, doc)

	assert.Equal(t, "GET", doc.Endpoint.Method)
	assert.Equal(t, "/items/{id}", doc.Endpoint.Path)
	assert.Equal(t, "Runtime extracted user", doc.Operation.Summary)
	assert.Equal(t, "Loaded through the default AST extractor.", doc.Operation.Description)
	require.Len(t, doc.Operation.Parameters, 1)
	assert.Equal(t, "q", doc.Operation.Parameters[0].Name)
	assert.Equal(t, "query", doc.Operation.Parameters[0].In)
	assert.Equal(t, "Runtime query.", doc.Operation.Parameters[0].Description)
}

func TestDefaultOperationDocExtractorReadsHandlerStructDirectives(t *testing.T) {
	extractor := DefaultOperationDocExtractor()

	doc, ok := extractor(&_DocstringSpecHandler{}, "Get")
	require.True(t, ok)
	require.NotNil(t, doc)

	assert.Equal(t, "GET", doc.Endpoint.Method)
	assert.Equal(t, "/orgs", doc.Endpoint.Path)
	assert.Equal(t, "List organizations", doc.Operation.Summary)
	assert.Equal(t, []string{"Organization"}, doc.Operation.Tags)
	require.NotNil(t, doc.Operation.Security)
	require.Len(t, *doc.Operation.Security, 1)
	assert.Equal(t, []string{"read"}, (*doc.Operation.Security)[0]["OrganizationToken"])
}

func TestDefaultOperationDocExtractorDeduplicatesRepeatedTags(t *testing.T) {
	extractor := DefaultOperationDocExtractor()

	doc, ok := extractor(&_DocstringTagOverrideHandler{}, "Get")
	require.True(t, ok)
	require.NotNil(t, doc)

	assert.Equal(t, []string{"Organization"}, doc.Operation.Tags)
}

func TestDefaultOperationDocExtractorMethodTagsOverrideHandlerTags(t *testing.T) {
	extractor := DefaultOperationDocExtractor()

	doc, ok := extractor(&_DocstringTagOverrideHandler{}, "Post")
	require.True(t, ok)
	require.NotNil(t, doc)

	assert.Equal(t, []string{"Management"}, doc.Operation.Tags)
}

func TestDefaultOperationDocExtractorKeepsEndpointMethodLocal(t *testing.T) {
	extractor := DefaultOperationDocExtractor()

	doc, ok := extractor(&_DocstringEndpointOnStructHandler{}, "Get")
	require.True(t, ok)
	require.NotNil(t, doc)

	assert.Empty(t, doc.Endpoint.Method)
	assert.Empty(t, doc.Endpoint.Path)
	assert.Equal(t, []string{"StructOnly"}, doc.Operation.Tags)
}

func TestBuildOperationUsesDocstringResponseSchemaType(t *testing.T) {
	handler := &_DocstringSchemaHandler{}
	doc := Build([]OperationCandidate{
		{
			Path:          "/schema",
			Method:        "GET",
			Handler:       handler,
			HandlerMethod: "Get",
		},
	}, nil, BuildOptions{
		OperationDocExtractor: DefaultOperationDocExtractor(),
	})

	op := doc.Paths["/schema"].Get
	require.NotNil(t, op)
	resp200 := op.Responses["200"]
	require.NotNil(t, resp200)
	mt := resp200.Content["application/json"]
	require.NotNil(t, mt)
	require.NotNil(t, mt.Schema)
	require.NotEmpty(t, mt.Schema.Ref)

	componentName := strings.TrimPrefix(mt.Schema.Ref, "#/components/schemas/")
	schema := doc.Components.Schemas[componentName]
	require.NotNil(t, schema)
	assert.Equal(t, "object", schema.Type)
	require.Contains(t, schema.Properties, "id")
	assert.Equal(t, "string", schema.Properties["id"].Type)
	require.Contains(t, schema.Properties, "name")
	assert.Equal(t, "Organization name", schema.Properties["name"].Description)
	assert.NotContains(t, schema.Required, "name")
}

func TestBuildOperationUsesDocstringSchemaTypeAcrossSchemaHolders(t *testing.T) {
	handler := &_DocstringSchemaHandler{}
	doc := Build([]OperationCandidate{
		{
			Path:          "/schema",
			Method:        "POST",
			Handler:       handler,
			HandlerMethod: "Post",
		},
	}, nil, BuildOptions{
		OperationDocExtractor: func(handler any, methodName string) (*OperationDoc, bool) {
			return _ParseOpenAPIDocCommentWithContext(`
@goai.endpoint POST /schema
@goai.param query filter object optional "Filter." schemaType=_DocstringSchemaResponse
@goai.requestBody required application/json object "Payload." schemaType=_DocstringSchemaResponse
@goai.response 201 "Created." goType=_DocstringSchemaResponse
@goai.header 400 X-Error object "Error metadata." schemaType=_DocstringSchemaResponse
`, methodName, &_DocParseContext{
				_SchemaBuilder: _DocstringPackageASTBuilder(t),
			})
		},
	})

	op := doc.Paths["/schema"].Post
	require.NotNil(t, op)

	param := _FindParameter(t, op.Parameters, "query", "filter")
	require.NotNil(t, param.Schema)
	assert.True(t, strings.HasPrefix(param.Schema.Ref, "#/components/schemas/"))

	require.NotNil(t, op.RequestBody)
	reqMT := op.RequestBody.Content["application/json"]
	require.NotNil(t, reqMT)
	require.NotNil(t, reqMT.Schema)
	assert.Equal(t, param.Schema.Ref, reqMT.Schema.Ref)

	resp201 := op.Responses["201"]
	require.NotNil(t, resp201)
	respMT := resp201.Content["application/json"]
	require.NotNil(t, respMT)
	require.NotNil(t, respMT.Schema)
	assert.Equal(t, param.Schema.Ref, respMT.Schema.Ref)

	resp400 := op.Responses["400"]
	require.NotNil(t, resp400)
	header := resp400.Headers["X-Error"]
	require.NotNil(t, header)
	require.NotNil(t, header.Schema)
	assert.Equal(t, param.Schema.Ref, header.Schema.Ref)

	componentName := strings.TrimPrefix(param.Schema.Ref, "#/components/schemas/")
	require.Contains(t, doc.Components.Schemas, componentName)
}

func TestBuildOperationUsesDocstringSchemaNameFromTypeDoc(t *testing.T) {
	handler := &_DocstringSchemaHandler{}
	doc := Build([]OperationCandidate{
		{
			Path:          "/schema",
			Method:        "GET",
			Handler:       handler,
			HandlerMethod: "Get",
		},
	}, nil, BuildOptions{
		OperationDocExtractor: func(handler any, methodName string) (*OperationDoc, bool) {
			return _ParseOpenAPIDocCommentWithContext(`
@goai.endpoint GET /schema
@goai.response 200 "Named response." schemaType=_DocstringCustomNameResponse
`, methodName, &_DocParseContext{
				_SchemaBuilder: _DocstringPackageASTBuilder(t),
			})
		},
	})

	op := doc.Paths["/schema"].Get
	require.NotNil(t, op)
	mt := op.Responses["200"].Content["application/json"]
	require.NotNil(t, mt)
	require.NotNil(t, mt.Schema)
	assert.Equal(t, "#/components/schemas/PublicDocstringResponse", mt.Schema.Ref)

	schema := doc.Components.Schemas["PublicDocstringResponse"]
	require.NotNil(t, schema)
	require.Contains(t, schema.Properties, "id")
}

func TestBuildOperationUsesDocstringGenericSchemaType(t *testing.T) {
	handler := &_DocstringSchemaHandler{}
	doc := Build([]OperationCandidate{
		{
			Path:          "/schema",
			Method:        "GET",
			Handler:       handler,
			HandlerMethod: "Get",
		},
	}, nil, BuildOptions{
		OperationDocExtractor: func(handler any, methodName string) (*OperationDoc, bool) {
			return _ParseOpenAPIDocCommentWithContext(`
@goai.endpoint GET /schema
@goai.response 200 "Paged response." schemaType=_DocstringPage[_DocstringSchemaResponse]
`, methodName, &_DocParseContext{
				_SchemaBuilder: _DocstringPackageASTBuilder(t),
			})
		},
	})

	op := doc.Paths["/schema"].Get
	require.NotNil(t, op)
	mt := op.Responses["200"].Content["application/json"]
	require.NotNil(t, mt)
	require.NotNil(t, mt.Schema)
	require.NotEmpty(t, mt.Schema.Ref)

	pageName := strings.TrimPrefix(mt.Schema.Ref, "#/components/schemas/")
	pageSchema := doc.Components.Schemas[pageName]
	require.NotNil(t, pageSchema)
	require.Contains(t, pageSchema.Properties, "data")
	dataSchema := pageSchema.Properties["data"]
	require.NotNil(t, dataSchema)
	require.NotEmpty(t, dataSchema.Ref)

	itemName := strings.TrimPrefix(dataSchema.Ref, "#/components/schemas/")
	require.Contains(t, doc.Components.Schemas, itemName)
	require.Contains(t, pageSchema.Properties, "items")
	require.NotNil(t, pageSchema.Properties["items"].Items)
	assert.Equal(t, dataSchema.Ref, pageSchema.Properties["items"].Items.Ref)
}

func TestBuildOperationUsesDocstringForwardedGenericSchemaType(t *testing.T) {
	handler := &_DocstringSchemaHandler{}
	doc := Build([]OperationCandidate{
		{
			Path:          "/schema",
			Method:        "GET",
			Handler:       handler,
			HandlerMethod: "Get",
		},
	}, nil, BuildOptions{
		OperationDocExtractor: func(handler any, methodName string) (*OperationDoc, bool) {
			return _ParseOpenAPIDocCommentWithContext(`
@goai.endpoint GET /schema
@goai.response 200 "Envelope response." schemaType=_DocstringEnvelope[_DocstringSchemaResponse]
`, methodName, &_DocParseContext{
				_SchemaBuilder: _DocstringPackageASTBuilder(t),
			})
		},
	})

	op := doc.Paths["/schema"].Get
	require.NotNil(t, op)
	mt := op.Responses["200"].Content["application/json"]
	require.NotNil(t, mt)
	require.NotNil(t, mt.Schema)
	require.NotEmpty(t, mt.Schema.Ref)

	envelopeName := strings.TrimPrefix(mt.Schema.Ref, "#/components/schemas/")
	envelopeSchema := doc.Components.Schemas[envelopeName]
	require.NotNil(t, envelopeSchema)
	pageSchemaRef := envelopeSchema.Properties["page"].Ref
	require.NotEmpty(t, pageSchemaRef)

	pageName := strings.TrimPrefix(pageSchemaRef, "#/components/schemas/")
	pageSchema := doc.Components.Schemas[pageName]
	require.NotNil(t, pageSchema)
	dataSchema := pageSchema.Properties["data"]
	require.NotNil(t, dataSchema)
	require.NotEmpty(t, dataSchema.Ref)
	assert.NotContains(t, dataSchema.Ref, ".T")
}

func TestBuildOperationUsesDocstringImportedSchemaType(t *testing.T) {
	handler := &_DocstringSchemaHandler{}
	doc := Build([]OperationCandidate{
		{
			Path:          "/schema",
			Method:        "GET",
			Handler:       handler,
			HandlerMethod: "Get",
		},
	}, nil, BuildOptions{
		OperationDocExtractor: func(handler any, methodName string) (*OperationDoc, bool) {
			return _ParseOpenAPIDocCommentWithContext(`
@goai.endpoint GET /schema
@goai.response 200 "Imported response." schemaType=docschemafixture.SharedResponse
`, methodName, &_DocParseContext{
				_SchemaBuilder: _DocstringPackageASTBuilder(t),
			})
		},
	})

	op := doc.Paths["/schema"].Get
	require.NotNil(t, op)
	mt := op.Responses["200"].Content["application/json"]
	require.NotNil(t, mt)
	require.NotNil(t, mt.Schema)
	assert.Equal(t, "#/components/schemas/github.com.yetiz-org.gone.goai.internal.testfixtures.docschemafixture.SharedResponse", mt.Schema.Ref)

	schema := doc.Components.Schemas["github.com.yetiz-org.gone.goai.internal.testfixtures.docschemafixture.SharedResponse"]
	require.NotNil(t, schema)
	require.Contains(t, schema.Properties, "id")
	require.Contains(t, schema.Properties, "source")
}

func TestBuildOperationUsesDocstringAliasImportedGenericSchemaType(t *testing.T) {
	handler := &_DocstringSchemaHandler{}
	doc := Build([]OperationCandidate{
		{
			Path:          "/schema",
			Method:        "GET",
			Handler:       handler,
			HandlerMethod: "Get",
		},
	}, nil, BuildOptions{
		OperationDocExtractor: func(handler any, methodName string) (*OperationDoc, bool) {
			return _ParseOpenAPIDocCommentWithContext(`
@goai.endpoint GET /schema
@goai.response 200 "Alias imported generic response." schemaType=aliasschema.Page[aliasschema.SharedResponse]
`, methodName, &_DocParseContext{
				_SchemaBuilder: _DocstringPackageASTBuilder(t),
			})
		},
	})

	op := doc.Paths["/schema"].Get
	require.NotNil(t, op)
	mt := op.Responses["200"].Content["application/json"]
	require.NotNil(t, mt)
	require.NotNil(t, mt.Schema)
	require.NotEmpty(t, mt.Schema.Ref)

	pageName := strings.TrimPrefix(mt.Schema.Ref, "#/components/schemas/")
	pageSchema := doc.Components.Schemas[pageName]
	require.NotNil(t, pageSchema)
	dataSchema := pageSchema.Properties["data"]
	require.NotNil(t, dataSchema)
	assert.Equal(t, "#/components/schemas/github.com.yetiz-org.gone.goai.internal.testfixtures.docaliasschema.SharedResponse", dataSchema.Ref)

	schema := doc.Components.Schemas["github.com.yetiz-org.gone.goai.internal.testfixtures.docaliasschema.SharedResponse"]
	require.NotNil(t, schema)
	require.Contains(t, schema.Properties, "id")
}

func TestBuildOperationUsesDocstringAliasToImportedSchemaType(t *testing.T) {
	handler := &_DocstringSchemaHandler{}
	doc := Build([]OperationCandidate{
		{
			Path:          "/schema",
			Method:        "GET",
			Handler:       handler,
			HandlerMethod: "Get",
		},
	}, nil, BuildOptions{
		OperationDocExtractor: func(handler any, methodName string) (*OperationDoc, bool) {
			return _ParseOpenAPIDocCommentWithContext(`
@goai.endpoint GET /schema
@goai.response 200 "Alias response." schemaType=_DocstringAliasResponse
`, methodName, &_DocParseContext{
				_SchemaBuilder: _DocstringPackageASTBuilder(t),
			})
		},
	})

	op := doc.Paths["/schema"].Get
	require.NotNil(t, op)
	mt := op.Responses["200"].Content["application/json"]
	require.NotNil(t, mt)
	require.NotNil(t, mt.Schema)
	assert.Equal(t, "#/components/schemas/github.com.yetiz-org.gone.goai.internal.testfixtures.docschemafixture.SharedResponse", mt.Schema.Ref)

	schema := doc.Components.Schemas["github.com.yetiz-org.gone.goai.internal.testfixtures.docschemafixture.SharedResponse"]
	require.NotNil(t, schema)
	require.Contains(t, schema.Properties, "id")
	require.Contains(t, schema.Properties, "source")
}

func TestBuildOperationUsesDocstringImportedSchemaTypeWithSameShortPackageName(t *testing.T) {
	handler := &_DocstringSchemaHandler{}
	doc := Build([]OperationCandidate{
		{
			Path:          "/schema",
			Method:        "GET",
			Handler:       handler,
			HandlerMethod: "Get",
		},
	}, nil, BuildOptions{
		OperationDocExtractor: func(handler any, methodName string) (*OperationDoc, bool) {
			return _ParseOpenAPIDocCommentWithContext(`
@goai.endpoint GET /schema
@goai.requestBody required application/json object "First user." schemaType=firstmodels.User
@goai.response 200 "Second user." schemaType=secondmodels.User
`, methodName, &_DocParseContext{
				_SchemaBuilder: _DocstringPackageASTBuilder(t),
			})
		},
	})

	firstName := "github.com.yetiz-org.gone.goai.internal.testfixtures.collision.first.models.User"
	secondName := "github.com.yetiz-org.gone.goai.internal.testfixtures.collision.second.models.User"
	require.Contains(t, doc.Components.Schemas, firstName)
	require.Contains(t, doc.Components.Schemas, secondName)
	assert.Contains(t, doc.Components.Schemas[firstName].Properties, "first_id")
	assert.Contains(t, doc.Components.Schemas[secondName].Properties, "second_id")

	op := doc.Paths["/schema"].Get
	require.NotNil(t, op)
	reqMT := op.RequestBody.Content["application/json"]
	require.NotNil(t, reqMT)
	assert.Equal(t, "#/components/schemas/"+firstName, reqMT.Schema.Ref)

	respMT := op.Responses["200"].Content["application/json"]
	require.NotNil(t, respMT)
	assert.Equal(t, "#/components/schemas/"+secondName, respMT.Schema.Ref)
}

func TestBuildOperationDoesNotGuessPackageNameWhenAliasMissing(t *testing.T) {
	handler := &_DocstringSchemaHandler{}
	doc := Build([]OperationCandidate{
		{
			Path:          "/schema",
			Method:        "GET",
			Handler:       handler,
			HandlerMethod: "Get",
		},
	}, nil, BuildOptions{
		OperationDocExtractor: func(handler any, methodName string) (*OperationDoc, bool) {
			return _ParseOpenAPIDocCommentWithContext(`
@goai.endpoint GET /schema
@goai.response 200 "Unknown alias." schemaType=models.User
`, methodName, &_DocParseContext{
				_SchemaBuilder: _DocstringPackageASTBuilder(t),
			})
		},
	})

	op := doc.Paths["/schema"].Get
	require.NotNil(t, op)
	mt := op.Responses["200"].Content["application/json"]
	require.NotNil(t, mt)
	require.NotNil(t, mt.Schema)
	assert.Empty(t, mt.Schema.Ref)
	assert.Equal(t, "models.User", mt.Schema.Type)
}

func TestDocstringSchemaTypeDoesNotResolveCurrentPackageSelector(t *testing.T) {
	builder := _ASTSchemaBuilderFromSource(t, "example.test/models", `package models

type User struct {
	ID string `+"`json:\"id\"`"+`
}
`)

	schema, ok := builder._SchemaForTypeName("models.User")

	assert.False(t, ok)
	assert.Nil(t, schema)
}

func TestDocstringSchemaTypeRespectsTimeImportAlias(t *testing.T) {
	builder := _DocstringPackageASTBuilderFromSource(t, `package goai

import time "github.com/yetiz-org/gone/goai/internal/testfixtures/timeshadow"
`)

	schema, ok := builder._SchemaForTypeName("time.Time")

	require.True(t, ok)
	require.NotNil(t, schema)
	assert.Equal(t, "#/components/schemas/github.com.yetiz-org.gone.goai.internal.testfixtures.timeshadow.Time", schema.Ref)
	component := builder._Components["github.com.yetiz-org.gone.goai.internal.testfixtures.timeshadow.Time"]
	require.NotNil(t, component)
	assert.Contains(t, component.Properties, "value")
}

func TestDocstringSchemaTypeUsesDeclaredPackageNameForUnaliasedImport(t *testing.T) {
	builder := _DocstringPackageASTBuilderFromSource(t, `package goai

import "github.com/yetiz-org/gone/goai/internal/testfixtures/pkgname/schema-v1"
`)

	schema, ok := builder._SchemaForTypeName("schema.User")

	require.True(t, ok)
	require.NotNil(t, schema)
	assert.Equal(t, "#/components/schemas/github.com.yetiz-org.gone.goai.internal.testfixtures.pkgname.schema-v1.User", schema.Ref)
}

func TestDocstringImportedGenericSchemaTypeUsesImportPathInComponentName(t *testing.T) {
	first := _DocstringPackageASTBuilderFromSource(t, `package goai

import (
	models "github.com/yetiz-org/gone/goai/internal/testfixtures/collision/first/models"
	page "github.com/yetiz-org/gone/goai/internal/testfixtures/generic/page"
)
`)
	second := _DocstringPackageASTBuilderFromSource(t, `package goai

import (
	models "github.com/yetiz-org/gone/goai/internal/testfixtures/collision/second/models"
	page "github.com/yetiz-org/gone/goai/internal/testfixtures/generic/page"
)
`)

	firstSchema, ok := first._SchemaForTypeName("page.Page[models.User]")
	require.True(t, ok)
	secondSchema, ok := second._SchemaForTypeName("page.Page[models.User]")
	require.True(t, ok)

	require.NotEmpty(t, firstSchema.Ref)
	require.NotEmpty(t, secondSchema.Ref)
	assert.NotEqual(t, firstSchema.Ref, secondSchema.Ref)
	assert.Contains(t, firstSchema.Ref, "collision.first.models.User")
	assert.Contains(t, secondSchema.Ref, "collision.second.models.User")

	firstPage := first._Components[strings.TrimPrefix(firstSchema.Ref, "#/components/schemas/")]
	require.NotNil(t, firstPage)
	firstData := firstPage.Properties["data"]
	require.NotNil(t, firstData)
	assert.Contains(t, firstData.Ref, "collision.first.models.User")

	secondPage := second._Components[strings.TrimPrefix(secondSchema.Ref, "#/components/schemas/")]
	require.NotNil(t, secondPage)
	secondData := secondPage.Properties["data"]
	require.NotNil(t, secondData)
	assert.Contains(t, secondData.Ref, "collision.second.models.User")
}

func TestDocstringSourceImportAliasWinsOverInheritedSelfAlias(t *testing.T) {
	builder := _DocstringPackageASTBuilderFromSource(t, `package goai

import models "github.com/yetiz-org/gone/goai/internal/testfixtures/selfalias/box"
`)

	schema, ok := builder._SchemaForTypeName("models.Box")

	require.True(t, ok)
	require.NotNil(t, schema)
	box := builder._Components[strings.TrimPrefix(schema.Ref, "#/components/schemas/")]
	require.NotNil(t, box)
	data := box.Properties["data"]
	require.NotNil(t, data)
	assert.Contains(t, data.Ref, "collision.second.models.User")
	assert.NotContains(t, data.Ref, "selfalias.box.User")
}

func TestDocstringForwardedImportedGenericArgKeepsSourceImports(t *testing.T) {
	first := _DocstringPackageASTBuilderFromSource(t, `package goai

import (
	models "github.com/yetiz-org/gone/goai/internal/testfixtures/collision/first/models"
	outer "github.com/yetiz-org/gone/goai/internal/testfixtures/generic/outer"
)
`)
	second := _DocstringPackageASTBuilderFromSource(t, `package goai

import (
	models "github.com/yetiz-org/gone/goai/internal/testfixtures/collision/second/models"
	outer "github.com/yetiz-org/gone/goai/internal/testfixtures/generic/outer"
)
`)

	firstSchema, ok := first._SchemaForTypeName("outer.Envelope[models.User]")
	require.True(t, ok)
	secondSchema, ok := second._SchemaForTypeName("outer.Envelope[models.User]")
	require.True(t, ok)

	firstEnvelope := first._Components[strings.TrimPrefix(firstSchema.Ref, "#/components/schemas/")]
	require.NotNil(t, firstEnvelope)
	firstPageRef := firstEnvelope.Properties["page"].Ref
	require.NotEmpty(t, firstPageRef)
	firstPage := first._Components[strings.TrimPrefix(firstPageRef, "#/components/schemas/")]
	require.NotNil(t, firstPage)
	assert.Contains(t, firstPage.Properties["data"].Ref, "collision.first.models.User")

	secondEnvelope := second._Components[strings.TrimPrefix(secondSchema.Ref, "#/components/schemas/")]
	require.NotNil(t, secondEnvelope)
	secondPageRef := secondEnvelope.Properties["page"].Ref
	require.NotEmpty(t, secondPageRef)
	secondPage := second._Components[strings.TrimPrefix(secondPageRef, "#/components/schemas/")]
	require.NotNil(t, secondPage)
	assert.Contains(t, secondPage.Properties["data"].Ref, "collision.second.models.User")
}

func TestParseOpenAPIDocCommentIgnoresUnmarkedHandlerComments(t *testing.T) {
	text := `Get GET /api/v1/me
Internal note: this handler depends on token cache warming.
Do not expose this operational detail.

Query parameters:
  - debug: internal only
`

	doc, ok := _ParseOpenAPIDocComment(text, "Get")
	assert.False(t, ok)
	assert.Nil(t, doc)
}

func TestParseOpenAPIDocCommentKeepsJSONValuesWithSpacesIntact(t *testing.T) {
	text := `Get documents a JSON schema with spaces.

@goai.endpoint GET /errors
@goai.response 400 "Invalid request." mediaType=application/json schema={"type":"object","description":"bad request payload","properties":{"message":{"type":"string","example":"bad request"}}} example={"message":"bad request"}
`

	doc, ok := _ParseOpenAPIDocComment(text, "Get")
	require.True(t, ok)
	require.NotNil(t, doc)

	resp := doc.Operation.Responses["400"]
	require.NotNil(t, resp)
	mt := resp.Content["application/json"]
	require.NotNil(t, mt)
	require.NotNil(t, mt.Schema)
	assert.Equal(t, "bad request payload", mt.Schema.Description)
	assert.Equal(t, "bad request", mt.Schema.Properties["message"].Example)
	assert.Equal(t, map[string]any{"message": "bad request"}, mt.Example)
}

func TestParseOpenAPIDocCommentKeepsRequestExampleBeforeRequestBody(t *testing.T) {
	text := `Post documents a request example before the request body.

@goai.endpoint POST /items
@goai.example request application/json default "Default request" {"name":"Alice"}
@goai.requestBody required application/json object "Create payload." schema={"type":"object","properties":{"name":{"type":"string"}}}
`

	doc, ok := _ParseOpenAPIDocComment(text, "Post")
	require.True(t, ok)
	require.NotNil(t, doc)
	require.NotNil(t, doc.Operation.RequestBody)

	mt := doc.Operation.RequestBody.Content["application/json"]
	require.NotNil(t, mt)
	require.NotNil(t, mt.Schema)
	assert.Equal(t, "object", mt.Schema.Type)
	require.Contains(t, mt.Examples, "default")
	assert.Equal(t, "Default request", mt.Examples["default"].Summary)
	assert.Equal(t, map[string]any{"name": "Alice"}, mt.Examples["default"].Value)
}

func TestParseOpenAPIDocCommentKeepsParamExampleBeforeParam(t *testing.T) {
	text := `Get documents a parameter example before the parameter.

@goai.endpoint GET /items/{id}
@goai.example param path id default "Default ID" "item_1"
@goai.param path id string required "Item ID." deprecated allowEmptyValue allowReserved style=simple
`

	doc, ok := _ParseOpenAPIDocComment(text, "Get")
	require.True(t, ok)
	require.NotNil(t, doc)
	require.Len(t, doc.Operation.Parameters, 1)

	param := doc.Operation.Parameters[0]
	assert.Equal(t, "id", param.Name)
	assert.Equal(t, "path", param.In)
	assert.Equal(t, "Item ID.", param.Description)
	assert.True(t, param.Required)
	assert.True(t, param.Deprecated)
	assert.True(t, param.AllowEmptyValue)
	assert.True(t, param.AllowReserved)
	assert.Equal(t, "simple", param.Style)
	require.NotNil(t, param.Schema)
	assert.Equal(t, "string", param.Schema.Type)
	require.Contains(t, param.Examples, "default")
	assert.Equal(t, "Default ID", param.Examples["default"].Summary)
	assert.Equal(t, "item_1", param.Examples["default"].Value)
}

func TestParseOpenAPIDocCommentAppendsTargetDescriptions(t *testing.T) {
	text := `Post documents multi-line descriptions on nested OpenAPI objects.

@goai.endpoint POST /items
@goai.param query q string optional "Search query."
@goai.param.description query q Supports partial matching.
@goai.param.description query q Multiple words are treated as AND.
@goai.requestBody required application/json object "Create payload."
@goai.requestBody.description Includes item metadata.
@goai.requestBody.description Unknown fields are ignored.
@goai.response 400 "Invalid request."
@goai.response.description 400 Returned when validation fails.
@goai.response.description 400 Field-level errors are included.
@goai.header 400 X-Request-ID string "Request trace ID."
@goai.header.description 400 X-Request-ID Use this value when reporting issues.
@goai.header.description 400 X-Request-ID It is stable for one request.
@goai.link 400 retry items.retry "Retry failed item."
@goai.link.description 400 retry The client may retry after fixing the payload.
@goai.example.description param query q default Query example description before value.
@goai.example param query q default "Default query" "alice"
@goai.example.description param query q default Query example description after value.
@goai.example request application/json default "Default body" {"name":"Alice"}
@goai.example.description request application/json default Body example description.
@goai.example response 400 application/json invalid "Invalid response" {"message":"bad request"}
@goai.example.description response 400 application/json invalid Shows the standard error envelope.
@goai.example.description response 400 application/json invalid The message is localized by clients.
@goai.server https://api.example.com "Production API"
@goai.server.description https://api.example.com Primary production region.
@goai.externalDocs https://docs.example.com/items "Item docs"
@goai.externalDocs.description Read before integrating item writes.
`

	doc, ok := _ParseOpenAPIDocComment(text, "Post")
	require.True(t, ok)
	require.NotNil(t, doc)

	query := _FindParameter(t, doc.Operation.Parameters, "query", "q")
	assert.Equal(t, "Search query.\nSupports partial matching.\nMultiple words are treated as AND.", query.Description)
	require.NotNil(t, doc.Operation.RequestBody)
	assert.Equal(t, "Create payload.\nIncludes item metadata.\nUnknown fields are ignored.", doc.Operation.RequestBody.Description)

	resp400 := doc.Operation.Responses["400"]
	require.NotNil(t, resp400)
	assert.Equal(t, "Invalid request.\nReturned when validation fails.\nField-level errors are included.", resp400.Description)
	header := resp400.Headers["X-Request-ID"]
	require.NotNil(t, header)
	assert.Equal(t, "Request trace ID.\nUse this value when reporting issues.\nIt is stable for one request.", header.Description)
	link := resp400.Links["retry"]
	require.NotNil(t, link)
	assert.Equal(t, "Retry failed item.\nThe client may retry after fixing the payload.", link.Description)
	paramExample := query.Examples["default"]
	require.NotNil(t, paramExample)
	assert.Equal(t, "Query example description before value.\nQuery example description after value.", paramExample.Description)
	requestExample := doc.Operation.RequestBody.Content["application/json"].Examples["default"]
	require.NotNil(t, requestExample)
	assert.Equal(t, "Body example description.", requestExample.Description)
	example := resp400.Content["application/json"].Examples["invalid"]
	require.NotNil(t, example)
	assert.Equal(t, "Shows the standard error envelope.\nThe message is localized by clients.", example.Description)

	require.Len(t, doc.Operation.Servers, 1)
	assert.Equal(t, "Production API\nPrimary production region.", doc.Operation.Servers[0].Description)
	require.NotNil(t, doc.Operation.ExternalDocs)
	assert.Equal(t, "Item docs\nRead before integrating item writes.", doc.Operation.ExternalDocs.Description)
}

func TestParseOpenAPIDocCommentKeepsDescriptionsBeforeStructuralDirectives(t *testing.T) {
	text := `Post documents descriptions before structural directives.

@goai.endpoint POST /items
@goai.param.description query q Search query preface.
@goai.param query q string optional "Search query."
@goai.requestBody.description Create payload preface.
@goai.requestBody required application/json object "Create payload."
@goai.response.description 400 Invalid request preface.
@goai.response 400 "Invalid request."
@goai.header.description 400 X-Request-ID Request trace preface.
@goai.header 400 X-Request-ID string "Request trace ID."
@goai.link.description 400 retry Retry preface.
@goai.link 400 retry items.retry "Retry failed item."
@goai.server.description https://api.example.com Production preface.
@goai.server https://api.example.com "Production API"
@goai.externalDocs.description Docs preface.
@goai.externalDocs https://docs.example.com/items "Item docs"
`

	doc, ok := _ParseOpenAPIDocComment(text, "Post")
	require.True(t, ok)
	require.NotNil(t, doc)

	query := _FindParameter(t, doc.Operation.Parameters, "query", "q")
	assert.Equal(t, "Search query preface.\nSearch query.", query.Description)
	require.NotNil(t, doc.Operation.RequestBody)
	assert.Equal(t, "Create payload preface.\nCreate payload.", doc.Operation.RequestBody.Description)

	resp400 := doc.Operation.Responses["400"]
	require.NotNil(t, resp400)
	assert.Equal(t, "Invalid request preface.\nInvalid request.", resp400.Description)
	assert.Equal(t, "Request trace preface.\nRequest trace ID.", resp400.Headers["X-Request-ID"].Description)
	assert.Equal(t, "Retry preface.\nRetry failed item.", resp400.Links["retry"].Description)
	require.Len(t, doc.Operation.Servers, 1)
	assert.Equal(t, "Production preface.\nProduction API", doc.Operation.Servers[0].Description)
	require.NotNil(t, doc.Operation.ExternalDocs)
	assert.Equal(t, "Docs preface.\nItem docs", doc.Operation.ExternalDocs.Description)
}

func TestParseOpenAPIDocCommentDoesNotDuplicateStructuralDescriptions(t *testing.T) {
	text := `Get repeats structural directives to add more content.

@goai.endpoint GET /items
@goai.response 200 "OK" mediaType=application/json type=object
@goai.response 200 "OK" mediaType=text/plain type=string
@goai.server https://api.example.com "Production API"
@goai.server https://api.example.com "Production API"
`

	doc, ok := _ParseOpenAPIDocComment(text, "Get")
	require.True(t, ok)
	require.NotNil(t, doc)

	resp200 := doc.Operation.Responses["200"]
	require.NotNil(t, resp200)
	assert.Equal(t, "OK", resp200.Description)
	require.Contains(t, resp200.Content, "application/json")
	require.Contains(t, resp200.Content, "text/plain")
	require.Len(t, doc.Operation.Servers, 1)
	assert.Equal(t, "Production API", doc.Operation.Servers[0].Description)
}

func TestDocstringCacheParsePackageHonorsBuildConstraints(t *testing.T) {
	dir := t.TempDir()
	activePath := filepath.Join(dir, "active.go")
	ignoredPath := filepath.Join(dir, "ignored.go")
	customPath := filepath.Join(dir, "custom.go")

	require.NoError(t, os.WriteFile(activePath, []byte("package p\n\ntype Active struct{}\n"), 0o644))
	require.NoError(t, os.WriteFile(ignoredPath, []byte("//go:build goai_never\n\npackage p\n\ntype Ignored struct{}\n"), 0o644))
	require.NoError(t, os.WriteFile(customPath, []byte("//go:build goai_custom\n\npackage p\n\ntype Custom struct{}\n"), 0o644))

	cache := &_DocstringCache{
		fset:     token.NewFileSet(),
		files:    map[string]*ast.File{},
		packages: map[string][]*ast.File{},
	}
	files := cache._ParsePackage(activePath, "p")
	require.Len(t, files, 1)

	builder := _NewASTSchemaBuilder(files, "example.test/p")
	assert.Contains(t, builder._TypeSpecs, "Active")
	assert.NotContains(t, builder._TypeSpecs, "Ignored")
	assert.NotContains(t, builder._TypeSpecs, "Custom")

	customCache := &_DocstringCache{
		fset:      token.NewFileSet(),
		files:     map[string]*ast.File{},
		packages:  map[string][]*ast.File{},
		buildTags: []string{"goai_custom"},
	}
	customFiles := customCache._ParsePackage(activePath, "p")
	customBuilder := _NewASTSchemaBuilder(customFiles, "example.test/p")
	assert.Contains(t, customBuilder._TypeSpecs, "Custom")
}

func TestMergeOperationDocSchemasPromotesCollidingShortComponentName(t *testing.T) {
	components := NewComponents()
	components.Schemas["models.User"] = &Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"base": {Type: "string"},
		},
	}

	schemaBld := newSchemaBuilder(components)
	schemaBld.pkgOfName["models.User"] = "example.com/base/models"

	doc := &OperationDoc{
		Schemas: map[string]*Schema{
			"models.User": {
				Type: "object",
				Properties: map[string]*Schema{
					"id": {Type: "string"},
				},
			},
		},
		SchemaPackages: map[string]string{
			"models.User": "example.com/current/models",
		},
		Operation: Operation{
			Responses: map[string]*Response{
				"200": {
					Content: map[string]*MediaType{
						"application/json": {
							Schema: &Schema{Ref: "#/components/schemas/models.User"},
						},
					},
				},
			},
		},
	}

	_MergeOperationDocSchemas(schemaBld, doc)

	promotedName := "example.com.current.models.User"
	require.Contains(t, components.Schemas, promotedName)
	assert.Contains(t, components.Schemas["models.User"].Properties, "base")
	assert.Contains(t, components.Schemas[promotedName].Properties, "id")
	assert.Equal(t, "#/components/schemas/"+promotedName, doc.Operation.Responses["200"].Content["application/json"].Schema.Ref)
	assert.Equal(t, "example.com/current/models", schemaBld.pkgOfName[promotedName])
}

func TestSchemaBuilderUsesGOAISchemaName(t *testing.T) {
	components := NewComponents()
	schemaBld := newSchemaBuilder(components)

	schema := schemaBld.build(reflect.TypeOf(_CustomSchemaNameResponse{}))

	require.NotNil(t, schema)
	assert.Equal(t, "#/components/schemas/PublicCustomResponse", schema.Ref)
	require.Contains(t, components.Schemas, "PublicCustomResponse")
	assert.Contains(t, components.Schemas["PublicCustomResponse"].Properties, "id")
}

func TestBuildOperationAppliesOpenAPIDocFallbackWithoutOverridingSpec(t *testing.T) {
	Reset()
	defer Reset()

	handler := &_DocstringTestHandler{}
	Register(handler, "Get", nil, nil,
		WithSummary("Spec summary"),
		WithParam(PathParam{
			Name:        "q",
			In:          "query",
			Description: "Spec query.",
			Required:    false,
		}),
		WithSuccessDescription("Spec success."),
	)

	docOp := &OperationDoc{
		Endpoint: OperationEndpoint{Method: "GET", Path: "/items/{id}"},
		Operation: Operation{
			Summary:     "Doc summary",
			Description: "Doc description.",
			Parameters: []*Parameter{
				{
					Name:        "id",
					In:          "path",
					Description: "Doc ID.",
					Schema:      &Schema{Type: "string"},
				},
				{
					Name:        "q",
					In:          "query",
					Description: "Doc query.",
					Required:    true,
					Schema:      &Schema{Type: "string"},
					Content: map[string]*MediaType{
						"application/json": {Schema: &Schema{Type: "string"}},
					},
					AllowReserved: true,
				},
				{
					Name:        "l",
					In:          "query",
					Description: "Doc limit.",
					Schema:      &Schema{Type: "integer", Format: "int32"},
				},
			},
			Responses: map[string]*Response{
				"200": {Description: "Doc success."},
				"404": {Description: "Not found."},
			},
		},
	}

	doc := Build([]OperationCandidate{
		{
			Path:          "/items/{id}",
			Method:        "GET",
			Handler:       handler,
			HandlerMethod: "Get",
			PathParams: []PathParam{
				{
					Name:        "id",
					In:          "path",
					Required:    true,
					Description: "Route ID.",
				},
			},
		},
	}, nil, BuildOptions{
		OperationDocExtractor: func(handler any, methodName string) (*OperationDoc, bool) {
			return docOp, true
		},
	})

	op := doc.Paths["/items/{id}"].Get
	require.NotNil(t, op)

	assert.Equal(t, "Spec summary", op.Summary)
	assert.Equal(t, "Doc description.", op.Description)

	require.Len(t, op.Parameters, 3)
	assert.Equal(t, "Route ID.", _FindParameter(t, op.Parameters, "path", "id").Description)
	query := _FindParameter(t, op.Parameters, "query", "q")
	assert.Equal(t, "Spec query.", query.Description)
	assert.False(t, query.Required)
	assert.False(t, query.AllowReserved)
	require.NotNil(t, query.Schema)
	assert.Empty(t, query.Content)
	assert.Equal(t, "Doc limit.", _FindParameter(t, op.Parameters, "query", "l").Description)

	require.Contains(t, op.Responses, "200")
	assert.Equal(t, "Spec success.", op.Responses["200"].Description)
	require.Contains(t, op.Responses, "404")
	assert.Equal(t, "Not found.", op.Responses["404"].Description)
}

func TestBuildOperationIgnoresDocFallbackWhenEndpointDoesNotMatch(t *testing.T) {
	handler := &_DocstringTestHandler{}
	doc := Build([]OperationCandidate{
		{
			Path:          "/items/{id}",
			Method:        "GET",
			Handler:       handler,
			HandlerMethod: "Get",
		},
	}, nil, BuildOptions{
		OperationDocExtractor: func(handler any, methodName string) (*OperationDoc, bool) {
			return &OperationDoc{
				Endpoint:  OperationEndpoint{Method: "POST", Path: "/wrong"},
				Operation: Operation{Summary: "Wrong endpoint doc"},
			}, true
		},
	})

	op := doc.Paths["/items/{id}"].Get
	require.NotNil(t, op)
	assert.Empty(t, op.Summary)
}

func TestBuildOperationIgnoresDocFallbackWithoutEndpointDirective(t *testing.T) {
	handler := &_DocstringTestHandler{}
	doc := Build([]OperationCandidate{
		{
			Path:          "/items/{id}",
			Method:        "GET",
			Handler:       handler,
			HandlerMethod: "Get",
		},
	}, nil, BuildOptions{
		OperationDocExtractor: func(handler any, methodName string) (*OperationDoc, bool) {
			return &OperationDoc{Operation: Operation{Summary: "Missing endpoint doc"}}, true
		},
	})

	op := doc.Paths["/items/{id}"].Get
	require.NotNil(t, op)
	assert.Empty(t, op.Summary)
}

func TestBuildOperationMergesDocExamplesIntoGeneratedContent(t *testing.T) {
	Reset()
	defer Reset()

	handler := &_DocstringTestHandler{}
	Register(handler, "POST", (*_DocMergeRequest)(nil), (*_DocMergeResponse)(nil))

	doc := Build([]OperationCandidate{
		{
			Path:          "/items",
			Method:        "POST",
			Handler:       handler,
			HandlerMethod: "Post",
		},
	}, nil, BuildOptions{
		OperationDocExtractor: func(handler any, methodName string) (*OperationDoc, bool) {
			return &OperationDoc{
				Endpoint: OperationEndpoint{Method: "POST", Path: "/items"},
				Operation: Operation{
					RequestBody: &RequestBody{
						Content: map[string]*MediaType{
							"application/json": {
								Example: map[string]any{"name": "Alice"},
							},
						},
					},
					Responses: map[string]*Response{
						"201": {
							Content: map[string]*MediaType{
								"application/json": {
									Examples: map[string]*Example{
										"created": {
											Summary: "Created item",
											Value:   map[string]any{"id": "item_1"},
										},
									},
								},
							},
						},
					},
				},
			}, true
		},
	})

	op := doc.Paths["/items"].Post
	require.NotNil(t, op)
	require.NotNil(t, op.RequestBody)
	reqJSON := op.RequestBody.Content["application/json"]
	require.NotNil(t, reqJSON)
	require.NotNil(t, reqJSON.Schema)
	assert.Equal(t, map[string]any{"name": "Alice"}, reqJSON.Example)

	respJSON := op.Responses["201"].Content["application/json"]
	require.NotNil(t, respJSON)
	require.NotNil(t, respJSON.Schema)
	require.Contains(t, respJSON.Examples, "created")
	assert.Equal(t, "Created item", respJSON.Examples["created"].Summary)
	assert.Equal(t, map[string]any{"id": "item_1"}, respJSON.Examples["created"].Value)
}

func TestBuildOperationMergesDocResponseHeadersAndLinksByName(t *testing.T) {
	Reset()
	defer Reset()

	specCallback := Callback{
		"{$request.body#/specCallbackUrl}": {
			Post: &Operation{
				Responses: map[string]*Response{
					"200": {Description: "Spec callback accepted."},
				},
			},
		},
	}
	handler := &_DocstringTestHandler{}
	Register(handler, "GET", nil, nil,
		WithCallback("specCallback", specCallback),
		WithResponse("400", "Spec bad request.",
			WithResponseHeader(HeaderDef{
				Name:        "X-Error-Code",
				Description: "Spec error code.",
				Example:     "spec_error",
			}),
		),
	)

	doc := Build([]OperationCandidate{
		{
			Path:          "/items/{id}",
			Method:        "GET",
			Handler:       handler,
			HandlerMethod: "Get",
			SecurityRefs: []SecurityRef{
				{Scheme: "RouteAuth", Scopes: []string{"read"}},
			},
		},
	}, nil, BuildOptions{
		OperationDocExtractor: func(handler any, methodName string) (*OperationDoc, bool) {
			return &OperationDoc{
				Endpoint: OperationEndpoint{Method: "GET", Path: "/items/{id}"},
				Operation: Operation{
					Security: &[]map[string][]string{
						{"DocAuth": {}},
					},
					Callbacks: map[string]Callback{
						"specCallback": {
							"{$request.body#/docOverrideUrl}": {
								Post: &Operation{
									Responses: map[string]*Response{
										"200": {Description: "Doc override ignored."},
									},
								},
							},
						},
						"docCallback": {
							"{$request.body#/docCallbackUrl}": {
								Post: &Operation{
									Responses: map[string]*Response{
										"200": {Description: "Doc callback accepted."},
									},
								},
							},
						},
					},
					Responses: map[string]*Response{
						"400": {
							Description: "Doc bad request.",
							Headers: map[string]*Header{
								"X-Error-Code": {
									Description: "Doc error code.",
									Example:     "doc_error",
									Style:       "simple",
									Content: map[string]*MediaType{
										"application/json": {Schema: &Schema{Type: "string"}},
									},
									Examples: map[string]*Example{
										"doc": {
											Summary: "Doc error code",
											Value:   "doc_error",
										},
									},
								},
								"X-Request-ID": {
									Description: "Doc request trace.",
									Example:     "req_123",
								},
							},
							Links: map[string]*Link{
								"retry": {
									OperationID: "items.retry",
									Description: "Retry failed item.",
								},
							},
						},
					},
				},
			}, true
		},
	})

	op := doc.Paths["/items/{id}"].Get
	require.NotNil(t, op)
	require.NotNil(t, op.Security)
	require.Len(t, *op.Security, 1)
	assert.Equal(t, []string{"read"}, (*op.Security)[0]["RouteAuth"])
	assert.NotContains(t, (*op.Security)[0], "DocAuth")

	resp400 := op.Responses["400"]
	require.NotNil(t, resp400)
	assert.Equal(t, "Spec bad request.", resp400.Description)
	require.Contains(t, resp400.Headers, "X-Error-Code")
	assert.Equal(t, "Spec error code.", resp400.Headers["X-Error-Code"].Description)
	assert.Equal(t, "spec_error", resp400.Headers["X-Error-Code"].Example)
	assert.Equal(t, "simple", resp400.Headers["X-Error-Code"].Style)
	require.NotNil(t, resp400.Headers["X-Error-Code"].Schema)
	assert.Empty(t, resp400.Headers["X-Error-Code"].Content)
	require.Contains(t, resp400.Headers["X-Error-Code"].Examples, "doc")
	assert.Equal(t, "Doc error code", resp400.Headers["X-Error-Code"].Examples["doc"].Summary)
	require.Contains(t, resp400.Headers, "X-Request-ID")
	assert.Equal(t, "Doc request trace.", resp400.Headers["X-Request-ID"].Description)
	assert.Equal(t, "req_123", resp400.Headers["X-Request-ID"].Example)
	require.Contains(t, resp400.Links, "retry")
	assert.Equal(t, "items.retry", resp400.Links["retry"].OperationID)
	assert.Equal(t, "Retry failed item.", resp400.Links["retry"].Description)
	require.Contains(t, op.Callbacks, "specCallback")
	assert.Equal(t, specCallback, op.Callbacks["specCallback"])
	require.Contains(t, op.Callbacks, "docCallback")
	assert.Equal(t, "Doc callback accepted.", op.Callbacks["docCallback"]["{$request.body#/docCallbackUrl}"].Post.Responses["200"].Description)
}

func TestBuildOperationKeepsSpecHeaderParameterBeforeDocFallback(t *testing.T) {
	Reset()
	defer Reset()

	handler := &_DocstringTestHandler{}
	Register(handler, "GET", nil, nil,
		WithHeader(HeaderDef{
			In:          "header",
			Name:        "X-Trace",
			Description: "Spec trace header.",
			Required:    true,
			Example:     "spec_trace",
		}),
	)

	doc := Build([]OperationCandidate{
		{
			Path:          "/items/{id}",
			Method:        "GET",
			Handler:       handler,
			HandlerMethod: "Get",
		},
	}, nil, BuildOptions{
		OperationDocExtractor: func(handler any, methodName string) (*OperationDoc, bool) {
			return &OperationDoc{
				Endpoint: OperationEndpoint{Method: "GET", Path: "/items/{id}"},
				Operation: Operation{
					Parameters: []*Parameter{
						{
							Name:        "X-Trace",
							In:          "header",
							Description: "Doc trace header.",
							Schema:      &Schema{Type: "string"},
							Example:     "doc_trace",
						},
						{
							Name:        "X-Debug",
							In:          "header",
							Description: "Doc debug header.",
							Schema:      &Schema{Type: "string"},
						},
					},
				},
			}, true
		},
	})

	op := doc.Paths["/items/{id}"].Get
	require.NotNil(t, op)
	assert.Equal(t, 1, _CountParameters(op.Parameters, "header", "X-Trace"))
	trace := _FindParameter(t, op.Parameters, "header", "X-Trace")
	assert.Equal(t, "Spec trace header.", trace.Description)
	assert.True(t, trace.Required)
	assert.Equal(t, "spec_trace", trace.Example)
	debug := _FindParameter(t, op.Parameters, "header", "X-Debug")
	assert.Equal(t, "Doc debug header.", debug.Description)
}

func TestBuildOperationUsesDocSchemaInsteadOfDefaultPlaceholderSchema(t *testing.T) {
	handler := &_DocstringTestHandler{}
	doc := Build([]OperationCandidate{
		{
			Path:          "/items",
			Method:        "GET",
			Handler:       handler,
			HandlerMethod: "Get",
		},
	}, nil, BuildOptions{
		OperationDocExtractor: func(handler any, methodName string) (*OperationDoc, bool) {
			return &OperationDoc{
				Endpoint: OperationEndpoint{Method: "GET", Path: "/items"},
				Operation: Operation{
					Responses: map[string]*Response{
						"200": {
							Content: map[string]*MediaType{
								"application/json": {
									Schema: &Schema{Type: "array", Items: &Schema{Type: "string"}},
								},
							},
						},
					},
				},
			}, true
		},
	})

	op := doc.Paths["/items"].Get
	require.NotNil(t, op)
	assert.Equal(t, "OK", op.Responses["200"].Description)
	mt := op.Responses["200"].Content["application/json"]
	require.NotNil(t, mt)
	require.NotNil(t, mt.Schema)
	assert.Equal(t, "array", mt.Schema.Type)
	require.NotNil(t, mt.Schema.Items)
	assert.Equal(t, "string", mt.Schema.Items.Type)
}

func TestBuildOperationAppliesResponseSpecOptions(t *testing.T) {
	Reset()
	defer Reset()

	handler := &_DocstringTestHandler{}
	Register(handler, "GET", nil, (*_DocMergeResponse)(nil),
		WithResponse("200", "Documented success.",
			WithResponseExampleObject("ok", &Example{
				Summary: "Successful response",
				Value:   map[string]any{"id": "item_1"},
			}),
		),
		WithResponse("400", "",
			WithResponseSchema(reflect.TypeOf((*_ResponseSpecError)(nil))),
			WithResponseHeader(HeaderDef{
				Name:        "X-Error-Code",
				Description: "Stable error code.",
				Required:    true,
				Example:     "invalid_request",
			}),
			WithResponseExampleObject("bad", &Example{
				Summary: "Bad request",
				Value:   map[string]any{"message": "bad request"},
			}),
		),
		WithResponse("418", "",
			WithResponseMediaType("application/problem+json"),
			WithResponseSchemaPrebuilt(&Schema{
				Type: "object",
				Properties: map[string]*Schema{
					"teapot": {Type: "boolean"},
				},
			}),
			WithResponseExample(map[string]any{"teapot": true}),
		),
	)

	doc := Build([]OperationCandidate{
		{
			Path:          "/items/{id}",
			Method:        "GET",
			Handler:       handler,
			HandlerMethod: "Get",
		},
	}, nil, BuildOptions{})

	op := doc.Paths["/items/{id}"].Get
	require.NotNil(t, op)

	resp200 := op.Responses["200"]
	require.NotNil(t, resp200)
	assert.Equal(t, "Documented success.", resp200.Description)
	mt200 := resp200.Content["application/json"]
	require.NotNil(t, mt200)
	require.NotNil(t, mt200.Schema)
	assert.NotEmpty(t, mt200.Schema.Ref)
	require.Contains(t, mt200.Examples, "ok")
	assert.Equal(t, "Successful response", mt200.Examples["ok"].Summary)

	resp400 := op.Responses["400"]
	require.NotNil(t, resp400)
	assert.Equal(t, "Bad Request", resp400.Description)
	require.Contains(t, resp400.Headers, "X-Error-Code")
	assert.True(t, resp400.Headers["X-Error-Code"].Required)
	assert.Equal(t, "invalid_request", resp400.Headers["X-Error-Code"].Example)
	mt400 := resp400.Content["application/json"]
	require.NotNil(t, mt400)
	require.NotNil(t, mt400.Schema)
	assert.NotEmpty(t, mt400.Schema.Ref)
	require.Contains(t, mt400.Examples, "bad")
	assert.Equal(t, map[string]any{"message": "bad request"}, mt400.Examples["bad"].Value)

	resp418 := op.Responses["418"]
	require.NotNil(t, resp418)
	assert.Equal(t, "I'm a teapot", resp418.Description)
	mt418 := resp418.Content["application/problem+json"]
	require.NotNil(t, mt418)
	require.NotNil(t, mt418.Schema)
	assert.Equal(t, "object", mt418.Schema.Type)
	assert.Equal(t, "boolean", mt418.Schema.Properties["teapot"].Type)
	assert.Equal(t, map[string]any{"teapot": true}, mt418.Example)
}

func TestBuildOperationUsesMethodDefaultForEmptySuccessResponseSpecDescription(t *testing.T) {
	Reset()
	defer Reset()

	handler := &_DocstringTestHandler{}
	Register(handler, "DELETE", nil, nil,
		WithResponse("204", "",
			WithResponseHeader(HeaderDef{
				Name:        "X-Deleted",
				Description: "Deletion marker.",
				Example:     "true",
			}),
		),
	)

	doc := Build([]OperationCandidate{
		{
			Path:          "/items/{id}",
			Method:        "DELETE",
			Handler:       handler,
			HandlerMethod: "Delete",
		},
	}, nil, BuildOptions{})

	op := doc.Paths["/items/{id}"].Delete
	require.NotNil(t, op)
	resp204 := op.Responses["204"]
	require.NotNil(t, resp204)
	assert.Equal(t, "No Content", resp204.Description)
	require.Contains(t, resp204.Headers, "X-Deleted")
	assert.Equal(t, "Deletion marker.", resp204.Headers["X-Deleted"].Description)
	assert.Equal(t, "true", resp204.Headers["X-Deleted"].Example)
}

func _FindParameter(t *testing.T, params []*Parameter, in, name string) *Parameter {
	t.Helper()

	for _, p := range params {
		if p.In == in && p.Name == name {
			return p
		}
	}

	t.Fatalf("parameter %s:%s not found", in, name)
	return nil
}

func _CountParameters(params []*Parameter, in, name string) int {
	count := 0
	for _, p := range params {
		if p.In == in && p.Name == name {
			count++
		}
	}

	return count
}

func _DocstringPackageAST(t *testing.T) []*ast.File {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)

	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, filepath.Dir(file), nil, parser.ParseComments)
	require.NoError(t, err)
	require.Contains(t, pkgs, "goai")

	files := make([]*ast.File, 0, len(pkgs["goai"].Files))
	for _, file := range pkgs["goai"].Files {
		files = append(files, file)
	}

	return files
}

func _DocstringPackageASTBuilder(t *testing.T) *_ASTSchemaBuilder {
	t.Helper()

	_, path, _, ok := runtime.Caller(0)
	require.True(t, ok)

	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, filepath.Dir(path), nil, parser.ParseComments)
	require.NoError(t, err)
	require.Contains(t, pkgs, "goai")

	files := make([]*ast.File, 0, len(pkgs["goai"].Files))
	var primary *ast.File
	for filePath, file := range pkgs["goai"].Files {
		files = append(files, file)
		if filepath.Clean(filePath) == filepath.Clean(path) {
			primary = file
		}
	}
	require.NotNil(t, primary)

	return _NewASTSchemaBuilderWithPrimaryFile(
		files,
		"github.com/yetiz-org/gone/goai",
		filepath.Dir(path),
		nil,
		primary,
	)
}

func _DocstringPackageASTBuilderFromSource(t *testing.T, source string) *_ASTSchemaBuilder {
	t.Helper()

	return _ASTSchemaBuilderFromSource(t, "github.com/yetiz-org/gone/goai", source)
}

func _ASTSchemaBuilderFromSource(t *testing.T, pkgPath string, source string) *_ASTSchemaBuilder {
	t.Helper()

	_, path, _, ok := runtime.Caller(0)
	require.True(t, ok)

	fset := token.NewFileSet()
	primary, err := parser.ParseFile(fset, filepath.Join(filepath.Dir(path), "docstring_source_test.go"), source, parser.ParseComments)
	require.NoError(t, err)

	return _NewASTSchemaBuilderWithPrimaryFile(
		[]*ast.File{primary},
		pkgPath,
		filepath.Dir(path),
		nil,
		primary,
	)
}
