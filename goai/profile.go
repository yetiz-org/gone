package goai

import (
	"reflect"
	"strings"
)

// Profile is one named output bucket. Operations whose path/package/tag
// matches Include and does not match Exclude are emitted under this
// profile's output file.
type Profile struct {
	Name    string
	Include Selector
	Exclude Selector
}

// Selector chooses operations by path patterns, package paths, OpenAPI
// tags, or declared profile scopes. All non-empty fields combine with
// logical AND. A nil/empty Selector matches everything.
//
// Patterns:
//
//   - Paths use simple glob with "*" matching one segment and "**" matching
//     any number of segments.
//   - Packages match by literal prefix.
//   - Tags match the OpenAPI operation tags declared via Spec WithTag /
//     handler-level SpecProvider.GOAISpec / per-method providers, or via
//     goai.Register. This is what the README "tag-based filtering" copy
//     actually refers to.
//   - Profiles match the names returned by ProfileScope.GOAIProfileScope
//     (handler or acceptance level). Use this to attach an operation to
//     one of the profile buckets defined in goai.yaml when the operation
//     does not have a corresponding OpenAPI tag.
type Selector struct {
	Paths    []string `yaml:"paths,omitempty"`
	Packages []string `yaml:"packages,omitempty"`
	Tags     []string `yaml:"tags,omitempty"`
	Profiles []string `yaml:"profiles,omitempty"`
}

// IsEmpty reports whether the selector has no rules.
func (s Selector) IsEmpty() bool {
	return len(s.Paths) == 0 && len(s.Packages) == 0 && len(s.Tags) == 0 && len(s.Profiles) == 0
}

// Match reports whether the candidate matches every non-empty field on the
// selector. declaredProfiles is the union returned by ProfileScope across
// the handler and its acceptance chain; operationTags is the union of
// OpenAPI tags assigned to this operation by Spec / Register / classifier.
func (s Selector) Match(c OperationCandidate, declaredProfiles, operationTags []string) bool {
	if s.IsEmpty() {
		return true
	}

	if len(s.Paths) > 0 && !pathMatchesAny(c.Path, s.Paths) {
		return false
	}

	if len(s.Packages) > 0 && !packageMatchesAny(c.Handler, s.Packages) {
		return false
	}

	if len(s.Tags) > 0 {
		if !sliceIntersects(operationTags, s.Tags) {
			return false
		}
	}

	if len(s.Profiles) > 0 {
		if !sliceIntersects(declaredProfiles, s.Profiles) {
			return false
		}
	}

	return true
}

// Matches returns true when the candidate should appear in this profile's
// output. declaredProfiles supplies the values produced by ProfileScope or
// the BuiltinClassify result, so explicit profile declarations are
// respected. operationTags supplies the OpenAPI tags attached to this
// operation, so tag-based selectors work as the README documents.
func (p *Profile) Matches(c OperationCandidate, declaredProfiles, operationTags []string) bool {
	if p == nil {
		return true
	}

	// "all" profile is a special accept-all bucket.
	if strings.EqualFold(p.Name, "all") && p.Include.IsEmpty() && p.Exclude.IsEmpty() {
		return true
	}

	// Honour explicit ProfileScope declarations.
	if len(declaredProfiles) > 0 {
		for _, dp := range declaredProfiles {
			if strings.EqualFold(dp, p.Name) {
				return !excluded(c, p.Exclude, declaredProfiles, operationTags)
			}
		}
	}

	if !p.Include.Match(c, declaredProfiles, operationTags) {
		return false
	}

	if !p.Exclude.IsEmpty() && excluded(c, p.Exclude, declaredProfiles, operationTags) {
		return false
	}

	return true
}

// excluded reports whether the candidate matches *any* exclude rule. Unlike
// Selector.Match, exclusion uses logical OR across fields so a single hit
// is enough to drop the candidate.
func excluded(c OperationCandidate, sel Selector, declaredProfiles, operationTags []string) bool {
	if sel.IsEmpty() {
		return false
	}

	if pathMatchesAny(c.Path, sel.Paths) {
		return true
	}

	if packageMatchesAny(c.Handler, sel.Packages) {
		return true
	}

	if sliceIntersects(operationTags, sel.Tags) {
		return true
	}

	if sliceIntersects(declaredProfiles, sel.Profiles) {
		return true
	}

	return false
}

// BuiltinClassify is the no-op fallback classifier: every candidate lands in
// the "all" profile. Projects that need profile splits (public / internal /
// admin / etc.) supply their own classifier through BuildOptions, since the
// path conventions for those splits are project-specific.
func BuiltinClassify(c OperationCandidate) []string {
	_ = c

	return []string{"all"}
}

// pathMatchesAny reports whether path matches any of the supplied glob
// patterns. Patterns use:
//
//   - → exactly one path segment
//     ** → zero or more path segments
func pathMatchesAny(path string, patterns []string) bool {
	if len(patterns) == 0 {
		return false
	}

	segments := strings.Split(strings.Trim(path, "/"), "/")
	for _, pat := range patterns {
		if matchPathPattern(strings.Split(strings.Trim(pat, "/"), "/"), segments) {
			return true
		}
	}

	return false
}

// matchPathPattern is a small recursive segment matcher.
func matchPathPattern(pat, seg []string) bool {
	for i := 0; i < len(pat); i++ {
		token := pat[i]
		if token == "**" {
			// Try every possible suffix length.
			rest := pat[i+1:]
			for j := 0; j <= len(seg); j++ {
				if matchPathPattern(rest, seg[j:]) {
					return true
				}
			}

			return false
		}

		if i >= len(seg) {
			return false
		}

		if token == "*" {
			// Skip exactly one segment.
		} else if token != seg[i] {
			return false
		}
	}

	return len(seg) == len(pat)
}

// packageMatchesAny returns true when handler's package path has any of the
// supplied prefixes.
func packageMatchesAny(handler any, prefixes []string) bool {
	if handler == nil || len(prefixes) == 0 {
		return false
	}

	t := reflect.TypeOf(handler)
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	pkg := t.PkgPath()
	for _, p := range prefixes {
		if strings.HasPrefix(pkg, p) {
			return true
		}
	}

	return false
}

// sliceIntersects reports whether any element of a appears in b.
func sliceIntersects(a, b []string) bool {
	for _, x := range a {
		for _, y := range b {
			if x == y {
				return true
			}
		}
	}

	return false
}
