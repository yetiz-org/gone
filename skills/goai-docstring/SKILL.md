---
name: goai-docstring
description: Use when documenting gone framework Go handlers, handler methods or functions, request or response DTO structs, or OpenAPI metadata with goai doc comments, @goai directives, goai struct tags, schemaType, requestBody, response, param, and generated spec checks.
---

# Goai Docstring

## Overview

goai docstrings are OpenAPI metadata embedded in Go comments. Only `@goai.*` directives are parsed; ordinary comments remain private source documentation. Do not use swaggo annotations such as `@Summary`, `@Param`, `@Success`, `@Failure`, `@Router`, `@Accept`, or `@Produce`.

## When to Use

- Adding or reviewing OpenAPI docs for `ghttp` handlers.
- Writing handler struct comments, handler method/function comments, DTO structs, or `goai:"..."` schema tags.
- Checking generated `goai` YAML for route, schema, request, response, parameter, tag, security, or example drift.

Do not use when the project documents operations through `goai.Register` or `SpecProvider` only.

## Placement

| Location | Put Here |
| --- | --- |
| Handler struct doc | Shared `@goai.tag`, `@goai.security`, `@goai.server`, `@goai.externalDocs`. |
| Handler method/function doc | Endpoint-specific `@goai.endpoint`, `summary`, `description`, `operationId`, `param`, `requestBody`, `response`, examples, links, callbacks. |
| DTO struct doc | Go type comment plus optional `@goai.schemaName PublicComponentName`. |
| DTO fields | `json` tags and `goai:"description=...;example=...;minLength=...;maximum=..."` schema metadata. |

Route walking remains authoritative. `@goai.endpoint METHOD /path` documents and validates intent, but it does not replace the actual route tree.

## Quick Reference

| Need | Directive |
| --- | --- |
| Operation | `@goai.endpoint POST /v1/invoices` |
| Summary | `@goai.summary Create an invoice` |
| Description | repeat `@goai.description ...` lines |
| Query/path/header param | `@goai.param query limit integer optional "Page size." default=25` |
| JSON request body | `@goai.requestBody required application/json object "Invoice payload." schemaType=CreateInvoiceRequest` |
| JSON response | `@goai.response 201 "Invoice created." mediaType=application/json schemaType=CreateInvoiceResponse` |
| Error response | `@goai.response 400 "Invalid invoice payload." mediaType=application/json schemaType=ErrorResponse` |
| Example | `@goai.example response 201 application/json created example={"summary":"Created","value":{"id":"inv_123"}}` |
| Security | `@goai.security OAuth2 invoices:write` or `@goai.security none` |

Use `schemaType=SomeStruct` for local DTOs and `schemaType=alias.SomeStruct` for imported DTOs. The alias must match the handler file import alias. Use compact one-line JSON for `schema=...`, `example=...`, callbacks, and extensions.

## Example

```go
// CreateInvoiceRequest is the request body for POST /v1/invoices.
//
// @goai.schemaName CreateInvoiceRequest
type CreateInvoiceRequest struct {
	CustomerID  string `json:"customer_id" goai:"description=Stable customer identifier;example=cus_123;minLength=1"`
	AmountCents int    `json:"amount_cents" goai:"description=Invoice amount in cents;minimum=1"`
	CallbackURL string `json:"callback_url,omitempty" goai:"description=Webhook URL called after creation;format=uri"`
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
- Each request/response body references the full DTO with `schemaType` or explicit `schema`, not a single JSON field as a body param.
- Path params in `@goai.param path ... required` match the route template names.
- DTO fields have correct `json` tags; optional fields use `omitempty` only when optional in the API.
- DTO field constraints live in `goai:"..."` tags using supported keys: `description`, `example`, `format`, `enum`, `minLength`, `maxLength`, `minimum`, `maximum`, `uniqueItems`, `nullable`, `deprecated`, `readOnly`, `writeOnly`.
- Generate the spec with docstring extraction enabled, then inspect YAML for path, method, operationId, tags, security, requestBody, responses, examples, and component schema names.

## Common Mistakes

| Mistake | Fix |
| --- | --- |
| Writing `@Summary`, `@Param`, `@Router`, `@Success`. | Replace with `@goai.summary`, `@goai.param`, `@goai.endpoint`, `@goai.response`. |
| Documenting a JSON body field as `@goai.param body customer_id ...`. | Use one `@goai.requestBody ... schemaType=RequestDTO`. |
| Putting all directives on the method. | Move shared tag/security/server docs to the handler struct. |
| Relying on Go field comments for schema descriptions. | Add `goai:"description=..."` tags to DTO fields. |
| Assuming comments changed routing or status codes. | Check actual route registration and handler response behavior. |
