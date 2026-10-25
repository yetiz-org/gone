package gmcp

import (
	"bytes"
	"context"
	"encoding/json"
	jsonv2 "encoding/json/v2"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
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

type _BudgetOutputTask struct {
	ghttp.DefaultHTTPHandlerTask
	_Tool  Tool
	_Value any
}

func (t *_BudgetOutputTask) Get(ctx channel.HandlerContext, req *ghttp.Request, resp *ghttp.Response, params map[string]any) (failure ghttp.ErrorResponse) {
	resp.JsonResponse(t._Value)
	return nil
}

func (t *_BudgetOutputTask) MCPGet() (tool Tool) {
	return t._Tool
}

type _UnboundedSerializer struct {
	_Calls *atomic.Int32
}

func (v _UnboundedSerializer) MarshalJSON() (data []byte, rtErr error) {
	v._Calls.Add(1)
	return []byte(`{"unbounded":true}`), nil
}

type _BlockingBudgetTask struct {
	ghttp.DefaultHTTPHandlerTask
	_Entered chan struct{}
	_Release chan struct{}
	_Calls   atomic.Int32
}

func (t *_BlockingBudgetTask) Get(ctx channel.HandlerContext, req *ghttp.Request, resp *ghttp.Response, params map[string]any) (failure ghttp.ErrorResponse) {
	t._Calls.Add(1)
	select {
	case t._Entered <- struct{}{}:
	default:
	}

	select {
	case <-t._Release:
	case <-req.Request().Context().Done():
	}

	resp.JsonResponse(map[string]any{"ok": true})
	return nil
}

func (t *_BlockingBudgetTask) MCPGet() (tool Tool) {
	return NewTool[struct{}, map[string]any](&mcp.Tool{Name: "blocked"})
}

func TestServerOutputSchemasAndBudgets(t *testing.T) {
	nilTask := &_BudgetOutputTask{_Tool: NewTool[struct{}, any](&mcp.Tool{Name: "no_output"})}
	nilServer := _BindServer(Options{}, ghttp.NewSimpleRoute().SetEndpoint("/nil", nilTask))
	nilResult := _Call(t, nilServer, "no_output", map[string]any{})
	require.False(t, nilResult.IsError)
	assert.Empty(t, nilResult.StructuredContent, "nil any output retains the SDK's absent structuredContent semantics")

	schemaJSON := `{"type":"object","properties":{"added":{"type":"string","default":"kept"}}}`
	var schemaMap map[string]any
	require.NoError(t, json.Unmarshal([]byte(schemaJSON), &schemaMap))
	for _, schema := range []any{
		schemaMap, json.RawMessage(schemaJSON), json.RawMessage(`{"type":"object","properties":{"added":{"type":"string","def\u0061ult":"kept"}}}`), true,
		&jsonschema.Schema{Type: "object", Properties: map[string]*jsonschema.Schema{"added": {Type: "string", Default: json.RawMessage(`"kept"`)}}},
	} {
		definition := &mcp.Tool{Name: "budget", OutputSchema: schema}
		task := &_BudgetOutputTask{_Value: map[string]any{}, _Tool: NewTool[struct{}, map[string]any](definition)}
		server := _BindServer(Options{MaxOutputBytes: 1024}, ghttp.NewSimpleRoute().SetEndpoint("/budget", task))
		result := _Call(t, server, "budget", map[string]any{})
		require.False(t, result.IsError, "preset schema form must remain supported: %T", schema)
		if _, boolean := schema.(bool); boolean {
			assert.JSONEq(t, `{}`, string(result.StructuredContent))
		} else {
			assert.JSONEq(t, `{"added":"kept"}`, string(result.StructuredContent), "output defaults must remain applied")
		}

		assert.Equal(t, schema, definition.OutputSchema, "the public preset schema value must not be replaced")
		invalid := _Call(t, server, "budget", map[string]any{"unexpected": true})
		assert.True(t, invalid.IsError, "generic input validation must remain enabled")
	}

	for _, options := range []Options{{MaxResponseBodyBytes: 128}, {MaxOutputBytes: 128}} {
		for _, size := range []int{116, 117, 118} {
			task := &_LargeOutputTask{_Size: size}
			server := _BindServer(options, ghttp.NewSimpleRoute().SetEndpoint("/large", task))
			result := _Call(t, server, "large", map[string]any{})
			assert.Equal(t, size > 117, result.IsError, "128-byte JSON output boundary must be enforced: size %d, options %+v", size, options)
		}
	}

	blob := Blob{Data: DataURL{MediaType: "text/plain", Data: bytes.Repeat([]byte{'x'}, 64)}}
	encodedBlob, err := json.Marshal(blob)
	require.NoError(t, err)
	for _, limit := range []int64{int64(len(encodedBlob)) - 1, int64(len(encodedBlob)), int64(len(encodedBlob)) + 1} {
		task := &_BudgetOutputTask{_Value: map[string]any{}}
		task._Tool = NewTool[struct{}, Blob](&mcp.Tool{Name: "blob"}, WithOutputAdjuster(func(ctx context.Context, input struct{}, output Blob) (adjusted Blob, rtErr error) {
			return blob, nil
		}))
		server := _BindServer(Options{MaxOutputBytes: limit}, ghttp.NewSimpleRoute().SetEndpoint("/blob", task))
		result := _Call(t, server, "blob", map[string]any{})
		assert.Equal(t, limit < int64(len(encodedBlob)), result.IsError, "base64 expansion must be admitted before allocation")
		if !result.IsError {
			assert.JSONEq(t, string(encodedBlob), string(result.StructuredContent))
		}
	}

	alias := make([]any, 1)
	alias[0] = alias[:0]
	aliasTask := &_BudgetOutputTask{_Value: map[string]any{}}
	aliasTask._Tool = NewTool[struct{}, any](&mcp.Tool{Name: "alias"}, WithOutputAdjuster(func(ctx context.Context, input struct{}, output any) (adjusted any, rtErr error) {
		return alias, nil
	}))
	aliasServer := _BindServer(Options{}, ghttp.NewSimpleRoute().SetEndpoint("/alias", aliasTask))
	aliasResult := _Call(t, aliasServer, "alias", map[string]any{})
	require.False(t, aliasResult.IsError, "finite overlapping slices are not cycles")
	assert.JSONEq(t, `[[]]`, string(aliasResult.StructuredContent))

	secondAdjuster := atomic.Int32{}
	task := &_BudgetOutputTask{_Value: map[string]any{"small": true}}
	task._Tool = NewTool[struct{}, map[string]any](&mcp.Tool{Name: "adjusted"},
		WithOutputAdjuster(func(ctx context.Context, input struct{}, output map[string]any) (adjusted map[string]any, rtErr error) {
			return map[string]any{"large": strings.Repeat("x", 2048)}, nil
		}), WithOutputAdjuster(func(ctx context.Context, input struct{}, output map[string]any) (adjusted map[string]any, rtErr error) {
			secondAdjuster.Add(1)
			return output, nil
		}))
	server := _BindServer(Options{MaxOutputBytes: 1024}, ghttp.NewSimpleRoute().SetEndpoint("/adjusted", task))
	assert.True(t, _Call(t, server, "adjusted", map[string]any{}).IsError)
	assert.EqualValues(t, 1, secondAdjuster.Load(), "adjusters own their allocations; the final output buffer is capped")

	nilObject := &_BudgetOutputTask{_Tool: NewTool[struct{}, *struct{}](&mcp.Tool{Name: "nil_object", OutputSchema: &jsonschema.Schema{Type: "object"}})}
	nilObjectServer := _BindServer(Options{}, ghttp.NewSimpleRoute().SetEndpoint("/nil_object", nilObject))
	nilObjectResult := _Call(t, nilObjectServer, "nil_object", map[string]any{})
	require.False(t, nilObjectResult.IsError)
	assert.JSONEq(t, `{}`, string(nilObjectResult.StructuredContent), "preset object schemas keep nil-to-empty-object behavior")

	var nilSchema *jsonschema.Schema
	assert.Panics(t, func() {
		_BindServer(Options{}, ghttp.NewSimpleRoute().SetEndpoint("/nil", &_GetTask{_Tool: NewTool[struct{}, any](&mcp.Tool{Name: "nil", OutputSchema: nilSchema})}))
	}, "typed-nil preset schema is invalid")
	assert.Panics(t, func() {
		_BindServer(Options{MaxSchemaBytes: 128}, ghttp.NewSimpleRoute().SetEndpoint("/schema", &_GetTask{_Tool: NewTool[struct{}, any](&mcp.Tool{Name: "schema", OutputSchema: map[string]any{"description": strings.Repeat("x", 1024)}})}))
	}, "schema representation is bounded before SDK registration")
}

func TestServerRenderedOutput(t *testing.T) {
	value := map[string]any{
		"html": strings.Repeat("&amp;", 256<<10),
		"text": strings.Repeat("&", 256<<10),
	}
	for _, test := range []struct {
		name    string
		options Options
		wantErr bool
	}{
		{"defaults", Options{}, false},
		{"explicit REST cap", Options{MaxResponseBodyBytes: 1 << 20, MaxOutputBytes: 8 << 20}, true},
		{"explicit output cap", Options{MaxResponseBodyBytes: 8 << 20, MaxOutputBytes: 1 << 20}, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			task := &_BudgetOutputTask{_Value: value, _Tool: NewTool[struct{}, map[string]any](&mcp.Tool{Name: "preview"})}
			server := _BindServer(test.options, ghttp.NewSimpleRoute().SetEndpoint("/preview", task))
			response := _Serve(t, server, "tools/call", map[string]any{"name": "preview", "arguments": map[string]any{}})
			require.Equal(t, http.StatusOK, response.Code)
			assert.Less(t, response.Body.Len(), 16<<20)
			var envelope _Envelope
			require.NoError(t, jsonv2.Unmarshal(response.Body.Bytes(), &envelope))
			require.Nil(t, envelope.Error)
			var result _CallResult
			require.NoError(t, jsonv2.Unmarshal(envelope.Result, &result))
			require.Equal(t, test.wantErr, result.IsError)
			if test.wantErr {
				assert.Empty(t, result.StructuredContent)
				assert.Less(t, len(_Payload(t, result)), 1024)
				return
			}

			assert.JSONEq(t, _JSON(t, value), string(result.StructuredContent))
			require.Len(t, result.Content, 1)
			assert.Equal(t, string(result.StructuredContent), result.Content[0].Text)
		})
	}

	task := &_BudgetOutputTask{_Value: value, _Tool: NewTool[struct{}, map[string]any](&mcp.Tool{Name: "preview"})}
	server := _BindServer(Options{MaxResponseBytes: 1 << 20}, ghttp.NewSimpleRoute().SetEndpoint("/preview", task))
	response := _Serve(t, server, "tools/call", map[string]any{"name": "preview", "arguments": map[string]any{}})
	assert.Equal(t, http.StatusInternalServerError, response.Code)
	assert.JSONEq(t, `{"error":{"category":"internal_error"}}`, response.Body.String())
}

func TestServerDefaultOutputBoundaries(t *testing.T) {
	for _, test := range []struct {
		name     string
		limit    int
		adjusted bool
	}{
		{"REST", 2 << 20, false},
		{"adjusted output", 5 << 20, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			for _, delta := range []int{-1, 0, 1} {
				value := map[string]any{"data": strings.Repeat("x", test.limit-len(`{"data":""}`)+delta)}
				task := &_BudgetOutputTask{_Value: value, _Tool: NewTool[struct{}, map[string]any](&mcp.Tool{Name: "boundary"})}
				if test.adjusted {
					task._Value = map[string]any{}
					task._Tool = NewTool[struct{}, map[string]any](&mcp.Tool{Name: "boundary"}, WithOutputAdjuster(func(ctx context.Context, input struct{}, output map[string]any) (adjusted map[string]any, rtErr error) {
						return value, nil
					}))
				}

				server := _BindServer(Options{}, ghttp.NewSimpleRoute().SetEndpoint("/boundary", task))
				result := _Call(t, server, "boundary", map[string]any{})
				require.Equal(t, delta > 0, result.IsError, "encoded size %d", test.limit+delta)
				if result.IsError {
					assert.Empty(t, result.StructuredContent)
					assert.Less(t, len(_Payload(t, result)), 1024)
					continue
				}

				assert.Len(t, result.StructuredContent, test.limit+delta)
				assert.JSONEq(t, _JSON(t, value), string(result.StructuredContent))
			}
		})
	}
}

func TestServerCustomSerializersAndCycles(t *testing.T) {
	for _, cyclic := range []bool{false, true} {
		calls := atomic.Int32{}
		task := &_BudgetOutputTask{_Value: map[string]any{}}
		task._Tool = NewTool[struct{}, any](&mcp.Tool{Name: "custom"}, WithOutputAdjuster(func(ctx context.Context, input struct{}, output any) (adjusted any, rtErr error) {
			if cyclic {
				value := map[string]any{}
				value["cycle"] = value
				return value, nil
			}

			return _UnboundedSerializer{_Calls: &calls}, nil
		}))
		server := _BindServer(Options{}, ghttp.NewSimpleRoute().SetEndpoint("/custom", task))
		result := _Call(t, server, "custom", map[string]any{})
		assert.Equal(t, cyclic, result.IsError)
		if !cyclic {
			assert.JSONEq(t, `{"unbounded":true}`, string(result.StructuredContent))
			assert.EqualValues(t, 1, calls.Load(), "custom output is encoded once")
		}
	}
}

func TestResponseWriterEventBudget(t *testing.T) {
	const limit = int64(256)
	event := []byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{}}\n\n")
	t.Run("many compliant events have no lifetime cap", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		writer := &_ResponseWriter{_Writer: recorder, _Limit: limit}
		writer.Header().Set("Content-Type", "text/event-stream")
		data := bytes.Repeat(event, 20)
		n, err := writer.Write(data)
		require.NoError(t, err)
		assert.Equal(t, len(data), n)
		writer._Finish()
		assert.Greater(t, recorder.Body.Len(), int(limit), "events reset the budget even in one large SDK write")
		assert.Equal(t, 20, strings.Count(recorder.Body.String(), "event: message"))
	})
	for _, stream := range []bool{false, true} {
		t.Run(map[bool]string{false: "JSON overflow is atomic", true: "SSE overflow before commit"}[stream], func(t *testing.T) {
			recorder := httptest.NewRecorder()
			cancelled := false
			writer := &_ResponseWriter{_Writer: recorder, _Limit: limit, _Cancel: func() { cancelled = true }}
			writer.Header().Set("Content-Type", map[bool]string{false: "application/json", true: "text/event-stream"}[stream])
			_, err := writer.Write(bytes.Repeat([]byte{'x'}, int(limit)+1))
			require.ErrorIs(t, err, _ErrOutputTooLarge)
			assert.Zero(t, recorder.Body.Len(), "invalid partial data must not commit")
			writer._Finish()
			assert.True(t, cancelled)
			assert.Equal(t, http.StatusInternalServerError, recorder.Code)
			assert.JSONEq(t, `{"error":{"category":"internal_error"}}`, recorder.Body.String())
		})
	}
	t.Run("committed SSE terminates without replacement JSON", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		writer := &_ResponseWriter{_Writer: recorder, _Limit: limit}
		writer.Header().Set("Content-Type", "text/event-stream")
		_, err := writer.Write(event[:len(event)-1])
		require.NoError(t, err)
		assert.Zero(t, recorder.Body.Len(), "incomplete event stays buffered")
		_, err = writer.Write(event[len(event)-1:])
		require.NoError(t, err)
		before := recorder.Body.String()
		_, err = writer.Write(bytes.Repeat([]byte{'x'}, int(limit)+1))
		require.ErrorIs(t, err, _ErrOutputTooLarge)
		writer._Finish()
		assert.Equal(t, before, recorder.Body.String(), "committed SSE cannot be replaced or followed by a JSON error")
		assert.Equal(t, http.StatusOK, recorder.Code)
	})
}

func TestServerLegacyBatchAdmissionLimits(t *testing.T) {
	options := Options{MaxBatchCalls: 2, MaxRequestBodyBytes: 1024, MaxResponseBytes: 8192}
	items := &_ItemsTask{}
	server := _BindServer(options, _ItemsRoute(items))
	call := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"items_get","arguments":{"org":"o1","item":"i1"}}}`
	notification := `{"jsonrpc":"2.0","method":"notifications/initialized"}`
	accepted := _ServeBody(t, server, []byte("["+notification+","+call+"]"), map[string]string{"MCP-Protocol-Version": "2025-03-26"})
	assert.Equal(t, http.StatusOK, accepted.Code)
	assert.EqualValues(t, 1, items._GetCalls.Load(), "one opt-in call runs within the finite batch cap")
	rejected := _ServeBody(t, server, []byte("["+notification+","+notification+","+call+"]"), map[string]string{"MCP-Protocol-Version": "2025-03-26"})
	assert.Equal(t, http.StatusBadRequest, rejected.Code)
	assert.EqualValues(t, 1, items._GetCalls.Load(), "notifications count toward admission before any additional call runs")
	largeID := `{"jsonrpc":"2.0","id":"` + strings.Repeat("<", 200) + `","method":"tools/list"}`
	response := _ServeBody(t, server, []byte(largeID), nil)
	assert.Equal(t, http.StatusOK, response.Code, "request IDs are bounded by the actual response writer")
}

func TestServerCallConcurrencyBudget(t *testing.T) {
	task := &_BlockingBudgetTask{_Entered: make(chan struct{}, 1), _Release: make(chan struct{})}
	release := sync.OnceFunc(func() { close(task._Release) })
	t.Cleanup(release)
	server := _BindServer(Options{MaxConcurrentCalls: 1}, ghttp.NewSimpleRoute().SetEndpoint("/blocked", task))
	completed := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		completed <- _ServeBody(t, server, []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"blocked","arguments":{}}}`), nil)
	}()
	select {
	case <-task._Entered:
	case <-time.After(3 * time.Second):
		t.Fatal("first handler did not enter")
	}

	blocked := _Call(t, server, "blocked", map[string]any{})
	assert.True(t, blocked.IsError, "work beyond the cap must fail without queueing")
	assert.EqualValues(t, 1, task._Calls.Load(), "overflow never enters the handler")
	release()
	select {
	case first := <-completed:
		assert.Equal(t, http.StatusOK, first.Code)
		assert.Contains(t, first.Body.String(), `"ok":true`)
	case <-time.After(3 * time.Second):
		t.Fatal("first call did not complete")
	}

	assert.False(t, _Call(t, server, "blocked", map[string]any{}).IsError, "completed work releases its slot")
	assert.EqualValues(t, 2, task._Calls.Load())
}

func TestServerRejectsInvalidBudgets(t *testing.T) {
	for _, options := range []Options{
		{MaxOutputBytes: -1}, {MaxSchemaBytes: -1}, {MaxResponseBodyBytes: -1},
		{MaxResponseBytes: -1}, {MaxBatchCalls: -1}, {MaxConcurrentCalls: -1},
		{MaxOutputBytes: math.MaxInt64},
	} {
		assert.Panics(t, func() { New(&mcp.Implementation{Name: "invalid", Version: "1"}, options) }, "invalid budgets must fail at startup: %+v", options)
	}
}

func TestServerRejectsLargeOutputAtBufferBoundary(t *testing.T) {
	task := &_BudgetOutputTask{_Value: map[string]any{"data": strings.Repeat("x", 8<<20)}, _Tool: NewTool[struct{}, any](&mcp.Tool{Name: "preallocated"})}
	server := _BindServer(Options{}, ghttp.NewSimpleRoute().SetEndpoint("/preallocated", task))
	result := _Call(t, server, "preallocated", map[string]any{})
	require.True(t, result.IsError)
	assert.Empty(t, result.StructuredContent)
	assert.Less(t, len(_Payload(t, result)), 1024)
	capture := &_OutputCapture{_Limit: 1024}
	_, err := capture.Write(bytes.Repeat([]byte{'x'}, 1024))
	require.NoError(t, err)
	assert.LessOrEqual(t, capture._Body.Cap(), 1024)
	_, err = capture.Write([]byte{'x'})
	require.ErrorIs(t, err, _ErrOutputTooLarge)
	assert.Zero(t, capture._Body.Len())
}
