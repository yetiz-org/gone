package gmcp

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
	"github.com/yetiz-org/gone/ghttp"
)

func TestServerStreamingOutputContracts(t *testing.T) {
	for _, test := range []struct {
		name  string
		value any
		limit int64
	}{
		{"bytes", map[string]any{"data": make([]byte, 100001)}, 1 << 20},
		{"raw exact boundary", json.RawMessage(`{"x":1}`), 7},
		{"custom", _UnboundedSerializer{_Calls: new(atomic.Int32)}, 128},
		{"custom over cap", _UnboundedSerializer{_Calls: new(atomic.Int32)}, 5},
		{"schema extra below cap", &jsonschema.Schema{Type: "object", Extra: map[string]any{"x-note": "kept"}}, 128},
		{"schema extra", &jsonschema.Schema{Type: "object", Extra: map[string]any{"x-note": strings.Repeat("x", 1024)}}, 256},
	} {
		t.Run(test.name, func(t *testing.T) {
			expected, err := json.Marshal(test.value)
			require.NoError(t, err)
			task := &_BudgetOutputTask{_Value: map[string]any{}}
			task._Tool = NewTool[struct{}, any](&mcp.Tool{Name: "stream"}, WithOutputAdjuster(func(ctx context.Context, input struct{}, output any) (adjusted any, rtErr error) {
				return test.value, nil
			}))
			server := _BindServer(Options{MaxOutputBytes: test.limit}, ghttp.NewSimpleRoute().SetEndpoint("/stream", task))
			result := _Call(t, server, "stream", map[string]any{})
			require.Equal(t, int64(len(expected)) > test.limit, result.IsError)
			if !result.IsError {
				require.JSONEq(t, string(expected), string(result.StructuredContent))
			}
		})
	}
}

func TestServerLegacyInputOptions(t *testing.T) {
	for _, options := range []Options{{MaxRequestBodyBytes: -1}, {MaxRequestBodyBytes: 32 << 20}, {MaxBatchCalls: 2}, {MaxResponseBytes: 128}, {MaxBodyBytes: -1}} {
		require.NotPanics(t, func() {
			server := _BindServer(options, ghttp.NewSimpleRoute().SetEndpoint("/small", &_BudgetOutputTask{_Value: map[string]any{}, _Tool: NewTool[struct{}, any](&mcp.Tool{Name: "small"})}))
			require.False(t, _Call(t, server, "small", map[string]any{}).IsError)
		})
	}
	server := New(&mcp.Implementation{Name: "input", Version: "1"}, Options{})
	require.Zero(t, server._Options.MaxBodyBytes)
	body := bytes.Repeat([]byte{' '}, (4<<20)+1)
	body = append(body, []byte(`{"jsonrpc":"2.0","id":1,"method":"ping"}`)...)
	server = New(&mcp.Implementation{Name: "input", Version: "1"}, Options{MaxRequestBodyBytes: -1})
	require.Equal(t, http.StatusOK, _ServeBody(t, server, body, nil).Code)
}

func TestServerNonPathIDs(t *testing.T) {
	type input struct {
		ID ID `json:"id" gmcp:"query=id"`
	}
	route := ghttp.NewSimpleRoute().SetEndpoint("/query", &_BudgetOutputTask{_Value: map[string]any{}, _Tool: NewTool[input, any](&mcp.Tool{Name: "query"})})
	server := _BindServer(Options{IDPattern: ".+"}, route)
	require.False(t, _Call(t, server, "query", map[string]any{"id": "tenant/a"}).IsError)
	type bodyInput struct {
		Body struct {
			ID ID `json:"id"`
		} `json:"body" gmcp:"body"`
	}
	schema := server._NewInput(reflect.TypeFor[bodyInput]())._Schema
	resolved, err := schema.Resolve(nil)
	require.NoError(t, err)
	require.NoError(t, resolved.Validate(map[string]any{"body": map[string]any{"id": "tenant/a"}}))
}

func BenchmarkStreamingOutput(b *testing.B) {
	server := New(&mcp.Implementation{Name: "bench", Version: "1"}, Options{})
	value := make([]int, 1000)
	route := &_Route{_Server: server}
	b.Run("ints", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if _, err := route._Output(value); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func TestResponseWriterSmallErrorBudget(t *testing.T) {
	recorder := httptest.NewRecorder()
	writer := &_ResponseWriter{_Writer: recorder, _Limit: 1}
	writer.Header().Set("Content-Type", "application/json")
	_, err := writer.Write([]byte(`{}`))
	require.ErrorIs(t, err, _ErrOutputTooLarge)
	writer._Finish()
	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	require.LessOrEqual(t, recorder.Body.Len(), 1)
}

func TestServerTypedNilOutput(t *testing.T) {
	pointer := &_BudgetOutputTask{_Value: []byte(`"x"`)}
	pointer._Tool = NewTool[struct{}, *string](&mcp.Tool{Name: "nil_pointer", OutputSchema: &jsonschema.Schema{Type: "string"}}, WithOutputAdjuster(func(ctx context.Context, input struct{}, output *string) (adjusted *string, rtErr error) {
		return nil, nil
	}))
	dynamic := &_BudgetOutputTask{_Value: []byte(`"x"`)}
	dynamic._Tool = NewTool[struct{}, any](&mcp.Tool{Name: "nil_any"}, WithOutputAdjuster(func(ctx context.Context, input struct{}, output any) (adjusted any, rtErr error) {
		return (*string)(nil), nil
	}))
	server := _BindServer(Options{}, ghttp.NewSimpleRoute().SetEndpoint("/pointer", pointer).SetEndpoint("/dynamic", dynamic))

	result := _Call(t, server, "nil_pointer", map[string]any{})
	require.False(t, result.IsError)
	require.JSONEq(t, `""`, string(result.StructuredContent), "a declared pointer Out keeps the SDK element zero")

	result = _Call(t, server, "nil_any", map[string]any{})
	require.False(t, result.IsError)
	require.JSONEq(t, `null`, string(result.StructuredContent), "a dynamic nil pointer stays null")
}
