package goai

import (
	"reflect"
	"sort"
	"sync"

	"github.com/yetiz-org/gone/ghttp"
	kklogger "github.com/yetiz-org/goth-kklogger"
)

// Entry captures a single registration: which handler+method, and which Go
// types describe its request/response bodies. req or resp may be nil when
// the operation takes no body or returns no body.
type Entry struct {
	Handler  ghttp.HandlerTask
	Method   string // upper-case HTTP method
	ReqType  reflect.Type
	RespType reflect.Type
	Spec     Spec
}

var (
	registryMu sync.RWMutex
	registry   = map[string]map[string]Entry{}
)

// Register associates a Spec, request type and response type with a
// (handler, method) pair. method is normalised to upper-case. req and
// resp may be nil; if non-nil, they should be a typed-nil pointer
// (e.g. (*MyResp)(nil)) so that reflect can extract the element type
// regardless of whether the value is the zero value of a struct.
//
// Calling Register more than once for the same handler+method overwrites
// the existing entry; this lets generated init() files coexist with
// hand-written project-level overrides.
func Register(handler ghttp.HandlerTask, method string, req any, resp any, opts ...Option) {
	if handler == nil {
		kklogger.WarnJ("goai:Register#input!nil_handler", "Register called with nil handler")

		return
	}

	if method == "" {
		kklogger.WarnJ("goai:Register#input!empty_method", "Register called with empty method")

		return
	}

	method = upper(method)
	key := handlerKey(handler)

	entry := Entry{
		Handler:  handler,
		Method:   method,
		ReqType:  extractType(req),
		RespType: extractType(resp),
		Spec:     NewSpec(opts...),
	}

	registryMu.Lock()
	defer registryMu.Unlock()

	bucket, ok := registry[key]
	if !ok {
		bucket = map[string]Entry{}
		registry[key] = bucket
	}

	bucket[method] = entry
}

// Lookup returns the Entry registered for (handler, method), if any.
func Lookup(handler ghttp.HandlerTask, method string) (Entry, bool) {
	if handler == nil {
		return Entry{}, false
	}

	registryMu.RLock()
	defer registryMu.RUnlock()

	bucket, ok := registry[handlerKey(handler)]
	if !ok {
		return Entry{}, false
	}

	entry, ok := bucket[upper(method)]

	return entry, ok
}

// LookupAll returns every Entry registered for handler, indexed by method.
// The returned map is a copy and safe to iterate.
func LookupAll(handler ghttp.HandlerTask) map[string]Entry {
	if handler == nil {
		return nil
	}

	registryMu.RLock()
	defer registryMu.RUnlock()

	bucket, ok := registry[handlerKey(handler)]
	if !ok {
		return nil
	}

	out := make(map[string]Entry, len(bucket))
	for method, entry := range bucket {
		out[method] = entry
	}

	return out
}

// All returns every Entry currently registered, sorted by handler-key for
// deterministic iteration.
func All() []Entry {
	registryMu.RLock()
	defer registryMu.RUnlock()

	keys := make([]string, 0, len(registry))
	for k := range registry {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	var out []Entry
	for _, k := range keys {
		bucket := registry[k]
		methods := make([]string, 0, len(bucket))
		for m := range bucket {
			methods = append(methods, m)
		}

		sort.Strings(methods)
		for _, m := range methods {
			out = append(out, bucket[m])
		}
	}

	return out
}

// Reset clears the registry. Intended for tests.
func Reset() {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry = map[string]map[string]Entry{}
}

// handlerKey returns a stable key for a handler value.
//
// gone projects follow a strict "one singleton per handler type" pattern
// (`var Handler<Name> = &<Name>{}`), so the fully-qualified Go type name is
// both unique enough and stable across calls. Using the type name avoids
// the trap of zero-sized struct pointer aliasing — the Go runtime is
// allowed to give multiple `&EmptyStruct{}` allocations the same address,
// which silently collapses pointer-based maps.
//
// Register keys by handler type, so more than one instance of the same
// handler type shares one entry. Projects that need per-instance
// customisation should embed differentiation into the type (e.g. distinct
// types per route) rather than reusing a single struct.
func handlerKey(h ghttp.HandlerTask) string {
	if h == nil {
		return ""
	}

	t := reflect.TypeOf(h)
	if t == nil {
		return ""
	}

	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	pkg := t.PkgPath()
	if pkg == "" {
		return t.String()
	}

	return pkg + "." + t.Name()
}

// extractType reads reflect.Type from a typed-nil pointer or any concrete
// value. Returns nil when the input is nil/untyped.
func extractType(v any) reflect.Type {
	if v == nil {
		return nil
	}

	t := reflect.TypeOf(v)
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	return t
}

// upper returns the upper-case form of an ASCII HTTP method name.
func upper(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' {
			c -= 'a' - 'A'
		}
		b[i] = c
	}

	return string(b)
}
