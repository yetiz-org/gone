// Package goai generates OpenAPI 3.0.3 documents from a gone-framework
// route tree without forcing handlers into a special shape.
//
// goai is intentionally additive to ghttp: nothing in this package mutates
// existing route or handler types. Operations registered via goai.Register
// receive fully fleshed-out request/response schemas; the rest appear with
// auto-discovered paths and method shapes only.
//
// # Optional handler/acceptance interfaces
//
// Implement these to refine the generated spec without touching goai's API:
//
//   - SpecProvider       — supplies one Spec applied to every HTTP method on
//     the handler. Use for the common case where all
//     methods share metadata.
//   - <Method>SpecProvider — per-Go-method Spec providers (IndexSpecProvider,
//     GetSpecProvider, HeadSpecProvider, CreateSpecProvider, PostSpecProvider,
//     PatchSpecProvider, PutSpecProvider, DeleteSpecProvider,
//     OptionsSpecProvider, TraceSpecProvider). Each method
//     name mirrors the handler's Go method on the struct
//     (Get → GOAIGetSpec, Post → GOAIPostSpec, ...).
//     Per-method providers override SpecProvider when both
//     are implemented.
//   - SecurityProvider   — declares required security scheme + scopes.
//   - PathParamInjector  — declares which path parameters this acceptance
//     injects into the request (e.g. {organization_id}).
//   - ProfileScope       — declares which output profiles this node belongs
//     to (e.g. mgmt-only).
//
// # Quick start
//
// From a project's bootstrap:
//
//	goai.Register(myhandler.HandlerMe, "Get",
//	    nil, (*myhandler.MeGetResponse)(nil),
//	    goai.WithTag("User"),
//	    goai.WithDescription("Returns the current authenticated user."))
//
// To serve the spec at runtime:
//
//	route.SetEndpoint("/openapi.yaml", goai.Handler(&route))
//
// To generate a spec file from a CLI binary:
//
//	goai.RunCLI(factory, goai.RunOptions{
//	    Title:         "My API",
//	    Version:       "1.0.0",
//	    DefaultOutput: "docs/openapi.yaml",
//	})
//
// # OpenAPI 3.0.3 coverage
//
// All Object types defined by the OpenAPI 3.0.3 specification are modeled
// in openapi.go — including License, ServerVariable, ExternalDocumentation,
// Encoding, Example, Link, Callback, Discriminator, and XML. Specification
// Extensions ("x-*" fields) are supported on every applicable object via
// the inline `Extensions map[string]any` field.
//
// # Examples
//
// Each subdirectory under `examples/` is a runnable program demonstrating
// one feature: quickstart, customschema, docstring, full, runtime, merge.
//
// See README.md for the full reference.
package goai

// Version reports the goai package version. It is bumped manually on
// every release of github.com/yetiz-org/gone that contains visible goai
// changes.
const Version = "0.2.0"
