package goai

import (
	"reflect"
	"strings"

	"github.com/yetiz-org/gone/ghttp"
)

// Classifier turns a route path and its acceptance chain into reasonable
// default tag + security values. Builder consults it ONLY when a candidate
// has no explicit Spec entry from SpecProvider / goai.Register / SecurityProvider.
//
// Classifier is the zero-config fallback: it lets a project bootstrap a spec
// before any handler implements SpecProvider. Once handlers self-declare
// metadata, project-level Classifier rules become noise — prefer the
// per-handler interfaces and let the rule list decay to whatever truly
// belongs at the framework level (static assets, well-known routes, etc.).
type Classifier struct {
	// PathTagRules maps a path-prefix glob to a tag name. The first matching
	// rule wins; "**" matches anything.
	PathTagRules []PathTagRule
	// AcceptanceSecurityRules maps an acceptance type-name suffix to a
	// (scheme, scopes) pair. The first matching rule per acceptance wins.
	// Type-name suffix is matched case-sensitively against the unqualified
	// Go type name of the acceptance value.
	AcceptanceSecurityRules []AcceptanceSecurityRule
}

// PathTagRule maps one path glob to a tag.
//
// Glob syntax:
//   - "*" matches a single path segment.
//   - "**" matches any number of segments (including zero).
//   - Literal segments match exactly.
type PathTagRule struct {
	Pattern string
	Tag     string
}

// AcceptanceSecurityRule maps an acceptance type-name match to a security
// scheme reference.
type AcceptanceSecurityRule struct {
	// TypeName is matched against the unqualified Go type name of each
	// acceptance in the chain. Match is case-sensitive substring; the first
	// acceptance whose type name contains TypeName wins.
	TypeName string
	Scheme   string
	Scopes   []string
}

// DefaultClassifier returns an empty classifier. Projects supply their own
// PathTagRules and AcceptanceSecurityRules through BuildOptions; goai itself
// has no opinion on what tags or security schemes a route should map to.
func DefaultClassifier() *Classifier {
	return &Classifier{}
}

// ClassifyTag returns the first matching tag for path, or "" if none match.
func (c *Classifier) ClassifyTag(path string) string {
	if c == nil {
		return ""
	}

	for _, rule := range c.PathTagRules {
		if matchPathGlob(rule.Pattern, path) {
			return rule.Tag
		}
	}

	return ""
}

// ClassifySecurity returns the security refs derived from the acceptance
// chain. An acceptance that already implements SecurityProvider is honoured
// as-is; for all others, the type-name rules are consulted.
func (c *Classifier) ClassifySecurity(acceptances []ghttp.Acceptance) []SecurityRef {
	if c == nil {
		return nil
	}

	seen := map[string]struct{}{}
	var out []SecurityRef

	add := func(ref SecurityRef) {
		if ref.Scheme == "" {
			return
		}

		if _, dup := seen[ref.Scheme]; dup {
			return
		}

		seen[ref.Scheme] = struct{}{}
		out = append(out, ref)
	}

	for _, a := range acceptances {
		if a == nil {
			continue
		}

		if sp, ok := a.(SecurityProvider); ok {
			scheme, scopes := sp.SecurityRequirement()
			if scheme != "" {
				add(SecurityRef{Scheme: scheme, Scopes: scopes})

				continue
			}
		}

		typeName := acceptanceTypeName(a)
		for _, rule := range c.AcceptanceSecurityRules {
			if rule.TypeName != "" && strings.Contains(typeName, rule.TypeName) {
				add(SecurityRef{Scheme: rule.Scheme, Scopes: rule.Scopes})

				break
			}
		}
	}

	return out
}

// acceptanceTypeName returns the unqualified Go type name of the acceptance
// value, dereferencing pointers as needed.
func acceptanceTypeName(a ghttp.Acceptance) string {
	if a == nil {
		return ""
	}

	t := reflect.TypeOf(a)
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	if t == nil {
		return ""
	}

	return t.Name()
}

// matchPathGlob matches a path against a glob with "*" (single segment) and
// "**" (any number of segments) wildcards.
func matchPathGlob(pattern, path string) bool {
	if pattern == "" {
		return false
	}

	if pattern == path {
		return true
	}

	pParts := splitSegments(pattern)
	tParts := splitSegments(path)

	return globMatch(pParts, tParts)
}

func splitSegments(p string) []string {
	p = strings.TrimPrefix(p, "/")
	if p == "" {
		return nil
	}

	return strings.Split(p, "/")
}

func globMatch(pattern, target []string) bool {
	pi, ti := 0, 0
	for pi < len(pattern) {
		seg := pattern[pi]
		switch {
		case seg == "**":
			if pi == len(pattern)-1 {
				return true
			}

			for k := ti; k <= len(target); k++ {
				if globMatch(pattern[pi+1:], target[k:]) {
					return true
				}
			}

			return false
		case ti >= len(target):
			return false
		case seg == "*":
			pi++
			ti++
		case strings.HasSuffix(seg, "**"):
			prefix := strings.TrimSuffix(seg, "**")
			if !strings.HasPrefix(target[ti], prefix) {
				return false
			}

			pi++
			ti++
		case strings.HasSuffix(seg, "*"):
			prefix := strings.TrimSuffix(seg, "*")
			if !strings.HasPrefix(target[ti], prefix) {
				return false
			}

			pi++
			ti++
		case seg == target[ti]:
			pi++
			ti++
		default:
			return false
		}
	}

	return ti == len(target)
}
