package goai

import (
	"reflect"
	"runtime"
	"strings"

	"github.com/yetiz-org/gone/ghttp"
)

// OperationCandidate is one candidate operation extracted from the route
// tree. Builder later filters by profile and combines with registry entries
// to produce the final OpenAPI operation.
type OperationCandidate struct {
	Path        string
	Method      string
	Handler     ghttp.HandlerTask
	Acceptances []ghttp.Acceptance
	// HandlerMethod is the Go method name (Index/Get/Post/Patch/Put/Delete/
	// Options/Create) that produced this candidate. Builder uses it together
	// with Method to choose registry lookup keys and operationId components.
	HandlerMethod string
	// PathParams collected from upstream PathParamInjectors plus any leaf id
	// the walker auto-appends (item-level operations get an "id" path param).
	PathParams []PathParam
	// SecurityRefs collected from upstream SecurityProviders.
	SecurityRefs []SecurityRef
	// SecurityExplicitlyEmpty is true when at least one acceptance
	// implements MethodAwareSecurityProvider AND its generic
	// SecurityProvider would have advertised a scheme, but its method-aware
	// variant returned ("", nil) for this candidate's HTTP method. The
	// builder uses it to emit `security: []` so that the operation
	// overrides any document-level GlobalSecurity inheritance.
	SecurityExplicitlyEmpty bool
	// Profiles aggregated from ProfileScope-implementing nodes; empty means
	// "let the classifier decide".
	Profiles []string
}

// Walk traverses the route tree from the supplied RouteEntriesProvider and
// returns one OperationCandidate per (path, supported HTTP method).
//
// Detection is done by reflecting on the handler value: a handler-method is
// considered "implemented" when its function pointer differs from the
// corresponding default on ghttp.DefaultHTTPHandlerTask.
//
// gone routes follow a REST-style convention where one handler can serve
// both a collection and an item endpoint. The walker mirrors that convention
// by emitting two distinct paths when both styles are present:
//
//   - Index → bare path (collection list)
//   - Create → bare path (collection create)
//   - Post → bare path (collection / leaf action)
//   - Get → bare path + "/{id}" when the handler is collection-style;
//     otherwise the bare path (singleton-style)
//   - Patch / Put / Delete → bare path + "/{id}" when collection-style;
//     otherwise the bare path
//   - Options → bare path
//
// "Collection-style" means the handler overrides Index AND at least one of
// Get / Patch / Put / Delete. Handlers that only override item-level methods
// (a singleton resource exposing Get only, for example) are treated as
// singleton-style and emitted on the bare path.
func Walk(route ghttp.RouteEntriesProvider) []OperationCandidate {
	if route == nil {
		return nil
	}

	entries := route.RouteEntries()
	if len(entries) == 0 {
		return nil
	}

	var out []OperationCandidate
	for _, entry := range entries {
		if entry.Node == nil {
			continue
		}

		handler := entry.Node.HandlerTask()
		if handler == nil {
			continue
		}

		if h, ok := handler.(Hidden); ok && h.Hidden() {
			continue
		}

		emissions := planEmissions(handler)
		if len(emissions) == 0 {
			continue
		}

		acceptances := entry.Node.AggregatedAcceptances()
		profiles := collectProfiles(handler, acceptances)

		for _, em := range emissions {
			injected := collectInjectedParams(acceptances, entry.Path, EmissionContext{
				HTTPMethod: em.HTTP,
				GoMethod:   em.Go,
				ItemLevel:  em.ItemLevel,
			})

			path := expandPathWithParams(entry.Path, injected)
			pathParams := append([]PathParam{}, injected...)

			if em.ItemLevel && !injectedHasItemID(injected, path) {
				if !strings.HasSuffix(path, "}") || !strings.Contains(path, "/{") {
					itemIDName := itemIDNameFor(handler, path)
					path = strings.TrimRight(path, "/") + "/{" + itemIDName + "}"
					// Skip the auto-append when an injector already
					// contributed a PathParam with this name. Happens when
					// an injector's AnchorAfter is the last path segment:
					// expandPathWithParams drops the placeholder from the
					// path but leaves the entry in pathParams, and the
					// item-level branch would otherwise produce a second
					// entry under the same name.
					if !pathParamsHasName(pathParams, itemIDName) {
						pathParams = append(pathParams, PathParam{
							Name:        itemIDName,
							In:          "path",
							Required:    true,
							Description: "Resource identifier",
						})
					}
				}
			}

			// Security is method-aware: an acceptance that exempts certain
			// HTTP methods (e.g. CSRF skip on GET, bearer-less token
			// introspection on POST /tokeninfo) should not contribute a
			// security requirement on those methods. See
			// MethodAwareSecurityProvider.
			refs, explicitlyEmpty := collectSecurityForMethod(acceptances, em.HTTP)

			out = append(out, OperationCandidate{
				Path:                    path,
				Method:                  em.HTTP,
				Handler:                 handler,
				Acceptances:             acceptances,
				HandlerMethod:           em.Go,
				PathParams:              pathParams,
				SecurityRefs:            refs,
				SecurityExplicitlyEmpty: explicitlyEmpty,
				Profiles:                profiles,
			})
		}
	}

	return out
}

// emission is a single (handler-method, http-method, path-shape) decision.
type emission struct {
	Go        string // handler method name on the Go side
	HTTP      string // upper-case HTTP method
	ItemLevel bool   // true → emit on path + "/{id}"
}

// planEmissions inspects the handler's overrides and returns the operations
// the walker should emit. Registered entries narrow the emission set —
// when a handler has explicit goai.Register calls, only registered HTTP
// methods are emitted, but the Go method name and ItemLevel placement
// still come from the override-driven canonical map. This avoids the
// trap of treating the registry's HTTP-method key (uppercase, e.g. "GET")
// as a Go method name (which would produce nonsense like an "INDEX"
// operation or a "Get" handler emitted at bare path despite Index also
// being present).
func planEmissions(handler ghttp.HandlerTask) []emission {
	overrides := detectOverrides(handler)
	canonical := emissionsFromOverrides(overrides)

	registered := LookupAll(handler)
	if len(registered) == 0 {
		return dedupeEmissions(canonical)
	}

	// Filter the canonical list down to HTTP methods the developer
	// explicitly registered. Methods that were registered but have no
	// matching override (developer misuse) fall through as bare-path
	// emissions so the OpenAPI spec still surfaces them.
	wanted := make(map[string]struct{}, len(registered))
	for httpMethod := range registered {
		wanted[strings.ToUpper(httpMethod)] = struct{}{}
	}

	var out []emission
	covered := map[string]struct{}{}
	for _, em := range canonical {
		if _, ok := wanted[em.HTTP]; !ok {
			continue
		}

		out = append(out, em)
		covered[em.HTTP] = struct{}{}
	}

	for httpMethod := range wanted {
		if _, ok := covered[httpMethod]; ok {
			continue
		}

		out = append(out, emission{Go: titleCaseHTTPMethod(httpMethod), HTTP: httpMethod, ItemLevel: false})
	}

	return dedupeEmissions(out)
}

// emissionsFromOverrides translates the override-detection map into the
// canonical (Go method, HTTP method, item-level) emission list. Collection-
// style handlers (Index + at least one item verb) emit item verbs at
// "/{id}"; singleton-style handlers stay on the bare path.
func emissionsFromOverrides(overrides map[string]bool) []emission {
	if len(overrides) == 0 {
		return nil
	}

	hasIndex := overrides["Index"]
	hasItem := overrides["Get"] || overrides["Patch"] || overrides["Put"] || overrides["Delete"]
	collectionStyle := hasIndex && hasItem

	var out []emission

	if overrides["Index"] {
		out = append(out, emission{Go: "Index", HTTP: "GET", ItemLevel: false})
	}

	if overrides["Get"] {
		// Item-level only when the handler is collection-style (Index + item
		// methods present). Singleton-style handlers — including Me-like
		// resources — emit Get on the bare path.
		out = append(out, emission{Go: "Get", HTTP: "GET", ItemLevel: collectionStyle})
	}

	if overrides["Create"] {
		out = append(out, emission{Go: "Create", HTTP: "POST", ItemLevel: false})
	}

	if overrides["Post"] {
		// Post is treated as bare-path action when no Create exists, otherwise
		// item-level (rare but possible — e.g. POST /albums/{id}/release).
		itemLevel := collectionStyle && overrides["Create"]
		out = append(out, emission{Go: "Post", HTTP: "POST", ItemLevel: itemLevel})
	}

	if overrides["Patch"] {
		out = append(out, emission{Go: "Patch", HTTP: "PATCH", ItemLevel: collectionStyle})
	}

	if overrides["Put"] {
		out = append(out, emission{Go: "Put", HTTP: "PUT", ItemLevel: collectionStyle})
	}

	if overrides["Delete"] {
		out = append(out, emission{Go: "Delete", HTTP: "DELETE", ItemLevel: collectionStyle})
	}

	if overrides["Options"] {
		out = append(out, emission{Go: "Options", HTTP: "OPTIONS", ItemLevel: false})
	}

	if overrides["Trace"] {
		out = append(out, emission{Go: "Trace", HTTP: "TRACE", ItemLevel: false})
	}

	return out
}

// pathParamsHasName reports whether any PathParam in the slice already
// declares the given name on the path.
func pathParamsHasName(params []PathParam, name string) bool {
	for _, p := range params {
		if p.In != "" && p.In != "path" {
			continue
		}

		if p.Name == name {
			return true
		}
	}

	return false
}

// injectedHasItemID reports whether the injected PathParam slice
// already supplies an identifier the walker would otherwise auto-append
// for item-level emissions. Two signals count:
//
//   - PathParam.IsItemIdentifier=true on any path param. Injectors
//     opt into this when their placeholder is the item-level entity
//     id (e.g. {teams_id} on /admin/v1/teams/{teams_id}/activate
//     where the verb "activate" is the last segment but the item id
//     sits before it).
//   - A path param named "id" (the canonical generic identifier).
//     Treated as an implicit declaration of the item identifier.
func injectedHasItemID(injected []PathParam, path string) bool {
	for _, p := range injected {
		if p.In != "" && p.In != "path" {
			continue
		}

		if p.IsItemIdentifier || p.Name == "id" {
			return true
		}
	}

	return strings.Contains(path, "/{id}/") || strings.HasSuffix(path, "/{id}")
}

// itemIDNameFor decides the path-parameter name to use when the walker
// auto-appends an item-level segment to a collection-style handler path.
// The default convention is `<last route segment>_id` (e.g. an /albums
// collection produces /albums/{albums_id}), matching the naming
// convention used by hand-tuned OpenAPI specs that pluralise the
// collection name. Handlers can override the chosen name by implementing
// ItemIDProvider on the handler value; an empty return value falls back
// to the convention-based default.
func itemIDNameFor(handler ghttp.HandlerTask, currentPath string) string {
	if provider, ok := handler.(ItemIDProvider); ok {
		if name := strings.TrimSpace(provider.GOAIItemIDName()); name != "" {
			return name
		}
	}

	cleaned := strings.TrimRight(currentPath, "/")
	if cleaned == "" || cleaned == "/" {
		return "id"
	}

	if idx := strings.LastIndex(cleaned, "/"); idx >= 0 {
		last := cleaned[idx+1:]
		// Skip placeholder segments — they should never produce
		// "{foo_id}_id" double-suffixed names.
		if last != "" && !strings.HasPrefix(last, "{") {
			return last + "_id"
		}
	}

	return "id"
}

// titleCaseHTTPMethod returns the conventional Go method name for an
// HTTP method string. Used as the fallback Go-side label when a handler
// is registered for a method it does not actually implement, so callers
// downstream still see something sane in operationId synthesis.
func titleCaseHTTPMethod(httpMethod string) string {
	if httpMethod == "" {
		return ""
	}

	lower := strings.ToLower(httpMethod)

	return strings.ToUpper(lower[:1]) + lower[1:]
}

// dedupeEmissions removes (Go, HTTP, ItemLevel) duplicates while preserving
// the canonical ordering produced by planEmissions.
func dedupeEmissions(in []emission) []emission {
	seen := map[string]struct{}{}
	out := make([]emission, 0, len(in))
	for _, em := range in {
		key := em.Go + "|" + em.HTTP
		if _, dup := seen[key]; dup {
			continue
		}

		seen[key] = struct{}{}
		out = append(out, em)
	}

	return out
}

// goMethodNames lists every handler-side method goai recognises. Connect is
// deliberately omitted — gone supports it but it is rarely meaningful in API
// documentation.
var goMethodNames = []string{
	"Index", "Get", "Create", "Post", "Patch", "Put", "Delete", "Options", "Trace",
}

// detectOverrides returns a set keyed by Go method name indicating which of
// the canonical handler methods the handler declares directly (i.e. is NOT
// inherited unchanged through embedding).
//
// Detection mechanism: Go generates synthetic wrappers for every method
// that a leaf type promotes through embedded fields. These wrappers report
// their source location as "<autogenerated>" via runtime.FuncForPC.FileLine.
// A method declared directly on the leaf type — whether it overrides an
// embedded default or introduces a new method — reports a real source file
// path. Comparing the reported file against "<autogenerated>" gives a
// reliable override signal that is independent of how many embedded layers
// (e.g. ghttp.DefaultHTTPHandlerTask → endpoints.HandlerTask → leaf) sit
// between the runtime type and the canonical default.
func detectOverrides(handler ghttp.HandlerTask) map[string]bool {
	out := map[string]bool{}

	t := reflect.TypeOf(handler)
	if t == nil {
		return out
	}

	for _, name := range goMethodNames {
		m, ok := t.MethodByName(name)
		if !ok {
			continue
		}

		ptr := m.Func.Pointer()
		if ptr == 0 {
			continue
		}

		fn := runtime.FuncForPC(ptr)
		if fn == nil {
			continue
		}

		file, _ := fn.FileLine(ptr)
		if file == "" || file == "<autogenerated>" {
			continue
		}

		out[name] = true
	}

	return out
}

// collectInjectedParams gathers every PathParam declared by acceptances
// implementing one of the path-injection interfaces. Order is preserved
// (parent → child). The walker prefers the most specific interface an
// acceptance implements:
//
//  1. EmissionAwareParamInjector — receives both routePath and emission
//     context (HTTP method, Go method, item-level flag); used when an
//     injector needs to switch between Index-style and item-style
//     placeholder layouts on the same route.
//  2. PathAwareParamInjector — receives routePath only; used when the
//     same injector is reused under multiple route subtrees with
//     different placeholder conventions.
//  3. PathParamInjector — bare list, used when neither path nor
//     emission context is needed.
func collectInjectedParams(acceptances []ghttp.Acceptance, routePath string, ec EmissionContext) []PathParam {
	if len(acceptances) == 0 {
		return nil
	}

	var params []PathParam
	seen := map[string]struct{}{}
	for _, a := range acceptances {
		if a == nil {
			continue
		}

		var declared []PathParam
		if ea, ok := a.(EmissionAwareParamInjector); ok {
			declared = ea.InjectedParamsForEmission(routePath, ec)
		} else if pa, ok := a.(PathAwareParamInjector); ok {
			declared = pa.InjectedParamsFor(routePath)
		} else if injector, ok := a.(PathParamInjector); ok {
			declared = injector.InjectedParams()
		} else {
			continue
		}

		for _, p := range declared {
			key := p.In + ":" + p.Name
			if _, dup := seen[key]; dup {
				continue
			}

			seen[key] = struct{}{}
			params = append(params, p)
		}
	}

	return params
}

// collectSecurityForMethod gathers security requirements from acceptances
// for a single HTTP method. Each acceptance contributes at most one
// (scheme, scopes) pair. Acceptances that implement
// MethodAwareSecurityProvider get to scope their requirement per method;
// the rest fall back to the static SecurityProvider interface.
//
// The second return value reports whether at least one acceptance
// explicitly suppressed a security requirement on this method (i.e. its
// MethodAwareSecurityProvider returned "" while its static
// SecurityProvider would have returned a scheme). Builder uses this to
// emit an explicit `security: []` override that defeats document-level
// GlobalSecurity inheritance — without it, OpenAPI tooling would still
// flag the bearer-less operation as bearer-required.
func collectSecurityForMethod(acceptances []ghttp.Acceptance, httpMethod string) ([]SecurityRef, bool) {
	if len(acceptances) == 0 {
		return nil, false
	}

	method := strings.ToUpper(httpMethod)

	var refs []SecurityRef
	seen := map[string]struct{}{}
	explicitlyEmpty := false
	for _, a := range acceptances {
		if a == nil {
			continue
		}

		var (
			scheme  string
			scopes  []string
			skipped bool
		)

		if sp, ok := a.(MethodAwareSecurityProvider); ok {
			scheme, scopes = sp.SecurityRequirementFor(method)
			if scheme == "" {
				if generic, gok := a.(SecurityProvider); gok {
					if gscheme, _ := generic.SecurityRequirement(); gscheme != "" {
						skipped = true
					}
				}
			}
		} else if sp, ok := a.(SecurityProvider); ok {
			scheme, scopes = sp.SecurityRequirement()
		}

		if skipped {
			explicitlyEmpty = true
		}

		if scheme == "" {
			continue
		}

		if _, dup := seen[scheme]; dup {
			continue
		}

		seen[scheme] = struct{}{}
		refs = append(refs, SecurityRef{Scheme: scheme, Scopes: scopes})
	}

	return refs, explicitlyEmpty
}

// collectProfiles aggregates the union of profiles declared by the handler
// and the inherited acceptance chain. Returns the deduplicated list.
func collectProfiles(handler ghttp.HandlerTask, acceptances []ghttp.Acceptance) []string {
	seen := map[string]struct{}{}
	var out []string

	add := func(values []string) {
		for _, v := range values {
			if v == "" {
				continue
			}

			if _, dup := seen[v]; dup {
				continue
			}

			seen[v] = struct{}{}
			out = append(out, v)
		}
	}

	if scope, ok := handler.(ProfileScope); ok {
		add(scope.GOAIProfileScope())
	}

	for _, a := range acceptances {
		if scope, ok := a.(ProfileScope); ok {
			add(scope.GOAIProfileScope())
		}
	}

	return out
}

// expandPathWithParams takes the literal route path and inserts {name}
// placeholders per the supplied PathParam list.
//
// Behaviour:
//   - Params with In != "path" are ignored here (they go to query/header).
//   - Params with AnchorAfter == "" are appended at the end of the path.
//   - Params with AnchorAfter set are inserted directly after the matching
//     segment ONLY when the anchor segment is followed by at least one
//     more literal segment in the route. This preserves collection
//     endpoints — `/api/v1/teams` stays as the collection list and does
//     NOT become `/api/v1/teams/{teams_id}`, while
//     `/api/v1/teams/members` correctly becomes
//     `/api/v1/teams/{teams_id}/members`.
//   - Anchor-after params whose anchor segment is the LAST segment of
//     the path are dropped silently — the operation is item-level and
//     the walker may add its own `/{id}` separately.
//   - Anchor-after params whose anchor segment is NOT present in the
//     path at all are dropped silently — the injector advertises a
//     superset of possible params, so a missing anchor means this
//     particular route does not need that param. Falling back to
//     "append at end" would produce nonsense paths like
//     `/api/v1/teams/{teams_id}/members/{members_id}`
//     for routes that have no `members` segment.
//
// Anchor matching is case-sensitive, useful when routes use plural nouns
// like "teams".
func expandPathWithParams(path string, params []PathParam) string {
	if len(params) == 0 {
		if path == "" {
			return "/"
		}

		return path
	}

	clean := path
	if clean == "" {
		clean = "/"
	}

	segments := strings.Split(strings.TrimPrefix(clean, "/"), "/")
	insertions := map[int][]string{}
	var trailing []string

	for _, p := range params {
		if p.In != "" && p.In != "path" {
			continue
		}

		if p.Name == "" {
			continue
		}

		placeholder := "{" + p.Name + "}"
		anchor := strings.TrimSpace(p.AnchorAfter)
		if anchor == "" {
			trailing = append(trailing, placeholder)

			continue
		}

		// Locate the anchor segment. Three outcomes are possible:
		//   - anchor not present → drop silently. The injector advertises
		//     params for the union of routes attached to this acceptance,
		//     and routes missing the anchor segment do not own this param.
		//   - anchor IS the last segment → drop silently. The collection
		//     endpoint at the anchor itself (e.g. /api/v1/teams) must not
		//     pick up the anchored param.
		//   - anchor is present and followed by more segments → insert
		//     {placeholder} immediately after the anchor segment.
		for i, seg := range segments {
			if seg != anchor {
				continue
			}

			if i < len(segments)-1 {
				insertions[i] = append(insertions[i], placeholder)
			}

			break
		}
	}

	var out []string
	for i, seg := range segments {
		out = append(out, seg)
		out = append(out, insertions[i]...)
	}
	out = append(out, trailing...)

	return "/" + strings.Join(out, "/")
}
