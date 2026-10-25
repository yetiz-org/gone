package goai

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yetiz-org/gone/ghttp"
	"gopkg.in/yaml.v3"
)

type _SecurityTestAcceptance struct {
	ghttp.DispatchAcceptance
	ref SecurityRef
}

func (a *_SecurityTestAcceptance) SecurityRequirement() (scheme string, scopes []string) {
	return a.ref.Scheme, a.ref.Scopes
}

type _MethodSecurityTestAcceptance struct {
	_SecurityTestAcceptance
}

func (a *_MethodSecurityTestAcceptance) SecurityRequirementFor(method string) (scheme string, scopes []string) {
	return "", nil
}

type _ClassifiedSecurityTestAcceptance struct {
	ghttp.DispatchAcceptance
}

type _MethodOnlySecurityTestAcceptance struct {
	ghttp.DispatchAcceptance
	ref SecurityRef
}

func (a *_MethodOnlySecurityTestAcceptance) SecurityRequirementFor(method string) (scheme string, scopes []string) {
	return a.ref.Scheme, a.ref.Scopes
}

func TestBuildSecurityMethodAwareClassifier(t *testing.T) {
	prev := ghttp.SetSkipHandlerRegister(true)
	defer ghttp.SetSkipHandlerRegister(prev)
	cases := []struct {
		name        string
		acceptances []ghttp.Acceptance
		spec        []Option
		want        []map[string][]string
	}{
		{
			name:        "active classifier sibling survives method exemption",
			acceptances: []ghttp.Acceptance{&_ClassifiedSecurityTestAcceptance{}, &_MethodSecurityTestAcceptance{ref: SecurityRef{Scheme: "Bearer"}}},
			want:        []map[string][]string{{"Classified": {"read"}}},
		},
		{
			name:        "method-only exemption suppresses its classifier rule and inherits global",
			acceptances: []ghttp.Acceptance{&_MethodOnlySecurityTestAcceptance{}},
		},
		{
			name:        "positive method provider and classifier sibling conjoin Spec alternatives",
			acceptances: []ghttp.Acceptance{&_MethodOnlySecurityTestAcceptance{ref: SecurityRef{Scheme: "Method", Scopes: []string{"head"}}}, &_ClassifiedSecurityTestAcceptance{}},
			spec:        []Option{WithSecurity("OAuth", "write"), WithSecurity("ApiKey")},
			want:        []map[string][]string{{"Method": {"head"}, "Classified": {"read"}, "OAuth": {"write"}}, {"Method": {"head"}, "Classified": {"read"}, "ApiKey": nil}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			Reset()
			defer Reset()
			handler := &_WalkerHeadHandler{}
			route := ghttp.NewSimpleRoute()
			route.SetEndpoint("/secure", handler, tc.acceptances...)
			if len(tc.spec) > 0 {
				Register(handler, "HEAD", nil, nil, tc.spec...)
			}
			classifier := &Classifier{AcceptanceSecurityRules: []AcceptanceSecurityRule{
				{TypeName: "ClassifiedSecurity", Scheme: "Classified", Scopes: []string{"read"}},
				{TypeName: "MethodOnlySecurity", Scheme: "Classified", Scopes: []string{"wrong"}},
			}}

			doc := Build(Walk(route), nil, BuildOptions{GlobalSecurity: []map[string][]string{{"Global": {}}}, TagSecurityClassifier: classifier})

			op := doc.Paths["/secure"].Head
			require.NotNil(t, op)
			if tc.want == nil {
				assert.Nil(t, op.Security)

				return
			}
			require.NotNil(t, op.Security)
			assert.Equal(t, tc.want, *op.Security)
		})
	}
}

func TestBuildSecurityComposition(t *testing.T) {
	prev := ghttp.SetSkipHandlerRegister(true)
	defer ghttp.SetSkipHandlerRegister(prev)
	global := []map[string][]string{{"Global": {}}}
	cases := []struct {
		name       string
		refs       []SecurityRef
		spec       []Option
		exempt     bool
		classifier *Classifier
		inherit    bool
		want       []map[string][]string
	}{
		{name: "inherit global", inherit: true},
		{name: "one acceptance", refs: []SecurityRef{{Scheme: "Token"}}, want: []map[string][]string{{"Token": nil}}},
		{name: "different schemes are AND", refs: []SecurityRef{{Scheme: "Token"}, {Scheme: "Org", Scopes: []string{"member"}}}, want: []map[string][]string{{"Token": nil, "Org": {"member"}}}},
		{name: "same scheme scopes are stable union", refs: []SecurityRef{{Scheme: "OAuth", Scopes: []string{"read", "shared"}}, {Scheme: "OAuth", Scopes: []string{"write", "shared", "write"}}}, want: []map[string][]string{{"OAuth": {"read", "shared", "write"}}}},
		{name: "Spec preserves OR", spec: []Option{WithSecurity("OAuth", "read"), WithSecurity("ApiKey")}, want: []map[string][]string{{"OAuth": {"read"}}, {"ApiKey": nil}}},
		{name: "same scheme Spec alternatives stay distinct", spec: []Option{WithSecurity("OAuth", "read"), WithSecurity("OAuth", "write"), WithSecurity("OAuth", "read")}, want: []map[string][]string{{"OAuth": {"read"}}, {"OAuth": {"write"}}}},
		{name: "acceptance times Spec OR", refs: []SecurityRef{{Scheme: "Org", Scopes: []string{"member"}}}, spec: []Option{WithSecurity("OAuth", "read"), WithSecurity("ApiKey")}, want: []map[string][]string{{"Org": {"member"}, "OAuth": {"read"}}, {"Org": {"member"}, "ApiKey": nil}}},
		{name: "same scheme cross product unions scopes", refs: []SecurityRef{{Scheme: "OAuth", Scopes: []string{"base"}}}, spec: []Option{WithSecurity("OAuth", "read", "base"), WithSecurity("OAuth", "write")}, want: []map[string][]string{{"OAuth": {"base", "read"}}, {"OAuth": {"base", "write"}}}},
		{name: "method exemption overrides global and classifier", exempt: true, classifier: &Classifier{AcceptanceSecurityRules: []AcceptanceSecurityRule{{TypeName: "Security", Scheme: "Fallback"}}}, want: []map[string][]string{}},
		{name: "method exemption retains active sibling", exempt: true, refs: []SecurityRef{{Scheme: "Org"}}, want: []map[string][]string{{"Org": nil}}},
		{name: "method exemption retains explicit Spec", exempt: true, spec: []Option{WithSecurity("Form")}, want: []map[string][]string{{"Form": nil}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			Reset()
			defer Reset()
			handler := &_WalkerHeadHandler{}
			var acceptances []ghttp.Acceptance
			for _, ref := range tc.refs {
				acceptances = append(acceptances, &_SecurityTestAcceptance{ref: ref})
			}
			if tc.exempt {
				acceptances = append(acceptances, &_MethodSecurityTestAcceptance{ref: SecurityRef{Scheme: "Bearer"}})
			}
			route := ghttp.NewSimpleRoute()
			route.SetEndpoint("/secure", handler, acceptances...)
			if len(tc.spec) > 0 {
				Register(handler, "HEAD", nil, nil, tc.spec...)
			}

			doc := Build(Walk(route), nil, BuildOptions{GlobalSecurity: global, TagSecurityClassifier: tc.classifier})

			require.Contains(t, doc.Paths, "/secure")
			op := doc.Paths["/secure"].Head
			require.NotNil(t, op)
			if tc.inherit {
				assert.Nil(t, op.Security)
			} else {
				require.NotNil(t, op.Security)
				assert.Equal(t, tc.want, *op.Security)
			}
			assert.Equal(t, global, doc.Security)

			body, err := EmitYAML(doc)
			require.NoError(t, err)
			var wire struct {
				Paths map[string]map[string]struct {
					Security *[]map[string][]string `yaml:"security"`
				} `yaml:"paths"`
			}
			require.NoError(t, yaml.Unmarshal(body, &wire))
			security := wire.Paths["/secure"]["head"].Security
			if tc.inherit {
				assert.Nil(t, security)
			} else {
				require.NotNil(t, security)
				for _, requirement := range *security {
					for scheme, scopes := range requirement {
						assert.NotNil(t, scopes, "scope list for %s must be a YAML array", scheme)
					}
				}
			}
		})
	}
}

func TestBuildSecurityDocFallbackPrecedence(t *testing.T) {
	cases := []struct {
		name    string
		refs    []SecurityRef
		spec    []Option
		empty   bool
		doc     *OperationDoc
		inherit bool
		want    []map[string][]string
	}{
		{name: "acceptance wins", refs: []SecurityRef{{Scheme: "Route", Scopes: []string{"read"}}}, want: []map[string][]string{{"Route": {"read"}}}},
		{name: "Spec wins", spec: []Option{WithSecurity("Spec", "write")}, want: []map[string][]string{{"Spec": {"write"}}}},
		{name: "explicit empty wins", empty: true, want: []map[string][]string{}},
		{name: "doc only fallback", want: []map[string][]string{{"Doc": {"read"}}}},
		{name: "doc explicit empty", doc: &OperationDoc{Endpoint: OperationEndpoint{Method: "GET", Path: "/secure"}, Operation: Operation{Security: new([]map[string][]string{})}}, want: []map[string][]string{}},
		{name: "scoped doc fallback", doc: &OperationDoc{
			Endpoints: []OperationEndpoint{{Method: "GET", Path: "/secure"}, {Method: "GET", Path: "/other"}},
			Operation: Operation{Security: new([]map[string][]string{{"Common": {}}})},
			EndpointOperations: []OperationEndpointDoc{
				{Endpoint: OperationEndpoint{Method: "GET", Path: "/secure"}, Operation: Operation{Security: new([]map[string][]string{{"Scoped": {"read"}}})}},
				{Endpoint: OperationEndpoint{Method: "GET", Path: "/other"}, Operation: Operation{Security: new([]map[string][]string{{"Other": {}}})}},
			},
		}, want: []map[string][]string{{"Scoped": {"read"}}}},
		{name: "unmatched doc ignored", doc: &OperationDoc{Endpoint: OperationEndpoint{Method: "POST", Path: "/other"}, Operation: Operation{Security: new([]map[string][]string{{"Wrong": {}}})}}, inherit: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			Reset()
			defer Reset()
			handler := &_DocstringTestHandler{}
			if len(tc.spec) > 0 {
				Register(handler, "GET", nil, nil, tc.spec...)
			}
			source := tc.doc
			if source == nil {
				source = &OperationDoc{Endpoint: OperationEndpoint{Method: "GET", Path: "/secure"}, Operation: Operation{Security: new([]map[string][]string{{"Doc": {"read"}}})}}
			}

			doc := Build([]OperationCandidate{{Path: "/secure", Method: "GET", Handler: handler, HandlerMethod: "Get", SecurityRefs: tc.refs, SecurityExplicitlyEmpty: tc.empty}}, nil, BuildOptions{OperationDocExtractor: func(handler any, methodName string) (*OperationDoc, bool) {
				return source, true
			}})

			op := doc.Paths["/secure"].Get
			require.NotNil(t, op)
			if tc.inherit {
				assert.Nil(t, op.Security)
			} else {
				require.NotNil(t, op.Security)
				assert.Equal(t, tc.want, *op.Security)
			}
		})
	}
}

func TestClassifySecurityMergesScopes(t *testing.T) {
	first := &_SecurityTestAcceptance{ref: SecurityRef{Scheme: "OAuth", Scopes: []string{"read", "shared"}}}
	second := &_SecurityTestAcceptance{ref: SecurityRef{Scheme: "OAuth", Scopes: []string{"write", "shared"}}}
	classifier := &Classifier{}

	refs := classifier.ClassifySecurity([]ghttp.Acceptance{first, second})

	require.Len(t, refs, 1)
	assert.Equal(t, SecurityRef{Scheme: "OAuth", Scopes: []string{"read", "shared", "write"}}, refs[0])
	assert.Equal(t, []string{"read", "shared"}, first.ref.Scopes)
	assert.Equal(t, []string{"write", "shared"}, second.ref.Scopes)
}

type _ChangingSecurityTestAcceptance struct {
	ghttp.DispatchAcceptance
	calls int
}

func (a *_ChangingSecurityTestAcceptance) SecurityRequirement() (scheme string, scopes []string) {
	a.calls++
	if a.calls == 1 {
		return "Before", nil
	}

	return "After", nil
}

func TestBuildSecurityUsesWalkSnapshot(t *testing.T) {
	prev := ghttp.SetSkipHandlerRegister(true)
	defer ghttp.SetSkipHandlerRegister(prev)
	Reset()
	defer Reset()
	acceptance := &_ChangingSecurityTestAcceptance{}
	route := ghttp.NewSimpleRoute()
	route.SetEndpoint("/secure", &_WalkerHeadHandler{}, acceptance)
	classifier := &Classifier{AcceptanceSecurityRules: []AcceptanceSecurityRule{{TypeName: "ChangingSecurity", Scheme: "Fallback"}}}

	doc := Build(Walk(route), nil, BuildOptions{TagSecurityClassifier: classifier})

	op := doc.Paths["/secure"].Head
	require.NotNil(t, op)
	require.NotNil(t, op.Security)
	assert.Equal(t, []map[string][]string{{"Before": nil}}, *op.Security)
	assert.Equal(t, 1, acceptance.calls)
}

func TestBuildSecurityManualCandidateKeepsProviders(t *testing.T) {
	Reset()
	defer Reset()
	candidate := OperationCandidate{Path: "/secure", Method: "GET", Handler: &_DocstringTestHandler{}, HandlerMethod: "Get", Acceptances: []ghttp.Acceptance{
		&_SecurityTestAcceptance{ref: SecurityRef{Scheme: "Token"}},
		&_ClassifiedSecurityTestAcceptance{},
	}}
	classifier := &Classifier{AcceptanceSecurityRules: []AcceptanceSecurityRule{{TypeName: "ClassifiedSecurity", Scheme: "Classified"}}}

	doc := Build([]OperationCandidate{candidate}, nil, BuildOptions{TagSecurityClassifier: classifier})

	op := doc.Paths["/secure"].Get
	require.NotNil(t, op)
	require.NotNil(t, op.Security)
	assert.Equal(t, []map[string][]string{{"Token": nil, "Classified": nil}}, *op.Security)
}
