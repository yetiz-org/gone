package goai

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/yetiz-org/gone/ghttp"
	"gopkg.in/yaml.v3"
)

// RouteFactory builds the project's route tree on demand. RunCLI calls it
// after ghttp.SetSkipHandlerRegister(true) so route construction must not
// depend on real DB/cache/queue connectivity.
type RouteFactory func() ghttp.RouteEntriesProvider

// RunOptions holds the project-specific spec configuration. Only Servers,
// Tags, GlobalSecurity and SecuritySchemes are typically project-specific —
// the rest have sensible defaults.
type RunOptions struct {
	// Title is the OpenAPI info.title default. CLI flag -title overrides.
	Title string

	// Description is the OpenAPI info.description.
	Description string

	// Version is the OpenAPI info.version default. CLI flag -version overrides.
	Version string

	// TermsOfService is the OpenAPI info.termsOfService URL.
	TermsOfService string

	// Contact populates Document.Info.Contact.
	Contact *Contact

	// License populates Document.Info.License (e.g. {Name: "MIT"}).
	License *License

	// ExternalDocs populates the top-level Document.ExternalDocs.
	ExternalDocs *ExternalDocumentation

	// Servers populates Document.Servers verbatim.
	Servers []Server

	// Tags populates Document.Tags verbatim. Hand-listing them controls the
	// final tag ordering in the emitted yaml.
	Tags []Tag

	// GlobalSecurity is propagated as Document.Security (the top-level
	// requirement applied to every operation that does not declare its own).
	GlobalSecurity []map[string][]string

	// TagSecurityClassifier provides per-path tag and per-acceptance security
	// fallbacks. nil → goai.DefaultClassifier() inside Build.
	TagSecurityClassifier *Classifier

	// SecuritySchemes are merged into Document.Components.SecuritySchemes
	// after Build. Wins over any classifier-injected scheme reference that
	// happens to share a name.
	SecuritySchemes map[string]*SecurityScheme

	// DefaultOutput is the -o flag default. Must be a writable file path or
	// "-" for stdout. Empty string falls back to "openapi.generated.yaml".
	DefaultOutput string

	// BaseSpec, when non-nil, is the hand-tuned OpenAPI yaml that the
	// auto-generated structure is merged onto. The hand-tuned content wins
	// for everything that is already present (info, tags, paths,
	// components.schemas, components.securitySchemes); auto-discovered
	// entries fill gaps. Use this when the project keeps a curated
	// `docs/openapi/openapi.yaml` as the canonical source of truth and
	// wants the generator to surface routes absent from it.
	//
	// Mutually exclusive with BaseSpecPath; BaseSpec wins when both are set.
	BaseSpec []byte

	// BaseSpecPath, when non-empty, is the filesystem path to the
	// hand-tuned OpenAPI yaml. RunCLI loads it on startup and feeds the
	// bytes through the same merge path as BaseSpec. Empty path means "no
	// merge, emit pure auto-generated yaml".
	BaseSpecPath string

	// RestrictToBaseSpecPaths, when true and a base spec is supplied,
	// constrains the generator to ONLY emit operations whose path also
	// appears in the base spec. Auto-discovered routes absent from the
	// hand-tuned yaml are dropped from the output.
	//
	// Use this when the curated `docs/openapi/openapi.yaml` is treated as
	// the canonical endpoint registry — i.e. a route does not "exist" for
	// API consumers until it has been documented. This is the strict
	// inverse of the default behavior, which surfaces every walked route
	// regardless of base-spec coverage.
	RestrictToBaseSpecPaths bool

	// ExcludePaths is an explicit blocklist of path globs. Paths matching
	// any of these patterns are dropped from the generated document
	// before merge, regardless of whether RestrictToBaseSpecPaths is set.
	//
	// Glob syntax: "*" matches one segment, "**" matches any number of
	// segments. Examples:
	//
	//	{"/static/**", "/favicon.ico", "/robots.txt"} // drop static asset routes
	//	{"/debug/**"}                                  // drop debug-only routes
	ExcludePaths []string

	// OperationDocExtractor enables source-level Go doc-comment extraction
	// for operation-level OpenAPI metadata. The built-in extractor only
	// reads namespaced `@goai.*` directives from handler struct and method
	// docs, so ordinary implementation comments stay private. Use
	// DefaultOperationDocExtractor() or
	// DefaultOperationDocExtractorWithBuildTags() to turn on the AST-based
	// implementation; pass nil (the default) to disable docstring
	// fallback entirely.
	//
	// Explicit Spec values, route-derived parameters, generated schemas,
	// and acceptance-derived security always win; docstring content only
	// fills gaps.
	OperationDocExtractor OperationDocExtractor

	// Args is the argv slice the CLI parses. nil → os.Args[1:].
	Args []string

	// Stdout is where yaml goes when DefaultOutput resolves to "-". nil →
	// os.Stdout. Tests inject a buffer here.
	Stdout io.Writer

	// Stderr is where progress and error messages go. nil → os.Stderr.
	Stderr io.Writer

	// Exit is invoked on terminal failure with a non-zero code. nil →
	// os.Exit. Tests inject a recorder.
	Exit func(int)
}

// RunCLI is the canonical entry point for project-side `cmd/goaispec` (or
// equivalent) binaries. It parses CLI flags, walks the supplied route tree,
// builds an OpenAPI 3.0.3 document, and writes the yaml to the resolved
// output destination.
//
// Typical usage:
//
//	func main() {
//		os.Setenv("APP_DEBUG", "true")
//		goai.RunCLI(
//			func() ghttp.RouteEntriesProvider { return handlers.NewAppRoute() },
//			goai.RunOptions{
//				Title:         "My API",
//				Version:       "1.0.0",
//				Servers:       []goai.Server{{URL: "https://api.example.com"}},
//				DefaultOutput: "docs/openapi.yaml",
//				SecuritySchemes: map[string]*goai.SecurityScheme{
//					"OAuth2": {Type: "oauth2", Flows: ...},
//				},
//			},
//		)
//	}
//
// RunCLI does NOT panic on failure; it writes a diagnostic to Stderr and
// invokes Exit(1). Callers that want non-fatal behaviour should call Walk,
// Build and EmitYAML directly.
func RunCLI(factory RouteFactory, opts RunOptions) {
	stdout := opts.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}

	stderr := opts.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}

	exit := opts.Exit
	if exit == nil {
		exit = os.Exit
	}

	args := opts.Args
	if args == nil {
		args = os.Args[1:]
	}

	defaultOut := opts.DefaultOutput
	if defaultOut == "" {
		defaultOut = "openapi.generated.yaml"
	}

	fs := flag.NewFlagSet("goaispec", flag.ContinueOnError)
	fs.SetOutput(stderr)
	output := fs.String("o", defaultOut, "output path; '-' for stdout")
	title := fs.String("title", opts.Title, "OpenAPI info.title")
	version := fs.String("version", opts.Version, "OpenAPI info.version")
	if err := fs.Parse(args); err != nil {
		exit(2)

		return
	}

	if factory == nil {
		fmt.Fprintln(stderr, "goai: route factory is nil")
		exit(1)

		return
	}

	prev := ghttp.SetSkipHandlerRegister(true)
	defer ghttp.SetSkipHandlerRegister(prev)

	route := factory()
	if route == nil {
		fmt.Fprintln(stderr, "goai: route factory returned nil")
		exit(1)

		return
	}

	candidates := Walk(route)
	if len(candidates) == 0 {
		fmt.Fprintln(stderr, "goai: walker returned 0 candidates — route tree is empty")
		exit(1)

		return
	}

	// Load base spec early so its path inventory can drive the include filter.
	baseSpec := opts.BaseSpec
	if len(baseSpec) == 0 && opts.BaseSpecPath != "" {
		loaded, err := os.ReadFile(opts.BaseSpecPath)
		if err != nil {
			fmt.Fprintf(stderr, "goai: read base spec %s: %v\n", opts.BaseSpecPath, err)
			exit(1)

			return
		}

		baseSpec = loaded
	}

	profile := buildPathFilterProfile(baseSpec, opts.RestrictToBaseSpecPaths, opts.ExcludePaths)

	doc := Build(candidates, profile, BuildOptions{
		Title:                 *title,
		Description:           opts.Description,
		Version:               *version,
		TermsOfService:        opts.TermsOfService,
		Contact:               opts.Contact,
		License:               opts.License,
		Servers:               opts.Servers,
		Tags:                  opts.Tags,
		GlobalSecurity:        opts.GlobalSecurity,
		ExternalDocs:          opts.ExternalDocs,
		TagSecurityClassifier: opts.TagSecurityClassifier,
		OperationDocExtractor: opts.OperationDocExtractor,
	})

	if doc.Components == nil {
		doc.Components = NewComponents()
	}

	for name, scheme := range opts.SecuritySchemes {
		doc.Components.SecuritySchemes[name] = scheme
	}

	body, err := EmitYAML(doc)
	if err != nil {
		fmt.Fprintf(stderr, "goai: emit failed: %v\n", err)
		exit(1)

		return
	}

	if len(baseSpec) > 0 {
		merged, err := Merge3Way(body, baseSpec, nil)
		if err != nil {
			fmt.Fprintf(stderr, "goai: merge with base spec failed: %v\n", err)
			exit(1)

			return
		}

		body = merged
	}

	if *output == "-" {
		if _, err := stdout.Write(body); err != nil {
			fmt.Fprintf(stderr, "goai: stdout write failed: %v\n", err)
			exit(1)

			return
		}

		return
	}

	if err := os.WriteFile(*output, body, 0o644); err != nil {
		fmt.Fprintf(stderr, "goai: write %s failed: %v\n", *output, err)
		exit(1)

		return
	}

	finalPaths, finalOps := countPathsAndOpsFromYAML(body)
	if finalPaths == 0 && finalOps == 0 {
		finalPaths = len(doc.Paths)
		finalOps = CountOperations(doc)
	}

	mergeNote := ""
	if len(baseSpec) > 0 {
		mergeNote = " (merged with base spec)"
	}

	fmt.Fprintf(stderr, "goai: wrote %d bytes to %s (%d operations across %d paths)%s\n",
		len(body), *output, finalOps, finalPaths, mergeNote)
}

// buildPathFilterProfile assembles a Profile that filters operations based
// on the supplied path policy. Returns nil when neither
// RestrictToBaseSpecPaths nor ExcludePaths is in effect — Build() treats a
// nil Profile as "accept everything".
//
// The Profile uses Selector.Paths (glob-aware) for both Include and
// Exclude, so callers can mix exact paths from the base spec with wildcard
// patterns like "/static/**" without changing the matcher.
func buildPathFilterProfile(baseSpec []byte, restrictToBase bool, exclude []string) *Profile {
	useRestrict := restrictToBase && len(baseSpec) > 0
	if !useRestrict && len(exclude) == 0 {
		return nil
	}

	profile := &Profile{Name: "filtered"}
	if useRestrict {
		profile.Include = Selector{Paths: extractPathsFromYAML(baseSpec)}
	}

	if len(exclude) > 0 {
		profile.Exclude = Selector{Paths: append([]string(nil), exclude...)}
	}

	return profile
}

// filterYAMLByPaths returns a copy of body with the `paths:` mapping pruned
// down to entries whose key is in `allowed`. Sections other than `paths`
// are left untouched. An empty allowed list is treated as "no paths
// pass" — the returned yaml has an empty `paths:` map. Returns body
// unchanged only when the document has no paths section.
func filterYAMLByPaths(body []byte, allowed []string) []byte {
	if len(body) == 0 {
		return body
	}

	allow := make(map[string]struct{}, len(allowed))
	for _, p := range allowed {
		allow[p] = struct{}{}
	}

	return filterPathsByPredicate(body, func(path string) bool {
		_, ok := allow[path]

		return ok
	})
}

// filterYAMLByPathGlobs returns a copy of body with the `paths:` mapping
// pruned down to entries whose key matches the include glob list AND does
// not match the exclude glob list. Glob syntax follows the same rules as
// Selector path matching ("*" → one segment, "**" → any number of
// segments). Returns body unchanged when neither include nor exclude is
// supplied. An empty include list matches every base-spec path; non-empty
// excludes can still drop matches.
//
// Use this for per-profile yaml emission: walker-produced paths often use
// the literal mounted form (e.g. /api/v1/teams/members) while the
// hand-tuned base spec uses parameterised forms
// (/api/v1/teams/{teams_id}/members). Filtering the merge source by
// literal walker paths drops the parameterised entries; glob filtering
// keeps them.
func filterYAMLByPathGlobs(body []byte, includes, excludes []string) []byte {
	if len(body) == 0 || (len(includes) == 0 && len(excludes) == 0) {
		return body
	}

	return filterPathsByPredicate(body, func(path string) bool {
		if len(includes) > 0 && !pathMatchesAnyGlob(path, includes) {
			return false
		}

		if len(excludes) > 0 && pathMatchesAnyGlob(path, excludes) {
			return false
		}

		return true
	})
}

// filterPathsByPredicate is the shared core for filterYAMLByPaths and
// filterYAMLByPathGlobs. keep reports whether a given path key should be
// retained in the output.
func filterPathsByPredicate(body []byte, keep func(path string) bool) []byte {
	var doc yaml.Node
	if err := yaml.Unmarshal(body, &doc); err != nil {
		return body
	}

	root := documentRoot(&doc)
	if root == nil || root.Kind != yaml.MappingNode {
		return body
	}

	paths := mappingValue(root, "paths")
	if paths == nil || paths.Kind != yaml.MappingNode {
		return body
	}

	kept := make([]*yaml.Node, 0, len(paths.Content))
	for i := 0; i+1 < len(paths.Content); i += 2 {
		key := paths.Content[i]
		if !keep(key.Value) {
			continue
		}

		kept = append(kept, paths.Content[i], paths.Content[i+1])
	}

	paths.Content = kept

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		_ = enc.Close()

		return body
	}

	if err := enc.Close(); err != nil {
		return body
	}

	return buf.Bytes()
}

// extractPathsFromYAML returns the literal path keys declared under
// `paths:` in an OpenAPI yaml document. Returns an empty slice when the
// input is empty, unparseable, or carries no paths section.
func extractPathsFromYAML(body []byte) []string {
	if len(body) == 0 {
		return nil
	}

	var node yaml.Node
	if err := yaml.Unmarshal(body, &node); err != nil {
		return nil
	}

	root := documentRoot(&node)
	if root == nil || root.Kind != yaml.MappingNode {
		return nil
	}

	paths := mappingValue(root, "paths")
	if paths == nil || paths.Kind != yaml.MappingNode {
		return nil
	}

	out := make([]string, 0, len(paths.Content)/2)
	for i := 0; i+1 < len(paths.Content); i += 2 {
		out = append(out, paths.Content[i].Value)
	}

	return out
}

// countPathsAndOpsFromYAML walks a yaml.Node tree built from the supplied
// bytes and returns the number of distinct paths plus the total number of
// HTTP operations beneath them. Returns (0, 0) when the input is not a
// valid OpenAPI document. Used after Merge3Way so the diagnostic line
// reflects the final emitted yaml — including hand-tuned paths the
// in-memory Document does not know about.
func countPathsAndOpsFromYAML(body []byte) (int, int) {
	if len(body) == 0 {
		return 0, 0
	}

	var node yaml.Node
	if err := yaml.Unmarshal(body, &node); err != nil {
		return 0, 0
	}

	root := documentRoot(&node)
	if root == nil || root.Kind != yaml.MappingNode {
		return 0, 0
	}

	paths := mappingValue(root, "paths")
	if paths == nil || paths.Kind != yaml.MappingNode {
		return 0, 0
	}

	httpVerbs := map[string]bool{
		"get": true, "put": true, "post": true, "delete": true,
		"options": true, "head": true, "patch": true, "trace": true,
	}

	pathCount := 0
	opCount := 0
	for i := 0; i+1 < len(paths.Content); i += 2 {
		pathCount++
		item := paths.Content[i+1]
		if item.Kind != yaml.MappingNode {
			continue
		}

		for j := 0; j+1 < len(item.Content); j += 2 {
			if httpVerbs[item.Content[j].Value] {
				opCount++
			}
		}
	}

	return pathCount, opCount
}

// RunFromConfigOption tunes RunCLIFromConfig.
type RunFromConfigOption func(*runFromConfigOpts)

type runFromConfigOpts struct {
	classifier *Classifier
}

// WithClassifier injects a fallback Classifier for handlers that do not
// implement SpecProvider or SecurityProvider.
func WithClassifier(c *Classifier) RunFromConfigOption {
	return func(o *runFromConfigOpts) { o.classifier = c }
}

// RunCLIFromConfig is the multi-output entry point. It loads goai.yaml
// from configPath, walks the route tree once, and emits one yaml per
// declared profile in the config (or a single yaml when no profiles are
// declared). The hand-tuned base spec, when configured, is merged into
// every emitted yaml.
//
// configPath may point at either the goai.yaml file directly or the
// directory containing it.
func RunCLIFromConfig(configPath string, factory RouteFactory, opts ...RunFromConfigOption) {
	stderr := os.Stderr
	exit := os.Exit

	o := runFromConfigOpts{}
	for _, fn := range opts {
		fn(&o)
	}

	cfgDir := configPath
	if info, err := os.Stat(configPath); err == nil && !info.IsDir() {
		cfgDir = filepath.Dir(configPath)
	}

	cfg, err := LoadConfig(cfgDir)
	if err != nil {
		fmt.Fprintf(stderr, "goai: load config: %v\n", err)
		exit(1)

		return
	}

	if factory == nil {
		fmt.Fprintln(stderr, "goai: route factory is nil")
		exit(1)

		return
	}

	prev := ghttp.SetSkipHandlerRegister(true)
	defer ghttp.SetSkipHandlerRegister(prev)

	route := factory()
	if route == nil {
		fmt.Fprintln(stderr, "goai: route factory returned nil")
		exit(1)

		return
	}

	candidates := Walk(route)
	if len(candidates) == 0 {
		fmt.Fprintln(stderr, "goai: walker returned 0 candidates — route tree is empty")
		exit(1)

		return
	}

	var baseSpec []byte
	if cfg.BaseSpecPath != "" {
		baseSpecPath := resolveConfigPath(cfgDir, cfg.BaseSpecPath)
		baseSpec, err = os.ReadFile(baseSpecPath)
		if err != nil {
			fmt.Fprintf(stderr, "goai: read base spec %s: %v\n", baseSpecPath, err)
			exit(1)

			return
		}
	}

	build := cfg.ToBuildOptions()
	build.TagSecurityClassifier = o.classifier

	// Apply framework-level path filtering once. The resulting candidate
	// list is what every profile shares; per-profile selectors then narrow
	// further by tag/package/path.
	candidates = applyFrameworkFilter(candidates, baseSpec, cfg.RestrictToBaseSpecPaths, cfg.ExcludePaths)

	if len(cfg.Profiles) == 0 {
		out := cfg.DefaultOutput
		if out == "" {
			out = "openapi.generated.yaml"
		}

		out = resolveConfigPath(cfgDir, out)

		if err := emitOne(stderr, candidates, nil, build, cfg.SecuritySchemes, baseSpec, false, nil, out); err != nil {
			fmt.Fprintf(stderr, "goai: %v\n", err)
			exit(1)
		}

		return
	}

	for name := range cfg.Profiles {
		profile := cfg.BuildProfile(name)

		out := cfg.Output[name]
		if out == "" {
			out = "openapi." + name + ".yaml"
		}

		out = resolveConfigPath(cfgDir, out)

		if err := emitOne(stderr, candidates, profile, build, cfg.SecuritySchemes, baseSpec, false, nil, out); err != nil {
			fmt.Fprintf(stderr, "goai: profile %s: %v\n", name, err)
			exit(1)

			return
		}
	}
}

// resolveConfigPath joins relative paths declared in goai.yaml against the
// config file's directory, matching the contract documented on Config.Output.
// "-" (stdout marker) and absolute paths pass through unchanged.
func resolveConfigPath(cfgDir, path string) string {
	if path == "" || path == "-" {
		return path
	}

	if filepath.IsAbs(path) {
		return path
	}

	return filepath.Join(cfgDir, path)
}

// applyFrameworkFilter drops candidates whose path is in ExcludePaths or
// (when RestrictToBaseSpecPaths is set) is not declared in the base spec.
// Returns the original slice when neither restriction is active.
func applyFrameworkFilter(candidates []OperationCandidate, baseSpec []byte, restrictToBase bool, excludePaths []string) []OperationCandidate {
	if !restrictToBase && len(excludePaths) == 0 {
		return candidates
	}

	var basePaths map[string]struct{}
	if restrictToBase && len(baseSpec) > 0 {
		basePaths = map[string]struct{}{}
		for _, p := range extractPathsFromYAML(baseSpec) {
			basePaths[p] = struct{}{}
		}
	}

	out := candidates[:0:0]
	for _, c := range candidates {
		if basePaths != nil {
			if _, ok := basePaths[c.Path]; !ok {
				continue
			}
		}

		if pathMatchesAnyGlob(c.Path, excludePaths) {
			continue
		}

		out = append(out, c)
	}

	return out
}

// pathMatchesAnyGlob mirrors the glob semantics of Selector path matching.
func pathMatchesAnyGlob(path string, patterns []string) bool {
	if len(patterns) == 0 {
		return false
	}

	segments := splitSegments(path)
	for _, pat := range patterns {
		if globMatch(splitSegments(pat), segments) {
			return true
		}
	}

	return false
}

// emitOne builds, emits, optionally merges, and writes a single yaml file.
func emitOne(stderr io.Writer, candidates []OperationCandidate, profile *Profile, build BuildOptions, securitySchemes map[string]*SecurityScheme, baseSpec []byte, restrictToBase bool, exclude []string, output string) error {
	if profile == nil {
		profile = buildPathFilterProfile(baseSpec, restrictToBase, exclude)
	}

	doc := Build(candidates, profile, build)

	if doc.Components == nil {
		doc.Components = NewComponents()
	}

	for name, scheme := range securitySchemes {
		doc.Components.SecuritySchemes[name] = scheme
	}

	body, err := EmitYAML(doc)
	if err != nil {
		return fmt.Errorf("emit failed: %w", err)
	}

	mergeSource := baseSpec
	if len(mergeSource) > 0 && profile != nil && (!profile.Include.IsEmpty() || !profile.Exclude.IsEmpty()) {
		// Per-profile output: filter the base spec down to paths that match
		// this profile's selectors. Glob-aware so that base-spec paths with
		// {param} placeholders (e.g. /api/v1/teams/{teams_id}/members) are
		// kept when the profile's include glob covers them (e.g.
		// /api/v1/**), even if the walker only produced the literal
		// mounted form (/api/v1/teams/members).
		//
		// Falls back to walker-produced paths when the selector has no path
		// rules — used by profile selectors that key off tags or packages
		// rather than path globs.
		if len(profile.Include.Paths) > 0 || len(profile.Exclude.Paths) > 0 {
			mergeSource = filterYAMLByPathGlobs(baseSpec, profile.Include.Paths, profile.Exclude.Paths)
		} else {
			allowed := make([]string, 0, len(doc.Paths))
			for p := range doc.Paths {
				allowed = append(allowed, p)
			}

			mergeSource = filterYAMLByPaths(baseSpec, allowed)
		}
	}

	if len(mergeSource) > 0 {
		merged, err := Merge3Way(body, mergeSource, nil)
		if err != nil {
			return fmt.Errorf("merge with base spec failed: %w", err)
		}

		body = merged
	}

	if output == "-" {
		_, err = os.Stdout.Write(body)

		return err
	}

	if err := os.WriteFile(output, body, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", output, err)
	}

	finalPaths, finalOps := countPathsAndOpsFromYAML(body)
	if finalPaths == 0 && finalOps == 0 {
		finalPaths = len(doc.Paths)
		finalOps = CountOperations(doc)
	}

	mergeNote := ""
	if len(baseSpec) > 0 {
		mergeNote = " (merged with base spec)"
	}

	fmt.Fprintf(stderr, "goai: wrote %d bytes to %s (%d operations across %d paths)%s\n",
		len(body), output, finalOps, finalPaths, mergeNote)

	return nil
}

// CountOperations returns the total number of HTTP operations declared in
// the document across all paths and methods. Useful for CLI diagnostics
// and snapshot tests.
func CountOperations(doc *Document) int {
	if doc == nil {
		return 0
	}

	count := 0
	for _, p := range doc.Paths {
		if p == nil {
			continue
		}

		if p.Get != nil {
			count++
		}

		if p.Post != nil {
			count++
		}

		if p.Put != nil {
			count++
		}

		if p.Patch != nil {
			count++
		}

		if p.Delete != nil {
			count++
		}

		if p.Options != nil {
			count++
		}

		if p.Head != nil {
			count++
		}

		if p.Trace != nil {
			count++
		}
	}

	return count
}
