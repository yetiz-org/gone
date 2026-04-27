# goai — OpenAPI 3.0.3 generator for the gone framework

`goai` walks a `gone-framework` route tree and emits an OpenAPI 3.0.3 YAML
document that downstream tooling (Swagger UI, Redoc, Spectral, code
generators) can consume directly.

It is intentionally additive to `ghttp`: nothing in this package mutates
existing route or handler types, so adopting goai costs nothing for code
that does not register with it. Operations that *do* register with
`goai.Register` get fully fleshed-out request/response schemas; the rest
appear with auto-discovered paths and method shapes only.

---

## Why goai

| Existing pain                                                 | goai's answer                                                                   |
| ------------------------------------------------------------- | ------------------------------------------------------------------------------- |
| Hand-edited OpenAPI files drift from the route tree           | `Walk` reflects the live route tree → drift is detectable on every CI run.      |
| Code-first generators force handlers into a particular shape  | goai is **opt-in**; handlers stay vanilla `ghttp.HandlerTask` types.            |
| Generators clobber hand-tuned prose, examples, and `x-*` tags | `Merge3Way` keeps hand-tuned content; auto-discovered entries fill gaps only.   |
| Per-environment specs (public vs. internal) need rebuilds     | `Profile` + `Selector` scope the same route tree to multiple output buckets.   |

---

## OpenAPI 3.0.3 coverage

Every Object defined by [OpenAPI 3.0.3](https://spec.openapis.org/oas/v3.0.3)
has a Go counterpart in `openapi.go`:

| Spec object                 | Go type                  | Notes                                                                |
| --------------------------- | ------------------------ | -------------------------------------------------------------------- |
| OpenAPI Object              | `Document`               | Root, with `Extensions` for `x-*` keys.                              |
| Info Object                 | `Info`                   | Includes `termsOfService`, `contact`, `license`.                     |
| Contact Object              | `Contact`                |                                                                      |
| License Object              | `License`                |                                                                      |
| Server Object               | `Server`                 | Supports `variables`.                                                |
| Server Variable Object      | `ServerVariable`         |                                                                      |
| Components Object           | `Components`             | All nine sub-maps: schemas, responses, parameters, examples, requestBodies, headers, securitySchemes, links, callbacks. |
| Paths Object                | `map[string]*PathItem`   | Map keyed by URL template.                                            |
| Path Item Object            | `PathItem`               | Includes `$ref`, `servers`, all eight HTTP verbs.                    |
| Operation Object            | `Operation`              | Includes `externalDocs`, `callbacks`, `servers`.                     |
| External Documentation      | `ExternalDocumentation`  |                                                                      |
| Parameter Object            | `Parameter`              | Includes `deprecated`, `allowEmptyValue`, `explode`, `allowReserved`, `examples`, `content`. |
| Request Body Object         | `RequestBody`            |                                                                      |
| Media Type Object           | `MediaType`              | Includes multi-`examples` and `encoding`.                            |
| Encoding Object             | `Encoding`               |                                                                      |
| Responses Object            | `map[string]*Response`   |                                                                      |
| Response Object             | `Response`               | Includes `links`.                                                    |
| Callback Object             | `Callback`               | Modeled as `map[string]*PathItem`.                                   |
| Example Object              | `Example`                |                                                                      |
| Link Object                 | `Link`                   |                                                                      |
| Header Object               | `Header`                 | Mirrors the parameter shape (no `name`/`in`).                        |
| Tag Object                  | `Tag`                    | Includes `externalDocs`.                                             |
| Reference Object            | `$ref` field on Schema   |                                                                      |
| Schema Object               | `Schema`                 | Full validation surface — see [Schema reference](#schema-reference). |
| Discriminator Object        | `Discriminator`          |                                                                      |
| XML Object                  | `XML`                    |                                                                      |
| Security Scheme Object      | `SecurityScheme`         | apiKey, http, oauth2, openIdConnect.                                 |
| OAuth Flows Object          | `OAuthFlows`             | implicit, password, clientCredentials, authorizationCode.            |
| OAuth Flow Object           | `OAuthFlow`              |                                                                      |
| Security Requirement Object | `[]map[string][]string`  |                                                                      |
| **Specification Extensions** | `Extensions map[string]any` (inline yaml) | Available on every object that allows `x-*` per the spec. |

> If a downstream validator (Spectral, Redocly Lint) flags any spec field
> goai cannot emit, that is a bug — please open an issue.

---

## Installation

`goai` lives inside the `gone` module. Use it from any project that already
imports `gone`:

```go
import "github.com/yetiz-org/gone/goai"
```

There is no separate Go module. A CLI binary `goai` (in
[`cmd/goai`](./cmd/goai)) provides `emit`, `merge`, `lint`, and
`version` subcommands. Route walking still requires a project-side
binary that imports the project's handler package at compile time
(typically `cmd/goaispec`); `goai emit` is a convenience wrapper that
locates and runs it. See [Quick start](#quick-start) below.

---

## Quick start

### 1. Build a route tree

```go
route := ghttp.NewSimpleRoute()
route.SetEndpoint("/hello", &HelloHandler{})
route.SetEndpoint("/api/v1/me", &MeHandler{})
```

### 2. Generate the spec from a CLI

Add a `cmd/goaispec/main.go` to your project:

```go
package main

import (
    "github.com/yetiz-org/gone/ghttp"
    "github.com/yetiz-org/gone/goai"
    "myproject/handlers"
)

func main() {
    goai.RunCLI(
        func() ghttp.RouteEntriesProvider { return handlers.NewAppRoute() },
        goai.RunOptions{
            Title:         "My API",
            Version:       "1.0.0",
            DefaultOutput: "docs/openapi/openapi.generated.yaml",
            Servers: []goai.Server{
                {URL: "https://api.example.com"},
            },
        },
    )
}
```

Run it:

```bash
go run ./cmd/goaispec               # writes to DefaultOutput
go run ./cmd/goaispec -o /tmp/x.yaml
go run ./cmd/goaispec -o -          # writes to stdout
goai emit --root . -o /tmp/x.yaml   # delegates to ./cmd/goaispec
```

### 3. Serve the spec at runtime

```go
route.SetEndpoint("/openapi.yaml", goai.Handler(route,
    goai.WithRuntimeBuildOptions(goai.BuildOptions{
        Title:   "My API",
        Version: "1.0.0",
    }),
    goai.WithRuntimeCacheTTL(30*time.Second),
))
```

Now `GET /openapi.yaml` returns the live spec.

See the [`examples/`](./examples) directory for runnable code. Each
sub-directory is a self-contained `main.go`:

| Path                          | What it shows                                                       |
| ----------------------------- | ------------------------------------------------------------------- |
| `examples/quickstart/`        | Minimum viable pipeline: route → Walk → Build → EmitYAML.           |
| `examples/customschema/`      | `goai.Register` + struct tags (`goai:"..."`) for typed schemas.     |
| `examples/docstring/`         | Handler doc comments with `@goai.*` OpenAPI directives.              |
| `examples/full/`              | Every `RunOptions` field, including License, ServerVariables.       |
| `examples/runtime/`           | Mounting the runtime handler at `/openapi.yaml`.                    |
| `examples/merge/`             | Three-way merge with a hand-tuned baseline.                         |

Run any of them:

```bash
go run ./goai/examples/quickstart
go run ./goai/examples/docstring
go run ./goai/examples/full -o -
go run ./goai/examples/merge
```

---

## Core concepts

### The pipeline

```
route (RouteEntriesProvider)
        │
        ▼
   goai.Walk         → []OperationCandidate     (one per discovered HTTP op)
        │
        ▼
   goai.Build        → *Document                (filtered by optional Profile)
        │
        ▼
   goai.EmitYAML     → []byte                   (deterministic, 2-space indent)
        │
   (optional) goai.Merge3Way
        │
        ▼
        file / stdout / runtime response
```

`RunCLI` is a thin wrapper around the pipeline that adds flag parsing,
output handling, and three-way merge. Use it from project-side `main.go`
binaries; call the lower-level functions directly for non-CLI use cases
(tests, code generation, runtime serving).

### Walker rules

`goai.Walk` reflects on each handler to decide which operations to emit:

- A handler-method counts as **implemented** when its source location is
  *not* `<autogenerated>`. This catches both direct definitions on the
  leaf type and hand-written overrides of an embedded default — without
  forcing you to register every method explicitly.
- The walker emits one operation per (path, HTTP method).
- `Index` and `Get` distinguish collection vs. item endpoints: a handler
  that defines BOTH `Index` and at least one of `Get`/`Patch`/`Put`/
  `Delete` is "collection-style", and the item-level methods get an
  appended `/{id}` path. Handlers with only item-level methods (e.g. a
  `/me` resource that only defines `Get`) emit on the bare path.

### Acceptance-driven security and parameters

Acceptances on a route node can implement these optional interfaces:

- `goai.SecurityProvider` — declare an `(scheme, scopes)` requirement.
  The walker collects every provider on the route's acceptance chain and
  attaches them to each operation.
- `goai.MethodAwareSecurityProvider` — method-specific variant of
  `SecurityProvider`. Returning an empty scheme for one method explicitly
  suppresses inherited/global security for that operation.
- `goai.PathParamInjector` — declare path parameters this acceptance
  injects into the request (e.g. an `OrgScope` acceptance that prepends
  `/{organization_id}`).
- `goai.PathAwareParamInjector` — path-aware variant for acceptances reused
  under multiple route subtrees.
- `goai.EmissionAwareParamInjector` — receives `EmissionContext` so an
  injector can distinguish collection vs. item-level operations.
- `goai.ProfileScope` — declare which output profiles the operation
  belongs to (e.g. `mgmt-only`).
- `goai.ItemIDProvider` — let a handler override the generated item
  placeholder name instead of the default `<last-segment>_id`.
- `goai.Hidden` — drop framework-internal handlers from generated specs.

Use these when documenting cross-cutting middleware behavior. For
per-handler overrides, prefer `goai.Register` or `SpecProvider`.

### Per-handler customization

Two equivalent paths:

**`goai.Register` (recommended for most cases):**

```go
goai.Register(handler.HandlerAlbums, "Post",
    (*CreateAlbumRequest)(nil),
    (*CreateAlbumResponse)(nil),
    goai.WithSummary("Create a new album"),
    goai.WithTag("Album"),
    goai.WithExample("application/json", exampleBody),
    goai.WithSecurity("OAuth2", "albums:write"),
)
```

**`SpecProvider` interface (when registration is awkward, e.g. inside a
codegen-managed file):**

```go
// Catch-all: applies to every HTTP method on this handler.
func (h *MeHandler) GOAISpec() goai.Spec {
    return goai.NewSpec(
        goai.WithTag("User"),
        goai.WithSecurity("OAuth2", "openid"),
    )
}

// Per-method override: only the Get() method gets this Spec, overriding
// whatever GOAISpec returned. Method names mirror the handler struct's
// HTTP method functions (Get → GOAIGetSpec, Post → GOAIPostSpec, ...).
func (h *MeHandler) GOAIGetSpec() goai.Spec {
    return goai.NewSpec(
        goai.WithSummary("Get the current user"),
        goai.WithTag("User"),
        goai.WithSecurity("OAuth2", "openid"),
    )
}
```

Available per-method providers: `IndexSpecProvider`, `GetSpecProvider`,
`CreateSpecProvider`, `PostSpecProvider`, `PatchSpecProvider`,
`PutSpecProvider`, `DeleteSpecProvider`, `OptionsSpecProvider`,
`TraceSpecProvider`. Implement only the ones whose Go method the handler
exposes.

The walker prefers `Register`. The `SpecProvider` family runs only when no
registry entry exists for that handler+method. Per-method providers take
precedence over the catch-all `SpecProvider` when both are implemented.

### Doc-comment fallback

Use doc-comment fallback when endpoint metadata belongs beside the handler
but should still be source-controlled as OpenAPI. The extractor reads only
namespaced `@goai.*` directives from the handler struct and method Go doc
comments; all other comment text remains private implementation
documentation and never flows into the generated yaml.

```go
goai.RunCLI(factory, goai.RunOptions{
    OperationDocExtractor: goai.DefaultOperationDocExtractor(),
})
```

or in `goai.yaml`:

```yaml
enableDocstringExtraction: true
docstringBuildTags:
  - api
```

Directive format:

| Directive | Shape |
| --- | --- |
| `@goai.endpoint` | `METHOD /path` |
| `@goai.summary` | `summary text` |
| `@goai.description` | `description line`; repeat to append lines |
| `@goai.operationId` | `operation.id` |
| `@goai.tag` | `TagName`; repeat for multiple tags |
| `@goai.deprecated` | no arguments |
| `@goai.param` | `<in> <name> <type> <required\|optional> "description" [schemaType=GoStruct] [goType=GoStruct] [key=value...]` |
| `@goai.requestBody` | `<required\|optional> <mediaType> <type> "description" [schemaType=GoStruct] [goType=GoStruct] [key=value...]` |
| `@goai.response` | `<status> "description" [mediaType=...] [type=...] [schemaType=GoStruct] [goType=GoStruct] [schema=...] [example=...]` |
| `@goai.header` | `<status> <name> <type> "description" [required] [schemaType=GoStruct] [goType=GoStruct] [key=value...]` |
| `@goai.link` | `<status> <name> <operationId\|operationRef> "description" [parameters=...] [requestBody=...] [server=...] [x-...]` |
| `@goai.example` | `param <in> <name> <exampleName> "summary" <json-value>` |
| `@goai.example` | `request <mediaType> <exampleName> "summary" <json-value>` |
| `@goai.example` | `response <status> <mediaType> <exampleName> "summary" <json-value>` |
| `@goai.example` | `response <status> <mediaType> <exampleName> example={<Example Object>}` |
| `@goai.security` | `<scheme> [scope...]`, or `none` for explicit no security |
| `@goai.server` | `<url> "description"` |
| `@goai.externalDocs` | `<url> "description"` |
| `@goai.callback` | `<name> <compact-json-callback-object>` |
| `@goai.extension` | `<x-key> <json-value>` |
| `@goai.param.description` | `<in> <name> description line`; repeat to append lines |
| `@goai.requestBody.description` | `description line`; repeat to append lines |
| `@goai.response.description` | `<status> description line`; repeat to append lines |
| `@goai.header.description` | `<status> <name> description line`; repeat to append lines |
| `@goai.link.description` | `<status> <name> description line`; repeat to append lines |
| `@goai.example.description` | `param <in> <name> <exampleName> description line`; repeat to append lines |
| `@goai.example.description` | `request <mediaType> <exampleName> description line`; repeat to append lines |
| `@goai.example.description` | `response <status> <mediaType> <exampleName> description line`; repeat to append lines |
| `@goai.server.description` | `<url> description line`; repeat to append lines |
| `@goai.externalDocs.description` | `description line`; repeat to append lines |
| `@goai.schemaName` | Type doc comment only: stable `components.schemas` key for that struct |

For common scalar fields, use directive tokens. For the full OpenAPI object
surface, use compact JSON values on `schema=...`, `example=...`,
`@goai.callback`, and `@goai.extension`; JSON may contain quoted strings
with spaces, but each structured value must stay on one directive line.

Conventions:

- Only lines starting with `@goai.` are parsed.
- `@goai.endpoint` documents and validates intent; the walked route method
  and path remain authoritative.
- Put shared operation metadata such as tags and security on the handler
  struct doc comment; put `@goai.endpoint`, summary, operationId,
  parameters, request bodies, responses, examples, and per-method overrides
  on the handler method doc comment.
- Quote descriptions when they contain spaces.
- `type` may be a scalar OpenAPI schema type, `array:<itemType>`, or a
  `$ref`/`ref:` target. Use `schema={...}` for full Schema Object fields.
  Use `schemaType=SomeStruct` or `goType=SomeStruct` to reference a Go
  struct in the same package. Use `schemaType=packageAlias.SomeStruct` or
  `goType=packageAlias.SomeStruct` for an imported package; the alias must
  match the handler source import name. Type aliases that point to imported
  structs are resolved to the imported struct component. goai emits resolved
  structs under `components.schemas` and uses a `$ref`. Imported component
  names include the full import path, so same short names from different
  packages do not collapse into the wrong schema. Generic structs such as
  `Page[Item]` are supported. Explicit `schema={...}` wins over struct
  resolution.
- Structs can choose their public component key. For docstring extraction,
  put `@goai.schemaName PublicName` on the struct type doc comment. For
  reflection-based `WithResponseSchema`, implement `GOAISchemaName() string`
  on the struct type. Custom names are sanitized and still checked for
  collisions during the build.
- When docstring schema structs live behind custom Go build tags, pass those
  tags through `docstringBuildTags` in `goai.yaml` or use
  `DefaultOperationDocExtractorWithBuildTags("tag")`. The default extractor
  also honors `GOFLAGS=-tags=...`.
- `x-*` options on parameters, responses, headers, and request bodies become
  Specification Extensions.
- Explicit Spec values, route-derived path parameters, generated request /
  response schemas, and acceptance-derived security win. Doc-comment
  metadata only fills gaps.

Example:

```go
// Google keeps shared OpenAPI metadata for all Google auth operations.
//
// @goai.tag Auth
// @goai.security none
type Google struct { ... }

// Get handles Google OAuth callback implementation details.
// Keep cache, redirect, and retry notes here for maintainers only.
//
// @goai.endpoint GET /api/v1/auth/google
// @goai.summary Handle Google OAuth callback
// @goai.description Verifies the returned state token, exchanges the code with Google,
// @goai.description and redirects the browser back to the application.
// @goai.operationId auth.googleCallback
// @goai.param query code string required "Authorization code returned by Google."
// @goai.param query state string required "CSRF state token returned with the redirect."
// @goai.response 302 "Redirects to the application callback URL."
// @goai.response 400 "Invalid OAuth callback payload." mediaType=application/json schemaType=OAuthError
// @goai.example response 400 application/json invalidState example={"summary":"Invalid state","description":"State token failed validation.","value":{"error":"invalid_state"}}
func (h *Google) Get(...) ghttp.ErrorResponse { ... }
```

becomes:

```yaml
/api/v1/auth/google:
  get:
    summary: Handle Google OAuth callback
    description: |
      Verifies the returned state token, exchanges the code with Google,
      and redirects the browser back to the application.
    operationId: auth.googleCallback
    tags:
      - Auth
    parameters:
      - name: code
        in: query
        description: Authorization code returned by Google.
        required: true
        schema:
          type: string
      - name: state
        in: query
        description: CSRF state token returned with the redirect.
        required: true
        schema:
          type: string
    responses:
      "302":
        description: Redirects to the application callback URL.
      "400":
        description: Invalid OAuth callback payload.
    security: []
```

### Per-status responses

Spec carries a `responses` map for non-success status codes. Use
`WithResponse` for distinct status codes, with optional schema or example:

```go
goai.NewSpec(
    goai.WithSummary("Get user profile"),
    goai.WithSuccessDescription("User profile"),
    goai.WithResponse("400", "Invalid request",
        goai.WithResponseSchema(reflect.TypeOf((*ErrorEnvelope)(nil)))),
    goai.WithResponse("404", "User not found"),
)
```

The matching success-status entry merges with the operation's
auto-generated success response so a registered response Go-type is
preserved alongside the spec-supplied description / examples / headers.
`WithSuccessDescription` is a shorthand when the only customisation is
the success description.

### Profile filtering

`Profile` selects a subset of operations for a given output bucket. The
canonical use case is splitting one OpenAPI document into several
audience-specific outputs (e.g. public, internal, admin).

Operations are assigned to a profile by either:

1. Implementing `ProfileScope.GOAIProfileScope()` on the handler or an
   acceptance to declare profile names directly.
2. Configuring `Profile.Include` / `Profile.Exclude` Selectors that
   match path globs, package prefixes, or OpenAPI tags.

`goai` ships no built-in path → profile rules; project-specific path
conventions are configured by the caller.

Pass the `*Profile` to `Build` to filter the output.

### Config-driven multi-output

For projects that emit more than one OpenAPI document, keep generation
policy in `goai.yaml` and call `RunCLIFromConfig` from the project-side
binary. The route tree is walked once; each configured profile gets its
own `Build` + `EmitYAML` pass.

```go
func main() {
    goai.RunCLIFromConfig(
        "goai.yaml",
        func() ghttp.RouteEntriesProvider { return handlers.NewAppRoute() },
    )
}
```

Minimal `goai.yaml`:

```yaml
title: My API
version: 1.0.0
baseSpecPath: docs/openapi/openapi.yaml
restrictToBaseSpecPaths: false
excludePaths:
  - /static/**
  - /debug/**
enableDocstringExtraction: true
docstringBuildTags:
  - api

profiles:
  public:
    include:
      paths: ["/api/v1/**"]
      tags: ["Public"]
    exclude:
      paths: ["/api/v1/internal/**"]
  internal:
    include:
      profiles: ["internal"]
  all: {}

output:
  public: docs/openapi/openapi.public.yaml
  internal: docs/openapi/openapi.internal.yaml
  all: docs/openapi/openapi.yaml
```

Config fields map to the corresponding `RunOptions` / `BuildOptions`
fields: `title`, `description`, `version`, `termsOfService`, `contact`,
`license`, `externalDocs`, `servers`, `tags`, `security`,
`securitySchemes`, `defaultOutput`, `baseSpecPath`,
`restrictToBaseSpecPaths`, `excludePaths`, `profiles`, `output`,
`enableDocstringExtraction`, and `docstringBuildTags`.

When `profiles` / `output` are omitted, the default buckets are `public`,
`mgmt`, `internal`, and `all`, with output files
`openapi.public.yaml`, `openapi.mgmt.yaml`, `openapi.internal.yaml`, and
`openapi.yaml`. Selectors can match `paths`, `packages`, OpenAPI `tags`,
or declared `profiles` from `ProfileScope.GOAIProfileScope()`. Include
selectors combine non-empty fields with logical AND; exclude selectors
drop an operation when any field matches.

`restrictToBaseSpecPaths: true` turns a base spec into the endpoint
registry: walked routes absent from `baseSpecPath` are excluded. This is
useful when endpoints should not become public documentation until the
curated OpenAPI file already lists them. `excludePaths` is a framework
blocklist applied before per-profile filtering.

### Three-way merge

`goai.Merge3Way(generated, existing, nil)` overlays the generator
output onto a hand-tuned baseline:

| Section                          | Merge policy                                                                      |
| -------------------------------- | --------------------------------------------------------------------------------- |
| `info`, `servers`, `security`, `externalDocs` | Existing wins verbatim.                                              |
| `tags`                           | Union — existing order preserved, generated tags missing from existing appended.  |
| `paths`                          | Union by path key — existing path entries kept verbatim; new paths appended.      |
| `components.*` (every sub-map)   | Union by name — existing entries kept; generated entries fill gaps only.          |

This is the canonical pattern when the project keeps a curated
`docs/openapi/openapi.yaml` as the source of truth (rich descriptions,
examples, `x-*` extensions) and wants the generator to *augment* without
overwriting hand-tuned prose. See [`examples/merge`](./examples/merge).

---

## API reference

### Top-level types

| Type             | Purpose                                                               |
| ---------------- | --------------------------------------------------------------------- |
| `RouteFactory`   | `func() ghttp.RouteEntriesProvider`. Used by `RunCLI`.                |
| `RunOptions`     | Project-side configuration for `RunCLI`.                              |
| `BuildOptions`   | Lower-level configuration for `Build`. Used by `RunCLI` and `Handler`. |
| `Config`         | `goai.yaml` schema used by `RunCLIFromConfig`.                         |
| `ConfigProfile`  | On-disk include/exclude selector pair for one profile.                 |
| `Spec`           | Per-operation metadata produced by Options. Immutable.                |
| `Option`         | `func(*Spec)`. Mutates a Spec during `NewSpec` / `Register`.          |
| `ResponseSpec`   | Per-status response metadata declared through `WithResponse`.          |
| `ResponseOption` | `func(*ResponseSpec)`. Mutates a `ResponseSpec`.                       |
| `Profile`        | Named output bucket with Include/Exclude selectors.                   |
| `Selector`       | Path / package / tag / profile rule set.                              |
| `Classifier`     | Path → tag and acceptance-typename → security mapping.                |
| `OperationDoc`   | Parsed `@goai.*` metadata for one handler method.                      |
| `OperationDocExtractor` | Hook used by Build/RunCLI for doc-comment fallback.            |
| `LintReport`     | Result from `LintYAML`, including errors and path/operation counts.    |

### Functional Options

| Option                                  | Effect                                                                  |
| --------------------------------------- | ----------------------------------------------------------------------- |
| `WithSummary(s)`                        | Sets `summary`.                                                         |
| `WithDescription(s)`                    | Sets `description`.                                                     |
| `WithOperationID(s)`                    | Sets `operationId` (defaults to `package + handler + method`).          |
| `WithTag(tags...)`                      | Appends to `tags`.                                                      |
| `WithExample(mediaType, value)`         | Single example for a media-type.                                        |
| `WithExampleObject(mediaType, name, *Example)` | Named Example object — supports summary/externalValue.            |
| `WithRequestMediaType(mt)`              | Override the default `application/json` for the request body.           |
| `WithRequestDescription(s)`             | Sets `requestBody.description`.                                         |
| `WithSuccessDescription(s)`             | Overrides the conventional success response description.                |
| `WithResponse(status, description, opts...)` | Adds or customizes a response status, including `default`.        |
| `WithResponseSchema(type)`              | Response option: build schema from a Go type.                           |
| `WithResponseSchemaPrebuilt(schema)`    | Response option: use a prebuilt schema verbatim.                        |
| `WithResponseMediaType(mt)`             | Response option: override response media type.                          |
| `WithResponseExample(value)`            | Response option: attach a single inline example.                        |
| `WithResponseExampleObject(name, ex)`   | Response option: attach a named Example object.                         |
| `WithResponseHeader(HeaderDef)`         | Response option: attach a response header to that status.               |
| `WithDeprecated()`                      | Marks the operation deprecated.                                         |
| `WithSecurity(scheme, scopes...)`       | Appends a security requirement.                                         |
| `WithHeader(HeaderDef)`                 | Documents a request or response header.                                 |
| `WithParam(PathParam)`                  | Documents an extra path/query/header parameter.                         |
| `WithExternalDocs(url, description)`    | Operation-level externalDocs.                                           |
| `WithCallback(name, Callback)`          | Operation-level callback under `callbacks.<name>`.                      |
| `WithOperationServer(Server)`           | Adds an operation-level server override.                                |

### Schema reference

Validation constraints can be declared via the `goai` struct tag. Tokens
are separated by `;`, key/value pairs by `=`. Unknown keys are silently
ignored.

```go
type CreateAlbumRequest struct {
    Title       string   `json:"title" goai:"description=The album title;example=Greatest Hits;minLength=1;maxLength=200"`
    ReleaseYear int      `json:"release_year" goai:"description=Year of release;minimum=1900;maximum=2100"`
    Genres      []string `json:"genres,omitempty" goai:"minItems=1;uniqueItems=true"`
}
```

| Tag key            | Schema field         | Type                | Notes                                                  |
| ------------------ | -------------------- | ------------------- | ------------------------------------------------------ |
| `title`            | `title`              | string              |                                                        |
| `description`,`desc` | `description`      | string              |                                                        |
| `example`          | `example`            | string              |                                                        |
| `default`          | `default`            | string              |                                                        |
| `format`           | `format`             | string              | `date-time`, `uuid`, `email`, ...                      |
| `enum`             | `enum`               | comma-separated     | `enum=red,green,blue`.                                 |
| `pattern`          | `pattern`            | regex string        |                                                        |
| `minLength`,`maxLength` | string bounds   | unsigned int        |                                                        |
| `minimum`,`maximum`     | numeric bounds  | float               |                                                        |
| `exclusiveMinimum`,`exclusiveMaximum` | bool flags | `=true` / bare key |                                                |
| `multipleOf`       | numeric multiple     | float               |                                                        |
| `minItems`,`maxItems` | array bounds      | unsigned int        |                                                        |
| `uniqueItems`      | array uniqueness     | bool                |                                                        |
| `minProperties`,`maxProperties` | object bounds | unsigned int   |                                                        |
| `nullable`         | OAS-3.0 nullable     | bool                | Pointer types are auto-nullable; tag overrides.        |
| `deprecated`       | property deprecation | bool                |                                                        |
| `readOnly`,`writeOnly` | request/response visibility | bool        |                                                        |

The reflection layer also recognises:

- `*T` → `nullable: true`.
- `time.Time` → `string`/`date-time`.
- `[]byte` → `string`/`byte` (base64-encoded).
- Embedded structs are flattened.
- `json:"-"` is honored (field skipped).
- `json:"name,omitempty"` controls the property name and required flag.

### CLI flags (RunCLI)

| Flag      | Default                  | Effect                                                |
| --------- | ------------------------ | ----------------------------------------------------- |
| `-o`      | `RunOptions.DefaultOutput` | Output path. `-` writes to stdout.                  |
| `-title` | `RunOptions.Title`       | Override `info.title`.                                |
| `-version` | `RunOptions.Version`   | Override `info.version`.                              |

Tests can drive `RunCLI` without spawning a subprocess by supplying
`Args`, `Stdout`, `Stderr`, and `Exit` on `RunOptions`.

### CLI subcommands

The standalone `goai` binary is a thin project helper plus two pure file
utilities:

| Command | Effect |
| ------- | ------ |
| `goai emit --root <dir> [-o <file>] [-args ...]` | Finds `<root>/cmd/goaispec`, `<root>/cmd/goaiprobe`, or `<root>/goaispec`, then runs `go run` against the first match. `-o` becomes the delegate binary's `-o`; anything after `-args` is forwarded verbatim. |
| `goai merge --generated <file> --existing <file> [-o <file>]` | Runs `Merge3Way` without walking routes. Existing hand-tuned content wins on overlap. `-o -` writes to stdout. |
| `goai lint <file>` | Runs `LintYAML`: yaml parse, OpenAPI 3.x, non-empty `info.title`/`info.version`, and at least one valid path mapping. |
| `goai version` | Prints the bundled package version. |
| `goai help` | Prints CLI usage. |

Examples:

```bash
goai emit --root . -o docs/openapi.generated.yaml -args -title "My API"
goai merge --generated docs/openapi.generated.yaml --existing docs/openapi.yaml -o docs/openapi.merged.yaml
goai lint docs/openapi.yaml
goai version
```

### Runtime handler

`goai.Handler(route, opts...)` returns a `ghttp.HandlerTask` that responds
to GET with the generated YAML.

| Option                           | Effect                                                       |
| -------------------------------- | ------------------------------------------------------------ |
| `WithRuntimeProfile(name)`       | Pick which profile to emit. Default `"all"`.                 |
| `WithRuntimeBuildOptions(opts)`  | Override `BuildOptions` (Title, Servers, Tags, ...).         |
| `WithRuntimeCacheTTL(d)`         | Cache the bytes for `d` between rebuilds. `0` disables cache. |

Mount it like any other `ghttp` endpoint:

```go
route.SetEndpoint("/openapi.yaml", goai.Handler(route))
```

---

## Recipes

### Add per-environment specs

```go
profile := goai.Profile{
    Name:    "internal",
    Include: goai.Selector{Paths: []string{"/api/v1/internal/**", "/api/v1/webhooks/**"}},
}

candidates := goai.Walk(route)
doc := goai.Build(candidates, &profile, goai.BuildOptions{
    Title:   "Internal API",
    Version: "1.0.0",
})
```

### Mark an endpoint deprecated

```go
goai.Register(archivedHandler, "Get",
    nil, (*ArchivedResponse)(nil),
    goai.WithDeprecated(),
    goai.WithDescription("Deprecated. Use /v2/foo instead."),
)
```

### Document a callback

```go
goai.Register(orderHandler, "Post",
    (*PlaceOrderRequest)(nil), (*PlaceOrderResponse)(nil),
    goai.WithCallback("orderShipped", goai.Callback{
        "{$request.body#/callbackUrl}": &goai.PathItem{
            Post: &goai.Operation{
                Summary:  "Server-to-callback URL when the order ships.",
                Responses: map[string]*goai.Response{
                    "200": {Description: "Acknowledged"},
                },
            },
        },
    }),
)
```

### Layer hand-tuned content

Keep the curated `docs/openapi/openapi.yaml` checked in. Have the
generator merge onto it instead of overwriting:

```go
goai.RunCLI(
    factory,
    goai.RunOptions{
        DefaultOutput: "docs/openapi/openapi.generated.yaml",
        BaseSpecPath:  "docs/openapi/openapi.yaml",   // ← merge target
        // ... other options
    },
)
```

---

## Logging

goai uses the project standard `kklogger` for any runtime warnings
(e.g. `Register` called with a nil handler). Logger names follow the
pattern `goai:Struct.Method#section!action`.

---

## Scope

- The schema builder reads only the goai-specific `goai:"..."` tag; it
  does not consult `validate:"..."` tags from `go-playground/validator`.

---

## License

Same license as the parent `gone` module. See the repository root.
