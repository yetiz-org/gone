package goai

import (
	"fmt"
	"reflect"
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
	// produces a tighter yaml when most operations are not yet registered.
	SuppressEmptySchemas bool
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
		operationTags := candidateOperationTags(c, tsClassifier)

		if profile != nil && !profile.Matches(c, profiles, operationTags) {
			continue
		}

		ensurePathItem(doc, c.Path)
		op := buildOperation(c, schemaBld, tsClassifier, opts.SuppressEmptySchemas)
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
//  4. Path-based fallback from the project Classifier.
func candidateOperationTags(c OperationCandidate, classifier *Classifier) []string {
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

	tags := spec.Tags()
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
func buildOperation(c OperationCandidate, schemaBld *schemaBuilder, classifier *Classifier, suppressEmpty bool) *Operation {
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
	op.Tags = spec.Tags()
	op.Deprecated = spec.Deprecated()
	op.ExternalDocs = spec.ExternalDocs()
	op.Callbacks = spec.Callbacks()
	op.Servers = spec.OperationServers()

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
				Required: true,
				Content:  map[string]*MediaType{mediaType: mt},
			}
		}
	}

	// Responses
	op.Responses = map[string]*Response{}

	successCode := successStatus(c.Method)
	successResp := &Response{Description: "OK"}
	if hasEntry && entry.RespType != nil {
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
	} else if !suppressEmpty {
		successResp.Content = map[string]*MediaType{
			"application/json": {Schema: &Schema{Type: "object"}},
		}
	}

	op.Responses[successCode] = successResp

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

	// Headers from Spec.ExtraHeaders attach onto the success response.
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
		} else {
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
