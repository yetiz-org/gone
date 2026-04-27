package goai

import (
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

// BuildOptions tunes Build behaviour.
type BuildOptions struct {
	Title          string
	Description    string
	Version        string
	TermsOfService string
	Contact        *Contact
	License        *License
	Servers        []Server
	Tags           []Tag
	GlobalSecurity []map[string][]string
	// ExternalDocs populates the top-level Document.ExternalDocs.
	ExternalDocs *ExternalDocumentation
	// Classifier returns the set of profile names a candidate belongs to
	// when the candidate did not declare any via ProfileScope. nil means
	// "use BuiltinClassify".
	Classifier func(c OperationCandidate) []string
	// TagSecurityClassifier provides default tags + security refs when a
	// candidate has no explicit Spec entry. nil means "use DefaultClassifier".
	TagSecurityClassifier *Classifier
	// SuppressEmptySchemas, when true, omits the application/json content
	// block on responses where the registered response type is nil. This
	// produces tighter yaml for operations without registered response types.
	SuppressEmptySchemas bool
	// OperationDocExtractor, when non-nil, is consulted as a fallback
	// source for operation-level OpenAPI metadata. Explicit Spec values,
	// route-derived parameters, generated schemas, and acceptance-derived
	// security always win; docstring content fills gaps.
	//
	// Use DefaultOperationDocExtractor() to enable the built-in AST-based
	// implementation. nil disables the fallback.
	OperationDocExtractor OperationDocExtractor
}

// Build assembles a single OpenAPI Document from candidates that match the
// supplied profile. profile may be nil, in which case all candidates pass.
func Build(candidates []OperationCandidate, profile *Profile, opts BuildOptions) *Document {
	if opts.Title == "" {
		opts.Title = "API"
	}

	if opts.Version == "" {
		opts.Version = "0.0.0"
	}

	classifier := opts.Classifier
	if classifier == nil {
		classifier = BuiltinClassify
	}

	tsClassifier := opts.TagSecurityClassifier
	if tsClassifier == nil {
		tsClassifier = DefaultClassifier()
	}

	doc := &Document{
		OpenAPI: "3.0.3",
		Info: Info{
			Title:          opts.Title,
			Description:    opts.Description,
			TermsOfService: opts.TermsOfService,
			Contact:        opts.Contact,
			License:        opts.License,
			Version:        opts.Version,
		},
		Servers:      opts.Servers,
		Tags:         opts.Tags,
		Paths:        map[string]*PathItem{},
		Components:   NewComponents(),
		Security:     opts.GlobalSecurity,
		ExternalDocs: opts.ExternalDocs,
	}

	schemaBld := newSchemaBuilder(doc.Components)

	for _, c := range candidates {
		profiles := c.Profiles
		if len(profiles) == 0 {
			profiles = classifier(c)
		}

		// Pre-compute the operation's OpenAPI tags so profile selectors
		// keyed on Tags match what the README promises (e.g. handler
		// declares WithTag("Public") and goai.yaml says
		// `include: { tags: [Public] }`). Building the full Operation
		// before profile filter would be wasteful, so we extract just
		// the tag list here.
		operationTags := candidateOperationTags(c, tsClassifier, opts.OperationDocExtractor)

		if profile != nil && !profile.Matches(c, profiles, operationTags) {
			continue
		}

		ensurePathItem(doc, c.Path)
		op := buildOperation(c, schemaBld, tsClassifier, opts.SuppressEmptySchemas, opts.OperationDocExtractor)
		doc.Paths[c.Path].SetOperation(c.Method, op)
	}

	return doc
}

// candidateOperationTags returns the OpenAPI tag list a candidate's
// Operation will end up carrying after Build, mirroring buildOperation's
// tag-resolution order:
//
//  1. Spec from goai.Register (if registered).
//  2. Spec from SpecProvider catch-all.
//  3. Spec from per-method SpecProvider override.
//  4. Handler or method docstring tags.
//  5. Path-based fallback from the project Classifier.
func candidateOperationTags(c OperationCandidate, classifier *Classifier, docExtractor OperationDocExtractor) []string {
	var spec Spec
	if entry, ok := Lookup(c.Handler, c.Method); ok {
		spec = entry.Spec
	} else {
		if base, ok := c.Handler.(SpecProvider); ok {
			spec = base.GOAISpec()
		}

		if perMethod, ok := perMethodSpec(c.Handler, c.HandlerMethod); ok {
			spec = perMethod
		}
	}

	tags := _DeduplicateStrings(spec.Tags())
	if len(tags) == 0 && docExtractor != nil {
		if doc, ok := docExtractor(c.Handler, c.HandlerMethod); ok {
			tags = _DeduplicateStrings(doc.Operation.Tags)
		}
	}

	if len(tags) == 0 && classifier != nil {
		if tag := classifier.ClassifyTag(c.Path); tag != "" {
			tags = []string{tag}
		}
	}

	return tags
}

// ensurePathItem inserts an empty PathItem under the given path if missing.
func ensurePathItem(doc *Document, path string) {
	if _, ok := doc.Paths[path]; !ok {
		doc.Paths[path] = &PathItem{}
	}
}

// buildOperation assembles a single Operation from a walker candidate and
// its registered Spec/types (if any).
func buildOperation(c OperationCandidate, schemaBld *schemaBuilder, classifier *Classifier, suppressEmpty bool, docExtractor OperationDocExtractor) *Operation {
	op := &Operation{
		OperationID: synthesiseOperationID(c),
	}

	// Pull registry entry if present for this (handler, method).
	entry, hasEntry := Lookup(c.Handler, c.Method)
	var spec Spec
	if hasEntry {
		spec = entry.Spec
	}

	// Augment from SpecProvider / per-method spec providers when registry
	// didn't override. Per-method providers (GOAIIndexSpec, GOAIGetSpec, ...)
	// take precedence over the catch-all SpecProvider (GOAISpec) so that a
	// handler can declare a default spec for all its methods and override
	// it per-method when needed.
	if !hasEntry {
		if base, ok := c.Handler.(SpecProvider); ok {
			spec = base.GOAISpec()
		}

		if perMethod, ok := perMethodSpec(c.Handler, c.HandlerMethod); ok {
			spec = perMethod
		}
	}

	if id := spec.OperationID(); id != "" {
		op.OperationID = id
	}

	op.Summary = spec.Summary()
	op.Description = spec.Description()

	op.Tags = _DeduplicateStrings(spec.Tags())
	op.Deprecated = spec.Deprecated()
	op.ExternalDocs = spec.ExternalDocs()
	op.Callbacks = spec.Callbacks()
	op.Servers = spec.OperationServers()

	var operationDoc *OperationDoc
	operationDocMatches := false
	if docExtractor != nil {
		if doc, ok := docExtractor(c.Handler, c.HandlerMethod); ok {
			operationDoc = doc
			operationDocMatches = _OperationDocMatches(doc, c)
			if len(op.Tags) == 0 && len(doc.Operation.Tags) > 0 {
				op.Tags = _DeduplicateStrings(doc.Operation.Tags)
			}
		}
	}

	// Tag fallback via classifier when handler/spec gave none.
	if len(op.Tags) == 0 && classifier != nil {
		if tag := classifier.ClassifyTag(c.Path); tag != "" {
			op.Tags = []string{tag}
		}
	}

	// Path parameters
	for _, p := range c.PathParams {
		op.Parameters = append(op.Parameters, paramFromPathParam(p, "path"))
	}

	// Extra parameters from spec
	for _, p := range spec.ExtraParams() {
		op.Parameters = append(op.Parameters, paramFromPathParam(p, p.In))
	}

	// Request body
	if hasEntry && entry.ReqType != nil {
		schema := schemaBld.build(entry.ReqType)
		if schema != nil {
			mediaType := spec.RequestMediaType()
			if mediaType == "" {
				mediaType = "application/json"
			}

			mt := &MediaType{Schema: schema}
			if ex, ok := spec.Examples()[mediaType]; ok {
				mt.Example = ex
			}

			if bucket, ok := spec.MultiExamples()[mediaType]; ok && len(bucket) > 0 {
				mt.Examples = bucket
			}

			op.RequestBody = &RequestBody{
				Description: spec.RequestDescription(),
				Required:    true,
				Content:     map[string]*MediaType{mediaType: mt},
			}
		}
	}

	// Responses
	op.Responses = map[string]*Response{}

	successCode := successStatus(c.Method)
	successDescription := spec.SuccessDescription()
	if successDescription == "" {
		successDescription = _DefaultSuccessDescription(c.Method)
	}

	successResp := &Response{Description: successDescription}
	needsDefaultSuccessContent := !suppressEmpty
	if hasEntry && entry.RespType != nil {
		needsDefaultSuccessContent = false
		schema := schemaBld.build(entry.RespType)
		if schema != nil {
			mt := &MediaType{Schema: schema}
			if ex, ok := spec.Examples()["application/json"]; ok {
				mt.Example = ex
			}

			if bucket, ok := spec.MultiExamples()["application/json"]; ok && len(bucket) > 0 {
				mt.Examples = bucket
			}

			successResp.Content = map[string]*MediaType{"application/json": mt}
		}
	}

	op.Responses[successCode] = successResp

	// Apply per-status responses declared via WithResponse. When the
	// status matches the operation's success code, the spec entry
	// merges into the already-built successResp so any registered
	// response Go-type is preserved alongside the spec-supplied
	// description / examples / headers. Other status codes are emitted
	// as fresh Response values.
	for status, rs := range spec.Responses() {
		if rs == nil {
			continue
		}

		_ApplyResponseSpec(op, status, rs, schemaBld)
	}

	// Security: registry/Spec wins; walker-collected wins next; classifier
	// fills only when neither produced anything.
	securityRefs := append([]SecurityRef(nil), c.SecurityRefs...)
	for _, r := range spec.Security() {
		securityRefs = append(securityRefs, r)
	}

	if len(securityRefs) == 0 && classifier != nil {
		securityRefs = classifier.ClassifySecurity(c.Acceptances)
	}

	if len(securityRefs) > 0 {
		// OpenAPI 3.0.3 semantics: each map in op.Security is a Security
		// Requirement Object. Multiple keys *within* one object combine as
		// AND; multiple objects in the array combine as OR. The Spec API
		// documents WithSecurity as appending OR alternatives, so each ref
		// becomes its own requirement object. Callers that want AND
		// composition for a single operation should declare it via the
		// document-level GlobalSecurity.
		sec := make([]map[string][]string, 0, len(securityRefs))
		for _, r := range securityRefs {
			sec = append(sec, map[string][]string{
				r.Scheme: append([]string(nil), r.Scopes...),
			})
		}

		op.Security = &sec
	} else if c.SecurityExplicitlyEmpty {
		// At least one acceptance bypassed its usual security requirement
		// on this HTTP method (e.g. bearer-less token introspection POST).
		// Emit an explicit empty array so the operation overrides any
		// document-level GlobalSecurity inheritance — `nil` would let the
		// global default flow through and mark the bypass as still
		// requiring auth.
		empty := []map[string][]string{}
		op.Security = &empty
	}

	// Headers from Spec.ExtraHeaders must exist before doc fallback so
	// doc parameters and response headers can only fill missing names.
	_ApplySpecExtraHeaders(op, successResp, spec)

	if operationDocMatches {
		_MergeOperationDocSchemas(schemaBld, operationDoc)
		_MergeOperationDocFallback(op, &operationDoc.Operation, spec, c.Method)
	}

	if needsDefaultSuccessContent && len(successResp.Content) == 0 {
		successResp.Content = map[string]*MediaType{
			"application/json": {Schema: &Schema{Type: "object"}},
		}
	}

	return op
}

// perMethodSpec dispatches to the matching per-Go-method SpecProvider
// implementation. handlerMethod is the Go method name reported by the
// walker (Index/Get/Create/Post/Patch/Put/Delete/Options/Trace). Returns
// (spec, true) when the handler implements the corresponding interface;
// otherwise (Spec{}, false).
func perMethodSpec(handler any, handlerMethod string) (Spec, bool) {
	switch handlerMethod {
	case "Index":
		if d, ok := handler.(IndexSpecProvider); ok {
			return d.GOAIIndexSpec(), true
		}
	case "Get":
		if d, ok := handler.(GetSpecProvider); ok {
			return d.GOAIGetSpec(), true
		}
	case "Create":
		if d, ok := handler.(CreateSpecProvider); ok {
			return d.GOAICreateSpec(), true
		}
	case "Post":
		if d, ok := handler.(PostSpecProvider); ok {
			return d.GOAIPostSpec(), true
		}
	case "Patch":
		if d, ok := handler.(PatchSpecProvider); ok {
			return d.GOAIPatchSpec(), true
		}
	case "Put":
		if d, ok := handler.(PutSpecProvider); ok {
			return d.GOAIPutSpec(), true
		}
	case "Delete":
		if d, ok := handler.(DeleteSpecProvider); ok {
			return d.GOAIDeleteSpec(), true
		}
	case "Options":
		if d, ok := handler.(OptionsSpecProvider); ok {
			return d.GOAIOptionsSpec(), true
		}
	case "Trace":
		if d, ok := handler.(TraceSpecProvider); ok {
			return d.GOAITraceSpec(), true
		}
	}

	return Spec{}, false
}

// synthesiseOperationID returns a stable operation id based on the handler
// type, the Go method name (Index/Get/Post/...), and the HTTP method. Using
// HandlerMethod keeps Index and Get distinct on collection-style handlers
// even though both surface as HTTP GET.
func synthesiseOperationID(c OperationCandidate) string {
	if c.Handler == nil {
		return ""
	}

	t := reflect.TypeOf(c.Handler)
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	pkg := t.PkgPath()
	last := pkg
	if idx := strings.LastIndex(pkg, "/"); idx >= 0 {
		last = pkg[idx+1:]
	}

	suffix := c.HandlerMethod
	if suffix == "" {
		suffix = titleCase(strings.ToLower(c.Method))
	}

	return fmt.Sprintf("%s%s%s", last, t.Name(), suffix)
}

// paramFromPathParam adapts our PathParam into the OpenAPI Parameter shape.
func paramFromPathParam(p PathParam, in string) *Parameter {
	if in == "" {
		in = "path"
	}

	param := &Parameter{
		Name:        p.Name,
		In:          in,
		Description: p.Description,
		Required:    p.Required || in == "path",
		Style:       p.Style,
		Schema:      &Schema{Type: "string"},
	}

	if p.Example != "" {
		param.Example = p.Example
	}

	return param
}

func _ApplySpecExtraHeaders(op *Operation, successResp *Response, spec Spec) {
	for _, h := range spec.ExtraHeaders() {
		if h.In == "header" {
			op.Parameters = append(op.Parameters, &Parameter{
				Name:        h.Name,
				In:          "header",
				Description: h.Description,
				Required:    h.Required,
				Schema:      &Schema{Type: "string"},
				Example:     h.Example,
			})

			continue
		}

		if successResp.Headers == nil {
			successResp.Headers = map[string]*Header{}
		}

		successResp.Headers[h.Name] = &Header{
			Description: h.Description,
			Required:    h.Required,
			Schema:      &Schema{Type: "string"},
			Example:     h.Example,
		}
	}
}

// successStatus returns the conventional success code for a method.
func successStatus(method string) string {
	switch method {
	case "POST":
		return "201"
	case "DELETE":
		return "204"
	default:
		return "200"
	}
}

// _DefaultSuccessDescription maps HTTP method to a generic OpenAPI response
// description used when the operation Spec did not supply one via
// WithSuccessDescription.
func _DefaultSuccessDescription(method string) string {
	switch method {
	case "POST":
		return "Created"
	case "DELETE":
		return "No Content"
	default:
		return "OK"
	}
}

// _DefaultDescriptionForStatus returns a sensible OpenAPI response
// description for an HTTP status code when WithResponse callers leave
// the description empty. Recognised numeric codes use the standard
// reason phrase from net/http.StatusText. The OpenAPI "default" key
// renders as "Default response". Anything unrecognised falls back to
// "OK" so the emitted yaml stays valid even on bespoke status keys.
func _DefaultDescriptionForStatus(status string) string {
	switch status {
	case "":
		return "OK"
	case "default":
		return "Default response"
	}

	if code, err := strconv.Atoi(status); err == nil {
		if text := http.StatusText(code); text != "" {
			return text
		}
	}

	return "OK"
}

// _ApplyResponseSpec merges a Spec-declared ResponseSpec into op.Responses
// under the given status code. When the status matches an existing
// response (typically the auto-generated success response), the merge
// preserves any registered response type by only overriding the
// description and content when the spec supplies them. New status codes
// produce a fresh Response value.
func _ApplyResponseSpec(op *Operation, status string, rs *ResponseSpec, schemaBld *schemaBuilder) {
	if op.Responses == nil {
		op.Responses = map[string]*Response{}
	}

	resp := op.Responses[status]
	if resp == nil {
		resp = &Response{}
		op.Responses[status] = resp
	}

	if rs.Description != "" {
		resp.Description = rs.Description
	}

	if resp.Description == "" {
		resp.Description = _DefaultDescriptionForStatus(status)
	}

	mediaType := rs.MediaType
	if mediaType == "" {
		mediaType = "application/json"
	}

	// Precedence: WithResponseSchemaPrebuilt (hand-authored $ref / inline)
	// beats WithResponseSchema (synthesised from Go type). Callers who want
	// the Go-type schema should not also call WithResponseSchemaPrebuilt.
	var schema *Schema
	switch {
	case rs.Schema != nil:
		schema = rs.Schema
	case rs.SchemaType != nil && schemaBld != nil:
		schema = schemaBld.build(rs.SchemaType)
	}

	if schema != nil || rs.Example != nil || len(rs.Examples) > 0 {
		if resp.Content == nil {
			resp.Content = map[string]*MediaType{}
		}

		mt := resp.Content[mediaType]
		if mt == nil {
			mt = &MediaType{}
			resp.Content[mediaType] = mt
		}

		if schema != nil {
			mt.Schema = schema
		}

		if rs.Example != nil {
			mt.Example = rs.Example
		}

		if len(rs.Examples) > 0 {
			if mt.Examples == nil {
				mt.Examples = map[string]*Example{}
			}

			for name, ex := range rs.Examples {
				mt.Examples[name] = ex
			}
		}
	}

	if len(rs.Headers) > 0 {
		if resp.Headers == nil {
			resp.Headers = map[string]*Header{}
		}

		for _, h := range rs.Headers {
			resp.Headers[h.Name] = &Header{
				Description: h.Description,
				Required:    h.Required,
				Schema:      &Schema{Type: "string"},
				Example:     h.Example,
			}
		}
	}
}

func _MergeOperationDocFallback(op *Operation, docOp *Operation, spec Spec, method string) {
	if op == nil || docOp == nil {
		return
	}

	if spec.OperationID() == "" && docOp.OperationID != "" {
		op.OperationID = docOp.OperationID
	}

	if op.Summary == "" {
		op.Summary = docOp.Summary
	}

	if op.Description == "" {
		op.Description = docOp.Description
	}

	if len(spec.Tags()) == 0 && len(docOp.Tags) > 0 {
		op.Tags = _DeduplicateStrings(docOp.Tags)
	}

	if !spec.Deprecated() && docOp.Deprecated {
		op.Deprecated = true
	}

	if spec.ExternalDocs() == nil && op.ExternalDocs == nil {
		op.ExternalDocs = docOp.ExternalDocs
	}

	_MergeCallbacksFallback(op, docOp.Callbacks)

	if len(spec.OperationServers()) == 0 && len(op.Servers) == 0 && len(docOp.Servers) > 0 {
		op.Servers = append([]Server(nil), docOp.Servers...)
	}

	_MergeOperationDocParameters(op, docOp.Parameters)
	_MergeOperationDocRequestBody(op, docOp.RequestBody)
	_MergeOperationDocResponses(op, docOp.Responses, spec, method)

	if op.Security == nil && docOp.Security != nil {
		op.Security = docOp.Security
	}

	if len(docOp.Extensions) > 0 {
		if op.Extensions == nil {
			op.Extensions = map[string]any{}
		}

		for key, value := range docOp.Extensions {
			if _, exists := op.Extensions[key]; !exists {
				op.Extensions[key] = value
			}
		}
	}
}

func _DeduplicateStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}

		if _, exists := seen[value]; exists {
			continue
		}

		seen[value] = struct{}{}
		out = append(out, value)
	}

	if len(out) == 0 {
		return nil
	}

	return out
}

func _MergeOperationDocSchemas(schemaBld *schemaBuilder, doc *OperationDoc) {
	if schemaBld == nil || schemaBld.components == nil || doc == nil || len(doc.Schemas) == 0 {
		return
	}

	if schemaBld.components.Schemas == nil {
		schemaBld.components.Schemas = map[string]*Schema{}
	}

	names := make([]string, 0, len(doc.Schemas))
	for name := range doc.Schemas {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		schema := doc.Schemas[name]
		pkg := doc.SchemaPackages[name]
		targetName := _OperationDocSchemaComponentName(schemaBld, name, pkg)
		if targetName != name {
			_RenameOperationDocSchemaRef(doc, name, targetName)
			schema = doc.Schemas[targetName]
			name = targetName
		}

		if _, exists := schemaBld.components.Schemas[name]; !exists {
			schemaBld.components.Schemas[name] = schema
		}

		if pkg != "" {
			schemaBld.pkgOfName[name] = pkg
		}
	}
}

func _OperationDocSchemaComponentName(schemaBld *schemaBuilder, name string, pkg string) string {
	if pkg == "" {
		return name
	}

	if existingPkg, seen := schemaBld.pkgOfName[name]; seen && existingPkg != pkg {
		return _FullOperationDocSchemaComponentName(name, pkg)
	}

	if _, exists := schemaBld.components.Schemas[name]; exists {
		existingPkg := schemaBld.pkgOfName[name]
		if existingPkg == "" || existingPkg != pkg {
			return _FullOperationDocSchemaComponentName(name, pkg)
		}
	}

	return name
}

func _FullOperationDocSchemaComponentName(name string, pkg string) string {
	if pkg == "" {
		return name
	}

	typeName := name
	shortPrefix := shortPkgName(pkg) + "."
	if strings.HasPrefix(name, shortPrefix) {
		typeName = strings.TrimPrefix(name, shortPrefix)
	}

	return strings.ReplaceAll(pkg, "/", ".") + "." + typeName
}

func _RenameOperationDocSchemaRef(doc *OperationDoc, oldName string, newName string) {
	if oldName == newName {
		return
	}

	schema := doc.Schemas[oldName]
	delete(doc.Schemas, oldName)
	doc.Schemas[newName] = schema

	if doc.SchemaPackages != nil {
		pkg := doc.SchemaPackages[oldName]
		delete(doc.SchemaPackages, oldName)
		doc.SchemaPackages[newName] = pkg
	}

	oldRef := "#/components/schemas/" + oldName
	newRef := "#/components/schemas/" + newName
	_RewriteSchemaRefsInOperation(&doc.Operation, oldRef, newRef)
	for _, schema := range doc.Schemas {
		_RewriteSchemaRef(schema, oldRef, newRef)
	}
}

func _RewriteSchemaRefsInOperation(op *Operation, oldRef string, newRef string) {
	if op == nil {
		return
	}

	for _, param := range op.Parameters {
		if param == nil {
			continue
		}

		_RewriteSchemaRef(param.Schema, oldRef, newRef)
		_RewriteSchemaRefsInMediaTypes(param.Content, oldRef, newRef)
	}

	if op.RequestBody != nil {
		_RewriteSchemaRefsInMediaTypes(op.RequestBody.Content, oldRef, newRef)
	}

	for _, resp := range op.Responses {
		if resp == nil {
			continue
		}

		_RewriteSchemaRefsInMediaTypes(resp.Content, oldRef, newRef)
		for _, header := range resp.Headers {
			if header == nil {
				continue
			}

			_RewriteSchemaRef(header.Schema, oldRef, newRef)
			_RewriteSchemaRefsInMediaTypes(header.Content, oldRef, newRef)
		}
	}

	for _, callback := range op.Callbacks {
		for _, pathItem := range callback {
			_RewriteSchemaRefsInPathItem(pathItem, oldRef, newRef)
		}
	}
}

func _RewriteSchemaRefsInPathItem(item *PathItem, oldRef string, newRef string) {
	if item == nil {
		return
	}

	_RewriteSchemaRefsInOperation(item.Get, oldRef, newRef)
	_RewriteSchemaRefsInOperation(item.Put, oldRef, newRef)
	_RewriteSchemaRefsInOperation(item.Post, oldRef, newRef)
	_RewriteSchemaRefsInOperation(item.Delete, oldRef, newRef)
	_RewriteSchemaRefsInOperation(item.Options, oldRef, newRef)
	_RewriteSchemaRefsInOperation(item.Head, oldRef, newRef)
	_RewriteSchemaRefsInOperation(item.Patch, oldRef, newRef)
	_RewriteSchemaRefsInOperation(item.Trace, oldRef, newRef)
	for _, param := range item.Parameters {
		if param == nil {
			continue
		}

		_RewriteSchemaRef(param.Schema, oldRef, newRef)
		_RewriteSchemaRefsInMediaTypes(param.Content, oldRef, newRef)
	}
}

func _RewriteSchemaRefsInMediaTypes(content map[string]*MediaType, oldRef string, newRef string) {
	for _, mt := range content {
		if mt == nil {
			continue
		}

		_RewriteSchemaRef(mt.Schema, oldRef, newRef)
	}
}

func _RewriteSchemaRef(schema *Schema, oldRef string, newRef string) {
	if schema == nil {
		return
	}

	if schema.Ref == oldRef {
		schema.Ref = newRef
	}

	_RewriteSchemaRef(schema.Items, oldRef, newRef)
	_RewriteSchemaRef(schema.Not, oldRef, newRef)
	for _, item := range schema.OneOf {
		_RewriteSchemaRef(item, oldRef, newRef)
	}
	for _, item := range schema.AllOf {
		_RewriteSchemaRef(item, oldRef, newRef)
	}
	for _, item := range schema.AnyOf {
		_RewriteSchemaRef(item, oldRef, newRef)
	}
	for _, prop := range schema.Properties {
		_RewriteSchemaRef(prop, oldRef, newRef)
	}
	if additional, ok := schema.AdditionalProperties.(*Schema); ok {
		_RewriteSchemaRef(additional, oldRef, newRef)
	}
}

func _OperationDocMatches(doc *OperationDoc, c OperationCandidate) bool {
	if doc == nil {
		return false
	}

	if doc.Endpoint.Method == "" || doc.Endpoint.Path == "" {
		return false
	}

	if !strings.EqualFold(doc.Endpoint.Method, c.Method) {
		return false
	}

	if doc.Endpoint.Path != c.Path {
		return false
	}

	return true
}

func _MergeOperationDocParameters(op *Operation, params []*Parameter) {
	for _, docParam := range params {
		if docParam == nil {
			continue
		}

		existing := _FindOperationParameter(op.Parameters, docParam.In, docParam.Name)
		if existing == nil {
			op.Parameters = append(op.Parameters, docParam)

			continue
		}

		_MergeParameterFallback(existing, docParam)
	}
}

func _FindOperationParameter(params []*Parameter, in, name string) *Parameter {
	for _, p := range params {
		if p == nil {
			continue
		}

		if p.In == in && p.Name == name {
			return p
		}
	}

	return nil
}

func _MergeParameterFallback(dst *Parameter, src *Parameter) {
	if dst.Description == "" {
		dst.Description = src.Description
	}

	if dst.Style == "" {
		dst.Style = src.Style
	}

	if dst.Explode == nil {
		dst.Explode = src.Explode
	}

	if dst.Schema == nil && len(dst.Content) == 0 {
		dst.Schema = src.Schema
	}

	if dst.Example == nil {
		dst.Example = src.Example
	}

	if len(dst.Examples) == 0 && len(src.Examples) > 0 {
		dst.Examples = src.Examples
	}

	if len(dst.Content) == 0 && len(src.Content) > 0 && dst.Schema == nil {
		dst.Content = src.Content
	} else {
		_MergeMediaTypesFallback(dst.Content, src.Content)
	}

	if len(src.Extensions) > 0 {
		if dst.Extensions == nil {
			dst.Extensions = map[string]any{}
		}

		for key, value := range src.Extensions {
			if _, exists := dst.Extensions[key]; !exists {
				dst.Extensions[key] = value
			}
		}
	}
}

func _MergeOperationDocRequestBody(op *Operation, docBody *RequestBody) {
	if docBody == nil {
		return
	}

	if op.RequestBody == nil {
		op.RequestBody = docBody

		return
	}

	if op.RequestBody.Description == "" {
		op.RequestBody.Description = docBody.Description
	}

	if !op.RequestBody.Required {
		op.RequestBody.Required = docBody.Required
	}

	if len(op.RequestBody.Content) == 0 && len(docBody.Content) > 0 {
		op.RequestBody.Content = docBody.Content
	} else {
		_MergeMediaTypesFallback(op.RequestBody.Content, docBody.Content)
	}

	if len(docBody.Extensions) > 0 {
		if op.RequestBody.Extensions == nil {
			op.RequestBody.Extensions = map[string]any{}
		}

		for key, value := range docBody.Extensions {
			if _, exists := op.RequestBody.Extensions[key]; !exists {
				op.RequestBody.Extensions[key] = value
			}
		}
	}
}

func _MergeOperationDocResponses(op *Operation, docResponses map[string]*Response, spec Spec, method string) {
	if len(docResponses) == 0 {
		return
	}

	if op.Responses == nil {
		op.Responses = map[string]*Response{}
	}

	for status, docResp := range docResponses {
		if docResp == nil {
			continue
		}

		resp := op.Responses[status]
		if resp == nil {
			op.Responses[status] = docResp

			continue
		}

		if docResp.Description != "" && !_SpecHasResponseDescription(spec, status, method) && _IsDefaultResponseDescription(status, resp.Description) {
			resp.Description = docResp.Description
		}

		_MergeResponseFallback(resp, docResp)
	}
}

func _SpecHasResponseDescription(spec Spec, status string, method string) bool {
	if status == successStatus(method) && spec.SuccessDescription() != "" {
		return true
	}

	if rs := spec.Responses()[status]; rs != nil && rs.Description != "" {
		return true
	}

	return false
}

func _IsDefaultResponseDescription(status string, description string) bool {
	return description == "" || description == _DefaultDescriptionForStatus(status)
}

func _MergeResponseFallback(dst *Response, src *Response) {
	if dst.Description == "" {
		dst.Description = src.Description
	}

	_MergeHeadersFallback(dst, src)

	if len(dst.Content) == 0 && len(src.Content) > 0 {
		dst.Content = src.Content
	} else {
		_MergeMediaTypesFallback(dst.Content, src.Content)
	}

	_MergeLinksFallback(dst, src)

	if len(src.Extensions) > 0 {
		if dst.Extensions == nil {
			dst.Extensions = map[string]any{}
		}

		for key, value := range src.Extensions {
			if _, exists := dst.Extensions[key]; !exists {
				dst.Extensions[key] = value
			}
		}
	}
}

func _MergeHeadersFallback(dst *Response, src *Response) {
	if len(src.Headers) == 0 {
		return
	}

	if dst.Headers == nil {
		dst.Headers = map[string]*Header{}
	}

	for name, header := range src.Headers {
		existing := dst.Headers[name]
		if existing == nil {
			dst.Headers[name] = header

			continue
		}

		_MergeHeaderFallback(existing, header)
	}
}

func _MergeHeaderFallback(dst *Header, src *Header) {
	if dst.Description == "" {
		dst.Description = src.Description
	}

	if dst.Style == "" {
		dst.Style = src.Style
	}

	if dst.Explode == nil {
		dst.Explode = src.Explode
	}

	if dst.Schema == nil && len(dst.Content) == 0 {
		dst.Schema = src.Schema
	}

	if dst.Example == nil {
		dst.Example = src.Example
	}

	if len(dst.Examples) == 0 && len(src.Examples) > 0 {
		dst.Examples = src.Examples
	}

	if len(dst.Content) == 0 && len(src.Content) > 0 && dst.Schema == nil {
		dst.Content = src.Content
	} else {
		_MergeMediaTypesFallback(dst.Content, src.Content)
	}

	if len(src.Extensions) > 0 {
		if dst.Extensions == nil {
			dst.Extensions = map[string]any{}
		}

		for key, value := range src.Extensions {
			if _, exists := dst.Extensions[key]; !exists {
				dst.Extensions[key] = value
			}
		}
	}
}

func _MergeCallbacksFallback(op *Operation, docCallbacks map[string]Callback) {
	if len(docCallbacks) == 0 {
		return
	}

	if op.Callbacks == nil {
		op.Callbacks = map[string]Callback{}
	}

	for name, callback := range docCallbacks {
		if _, exists := op.Callbacks[name]; !exists {
			op.Callbacks[name] = callback
		}
	}
}

func _MergeLinksFallback(dst *Response, src *Response) {
	if len(src.Links) == 0 {
		return
	}

	if dst.Links == nil {
		dst.Links = map[string]*Link{}
	}

	for name, link := range src.Links {
		if _, exists := dst.Links[name]; !exists {
			dst.Links[name] = link
		}
	}
}

func _MergeMediaTypesFallback(dst map[string]*MediaType, src map[string]*MediaType) {
	if len(dst) == 0 || len(src) == 0 {
		return
	}

	for mediaType, srcMT := range src {
		if srcMT == nil {
			continue
		}

		dstMT := dst[mediaType]
		if dstMT == nil {
			dst[mediaType] = srcMT

			continue
		}

		_MergeMediaTypeFallback(dstMT, srcMT)
	}
}

func _MergeMediaTypeFallback(dst *MediaType, src *MediaType) {
	if dst.Schema == nil {
		dst.Schema = src.Schema
	}

	if dst.Example == nil {
		dst.Example = src.Example
	}

	if len(dst.Examples) == 0 && len(src.Examples) > 0 {
		dst.Examples = src.Examples
	} else if len(src.Examples) > 0 {
		if dst.Examples == nil {
			dst.Examples = map[string]*Example{}
		}

		for name, ex := range src.Examples {
			if _, exists := dst.Examples[name]; !exists {
				dst.Examples[name] = ex
			}
		}
	}

	if len(dst.Encoding) == 0 && len(src.Encoding) > 0 {
		dst.Encoding = src.Encoding
	}

	if len(src.Extensions) > 0 {
		if dst.Extensions == nil {
			dst.Extensions = map[string]any{}
		}

		for key, value := range src.Extensions {
			if _, exists := dst.Extensions[key]; !exists {
				dst.Extensions[key] = value
			}
		}
	}
}

// titleCase capitalises the first byte of an ASCII string.
func titleCase(s string) string {
	if s == "" {
		return s
	}

	if s[0] >= 'a' && s[0] <= 'z' {
		return string(s[0]-('a'-'A')) + s[1:]
	}

	return s
}
