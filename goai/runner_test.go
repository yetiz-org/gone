package goai

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yetiz-org/gone/channel"
	"github.com/yetiz-org/gone/ghttp"
	firstmodels "github.com/yetiz-org/gone/goai/internal/testfixtures/collision/first/models"
	secondmodels "github.com/yetiz-org/gone/goai/internal/testfixtures/collision/second/models"
)

const _TwoProfileYAML = `title: API
version: 1.0.0
profiles:
  public:
    include:
      paths:
        - /public/**
  mgmt:
    include:
      paths:
        - /mgmt/**
output:
  public: public.yaml
  mgmt: mgmt.yaml
`

type _PublicHelloHandler struct {
	ghttp.DefaultHTTPHandlerTask
}

func (h *_PublicHelloHandler) Get(ctx channel.HandlerContext, req *ghttp.Request, resp *ghttp.Response, params map[string]any) (errResp ghttp.ErrorResponse) {
	return nil
}

type _MgmtHelloHandler struct {
	ghttp.DefaultHTTPHandlerTask
}

func (h *_MgmtHelloHandler) Get(ctx channel.HandlerContext, req *ghttp.Request, resp *ghttp.Response, params map[string]any) (errResp ghttp.ErrorResponse) {
	return nil
}

type _IsolationHandler struct {
	ghttp.DefaultHTTPHandlerTask
	specCalls *atomic.Int32
}

func (h *_IsolationHandler) Get(ctx channel.HandlerContext, req *ghttp.Request, resp *ghttp.Response, params map[string]any) (errResp ghttp.ErrorResponse) {
	return nil
}

func (h *_IsolationHandler) GOAISpec() (spec Spec) {
	if h.specCalls != nil {
		h.specCalls.Add(1)
	}

	return NewSpec(
		WithCallback("onEvent", Callback{
			"{$request.body#/url}": &PathItem{
				Post: &Operation{
					OperationID: "orig-cb",
					Responses: map[string]*Response{
						"200": {
							Description: "orig-cb-resp",
							Content: map[string]*MediaType{
								"application/json": {Schema: &Schema{Type: "object", Description: "orig-cb-schema"}},
							},
						},
					},
				},
			},
		}),
		WithResponse("400", "orig-400",
			WithResponseSchema(reflect.TypeOf(_IsolationNamedBody{})),
			WithResponseSchemaPrebuilt(&Schema{Type: "object", Description: "orig-400-schema"}),
			WithResponseExampleObject("named", &Example{Summary: "orig-400-ex"}),
		),
		WithExample("application/json", map[string]any{"k": "v"}),
		WithExampleObject("application/json", "named", &Example{Summary: "orig-ex", Value: map[string]any{"n": 1}}),
	)
}

type _IsolationNamedBody struct {
	ID string `json:"id"`
}

var _isolationSchemaNameCalls atomic.Int32

func (*_IsolationNamedBody) GOAISchemaName() string {
	_isolationSchemaNameCalls.Add(1)

	return "IsolationNamedBody"
}

func TestRunCLIFromConfigConcurrencyPrecedence(t *testing.T) {
	tests := []struct {
		name    string
		cli     *int
		option  *int
		yaml    int
		want    int
		wantErr bool
	}{
		{name: "builtin when yaml option and cli unset", yaml: 0, want: 1},
		{name: "yaml when option and cli unset", yaml: 4, want: 4},
		{name: "option over yaml", option: new(2), yaml: 4, want: 2},
		{name: "cli over option and yaml", cli: new(3), option: new(2), yaml: 4, want: 3},
		{name: "explicit cli zero overrides then defaults", cli: new(0), option: new(2), yaml: 4, want: 1},
		{name: "explicit option zero overrides yaml", option: new(0), yaml: 4, want: 1},
		{name: "option over yaml zero", option: new(2), yaml: 0, want: 2},
		{name: "explicit cli zero over yaml", cli: new(0), yaml: 4, want: 1},
		{name: "negative cli is invalid", cli: new(-1), option: new(2), yaml: 4, wantErr: true},
		{name: "negative option is invalid", option: new(-1), yaml: 4, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := _ResolveRunConcurrency(tc.cli, tc.option, tc.yaml)
			if tc.wantErr {
				assert.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}

	dir := t.TempDir()
	_WriteGoaiYAML(t, dir, _TwoProfileYAML)
	code := _RunCLIFromConfig(dir, _NewPublicMgmtFactory(nil), io.Discard, io.Discard,
		WithConcurrency(2),
		WithArgs([]string{"-concurrency", "0"}),
	)
	assert.Equal(t, 0, code)
	assert.FileExists(t, filepath.Join(dir, "public.yaml"))
	assert.FileExists(t, filepath.Join(dir, "mgmt.yaml"))
}

func TestRunCLIFromConfigArgsAndProfileSelection(t *testing.T) {
	origArgs := append([]string(nil), os.Args...)
	t.Cleanup(func() { os.Args = origArgs })
	t.Run("ambient args ignored without WithArgs", func(t *testing.T) {
		dir := t.TempDir()
		_WriteGoaiYAML(t, dir, _TwoProfileYAML)
		os.Args = []string{"goai", "-profile", "public"}
		code := _RunCLIFromConfig(dir, _NewPublicMgmtFactory(nil), io.Discard, io.Discard)
		assert.Equal(t, 0, code)
		assert.FileExists(t, filepath.Join(dir, "public.yaml"))
		assert.FileExists(t, filepath.Join(dir, "mgmt.yaml"))
	})

	t.Run("WithArgs selects one profile", func(t *testing.T) {
		dir := t.TempDir()
		_WriteGoaiYAML(t, dir, _TwoProfileYAML)
		code := _RunCLIFromConfig(dir, _NewPublicMgmtFactory(nil), io.Discard, io.Discard,
			WithArgs([]string{"-profile", "public"}),
		)

		assert.Equal(t, 0, code)
		assert.FileExists(t, filepath.Join(dir, "public.yaml"))
		assert.NoFileExists(t, filepath.Join(dir, "mgmt.yaml"))
	})

	t.Run("WithArgs profile flag is repeatable", func(t *testing.T) {
		dir := t.TempDir()
		_WriteGoaiYAML(t, dir, _TwoProfileYAML)
		code := _RunCLIFromConfig(dir, _NewPublicMgmtFactory(nil), io.Discard, io.Discard,
			WithArgs([]string{"-profile", "public", "-profile", "mgmt"}),
		)

		assert.Equal(t, 0, code)
		assert.FileExists(t, filepath.Join(dir, "public.yaml"))
		assert.FileExists(t, filepath.Join(dir, "mgmt.yaml"))
	})

	t.Run("selected profiles keep lexical order", func(t *testing.T) {
		dir := t.TempDir()
		_WriteGoaiYAML(t, dir, _TwoProfileYAML)
		cfg, err := LoadConfig(dir)
		require.NoError(t, err)
		jobs, usageErr, cfgErr := _ResolveRunTargets(cfg, dir, _runCLIFromConfigFlags{
			profiles: []string{"public", "mgmt"},
		})
		require.NoError(t, usageErr)
		require.NoError(t, cfgErr)
		require.Len(t, jobs, 2)
		assert.Equal(t, []string{"mgmt", "public"}, []string{jobs[0].Name, jobs[1].Name})
	})

	t.Run("duplicate profile is an error", func(t *testing.T) {
		dir := t.TempDir()
		_WriteGoaiYAML(t, dir, _TwoProfileYAML)
		called := false
		code := _RunCLIFromConfigRecorded(dir, _NewPublicMgmtFactory(&called), WithArgs([]string{"-profile", "public", "-profile", "public"}))
		assert.False(t, called)
		assert.Equal(t, 2, code)
		assert.NoFileExists(t, filepath.Join(dir, "public.yaml"))
		assert.NoFileExists(t, filepath.Join(dir, "mgmt.yaml"))
	})

	t.Run("profile names are case-sensitive", func(t *testing.T) {
		dir := t.TempDir()
		_WriteGoaiYAML(t, dir, _TwoProfileYAML)
		called := false
		code := _RunCLIFromConfigRecorded(dir, _NewPublicMgmtFactory(&called), WithArgs([]string{"-profile", "Public"}))
		assert.False(t, called)
		assert.Equal(t, 2, code)
		assert.NoFileExists(t, filepath.Join(dir, "public.yaml"))
		assert.NoFileExists(t, filepath.Join(dir, "mgmt.yaml"))
	})

	t.Run("profile flags are validated when config profiles are empty", func(t *testing.T) {
		tests := []struct {
			name        string
			args        []string
			wantMessage string
		}{
			{name: "empty", args: []string{"-profile", ""}, wantMessage: "must not be empty"},
			{name: "duplicate", args: []string{"-profile", "ghost", "-profile", "ghost"}, wantMessage: "duplicate profile"},
			{name: "unknown", args: []string{"-profile", "ghost"}, wantMessage: "unknown profile"},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				dir := t.TempDir()
				_WriteGoaiYAML(t, dir, "title: API\nversion: 1.0.0\nprofiles: {}\noutput: {}\n")
				called := false
				var stderr bytes.Buffer
				code := _RunCLIFromConfig(dir, _NewPublicMgmtFactory(&called), io.Discard, &stderr, WithArgs(tc.args))
				assert.Equal(t, 2, code)
				assert.False(t, called)
				assert.Contains(t, stderr.String(), tc.wantMessage)
				assert.NoFileExists(t, filepath.Join(dir, "openapi.generated.yaml"))
			})
		}
	})

	t.Run("output flag requires one effective target", func(t *testing.T) {
		dir := t.TempDir()
		_WriteGoaiYAML(t, dir, _TwoProfileYAML)
		called := false
		code := _RunCLIFromConfigRecorded(dir, _NewPublicMgmtFactory(&called), WithArgs([]string{"-o", "out.yaml"}))
		assert.False(t, called)
		assert.Equal(t, 2, code)
		assert.NoFileExists(t, filepath.Join(dir, "out.yaml"))
		assert.NoFileExists(t, filepath.Join(dir, "public.yaml"))
		assert.NoFileExists(t, filepath.Join(dir, "mgmt.yaml"))
	})

	t.Run("output flag with one profile writes resolved path", func(t *testing.T) {
		dir := t.TempDir()
		_WriteGoaiYAML(t, dir, _TwoProfileYAML)
		code := _RunCLIFromConfig(dir, _NewPublicMgmtFactory(nil), io.Discard, io.Discard,
			WithArgs([]string{"-profile", "public", "-o", "out.yaml"}),
		)

		assert.Equal(t, 0, code)
		assert.FileExists(t, filepath.Join(dir, "out.yaml"))
		assert.NoFileExists(t, filepath.Join(dir, "public.yaml"))
		assert.NoFileExists(t, filepath.Join(dir, "mgmt.yaml"))
	})
}

func TestRunCLIFromConfigRejectsAmbiguousOutputs(t *testing.T) {
	t.Run("duplicate resolved output path", func(t *testing.T) {
		dir := t.TempDir()
		_WriteGoaiYAML(t, dir, `title: API
version: 1.0.0
profiles:
  public:
    include:
      paths:
        - /public/**
  mgmt:
    include:
      paths:
        - /mgmt/**
output:
  public: same.yaml
  mgmt: same.yaml
`)

		called := false
		code := _RunCLIFromConfigRecorded(dir, _NewPublicMgmtFactory(&called))
		assert.False(t, called)
		assert.Equal(t, 1, code)
		assert.NoFileExists(t, filepath.Join(dir, "same.yaml"))
	})

	t.Run("multi-target stdout", func(t *testing.T) {
		dir := t.TempDir()
		_WriteGoaiYAML(t, dir, `title: API
version: 1.0.0
profiles:
  public:
    include:
      paths:
        - /public/**
  mgmt:
    include:
      paths:
        - /mgmt/**
output:
  public: "-"
  mgmt: "-"
`)

		called := false
		code := _RunCLIFromConfigRecorded(dir, _NewPublicMgmtFactory(&called))
		assert.False(t, called)
		assert.Equal(t, 1, code)
	})
}

func TestRunProfileJobsBoundsConcurrencyAndJoins(t *testing.T) {
	var inFlight atomic.Int32
	var maxInFlight atomic.Int32
	started := make(chan struct{}, 4)
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseWorkers := func() {
		releaseOnce.Do(func() { close(release) })
	}
	t.Cleanup(releaseWorkers)
	jobs := []_ProfileJob{
		{Name: "a"},
		{Name: "b"},
		{Name: "c"},
		{Name: "d"},
	}

	done := make(chan []_ProfileJobResult, 1)
	go func() {
		done <- _RunProfileJobs(2, jobs, func(job _ProfileJob) _ProfileJobResult {
			current := inFlight.Add(1)
			defer inFlight.Add(-1)
			for {
				seen := maxInFlight.Load()
				if current <= seen || maxInFlight.CompareAndSwap(seen, current) {
					break
				}
			}

			started <- struct{}{}
			<-release

			return _ProfileJobResult{Name: job.Name, Body: []byte(job.Name)}
		})
	}()

	for range 2 {
		select {
		case <-started:
		case <-time.After(5 * time.Second):
			t.Fatal("two profile jobs did not start concurrently")
		}
	}

	releaseWorkers()
	var results []_ProfileJobResult
	select {
	case results = <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("profile jobs did not join")
	}

	require.Len(t, results, 4)
	names := make([]string, 0, len(results))
	for _, result := range results {
		names = append(names, result.Name)
	}

	assert.ElementsMatch(t, []string{"a", "b", "c", "d"}, names)
	assert.Equal(t, 2, int(maxInFlight.Load()))
	assert.Equal(t, int32(0), inFlight.Load())
	for _, result := range results {
		require.Nil(t, result.Err)
		require.NotEmpty(t, result.Body)
	}
}

func TestRunCLIFromConfigParallelMatchesSequential(t *testing.T) {
	seqDir := t.TempDir()
	parDir := t.TempDir()
	_WriteGoaiYAML(t, seqDir, _TwoProfileYAML)
	_WriteGoaiYAML(t, parDir, _TwoProfileYAML)
	seqCode := _RunCLIFromConfig(seqDir, _NewPublicMgmtFactory(nil), io.Discard, io.Discard, WithConcurrency(1))
	parCode := _RunCLIFromConfig(parDir, _NewPublicMgmtFactory(nil), io.Discard, io.Discard, WithConcurrency(2))
	assert.Equal(t, 0, seqCode)
	assert.Equal(t, 0, parCode)
	for _, name := range []string{"public.yaml", "mgmt.yaml"} {
		seq, err := os.ReadFile(filepath.Join(seqDir, name))
		require.NoError(t, err)
		par, err := os.ReadFile(filepath.Join(parDir, name))
		require.NoError(t, err)
		assert.Equal(t, seq, par, name)
	}

	t.Run("excluded same-short-package schema collision", func(t *testing.T) {
		firstType := reflect.TypeOf(firstmodels.User{})
		secondType := reflect.TypeOf(secondmodels.User{})
		operations := []_PreparedOperation{
			{
				Candidate:              OperationCandidate{Path: "/excluded", Method: "GET"},
				HasRegistryEntry:       true,
				RespType:               firstType,
				PackagePath:            firstType.PkgPath(),
				SynthesizedOperationID: "excludedGet",
			},
			{
				Candidate:              OperationCandidate{Path: "/selected", Method: "GET"},
				HasRegistryEntry:       true,
				RespType:               secondType,
				PackagePath:            secondType.PkgPath(),
				SynthesizedOperationID: "selectedGet",
			},
		}
		profile := &Profile{Name: "selected", Include: Selector{Paths: []string{"/selected"}}}
		sequential := _PreparedSet{
			Operations:  operations,
			SchemaNames: _ResolveSchemaNames(operations[1:]),
		}
		parallel := _PreparedSet{
			Operations:  operations,
			SchemaNames: _ResolveSchemaNames(operations),
		}
		opts := BuildOptions{Title: "API", Version: "1.0.0"}
		sequentialBody, err := EmitYAML(_AssembleDocument(_ClonePreparedSet(sequential), profile, opts))
		require.NoError(t, err)
		parallelDoc := _AssembleDocument(_ClonePreparedSet(parallel), profile, opts)
		parallelBody, err := EmitYAML(parallelDoc)
		require.NoError(t, err)
		assert.Equal(t, sequentialBody, parallelBody)
		assert.Contains(t, parallelDoc.Components.Schemas, "models.User")
	})
}

func TestRunConfigProfileJobsStopsWritesAfterFirstLexicalFailure(t *testing.T) {
	dir := t.TempDir()
	blockedOutput := t.TempDir()
	cfg := &Config{
		Profiles: map[string]ConfigProfile{
			"a": {Include: Selector{Paths: []string{"/a"}}},
			"b": {Include: Selector{Paths: []string{"/b"}}},
		},
		Output: map[string]string{
			"a": blockedOutput,
			"b": "b.yaml",
		},
	}
	jobs := []_ProfileJob{
		{Name: "a", Output: blockedOutput},
		{Name: "b", Output: filepath.Join(dir, "b.yaml")},
	}
	candidates := []OperationCandidate{
		{Path: "/a", Method: "GET", Handler: &_PublicHelloHandler{}, HandlerMethod: "Get"},
		{Path: "/b", Method: "GET", Handler: &_MgmtHelloHandler{}, HandlerMethod: "Get"},
	}

	failed := _RunConfigProfileJobs(io.Discard, io.Discard, cfg, jobs, candidates, BuildOptions{}, nil, 2)
	assert.True(t, failed)
	assert.DirExists(t, blockedOutput)
	assert.NoFileExists(t, filepath.Join(dir, "b.yaml"))
}

func TestPreparedCandidatesDoNotShareMutableMetadata(t *testing.T) {
	_isolationSchemaNameCalls.Store(0)
	t.Cleanup(func() { _isolationSchemaNameCalls.Store(0) })

	var specCalls atomic.Int32
	var docCalls atomic.Int32
	handler := &_IsolationHandler{specCalls: &specCalls}
	opts := BuildOptions{
		OperationDocExtractor: func(handler any, methodName string) (*OperationDoc, bool) {
			docCalls.Add(1)

			return &OperationDoc{
				Endpoint: OperationEndpoint{Method: "GET", Path: "/items/{id}"},
				Operation: Operation{
					Summary: "doc-summary",
					Responses: map[string]*Response{
						"200": {
							Description: "orig-doc-resp",
							Content: map[string]*MediaType{
								"application/json": {Schema: &Schema{Type: "object", Description: "orig-doc-schema"}},
							},
						},
					},
				},
				Schemas: map[string]*Schema{
					"Box": {Type: "object", Description: "orig-box"},
				},
				EndpointOperations: []OperationEndpointDoc{{
					Endpoint:  OperationEndpoint{Method: "GET", Path: "/items/{id}"},
					Operation: Operation{Description: "orig-endpoint"},
					Schemas:   map[string]*Schema{"Box": {Type: "object", Description: "orig-endpoint-box"}},
				}},
			}, true
		},
	}

	prepared := _PrepareOperationCandidates([]OperationCandidate{{
		Path:          "/items/{id}",
		Method:        "GET",
		Handler:       handler,
		HandlerMethod: "Get",
		PathParams:    []PathParam{{Name: "id"}},
		Profiles:      []string{"public"},
		SecurityRefs:  []SecurityRef{{Scheme: "oauth2", Scopes: []string{"read"}}},
	}}, nil, opts)
	require.Len(t, prepared.Operations, 1)
	assert.Equal(t, int32(1), specCalls.Load())
	assert.Equal(t, int32(1), docCalls.Load())
	nameCalls := _isolationSchemaNameCalls.Load()
	assert.Greater(t, nameCalls, int32(0))
	assert.NotEmpty(t, prepared.Operations[0].PackagePath)
	expectedOperationID := prepared.Operations[0].SynthesizedOperationID
	assert.NotEmpty(t, expectedOperationID)
	assert.Nil(t, prepared.Operations[0].Candidate.Handler)

	cloneA := _ClonePreparedSet(prepared)
	cloneB := _ClonePreparedSet(prepared)
	opA := &cloneA.Operations[0]
	callbackItem := opA.Spec.callbacks["onEvent"]["{$request.body#/url}"]
	require.NotNil(t, callbackItem)
	require.NotNil(t, callbackItem.Post)
	callbackItem.Post.OperationID = "mutated-cb"
	callbackItem.Post.Responses["200"].Description = "mutated-cb-resp"
	callbackItem.Post.Responses["200"].Content["application/json"].Schema.Description = "mutated-cb-schema"
	require.NotNil(t, opA.Spec.responses["400"])
	opA.Spec.responses["400"].Description = "mutated-400"
	require.NotNil(t, opA.Spec.responses["400"].Schema)
	opA.Spec.responses["400"].Schema.Description = "mutated-400-schema"
	opA.Spec.examples["application/json"] = "mutated-example"
	opA.Spec.multiExamples["application/json"]["named"].Summary = "mutated-ex"
	require.NotNil(t, opA.OperationDoc)
	opA.OperationDoc.Schemas["Box"].Description = "mutated-box"
	opA.OperationDoc.Operation.Responses["200"].Description = "mutated-doc-resp"
	opA.OperationDoc.EndpointOperations[0].Schemas["Box"].Description = "mutated-endpoint-box"
	opA.Candidate.PathParams[0].Name = "mutated"
	opA.Candidate.Profiles[0] = "mutated"
	opA.SecurityRefs[0].Scheme = "mutated"
	opA.SecurityRefs[0].Scopes[0] = "mutated"

	orig := prepared.Operations[0]
	opB := cloneB.Operations[0]
	assert.Equal(t, "orig-cb", orig.Spec.callbacks["onEvent"]["{$request.body#/url}"].Post.OperationID)
	assert.Equal(t, "orig-cb", opB.Spec.callbacks["onEvent"]["{$request.body#/url}"].Post.OperationID)
	assert.Equal(t, "orig-cb-resp", orig.Spec.callbacks["onEvent"]["{$request.body#/url}"].Post.Responses["200"].Description)
	assert.Equal(t, "orig-cb-schema", orig.Spec.callbacks["onEvent"]["{$request.body#/url}"].Post.Responses["200"].Content["application/json"].Schema.Description)
	assert.Equal(t, "orig-400", orig.Spec.responses["400"].Description)
	assert.Equal(t, "orig-400-schema", orig.Spec.responses["400"].Schema.Description)
	assert.Equal(t, "orig-400-schema", opB.Spec.responses["400"].Schema.Description)
	assert.Equal(t, map[string]any{"k": "v"}, orig.Spec.examples["application/json"])
	assert.Equal(t, "orig-ex", orig.Spec.multiExamples["application/json"]["named"].Summary)
	assert.Equal(t, "orig-box", orig.OperationDoc.Schemas["Box"].Description)
	assert.Equal(t, "orig-doc-resp", orig.OperationDoc.Operation.Responses["200"].Description)
	assert.Equal(t, "orig-endpoint-box", orig.OperationDoc.EndpointOperations[0].Schemas["Box"].Description)
	assert.Equal(t, "id", orig.Candidate.PathParams[0].Name)
	assert.Equal(t, "public", orig.Candidate.Profiles[0])
	assert.Equal(t, "oauth2", orig.SecurityRefs[0].Scheme)
	assert.Equal(t, "read", orig.SecurityRefs[0].Scopes[0])
	assert.Equal(t, "id", opB.Candidate.PathParams[0].Name)
	assert.Equal(t, "public", opB.Candidate.Profiles[0])
	assert.Equal(t, "oauth2", opB.SecurityRefs[0].Scheme)

	cloneA.Operations[0].Candidate.Handler = nil
	cloneB.Operations[0].Candidate.Handler = nil
	packageProfile := &Profile{
		Name: "package",
		Include: Selector{
			Packages: []string{"github.com/yetiz-org/gone/goai"},
		},
	}
	docA := _AssembleDocument(cloneA, packageProfile, opts)
	docB := _AssembleDocument(cloneB, packageProfile, opts)
	for _, doc := range []*Document{docA, docB} {
		require.Contains(t, doc.Paths, "/items/{id}")
		require.NotNil(t, doc.Paths["/items/{id}"].Get)
		assert.Equal(t, expectedOperationID, doc.Paths["/items/{id}"].Get.OperationID)
	}
	assert.Equal(t, int32(1), specCalls.Load())
	assert.Equal(t, int32(1), docCalls.Load())
	assert.Equal(t, nameCalls, _isolationSchemaNameCalls.Load())
	assert.Equal(t, "orig-cb", orig.Spec.callbacks["onEvent"]["{$request.body#/url}"].Post.OperationID)
	assert.Equal(t, "orig-400-schema", orig.Spec.responses["400"].Schema.Description)
	assert.Equal(t, "orig-box", orig.OperationDoc.Schemas["Box"].Description)
}

func _NewPublicMgmtFactory(called *bool) (factory RouteFactory) {
	return func() ghttp.RouteEntriesProvider {
		if called != nil {
			*called = true
		}

		route := ghttp.NewSimpleRoute()
		route.SetEndpoint("/public/hello", &_PublicHelloHandler{})
		route.SetEndpoint("/mgmt/hello", &_MgmtHelloHandler{})
		return route
	}
}

func _RunCLIFromConfigRecorded(dir string, factory RouteFactory, extra ...RunFromConfigOption) (exitCode int) {
	return _RunCLIFromConfig(dir, factory, io.Discard, io.Discard, extra...)
}
