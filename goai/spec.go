package goai

import "reflect"

// Spec carries optional metadata for a single (handler, method) operation.
// All fields are deliberately unexported; callers shape Spec values through
// functional options exclusively. This keeps the public API forward
// compatible — new fields can be added without breaking call sites.
type Spec struct {
	summary            string
	description        string
	operationID        string
	tags               []string
	examples           map[string]any
	multiExamples      map[string]map[string]*Example
	deprecated         bool
	security           []SecurityRef
	extraHeaders       []HeaderDef
	extraParams        []PathParam
	externalDocs       *ExternalDocumentation
	callbacks          map[string]Callback
	servers            []Server
	requestMediaType   string
	requestDescription string
	successDescription string
	responses          map[string]*ResponseSpec
}

// ResponseSpec describes a single status-code response declared on an
// operation via WithResponse. All fields are optional; Description is the
// only one builder treats as required by OpenAPI semantics. When empty,
// success responses use the method default, recognised HTTP status codes
// use net/http.StatusText, "default" uses "Default response", and unknown
// status keys fall back to "OK" so the emitted yaml stays valid.
type ResponseSpec struct {
	// Description is rendered verbatim under responses.<status>.description.
	Description string

	// SchemaType, when non-nil, drives schema synthesis from a Go type
	// (the same path used for request bodies and successful response
	// bodies registered via goai.Register). Mutually exclusive with
	// Schema; Schema wins when both are set.
	SchemaType reflect.Type

	// Schema, when non-nil, is used directly without further synthesis.
	// Use this for hand-tuned schemas, $ref-based reuse, or to attach
	// schemas the schema builder cannot derive (e.g. raw JSON Schema
	// objects).
	Schema *Schema

	// MediaType overrides the response content media type. Empty falls
	// back to "application/json" when Schema or SchemaType is set; when
	// both Schema and SchemaType are nil the response carries no
	// content block regardless of MediaType.
	MediaType string

	// Example is a single inline example. Use Examples when richer
	// metadata (named summaries / descriptions / external values) is
	// needed.
	Example any

	// Examples is the per-name Example map. Mutually compatible with
	// Example — yaml encoders emit both fields.
	Examples map[string]*Example

	// Headers documents response headers attached to this status code.
	Headers []HeaderDef
}

// ResponseOption mutates a ResponseSpec built by WithResponse.
type ResponseOption func(*ResponseSpec)

// Option mutates a Spec during NewSpec or Register.
type Option func(*Spec)

// SecurityRef binds an operation to a named security scheme with optional
// scopes (used by OAuth2-style schemes).
type SecurityRef struct {
	Scheme string
	Scopes []string
}

// HeaderDef declares an extra response or request header that goai should
// document on the operation, on top of whatever the request body or query
// params already produce.
type HeaderDef struct {
	Name        string
	In          string // "header" | "response"
	Description string
	Required    bool
	Example     string
}

// PathParam describes a path-level parameter injected by an upstream
// acceptance/minortask. AnchorAfter, when non-empty, indicates the
// path segment after which this parameter should be inserted; an empty
// AnchorAfter means "append to the end of the inherited path".
//
// IsItemIdentifier signals that this placeholder is the item-level
// entity identifier for the operation. The walker's item-level
// emission auto-appends /{<lastSegment>_id} to a path that does not
// already end in a placeholder, on the assumption that the last
// segment is a collection that needs an item suffix. Routes whose
// last segment is a verb / action (e.g.
// /admin/v1/teams/{teams_id}/activate) opt out of that auto-suffix
// because the item identifier is the segment BEFORE the verb, not
// after it. Setting IsItemIdentifier=true on the {teams_id} param
// tells the walker the item id is already declared.
type PathParam struct {
	Name             string
	In               string // typically "path"; "query" / "header" allowed
	Style            string // OpenAPI parameter style; default "simple"
	Description      string
	AnchorAfter      string
	Example          string
	Required         bool
	IsItemIdentifier bool
}

// NewSpec returns a Spec built from the supplied options. Useful when an
// optional SpecProvider.GOAISpec returns Spec values.
func NewSpec(opts ...Option) Spec {
	s := Spec{examples: map[string]any{}}
	for _, opt := range opts {
		opt(&s)
	}

	return s
}

// WithSummary sets the operation summary (one-line title shown by Swagger UI).
func WithSummary(summary string) Option {
	return func(s *Spec) { s.summary = summary }
}

// WithDescription sets the long-form operation description.
func WithDescription(description string) Option {
	return func(s *Spec) { s.description = description }
}

// WithOperationID sets the operationId field. When omitted, builder synthesises
// it from package + handler + method.
func WithOperationID(id string) Option {
	return func(s *Spec) { s.operationID = id }
}

// WithTag appends one or more tags. Tags are used to group operations in the
// rendered spec and may also drive profile selectors.
func WithTag(tags ...string) Option {
	return func(s *Spec) { s.tags = append(s.tags, tags...) }
}

// WithExample attaches an example value for a given media type
// (e.g. "application/json"). The example value is caller-owned and must
// not be mutated while generation is running.
func WithExample(mediaType string, example any) Option {
	return func(s *Spec) {
		if s.examples == nil {
			s.examples = map[string]any{}
		}

		s.examples[mediaType] = example
	}
}

// WithDeprecated marks the operation as deprecated.
func WithDeprecated() Option {
	return func(s *Spec) { s.deprecated = true }
}

// WithExternalDocs attaches an operation-level externalDocs entry.
func WithExternalDocs(url, description string) Option {
	return func(s *Spec) {
		s.externalDocs = &ExternalDocumentation{URL: url, Description: description}
	}
}

// WithCallback registers a callback under the operation's callbacks map.
// `name` is the local identifier; `callback` is a map of runtime
// expressions (e.g. "{$request.body#/callbackUrl}") to PathItems.
func WithCallback(name string, callback Callback) Option {
	return func(s *Spec) {
		if s.callbacks == nil {
			s.callbacks = map[string]Callback{}
		}

		s.callbacks[name] = callback
	}
}

// WithOperationServer adds a server entry that overrides the document /
// path-level servers for this single operation.
func WithOperationServer(server Server) Option {
	return func(s *Spec) {
		s.servers = append(s.servers, server)
	}
}

// WithExampleObject attaches a named Example Object under a media type.
// Use this when you need richer metadata (summary, description,
// externalValue) than WithExample provides.
func WithExampleObject(mediaType, name string, ex *Example) Option {
	return func(s *Spec) {
		if s.multiExamples == nil {
			s.multiExamples = map[string]map[string]*Example{}
		}

		bucket := s.multiExamples[mediaType]
		if bucket == nil {
			bucket = map[string]*Example{}
		}

		bucket[name] = ex
		s.multiExamples[mediaType] = bucket
	}
}

// WithRequestMediaType overrides the default "application/json" media type
// used when goai auto-builds the request body schema from a registered
// request type. Use for operations that consume e.g. multipart/form-data
// or application/x-www-form-urlencoded.
func WithRequestMediaType(mediaType string) Option {
	return func(s *Spec) { s.requestMediaType = mediaType }
}

// WithRequestDescription sets the operation's request body description.
// Renders under requestBody.description in the emitted yaml.
func WithRequestDescription(description string) Option {
	return func(s *Spec) { s.requestDescription = description }
}

// WithSuccessDescription overrides the method default response description
// emitted under the conventional success status code (200 / 201 / 204).
// Use this when the success response merits a richer description than the
// generic default.
func WithSuccessDescription(description string) Option {
	return func(s *Spec) { s.successDescription = description }
}

// WithResponse declares an operation response under the given HTTP status
// code. Use multiple times for distinct status codes (e.g. "400", "404",
// "default"). description should be a short human-readable explanation of
// what the status code means in the operation's context. Additional
// content (schema, examples, headers) is layered on via ResponseOption.
//
// Calling WithResponse with the operation's conventional success status
// (matching successStatus(httpMethod)) updates that response's description
// in the same way as WithSuccessDescription, but lets the caller also
// attach a custom schema or examples — useful for handlers that document a
// response shape distinct from the registered Go response type.
func WithResponse(status string, description string, opts ...ResponseOption) Option {
	return func(s *Spec) {
		if s.responses == nil {
			s.responses = map[string]*ResponseSpec{}
		}

		entry := s.responses[status]
		if entry == nil {
			entry = &ResponseSpec{}
			s.responses[status] = entry
		}

		if description != "" {
			entry.Description = description
		}

		for _, opt := range opts {
			opt(entry)
		}
	}
}

// WithResponseSchema attaches a Go type whose synthesised schema becomes
// the response body. Use for non-success responses whose body shape is not
// captured by the registered (handler, method) response type — e.g. an
// error envelope on a 400 / 404 branch.
// Pointer types are automatically unwrapped so that *T behaves like T.
func WithResponseSchema(t reflect.Type) ResponseOption {
	for t != nil && t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return func(r *ResponseSpec) { r.SchemaType = t }
}

// WithResponseSchemaPrebuilt attaches a pre-built Schema verbatim under
// the response content block. Use for $ref-based reuse or when the schema
// is hand-authored.
func WithResponseSchemaPrebuilt(schema *Schema) ResponseOption {
	return func(r *ResponseSpec) { r.Schema = schema }
}

// WithResponseMediaType overrides the response content media type
// (default "application/json").
func WithResponseMediaType(mediaType string) ResponseOption {
	return func(r *ResponseSpec) { r.MediaType = mediaType }
}

// WithResponseExample attaches a single inline example to the response
// content block.
func WithResponseExample(example any) ResponseOption {
	return func(r *ResponseSpec) { r.Example = example }
}

// WithResponseExampleObject attaches a named Example Object to the
// response content block. Use multiple times for distinct named examples.
func WithResponseExampleObject(name string, example *Example) ResponseOption {
	return func(r *ResponseSpec) {
		if r.Examples == nil {
			r.Examples = map[string]*Example{}
		}

		r.Examples[name] = example
	}
}

// WithResponseHeader documents a response header attached to this status
// code's response. Use multiple times for distinct headers.
func WithResponseHeader(h HeaderDef) ResponseOption {
	return func(r *ResponseSpec) {
		r.Headers = append(r.Headers, h)
	}
}

// WithSecurity appends a SecurityRef. Multiple calls combine as logical OR
// at the operation level (per OpenAPI semantics).
func WithSecurity(scheme string, scopes ...string) Option {
	return func(s *Spec) {
		s.security = append(s.security, SecurityRef{Scheme: scheme, Scopes: scopes})
	}
}

// WithHeader documents an extra header. Use multiple times for multiple
// headers.
func WithHeader(h HeaderDef) Option {
	return func(s *Spec) { s.extraHeaders = append(s.extraHeaders, h) }
}

// WithParam documents an extra parameter (path/query/header) that goai
// cannot detect from the route tree alone.
func WithParam(p PathParam) Option {
	return func(s *Spec) { s.extraParams = append(s.extraParams, p) }
}

// Summary reports the spec summary, suitable for builder consumption.
func (s Spec) Summary() string { return s.summary }

// Description reports the spec description.
func (s Spec) Description() string { return s.description }

// OperationID reports the explicit operation id, or empty if unset.
func (s Spec) OperationID() string { return s.operationID }

// Tags returns a copy of the tag slice.
func (s Spec) Tags() []string {
	if len(s.tags) == 0 {
		return nil
	}

	out := make([]string, len(s.tags))
	copy(out, s.tags)

	return out
}

// Examples returns the example map (callers must not mutate). Arbitrary
// example values are treated as immutable caller data for the duration of
// generation; do not mutate them while goai is building or emitting.
func (s Spec) Examples() map[string]any { return s.examples }

// Deprecated reports whether the operation is deprecated.
func (s Spec) Deprecated() bool { return s.deprecated }

// Security returns the configured security refs.
func (s Spec) Security() []SecurityRef {
	if len(s.security) == 0 {
		return nil
	}

	out := make([]SecurityRef, len(s.security))
	copy(out, s.security)

	return out
}

// ExtraHeaders returns extra headers configured for the operation.
func (s Spec) ExtraHeaders() []HeaderDef {
	if len(s.extraHeaders) == 0 {
		return nil
	}

	out := make([]HeaderDef, len(s.extraHeaders))
	copy(out, s.extraHeaders)

	return out
}

// ExtraParams returns extra parameters configured for the operation.
func (s Spec) ExtraParams() []PathParam {
	if len(s.extraParams) == 0 {
		return nil
	}

	out := make([]PathParam, len(s.extraParams))
	copy(out, s.extraParams)

	return out
}

// ExternalDocs returns the operation-level external docs reference, or nil.
func (s Spec) ExternalDocs() *ExternalDocumentation { return s.externalDocs }

// Callbacks returns the operation's callbacks map (callers must not mutate).
func (s Spec) Callbacks() map[string]Callback { return s.callbacks }

// OperationServers returns the operation-level servers, if any.
func (s Spec) OperationServers() []Server {
	if len(s.servers) == 0 {
		return nil
	}

	out := make([]Server, len(s.servers))
	copy(out, s.servers)

	return out
}

// MultiExamples returns the per-media-type, per-name Example map. The
// returned map and its inner buckets must not be mutated by callers.
func (s Spec) MultiExamples() map[string]map[string]*Example { return s.multiExamples }

// RequestMediaType returns the operator-overridden request media type, or
// empty string when goai should fall back to "application/json".
func (s Spec) RequestMediaType() string { return s.requestMediaType }

// RequestDescription returns the operation request body description, or
// empty string when unset.
func (s Spec) RequestDescription() string { return s.requestDescription }

// SuccessDescription returns the override for the success response
// description, or empty string when the builder should use its default.
func (s Spec) SuccessDescription() string { return s.successDescription }

// Responses returns the per-status response specs declared via WithResponse.
// The returned map and its entries must not be mutated by callers.
func (s Spec) Responses() map[string]*ResponseSpec { return s.responses }

// SpecProvider supplies a Spec that applies to every HTTP method the handler
// implements. Use this when the spec metadata (tags, security, deprecated)
// is the same for all methods on the handler, or when the handler only
// implements one HTTP method — the most common case. The GOAI prefix on the
// method name keeps it distinct from the handler's HTTP method functions
// (Get, Post, ...) on the same struct, and the method name mirrors the
// returned Spec type.
//
// For handlers that need different specs per HTTP method, implement one of
// the per-method providers below (IndexSpecProvider, GetSpecProvider, ...).
// The per-method providers override SpecProvider when both are implemented.
// All SpecProvider methods must be deterministic and side-effect-free. goai
// may cache and reuse their results during serial metadata preparation;
// invocation count and order are not part of the API contract.
type SpecProvider interface {
	GOAISpec() Spec
}

// IndexSpecProvider supplies the Spec for the handler's Index() method (a
// collection-style HTTP GET that returns a list).
type IndexSpecProvider interface {
	GOAIIndexSpec() Spec
}

// GetSpecProvider supplies the Spec for the handler's Get() method (HTTP
// GET — either an item read on a collection-style handler or a singleton
// fetch).
type GetSpecProvider interface {
	GOAIGetSpec() Spec
}

// HeadSpecProvider supplies the Spec for the handler's Head() method (HTTP
// HEAD).
type HeadSpecProvider interface {
	GOAIHeadSpec() Spec
}

// CreateSpecProvider supplies the Spec for the handler's Create() method
// (HTTP POST that creates a collection member).
type CreateSpecProvider interface {
	GOAICreateSpec() Spec
}

// PostSpecProvider supplies the Spec for the handler's Post() method (HTTP
// POST — either a bare-path action or an item-level action depending on
// whether the handler also defines Create()).
type PostSpecProvider interface {
	GOAIPostSpec() Spec
}

// PatchSpecProvider supplies the Spec for the handler's Patch() method
// (HTTP PATCH).
type PatchSpecProvider interface {
	GOAIPatchSpec() Spec
}

// PutSpecProvider supplies the Spec for the handler's Put() method (HTTP
// PUT).
type PutSpecProvider interface {
	GOAIPutSpec() Spec
}

// DeleteSpecProvider supplies the Spec for the handler's Delete() method
// (HTTP DELETE).
type DeleteSpecProvider interface {
	GOAIDeleteSpec() Spec
}

// OptionsSpecProvider supplies the Spec for the handler's Options() method
// (HTTP OPTIONS).
type OptionsSpecProvider interface {
	GOAIOptionsSpec() Spec
}

// TraceSpecProvider supplies the Spec for the handler's Trace() method
// (HTTP TRACE).
type TraceSpecProvider interface {
	GOAITraceSpec() Spec
}

// SecurityProvider lets an acceptance declare that it enforces a particular
// security scheme for every HTTP method on every guarded route. Each
// acceptance can return one scheme + scopes. Builder aggregates these
// along the inherited acceptance chain.
//
// Acceptances that conditionally skip enforcement on certain HTTP methods
// (e.g. CSRF acceptances that exempt GET) should implement
// MethodAwareSecurityProvider instead — the walker prefers it when present
// so the OpenAPI security requirement matches actual runtime enforcement
// per method.
type SecurityProvider interface {
	SecurityRequirement() (scheme string, scopes []string)
}

// MethodAwareSecurityProvider is the per-HTTP-method variant of
// SecurityProvider. Returning ("", nil) for a method means "this acceptance
// does not contribute a security requirement on that method" — useful when
// the acceptance bypasses authentication for one method (e.g. RFC 7662
// token introspection at POST /tokeninfo with bearer-less form auth).
//
// method is the upper-case HTTP method ("GET", "POST", ...). Implementing
// MethodAwareSecurityProvider does NOT also require implementing
// SecurityProvider; the walker calls the method-aware variant first and
// only falls back to the static provider when the acceptance does not
// implement it.
type MethodAwareSecurityProvider interface {
	SecurityRequirementFor(method string) (scheme string, scopes []string)
}

// PathParamInjector lets an acceptance / minortask declare which path
// parameters it injects into params. Builder uses this to expand the
// route node's static path with {param} placeholders.
type PathParamInjector interface {
	InjectedParams() []PathParam
}

// PathAwareParamInjector is a path-aware specialization of PathParamInjector.
// When an injector is reused under multiple route subtrees that follow
// different path-parameter conventions (e.g. /api/v1/teams/{teams_id}/...
// vs /admin/v1/teams/{id}/...), the bare InjectedParams contract is too
// coarse — it cannot distinguish action endpoints under the same anchor
// segment from sub-resource endpoints. The walker calls InjectedParamsFor
// with the literal route path so the injector can return a tailored slice
// (or nil, to suppress injection entirely for that route).
//
// When an acceptance implements both PathParamInjector and
// PathAwareParamInjector, the walker calls InjectedParamsFor and ignores
// InjectedParams. Implementors typically delegate to the bare method
// when no path-specific tailoring is needed.
type PathAwareParamInjector interface {
	InjectedParamsFor(routePath string) []PathParam
}

// EmissionContext carries the per-emission information passed to
// EmissionAwareParamInjector. ItemLevel mirrors the walker's
// classification: false for Index / Create-style collection emissions,
// true for Get / Patch / Put / Delete-style item emissions where the
// walker normally appends a trailing /{id} placeholder.
type EmissionContext struct {
	HTTPMethod string
	GoMethod   string
	ItemLevel  bool
}

// EmissionAwareParamInjector lets an injector tailor PathParam output
// per (route, emission) pair. This is required for routes whose base
// spec uses different placeholder shapes for collection-level Index vs
// item-level Get / Patch / Delete emissions — e.g. base spec entries
// of /<action> for Index alongside /{id}/<action> for item ops, where
// a single PathParam slice cannot satisfy both shapes.
//
// When an acceptance implements EmissionAwareParamInjector, the walker
// calls InjectedParamsForEmission once per emission and ignores the
// other path-injection interfaces. Implementors typically delegate to
// PathAwareParamInjector or PathParamInjector when emission context is
// not needed.
type EmissionAwareParamInjector interface {
	InjectedParamsForEmission(routePath string, ctx EmissionContext) []PathParam
}

// ProfileScope lets a handler or acceptance declare which output profiles
// the operation belongs to. Returning an empty slice means "no preference"
// and lets the builder's classifier decide. The method name mirrors the
// interface name so its purpose is obvious at the call site.
type ProfileScope interface {
	GOAIProfileScope() []string
}

// ItemIDProvider lets a handler override the path-parameter name the
// walker uses when auto-appending an item-level segment to a
// collection-style handler. The default name is `<last route
// segment>_id` (e.g. /albums → /albums/{albums_id}); implement this
// interface to choose a different name. Returning an empty string falls
// back to the default.
type ItemIDProvider interface {
	GOAIItemIDName() string
}

// Hidden lets a handler declare it should not appear in any generated spec.
// Returning true drops every method on the handler from Walker output before
// Build sees them. Use for framework-internal handlers (static asset
// servers, health probes that should not be public-documented, etc.).
type Hidden interface {
	Hidden() bool
}
