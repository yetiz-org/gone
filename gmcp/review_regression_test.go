package gmcp

import (
	"context"
	"encoding/json"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yetiz-org/gone/channel"
	"github.com/yetiz-org/gone/ghttp"
)

type _ZeroBudgetSerializer int

func (v _ZeroBudgetSerializer) IsZero() (zero bool) {
	return false
}

func (v _ZeroBudgetSerializer) MarshalJSON() (data []byte, rtErr error) {
	return []byte(`{"executed":true}`), nil
}

var _OmissionCalls atomic.Int32

type _OmissionSlice []string

func (v _OmissionSlice) IsZero() (zero bool) {
	_OmissionCalls.Add(1)
	return true
}

type _OmissionMap map[string]string

func (v *_OmissionMap) IsZero() (zero bool) {
	_OmissionCalls.Add(1)
	return true
}

func TestServerEmbeddedOutputGuard(t *testing.T) {
	type hiddenEmbed struct {
		Exported _UnboundedSerializer `json:"exported"`
	}
	type VisibleEmbed struct {
		Exported _UnboundedSerializer `json:"exported"`
	}
	type hiddenPlain struct {
		Value string `json:"value"`
		Count int    `json:"count"`
	}
	type ambiguousLeft struct {
		Value _UnboundedSerializer
	}
	type ambiguousRight struct {
		Value _UnboundedSerializer
	}
	type unicodeEmbed struct {
		Value _ZeroBudgetSerializer `json:"🔒,omitzero"`
	}
	type quotedEmbed struct {
		Value _ZeroBudgetSerializer `json:"'🔒',omitzero"`
	}

	tests := []struct {
		name  string
		value func(*atomic.Int32) any
	}{
		{"unexported anonymous embed", func(calls *atomic.Int32) any {
			return struct{ hiddenEmbed }{hiddenEmbed{Exported: _UnboundedSerializer{_Calls: calls}}}
		}},
		{"unexported pointer embed", func(calls *atomic.Int32) any {
			return struct{ *hiddenEmbed }{&hiddenEmbed{Exported: _UnboundedSerializer{_Calls: calls}}}
		}},
		{"exact dash tag is ignored", func(calls *atomic.Int32) any {
			return struct {
				Value _UnboundedSerializer `json:"-"`
			}{Value: _UnboundedSerializer{_Calls: calls}}
		}},
		{"dash name with options is visible", func(calls *atomic.Int32) any {
			return struct {
				Value _UnboundedSerializer `json:"-,omitempty"`
			}{Value: _UnboundedSerializer{_Calls: calls}}
		}},
		{"custom zero predicate", func(calls *atomic.Int32) any {
			return struct {
				Value _ZeroBudgetSerializer `json:"value,omitzero"`
			}{}
		}},
		{"benign exported descendants", func(calls *atomic.Int32) any {
			return struct{ hiddenPlain }{hiddenPlain{Value: "kept", Count: 1}}
		}},
		{"nil embedded pointer", func(calls *atomic.Int32) any { return struct{ *hiddenPlain }{} }},
		{"outer field shadows hidden descendant", func(calls *atomic.Int32) any {
			return struct {
				hiddenEmbed
				Exported string `json:"exported"`
			}{hiddenEmbed: hiddenEmbed{Exported: _UnboundedSerializer{_Calls: calls}}, Exported: "safe"}
		}},
		{"outer field shadows visible descendant", func(calls *atomic.Int32) any {
			return struct {
				VisibleEmbed
				Exported string `json:"exported"`
			}{VisibleEmbed: VisibleEmbed{Exported: _UnboundedSerializer{_Calls: calls}}, Exported: "safe"}
		}},
		{"equally deep fields are ignored", func(calls *atomic.Int32) any {
			return struct {
				ambiguousLeft
				ambiguousRight
			}{ambiguousLeft{Value: _UnboundedSerializer{_Calls: calls}}, ambiguousRight{Value: _UnboundedSerializer{_Calls: calls}}}
		}},
		{"Unicode name remains distinct", func(calls *atomic.Int32) any {
			return struct {
				unicodeEmbed
				Value string
			}{Value: "safe"}
		}},
		{"Unicode name shadow", func(calls *atomic.Int32) any {
			return struct {
				unicodeEmbed
				Value string `json:"🔒"`
			}{Value: "safe"}
		}},
		{"quoted name falls back to Go field", func(calls *atomic.Int32) any {
			return struct {
				quotedEmbed
				Value string
			}{Value: "safe"}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			calls := atomic.Int32{}
			task := &_BudgetOutputTask{_Value: map[string]any{}}
			task._Tool = NewTool[struct{}, any](&mcp.Tool{Name: "embedded"}, WithOutputAdjuster(func(ctx context.Context, input struct{}, output any) (adjusted any, rtErr error) {
				return test.value(&calls), nil
			}))
			server := _BindServer(Options{}, ghttp.NewSimpleRoute().SetEndpoint("/embedded", task))
			result := _Call(t, server, "embedded", map[string]any{})
			wire, err := json.Marshal(test.value(new(atomic.Int32)))
			require.NoError(t, err)
			require.False(t, result.IsError)
			assert.JSONEq(t, string(wire), string(result.StructuredContent), "encoding/json owns field selection")
		})
	}
}

func TestServerOmissionCallbackAdmission(t *testing.T) {
	for _, test := range []struct {
		name  string
		value any
	}{
		{"empty slice both tags", struct {
			Value _OmissionSlice `json:"value,omitempty,omitzero"`
		}{Value: _OmissionSlice{}}},
		{"nil slice both tags", struct {
			Value _OmissionSlice `json:"value,omitempty,omitzero"`
		}{}},
		{"empty map pointer-method both tags", struct {
			Value _OmissionMap `json:"value,omitempty,omitzero"`
		}{Value: _OmissionMap{}}},
		{"nil map pointer-method both tags", struct {
			Value _OmissionMap `json:"value,omitempty,omitzero"`
		}{}},
		{"pointer value both tags", struct {
			Value *_OmissionMap `json:"value,omitempty,omitzero"`
		}{Value: new(_OmissionMap{})}},
		{"empty slice omitempty only", struct {
			Value _OmissionSlice `json:"value,omitempty"`
		}{Value: _OmissionSlice{}}},
		{"empty map omitempty only", struct {
			Value _OmissionMap `json:"value,omitempty"`
		}{Value: _OmissionMap{}}},
		{"untagged slice", struct{ Value _OmissionSlice }{Value: _OmissionSlice{}}},
		{"static any ignores dynamic predicate", struct {
			Value any `json:"value,omitempty,omitzero"`
		}{Value: _OmissionSlice{}}},
		{"nil pointer both tags", struct {
			Value *_OmissionMap `json:"value,omitempty,omitzero"`
		}{}},
		{"typed nil zeroer interface", struct {
			Value interface{ IsZero() bool } `json:"value,omitempty,omitzero"`
		}{Value: (*_OmissionMap)(nil)}},
		{"nil zeroer interface", struct {
			Value interface{ IsZero() bool } `json:"value,omitempty,omitzero"`
		}{}},
		{"vetted zero time", struct {
			Value time.Time `json:"value,omitempty,omitzero"`
		}{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			_OmissionCalls.Store(0)
			task := &_BudgetOutputTask{_Value: map[string]any{}}
			task._Tool = NewTool[struct{}, any](&mcp.Tool{Name: "omission"}, WithOutputAdjuster(func(ctx context.Context, input struct{}, output any) (adjusted any, rtErr error) {
				return test.value, nil
			}))
			server := _BindServer(Options{}, ghttp.NewSimpleRoute().SetEndpoint("/omission", task))
			result := _Call(t, server, "omission", map[string]any{})
			wire, err := json.Marshal(test.value)
			require.NoError(t, err)
			require.False(t, result.IsError)
			assert.JSONEq(t, string(wire), string(result.StructuredContent), "encoding/json owns omission callbacks")
		})
	}
}

func TestServerIDSchemaRuntimeIntersection(t *testing.T) {
	type taggedPath struct {
		Org  ID     `json:"org" gmcp:"path=orgs;pattern=.+"`
		Item string `json:"item" gmcp:"path;pattern=.+"`
	}

	for _, pattern := range []string{"", ".+", "i", "a|b"} {
		for _, tagged := range []bool{false, true} {
			tool := NewTool[_GetInput, _Echo](&mcp.Tool{Name: "schema_ids"})
			if tagged {
				tool = NewTool[taggedPath, _Echo](&mcp.Tool{Name: "schema_ids"})
			}

			server := _BindServer(Options{IDPattern: pattern}, ghttp.NewSimpleRoute().SetEndpoint("/orgs", &_OrgsTask{}).SetEndpoint("/orgs/items", &_GetTask{_Tool: tool}))
			tools, _ := _Tools(t, server)
			encoded, err := json.Marshal(tools["schema_ids"].InputSchema)
			require.NoError(t, err)
			var schema *jsonschema.Schema
			require.NoError(t, json.Unmarshal(encoded, &schema))
			resolved, err := schema.Resolve(nil)
			require.NoError(t, err)
			for _, candidate := range []struct {
				id        string
				defaultOK bool
				narrowOK  bool
			}{{"i", true, true}, {"ii", true, false}, {"prefixi", true, false}, {"iX", true, false}, {"a", true, false}, {"b", true, false}, {"ab", true, false}, {"i?secret=x", false, false}, {"i/child", false, false}, {".abc", false, false}, {"..", false, false}} {
				want := candidate.defaultOK
				anchor := "i"
				if pattern == "i" {
					want = candidate.narrowOK
				} else if pattern == "a|b" {
					want = candidate.id == "a" || candidate.id == "b"
					anchor = "a"
				}

				for _, arguments := range []map[string]any{{"org": candidate.id, "item": anchor}, {"org": anchor, "item": candidate.id}} {
					validation := resolved.Validate(arguments)
					result := _Call(t, server, "schema_ids", arguments)
					assert.Equal(t, want, validation == nil, "published schema must enforce immutable+configured path constraints: pattern=%q tagged=%v arguments=%v", pattern, tagged, arguments)
					assert.Equal(t, want, !result.IsError, "runtime accepts exactly the declared path intersection: pattern=%q tagged=%v arguments=%v", pattern, tagged, arguments)
				}
			}
		}
	}
}

type _PromotedInputTask struct {
	ghttp.DefaultHTTPHandlerTask
	_Tool  Tool
	_Calls atomic.Int32
}

func (t *_PromotedInputTask) MCPPost() (tool Tool) {
	return t._Tool
}

func (t *_PromotedInputTask) Post(ctx channel.HandlerContext, req *ghttp.Request, resp *ghttp.Response, params map[string]any) (failure ghttp.ErrorResponse) {
	t._Calls.Add(1)
	resp.JsonResponse(_Echo{ID: t.GetID("promoted", params), Query: req.Url().RawQuery, Body: string(req.Body().Bytes())})
	return nil
}

func TestServerPromotedOptionalInput(t *testing.T) {
	type OptionalQuery struct {
		Filter string `json:"filter,omitempty" gmcp:"query=filter"`
	}
	type OptionalFile struct {
		Proof *File `json:"proof,omitempty" gmcp:"file=proof"`
	}
	type optionalInput struct {
		*OptionalQuery
		*OptionalFile
	}
	type RequiredPath struct {
		ID string `json:"id" gmcp:"path"`
	}
	type requiredInput struct {
		*RequiredPath
	}

	t.Run("required path", func(t *testing.T) {
		task := &_PromotedInputTask{_Tool: NewTool[requiredInput, _Echo](&mcp.Tool{Name: "promoted"})}
		server := _BindServer(Options{}, ghttp.NewSimpleRoute().SetEndpoint("/promoted", task))
		result := _Call(t, server, "promoted", map[string]any{})
		require.True(t, result.IsError)
		assert.JSONEq(t, `{"error":{"category":"invalid_argument"}}`, _Payload(t, result))
		assert.Zero(t, task._Calls.Load(), "missing required fields must fail before the handler")

		result = _Call(t, server, "promoted", map[string]any{"id": "item-1"})
		require.False(t, result.IsError)
		assert.JSONEq(t, `{"function":"","id":"item-1"}`, string(result.StructuredContent))
		assert.EqualValues(t, 1, task._Calls.Load())
	})
	t.Run("optional query and file", func(t *testing.T) {
		task := &_PromotedInputTask{_Tool: NewTool[optionalInput, _Echo](&mcp.Tool{Name: "promoted"})}
		server := _BindServer(Options{}, ghttp.NewSimpleRoute().SetEndpoint("/promoted", task))
		for _, test := range []struct {
			name      string
			arguments map[string]any
			want      string
		}{
			{"absent", map[string]any{}, `{"function":""}`},
			{"present query", map[string]any{"filter": "red blue"}, `{"function":"","query":"filter=red+blue"}`},
		} {
			t.Run(test.name, func(t *testing.T) {
				result := _Call(t, server, "promoted", test.arguments)
				require.False(t, result.IsError)
				assert.JSONEq(t, test.want, string(result.StructuredContent), "omitted query/file fields must not create request content")
			})
		}
		assert.EqualValues(t, 2, task._Calls.Load())
	})
}

func TestRouteRetryAfter(t *testing.T) {
	route := &_Route{}
	for _, test := range []struct {
		value string
		want  time.Duration
	}{
		{"0", 0},
		{"1", time.Second},
		{"3", 3 * time.Second},
		{"4", 3 * time.Second},
		{"", 3 * time.Second},
		{"invalid", 3 * time.Second},
		{"-1", 3 * time.Second},
		{strconv.Itoa(int(^uint(0) >> 1)), 3 * time.Second},
	} {
		t.Run(test.value, func(t *testing.T) {
			assert.Equal(t, test.want, route._RetryAfter(test.value))
		})
	}
}
