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
| `examples/full/`              | Every `RunOptions` field, including License, ServerVariables.       |
| `examples/runtime/`           | Mounting the runtime handler at `/openapi.yaml`.                    |
| `examples/merge/`             | Three-way merge with a hand-tuned baseline.                         |

Run any of them:

```bash
go run ./goai/examples/quickstart
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
- `goai.PathParamInjector` — declare path parameters this acceptance
  injects into the request (e.g. an `OrgScope` acceptance that prepends
  `/{organization_id}`).
- `goai.ProfileScope` — declare which output profiles the operation
  belongs to (e.g. `mgmt-only`).

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

### Three-way merge

`goai.Merge3Way(generated, existing, overrides)` overlays the generator
output onto a hand-tuned baseline:

| Section                          | Merge policy                                                                      |
| -------------------------------- | --------------------------------------------------------------------------------- |
| `info`, `servers`, `security`, `externalDocs` | Existing wins verbatim.                                              |
| `tags`                           | Union — existing order preserved, generated tags missing from existing appended.  |
| `paths`                          | Union by path key — existing path entries kept verbatim; new paths appended.      |
| `components.*` (every sub-map)   | Union by name — existing entries kept; generated entries fill gaps only.          |

The `overrides` parameter is reserved for a future explicit-override
layer and is currently ignored.

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
| `Spec`           | Per-operation metadata produced by Options. Immutable.                |
| `Option`         | `func(*Spec)`. Mutates a Spec during `NewSpec` / `Register`.          |
| `Profile`        | Named output bucket with Include/Exclude selectors.                   |
| `Selector`       | Path / package / tag rule set.                                        |
| `Classifier`     | Path → tag and acceptance-typename → security mapping.                |

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
goai.Register(legacyHandler, "Get",
    nil, (*LegacyResponse)(nil),
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

## Limitations

- `goai gen` (codegen of `zz_goai_init.go` files) is unimplemented; use
  manual `goai.Register` calls in a project-side `cmd/goaispec` binary
  invoked through `goai emit` (or directly via `goai.RunCLI`).
- `Merge3Way`'s `overrides` parameter is accepted but ignored.
- The schema builder reads only the goai-specific `goai:"..."` tag; it
  does not consult `validate:"..."` tags from `go-playground/validator`.

---

## License

Same license as the parent `gone` module. See the repository root.
