---
name: goai-docstring
description: Use when documenting gone framework Go handlers, handler receiver methods, request or response DTO structs, or OpenAPI metadata with goai doc comments, @goai directives, goai struct tags, schemaType, requestBody, response, param, and generated spec checks.
---

# Goai Docstring

## Overview

goai docstrings are OpenAPI metadata embedded in Go comments. Only `@goai.*` directives are parsed; ordinary comments remain private source documentation. The default extractor reads handler receiver method comments plus the handler type doc; package-level functions are not parsed. Do not use swaggo annotations such as `@Summary`, `@Param`, `@Success`, `@Failure`, `@Router`, `@Accept`, or `@Produce`.

## When to Use

- Adding or reviewing OpenAPI docs for `ghttp` handlers.
- Writing handler struct comments, handler receiver method comments, DTO structs, or `goai:"..."` schema tags.
- Checking generated `goai` YAML for route, schema, request, response, parameter, tag, security, or example drift.

Do not use when the project documents operations through `goai.Register` or `SpecProvider` only.

## Placement

| Location | Put Here |
| --- | --- |
| Handler struct doc | Shared `@goai.tag`, `@goai.security`, `@goai.server`, `@goai.externalDocs`. |
| Handler receiver method doc | `@goai.endpoint`, summary, description, operationId, param, requestBody, response, examples, links, callbacks, and endpoint-scoped overrides. |
| DTO struct doc | Go type comment plus optional `@goai.schemaName PublicComponentName` or `@goai.componentName PublicComponentName`. |
| DTO fields | `json` tags and `goai:"description=...;example=...;minLength=...;maximum=..."` schema metadata. |

Route walking remains authoritative. `@goai.endpoint METHOD /path` documents and validates intent, but it does not replace the actual route tree. When one handler method is mounted on multiple paths, repeat `@goai.endpoint` for every documented route and use `@goai.endpoint.<directive> METHOD /path ...` for metadata that belongs to only one of those declared endpoints. Endpoint-scoped directives do not declare routes by themselves.

## Quick Reference

| Need | Directive |
| --- | --- |
| Operation | `@goai.endpoint POST /v1/invoices` |
| Endpoint-scoped override | `@goai.endpoint.param GET /v1/orgs/{orgs_id}/songs path orgs_id string required "Organization ID."` |
| Summary | `@goai.summary Create an invoice` |
| Description | repeat `@goai.description ...` lines |
| Query/path/header param | `@goai.param query limit integer optional "Page size." default=25` |
| JSON request body | `@goai.requestBody required application/json object "Invoice payload." schemaType=CreateInvoiceRequest` |
| JSON response | `@goai.response 201 "Invoice created." mediaType=application/json schemaType=CreateInvoiceResponse` |
| Error response | `@goai.response 400 "Invalid invoice payload." mediaType=application/json schemaType=ErrorResponse` |
| Header | `@goai.header 201 X-Request-ID string "Request trace ID." required` |
| Example | `@goai.example response 201 application/json created example={"summary":"Created","value":{"id":"inv_123"}}` |
| Security | `@goai.security OAuth2 invoices:write` or `@goai.security none` |
| Link/callback/extension | `@goai.link ...`, `@goai.callback name {...}`, `@goai.extension x-key {...}` |
| Long descriptions | repeat `@goai.requestBody.description`, `@goai.response.description`, `@goai.header.description`, `@goai.link.description`, `@goai.example.description`, `@goai.server.description`, `@goai.externalDocs.description` |

Use `schemaType=SomeStruct` for local DTOs and `schemaType=alias.SomeStruct` for imported DTOs. Generics such as `Page[Item]` and `alias.Page[alias.Item]` are supported. The alias must match the handler file import alias. Explicit `schema={...}` wins over struct resolution. Use compact one-line JSON for `schema=...`, `example=...`, callbacks, and extensions.

## Multiple Endpoints

Use repeated `@goai.endpoint` lines when the same method serves multiple API paths. Put shared metadata on the normal directives and path-specific metadata on `@goai.endpoint.<directive>`.

```go
// List returns songs.
//
// @goai.endpoint GET /api/v1/organizations/{organizations_id}/albums/songs
// @goai.endpoint GET /mgmt/v1/songs
// @goai.summary 列出歌曲
// @goai.description 依照可存取範圍回傳歌曲清單。
// @goai.param query limit integer optional "Page size."
// @goai.endpoint.param GET /api/v1/organizations/{organizations_id}/albums/songs path organizations_id string required "Organization ID."
// @goai.response 200 "OK" mediaType=application/json schemaType=SongListResponse
func (h *SongsHandler) Index(ctx channel.HandlerContext, req *ghttp.Request, resp *ghttp.Response, params map[string]any) ghttp.ErrorResponse {
	return nil
}
```

In this example `organizations_id` is emitted only for `/api/v1/organizations/{organizations_id}/albums/songs`; it must not appear on `/mgmt/v1/songs`. For Soundrise docs, write `summary` and `description` in Traditional Chinese and keep HTTP status descriptions such as `OK`, `Created`, `Bad Request`, and `Unauthorized` in English.

## Struct Tags

Supported `goai:"..."` keys: `title`, `description`/`desc`, `example`, `default`, `format`, `enum`, `pattern`, `minLength`, `maxLength`, `minimum`, `maximum`, `exclusiveMinimum`, `exclusiveMaximum`, `multipleOf`, `minItems`, `maxItems`, `uniqueItems`, `minProperties`, `maxProperties`, `readOnly`, `writeOnly`, `deprecated`, `nullable`.

Prefer adding `example=...` to DTO fields whenever the OpenAPI contract has a clear representative value. Struct tag examples are parsed by the generated schema type: string values stay strings, integers become JSON numbers, number fields become JSON numbers, booleans become JSON booleans, and array/object examples must be compact JSON.

Unknown keys are silently ignored, so typo checks must inspect generated YAML, not just Go source.

## Example

```go
// CreateInvoiceRequest is the request body for POST /v1/invoices.
//
// @goai.schemaName CreateInvoiceRequest
type CreateInvoiceRequest struct {
	CustomerID  string `json:"customer_id" goai:"description=Stable customer identifier;example=cus_123;minLength=1"`
	AmountCents int    `json:"amount_cents" goai:"description=Invoice amount in cents;example=1200;minimum=1"`
	CallbackURL string `json:"callback_url,omitempty" goai:"description=Webhook URL called after creation;example=https://example.com/callback;format=uri"`
}

// CreateInvoiceResponse is returned after an invoice is created.
type CreateInvoiceResponse struct {
	ID     string `json:"id" goai:"description=Created invoice identifier;example=inv_123"`
	Status string `json:"status" goai:"description=Initial invoice status;enum=pending,paid,void"`
}

// BillingHandler keeps shared OpenAPI metadata for billing operations.
//
// @goai.tag Billing
// @goai.security OAuth2 invoices:write
type BillingHandler struct {
	ghttp.DefaultHTTPHandlerTask
}

// Create creates an invoice for a customer.
//
// @goai.endpoint POST /v1/invoices
// @goai.summary Create an invoice
// @goai.description Creates an invoice and optionally registers a creation callback.
// @goai.operationId billing.createInvoice
// @goai.requestBody required application/json object "Invoice payload." schemaType=CreateInvoiceRequest
// @goai.response 201 "Invoice created." mediaType=application/json schemaType=CreateInvoiceResponse
// @goai.response 400 "Invalid invoice payload." mediaType=application/json schemaType=ErrorResponse
// @goai.example response 201 application/json created example={"summary":"Created","value":{"id":"inv_123","status":"pending"}}
func (h *BillingHandler) Create(ctx channel.HandlerContext, req *ghttp.Request, resp *ghttp.Response, params map[string]any) ghttp.ErrorResponse {
	return nil
}
```

## Check Before Completing

- Every parsed line starts with `@goai.`; no swaggo annotation remains.
- Shared metadata is on the handler struct; endpoint-specific metadata is on the method.
- Every mounted route that should receive method-level docs has its own `@goai.endpoint METHOD /path` line.
- Every `@goai.endpoint.<directive>` repeats the exact method and path from a declared `@goai.endpoint` line.
- Each request/response body references the full DTO with `schemaType` or explicit `schema`, not a single JSON field as a body param.
- Path params in `@goai.param path ... required` match the route template names.
- Path params that exist on only one mounted endpoint use `@goai.endpoint.param`, not shared `@goai.param`.
- DTO fields have correct `json` tags; optional fields use `omitempty` only when optional in the API.
- DTO field constraints use supported `goai:"..."` keys only; add representative examples where useful, and remember unknown keys are ignored.
- Docstring extraction is enabled with `OperationDocExtractor: goai.DefaultOperationDocExtractor()` or `enableDocstringExtraction: true` in `goai.yaml`.
- Generate the spec, then inspect YAML for path, method, operationId, tags, security, requestBody, responses, examples, and component schema names.

## Verification

For this repo's smoke test:

```bash
go run ./goai/examples/docstring > /tmp/goai-docstring.yaml
rg -n "operationId: books.list|BookPage|requestBody|responses:" /tmp/goai-docstring.yaml
```

For a project handler, run its goai generator, for example `go run ./cmd/goaispec -o /tmp/openapi.yaml` or `goai emit --root . -o /tmp/openapi.yaml`, then inspect the exact generated operation and component schema.

## Common Mistakes

| Mistake | Fix |
| --- | --- |
| Writing `@Summary`, `@Param`, `@Router`, `@Success`. | Replace with `@goai.summary`, `@goai.param`, `@goai.endpoint`, `@goai.response`. |
| Documenting a JSON body field as `@goai.param body customer_id ...`. | Use one `@goai.requestBody ... schemaType=RequestDTO`. |
| Putting all directives on the method. | Move shared tag/security/server docs to the handler struct. |
| Using `@goai.endpoint.param` without a matching `@goai.endpoint` line. | Add the endpoint declaration; scoped directives are overrides, not route declarations. |
| Putting a path param shared across one handler method but absent from some mounted routes on `@goai.param`. | Move it to `@goai.endpoint.param METHOD /path ...` for only the route that has that path variable. |
| Relying on Go field comments for schema descriptions. | Add `goai:"description=..."` tags to DTO fields. |
| Placing docs on package-level helper functions. | Put parsed directives on the handler receiver method. |
| Misspelling `goai` tag keys. | Check generated YAML because unknown keys are ignored. |
| Assuming comments changed routing or status codes. | Check actual route registration and handler response behavior. |
