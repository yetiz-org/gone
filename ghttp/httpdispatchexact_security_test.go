package ghttp

import (
	"bytes"
	"encoding/json"
	"encoding/json/jsontext"
	"maps"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/yetiz-org/gone/channel"
	buf "github.com/yetiz-org/goth-bytebuf"
)

type boundaryExactTask struct {
	DefaultHTTPHandlerTask
	calls []string
}

func (t *boundaryExactTask) PreCheck(req *Request, resp *Response, params map[string]any) (er ErrorResponse) {
	t.calls = append(t.calls, "pre")
	return nil
}

func (t *boundaryExactTask) Before(req *Request, resp *Response, params map[string]any) (er ErrorResponse) {
	t.calls = append(t.calls, "before")
	return nil
}

func (t *boundaryExactTask) After(req *Request, resp *Response, params map[string]any) (er ErrorResponse) {
	t.calls = append(t.calls, "after")
	return nil
}

func (t *boundaryExactTask) Index(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) (er ErrorResponse) {
	t.calls = append(t.calls, "index")
	return nil
}

func (t *boundaryExactTask) Get(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) (er ErrorResponse) {
	t.calls = append(t.calls, "get")
	return nil
}

func (t *boundaryExactTask) Create(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) (er ErrorResponse) {
	t.calls = append(t.calls, "create")
	return nil
}

func (t *boundaryExactTask) Post(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) (er ErrorResponse) {
	t.calls = append(t.calls, "post")
	return nil
}

type boundaryExactAcceptance struct {
	DispatchAcceptance
	task *boundaryExactTask
}

func (a *boundaryExactAcceptance) Do(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) (rtErr error) {
	a.task.calls = append(a.task.calls, "acceptance")
	return nil
}

func TestDispatchExactOperationAndTarget(t *testing.T) {
	for _, test := range []struct {
		name      string
		method    string
		path      string
		operation DispatchOperation
		id        string
		wrongNode bool
		want      string
	}{
		{name: "get has no index fallback", method: MethodGet, path: "/items", operation: DispatchGet, want: "get"},
		{name: "post has no create fallback", method: MethodPost, path: "/items", operation: DispatchPost, want: "post"},
		{name: "index remains exact", method: MethodGet, path: "/items", operation: DispatchIndex, want: "index"},
		{name: "create remains exact", method: MethodPost, path: "/items", operation: DispatchCreate, want: "create"},
		{name: "item get", method: MethodGet, path: "/items/abc", operation: DispatchGet, id: "abc", want: "get"},
		{name: "method mismatch is pre-effect", method: MethodGet, path: "/items", operation: DispatchPost},
		{name: "node mismatch is pre-effect", method: MethodGet, path: "/items", operation: DispatchGet, wrongNode: true},
		{name: "ID mismatch is pre-effect", method: MethodGet, path: "/items/abc", operation: DispatchGet, id: "other"},
		{name: "index refuses item placement", method: MethodGet, path: "/items/abc", operation: DispatchIndex, id: "abc"},
	} {
		t.Run(test.name, func(t *testing.T) {
			task := &boundaryExactTask{}
			route := NewSimpleRoute().SetEndpoint("/items", task, &boundaryExactAcceptance{task: task}).SetEndpoint("/other", &DefaultHTTPHandlerTask{})
			node := route.FindNode("/items")
			if test.wrongNode {
				node = route.FindNode("/other")
			}

			ch := &channel.DefaultChannel{}
			ch.Init()
			ctx := channel.NewMockHandlerContext()
			ctx.On("Channel").Return(ch)
			pack, err := NewDispatchHandler(route).DispatchExact(ctx, httptest.NewRequest(test.method, test.path, nil), nil, DispatchOptions{Operation: test.operation, Node: node, IDs: map[string]string{"items": test.id}, MaxResponseBytes: 1024})
			if test.want == "" {
				require.Error(t, err)
				require.Empty(t, task.calls, "no acceptance, task hook or method may run on a mismatched declaration")
			} else {
				require.NoError(t, err)
				require.NotNil(t, pack)
				require.Equal(t, http.StatusOK, pack.Response.StatusCode())
				require.Equal(t, []string{"acceptance", "pre", "before", test.want, "after"}, task.calls)
			}
		})
	}
}

type boundaryBudgetTask struct {
	DefaultHTTPHandlerTask
	write func(*Response)
}

func (t *boundaryBudgetTask) Get(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) (er ErrorResponse) {
	t.write(resp)
	return nil
}

type boundaryMarshaler struct {
	called *bool
}

func (m boundaryMarshaler) MarshalJSON() (data []byte, rtErr error) {
	*m.called = true
	return []byte(`"unsafe"`), nil
}

type boundaryStreamingMarshaler struct {
	called *bool
}

func (m boundaryStreamingMarshaler) MarshalJSONTo(enc *jsontext.Encoder) (rtErr error) {
	*m.called = true
	return enc.WriteToken(jsontext.String("unsafe"))
}

type boundaryTextAppender struct {
	called *bool
}

func (m boundaryTextAppender) AppendText(data []byte) (encoded []byte, rtErr error) {
	*m.called = true
	return append(data, "unsafe"...), nil
}

func TestDispatchExactResponseBudget(t *testing.T) {
	for _, kind := range []string{"raw", "bytes", "buffer", "json", "time", "raw_json", "shared_slice", "cycle", "marshaler", "streaming_marshaler", "text_appender", "encoder"} {
		want := "12345"
		switch kind {
		case "json":
			want = `{"value":"<ok>"}`
		case "time":
			want = `"2026-10-23T00:00:00Z"`
		case "raw_json":
			want = `{"value":"ok"}`
		case "shared_slice":
			want = `[[]]`
		case "marshaler", "streaming_marshaler", "text_appender":
			want = `"unsafe"`
		case "encoder":
			want = `{"value":"ok"}`
		}

		for _, limit := range []int64{int64(len(want)) - 1, int64(len(want)), int64(len(want)) + 1} {
			t.Run(kind+"/"+strconv.FormatInt(limit, 10), func(t *testing.T) {
				called := false
				write := func(resp *Response) {
					switch kind {
					case "raw":
						resp.SetBody(buf.NewByteBufString(want))
					case "bytes":
						resp.JsonResponse([]byte(want))
					case "buffer":
						resp.JsonResponse(bytes.NewBufferString(want))
					case "json":
						resp.JsonResponse(map[string]string{"value": "<ok>"})
					case "time":
						resp.JsonResponse(time.Date(2026, time.October, 23, 0, 0, 0, 0, time.UTC))
					case "raw_json":
						resp.JsonResponse(json.RawMessage(want))
					case "shared_slice":
						shared := make([]any, 1)
						shared[0] = shared[:0]
						resp.JsonResponse(shared)
					case "cycle":
						cyclic := make([]any, 1)
						cyclic[0] = cyclic
						resp.JsonResponse(cyclic)
					case "marshaler":
						resp.JsonResponse(boundaryMarshaler{called: &called})
					case "streaming_marshaler":
						resp.JsonResponse(boundaryStreamingMarshaler{called: &called})
					case "text_appender":
						resp.JsonResponse(boundaryTextAppender{called: &called})
					case "encoder":
						resp.SetJsonEncoderConfig(func(enc *json.Encoder) { called = true })
						resp.JsonResponse(map[string]string{"value": "ok"})
					}
				}
				task := &boundaryBudgetTask{write: write}
				route := NewSimpleRoute().SetEndpoint("/items", task)
				ch := &channel.DefaultChannel{}
				ch.Init()
				ctx := channel.NewMockHandlerContext()
				ctx.On("Channel").Return(ch)
				pack, err := NewDispatchHandler(route).DispatchExact(ctx, httptest.NewRequest(MethodGet, "/items", nil), nil, DispatchOptions{Operation: DispatchGet, Node: route.FindNode("/items"), IDs: map[string]string{"items": ""}, MaxResponseBytes: limit})
				if limit < int64(len(want)) || kind == "cycle" {
					require.Error(t, err)
					if pack != nil {
						require.Empty(t, pack.Response.Body().Bytes(), "failed output must discard its prefix")
					}
				} else {
					require.NoError(t, err)
					require.Equal(t, want, string(pack.Response.Body().Bytes()))
				}

				require.Equal(t, kind == "marshaler" || kind == "streaming_marshaler" || kind == "text_appender" || kind == "encoder", called)
			})
		}
	}
}

var boundaryZeroCalls atomic.Int32

type boundaryZeroValue struct{}

func (v boundaryZeroValue) IsZero() (zero bool) {
	boundaryZeroCalls.Add(1)
	return true
}

type boundaryPointerZero struct{}

func (v *boundaryPointerZero) IsZero() (zero bool) {
	boundaryZeroCalls.Add(1)
	return true
}

type boundaryZeroSlice []int

func (v boundaryZeroSlice) IsZero() (zero bool) {
	boundaryZeroCalls.Add(1)
	return true
}

type boundaryZeroMap map[string]int

func (v boundaryZeroMap) IsZero() (zero bool) {
	boundaryZeroCalls.Add(1)
	return true
}

func TestDispatchExactZeroPredicates(t *testing.T) {
	var nilPointer *boundaryPointerZero
	var typedNil interface{ IsZero() bool } = nilPointer
	zeroTime := time.Time{}
	for _, test := range []struct {
		name     string
		value    any
		callback bool
		limit    int64
	}{
		{name: "concrete", value: struct {
			Value boundaryZeroValue `json:"value,omitzero"`
		}{}, callback: true},
		{name: "addressed value", value: struct {
			Value boundaryPointerZero `json:"value,omitzero"`
		}{}, callback: true},
		{name: "pointer", value: struct {
			Value *boundaryPointerZero `json:"value,omitzero"`
		}{&boundaryPointerZero{}}, callback: true},
		{name: "declared interface", value: struct {
			Value interface{ IsZero() bool } `json:"value,omitzero"`
		}{boundaryZeroValue{}}, callback: true},
		{name: "empty slice before empty omission", value: struct {
			Value boundaryZeroSlice `json:"value,omitempty,omitzero"`
		}{boundaryZeroSlice{}}, callback: true},
		{name: "nil slice before empty omission", value: struct {
			Value boundaryZeroSlice `json:"value,omitempty,omitzero"`
		}{}, callback: true},
		{name: "empty map before empty omission", value: struct {
			Value boundaryZeroMap `json:"value,omitempty,omitzero"`
		}{boundaryZeroMap{}}, callback: true},
		{name: "nil map before empty omission", value: struct {
			Value boundaryZeroMap `json:"value,omitempty,omitzero"`
		}{}, callback: true},
		{name: "untagged", value: struct{ Value boundaryZeroValue }{}},
		{name: "empty only", value: struct {
			Value boundaryZeroSlice `json:"value,omitempty"`
		}{boundaryZeroSlice{}}},
		{name: "static any", value: struct {
			Value any `json:"value,omitzero"`
		}{boundaryZeroValue{}}},
		{name: "nil pointer", value: struct {
			Value *boundaryPointerZero `json:"value,omitzero"`
		}{}},
		{name: "nil interface", value: struct {
			Value interface{ IsZero() bool } `json:"value,omitzero"`
		}{}},
		{name: "typed nil interface", value: struct {
			Value interface{ IsZero() bool } `json:"value,omitzero"`
		}{typedNil}},
		{name: "typed nil any", value: struct {
			Value any `json:"value,omitzero"`
		}{nilPointer}},
		{name: "zero time", value: struct {
			Value time.Time `json:"value,omitzero"`
		}{}},
		{name: "zero time minimum output", value: struct {
			Value time.Time `json:"value,omitzero"`
		}{}, limit: 2},
		{name: "pointer zero time", value: struct {
			Value *time.Time `json:"value,omitzero"`
		}{&zeroTime}},
		{name: "declared time interface", value: struct {
			Value interface{ IsZero() bool } `json:"value,omitzero"`
		}{zeroTime}},
		{name: "static any time", value: struct {
			Value any `json:"value,omitzero"`
		}{zeroTime}},
		{name: "zero ancestor", value: struct {
			Child struct {
				Value *boundaryPointerZero `json:"value,omitzero"`
			} `json:"child,omitzero"`
		}{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			boundaryZeroCalls.Store(0)
			want, err := json.Marshal(test.value)
			require.NoError(t, err)
			require.Equal(t, test.callback, boundaryZeroCalls.Load() > 0, "stdlib callback control")
			limit := test.limit
			if limit == 0 {
				limit = 512
			}

			for _, bounded := range []bool{false, true} {
				boundaryZeroCalls.Store(0)
				task := &boundaryBudgetTask{write: func(resp *Response) { resp.JsonResponse(test.value) }}
				route := NewSimpleRoute().SetEndpoint("/items", task)
				ch := &channel.DefaultChannel{}
				ch.Init()
				ctx := &boundaryErrorContext{ch: ch}
				handler := NewDispatchHandler(route)
				var pack *Pack
				if bounded {
					pack, err = handler.DispatchExact(ctx, httptest.NewRequest(MethodGet, "/items", nil), nil, DispatchOptions{Operation: DispatchGet, Node: route.FindNode("/items"), IDs: map[string]string{"items": ""}, MaxResponseBytes: limit})
				} else {
					pack = handler.Dispatch(ctx, httptest.NewRequest(MethodGet, "/items", nil), nil)
					err = nil
				}

				require.NoError(t, err)
				require.JSONEq(t, string(want), string(pack.Response.Body().Bytes()))
				require.Equal(t, test.callback, boundaryZeroCalls.Load() > 0)
			}
		})
	}
}

type boundaryZeroPromoted struct {
	Value boundaryZeroValue `json:"value,omitzero"`
}

type boundaryZeroDefault struct {
	Value boundaryZeroValue `json:",omitzero"`
}

type boundarySafeTagged struct {
	Safe string `json:"Value"`
}

type boundaryZeroLeft struct{ boundaryZeroPromoted }

type boundaryZeroRight struct{ boundaryZeroPromoted }

type boundaryZeroDiamondShared struct{ boundaryZeroPromoted }

type boundaryZeroDiamondLeft struct{ boundaryZeroDiamondShared }

type boundaryZeroDiamondRight struct{ boundaryZeroDiamondShared }

type boundaryZeroSymbol struct {
	Value boundaryZeroValue `json:"🔒,omitzero"`
}

type boundaryZeroQuoted struct {
	Value boundaryZeroValue `json:"'🔒',omitzero"`
}

type boundaryZeroFlattened struct {
	Leaf string `json:"leaf"`
}

func (v boundaryZeroFlattened) IsZero() (zero bool) {
	boundaryZeroCalls.Add(1)
	return true
}

func TestDispatchExactSelectedFields(t *testing.T) {
	for _, test := range []struct {
		name     string
		value    any
		callback bool
	}{
		{name: "unexported promoted", value: struct{ boundaryZeroPromoted }{}, callback: true},
		{name: "shallower shadow", value: struct {
			boundaryZeroPromoted
			Value string `json:"value"`
		}{Value: "safe"}},
		{name: "different JSON case remains distinct", value: struct {
			boundaryZeroPromoted
			Value string
		}{Value: "safe"}, callback: true},
		{name: "same depth tag preference", value: struct {
			boundaryZeroDefault
			boundarySafeTagged
		}{boundarySafeTagged: boundarySafeTagged{Safe: "safe"}}},
		{name: "repeated embedded ambiguity", value: struct {
			boundaryZeroLeft
			boundaryZeroRight
		}{}},
		{name: "deep shared diamond actual v2 selection", value: struct {
			boundaryZeroDiamondLeft
			boundaryZeroDiamondRight
		}{}, callback: true},
		{name: "nil embedded pointer", value: struct{ *boundaryZeroPromoted }{}},
		{name: "unicode distinct name", value: struct {
			boundaryZeroSymbol
			Value string
		}{Value: "safe"}, callback: true},
		{name: "unicode same name shadow", value: struct {
			boundaryZeroSymbol
			Value string `json:"🔒"`
		}{Value: "safe"}},
		{name: "quoted name fallback shadow", value: struct {
			boundaryZeroQuoted
			Value string
		}{Value: "safe"}},
		{name: "flattened predicate unused", value: struct {
			boundaryZeroFlattened `json:",omitzero"`
		}{boundaryZeroFlattened{Leaf: "safe"}}},
		{name: "named unexported predicate omitted", value: struct {
			boundaryZeroFlattened `json:"named,omitzero"`
		}{boundaryZeroFlattened{Leaf: "safe"}}},
		{name: "ignored field", value: struct {
			Value boundaryZeroValue `json:"-"`
		}{}},
		{name: "dash name is not ignored", value: struct {
			Value boundaryZeroValue `json:"-,omitempty,omitzero"`
		}{}, callback: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			boundaryZeroCalls.Store(0)
			want, err := json.Marshal(test.value)
			require.NoError(t, err)
			require.Equal(t, test.callback, boundaryZeroCalls.Load() > 0, "stdlib field selection control")
			t.Logf("stdlib callback=%v wire=%s", boundaryZeroCalls.Load() > 0, want)
			boundaryZeroCalls.Store(0)
			task := &boundaryBudgetTask{write: func(resp *Response) { resp.JsonResponse(test.value) }}
			route := NewSimpleRoute().SetEndpoint("/items", task)
			ch := &channel.DefaultChannel{}
			ch.Init()
			pack, err := NewDispatchHandler(route).DispatchExact(&boundaryErrorContext{ch: ch}, httptest.NewRequest(MethodGet, "/items", nil), nil, DispatchOptions{Operation: DispatchGet, Node: route.FindNode("/items"), IDs: map[string]string{"items": ""}, MaxResponseBytes: 512})
			require.NoError(t, err)
			require.JSONEq(t, string(want), string(pack.Response.Body().Bytes()))
			require.Equal(t, test.callback, boundaryZeroCalls.Load() > 0)
		})
	}
}

func TestDispatchExactCustomParameterTargets(t *testing.T) {
	for _, routeCase := range []struct {
		name     string
		endpoint string
		path     string
		ids      map[string]string
	}{
		{name: "brace leaf id", endpoint: "/items/{id}", path: "/items/i", ids: map[string]string{"items": "i"}},
		{name: "brace leaf suffix", endpoint: "/items/{item_id}", path: "/items/i", ids: map[string]string{"items": "i"}},
		{name: "colon leaf", endpoint: "/items/:item_id", path: "/items/i", ids: map[string]string{"items": "i"}},
		{name: "brace ancestor default leaf", endpoint: "/users/{user_id}/items", path: "/users/u/items/i", ids: map[string]string{"users": "u", "items": "i"}},
		{name: "colon ancestor default leaf", endpoint: "/users/:user_id/items", path: "/users/u/items/i", ids: map[string]string{"users": "u", "items": "i"}},
		{name: "mixed ancestor leaf", endpoint: "/users/:user_id/items/{id}", path: "/users/u/items/i", ids: map[string]string{"users": "u", "items": "i"}},
		{name: "crossed custom keys", endpoint: "/users/:items_id/items/:users_id", path: "/users/u/items/i", ids: map[string]string{"users": "u", "items": "i"}},
		{name: "brace ancestor collection", endpoint: "/users/{user_id}/items", path: "/users/u/items", ids: map[string]string{"users": "u", "items": ""}},
	} {
		for _, source := range []string{"RouteEntries", "FindNode"} {
			for _, mismatch := range []string{"", "node", "endpoint ID", "ancestor ID", "missing endpoint ID", "extra ID", "method"} {
				if mismatch == "ancestor ID" && len(routeCase.ids) == 1 {
					continue
				}

				t.Run(routeCase.name+"/"+source+"/"+mismatch, func(t *testing.T) {
					task := &boundaryExactTask{}
					route := NewSimpleRoute().SetEndpoint(routeCase.endpoint, task, &boundaryExactAcceptance{task: task}).SetEndpoint("/other", task)
					var node RouteNode
					if source == "FindNode" {
						node = route.FindNode(routeCase.path)
					} else {
						for _, entry := range route.RouteEntries() {
							if entry.Path == routeCase.endpoint {
								node = entry.Node
							}
						}
					}

					require.NotNil(t, node)
					ids := maps.Clone(routeCase.ids)
					method := MethodGet
					switch mismatch {
					case "node":
						node = route.FindNode("/other")
					case "endpoint ID":
						ids["items"] = "other"
					case "ancestor ID":
						ids["users"] = "other"
					case "missing endpoint ID":
						delete(ids, "items")
					case "extra ID":
						ids["unknown"] = "other"
					case "method":
						method = MethodPost
					}

					ch := &channel.DefaultChannel{}
					ch.Init()
					pack, err := NewDispatchHandler(route).DispatchExact(&boundaryErrorContext{ch: ch}, httptest.NewRequest(method, routeCase.path, nil), nil, DispatchOptions{Operation: DispatchGet, Node: node, IDs: ids, MaxResponseBytes: 1024})
					if mismatch != "" {
						require.Error(t, err)
						require.Empty(t, task.calls, "mismatched declarations must not run acceptances or task hooks")
						return
					}

					require.NoError(t, err)
					require.NotNil(t, pack)
					require.Equal(t, http.StatusOK, pack.Response.StatusCode())
					require.Equal(t, []string{"acceptance", "pre", "before", "get", "after"}, task.calls)
				})
			}
		}
	}
}
