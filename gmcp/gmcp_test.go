package gmcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yetiz-org/gone/channel"
	"github.com/yetiz-org/gone/erresponse"
	"github.com/yetiz-org/gone/ghttp"
)

type _Echo struct {
	Function string `json:"function"`
	Org      string `json:"org,omitempty"`
	ID       string `json:"id,omitempty"`
	Query    string `json:"query,omitempty"`
	Tool     string `json:"tool,omitempty"`
	Language string `json:"language,omitempty"`
	Body     string `json:"body,omitempty"`
}

type _ListInput struct {
	Org   ID       `json:"org" gmcp:"path=orgs;description=Organization."`
	Limit int      `json:"limit,omitzero" gmcp:"query=l;minimum=1;maximum=100;example=20"`
	Since *Date    `json:"since,omitempty" gmcp:"query=since;deprecated"`
	Name  string   `json:"name,omitempty" gmcp:"query=name;maxLength=20;pattern=^[a-z]+$;format=hostname;example=abc"`
	Tags  []string `json:"tags,omitempty" gmcp:"query=tags;minItems=1;uniqueItems;items.enum=a,b;items.example=a"`
}

type _GetInput struct {
	Org  ID     `json:"org" gmcp:"path=orgs"`
	Item string `json:"item" gmcp:"path;description=Item."`
}

type _Label struct {
	Key   string  `json:"key" gmcp:"description=Label key."`
	Value *string `json:"value,omitempty"`
}

type _PatchBody struct {
	Name   *string           `json:"name" gmcp:"description=New name."`
	Notes  []string          `json:"notes,omitempty"`
	Labels []_Label          `json:"labels,omitempty" gmcp:"items.description=One label."`
	Parent *_Label           `json:"parent,omitempty"`
	ByKey  map[string]_Label `json:"by_key,omitempty"`
}

type _PatchInput struct {
	Org  ID         `json:"org" gmcp:"path=orgs"`
	Item ID         `json:"item" gmcp:"path"`
	Body _PatchBody `json:"body" gmcp:"body"`
}

type _OrgsTask struct {
	ghttp.DefaultHTTPHandlerTask
}

type _ItemsTask struct {
	ghttp.DefaultHTTPHandlerTask
	_Accepted   bool
	_GetCalls   atomic.Int32
	_PatchCalls atomic.Int32
}

func (t *_ItemsTask) _Echo(function string, req *ghttp.Request, params map[string]any) (echo _Echo) {
	tool, _ := params["tool"].(string)
	return _Echo{
		Function: function, Org: t.GetID("orgs", params), ID: t.GetID("items", params), Query: req.Url().RawQuery,
		Tool: tool, Language: req.Header().Get("Accept-Language"),
	}
}

func (t *_ItemsTask) Index(ctx channel.HandlerContext, req *ghttp.Request, resp *ghttp.Response, params map[string]any) (errResponse ghttp.ErrorResponse) {
	resp.JsonResponse(t._Echo("Index", req, params))
	return nil
}

func (t *_ItemsTask) Get(ctx channel.HandlerContext, req *ghttp.Request, resp *ghttp.Response, params map[string]any) (errResponse ghttp.ErrorResponse) {
	t._GetCalls.Add(1)
	if t.GetID("items", params) == "missing" {
		return erresponse.NotFound
	}

	if t._Accepted {
		resp.SetStatusCode(http.StatusAccepted)
		resp.SetHeader("Retry-After", "0")
	}

	resp.JsonResponse(t._Echo("Get", req, params))
	return nil
}

func (t *_ItemsTask) Patch(ctx channel.HandlerContext, req *ghttp.Request, resp *ghttp.Response, params map[string]any) (errResponse ghttp.ErrorResponse) {
	t._PatchCalls.Add(1)
	if t._Accepted {
		resp.SetStatusCode(http.StatusAccepted)
	}

	echo := t._Echo("Patch", req, params)
	echo.Body = string(req.Body().Bytes())
	resp.JsonResponse(echo)
	return nil
}

func (t *_ItemsTask) MCPIndex() (tool Tool) {
	return NewTool[_ListInput, _Echo](&mcp.Tool{Name: "items_list", Description: "List items."})
}

func (t *_ItemsTask) MCPGet() (tool Tool) {
	return NewTool[_GetInput, _Echo](&mcp.Tool{Name: "items_get", Description: "Read one item."})
}

func (t *_ItemsTask) MCPPatch() (tool Tool) {
	return NewTool[_PatchInput, _Echo](&mcp.Tool{Name: "items_update", Description: "Update one item."})
}

type _Money struct {
	Amount int64 `json:"amount" gmcp:"description=Fixed-point amount.;example=2864000000"`
	Scale  int   `json:"scale"`
}

type _ReportRow struct {
	Code  string  `json:"code" gmcp:"description=Platform code."`
	Money *_Money `json:"money,omitempty"`
}

type _Report struct {
	Rows   []_ReportRow      `json:"rows" gmcp:"description=Report rows.;deprecated"`
	ByCode map[string]_Money `json:"by_code,omitempty"`
	Hints  []string          `json:"hints,omitempty" gmcp:"items.description=Hint code.;items.example=partial_data"`
	Total  _Money            `json:"total"`
	ID     ID                `json:"id,omitempty"`
}

type _ReportTask struct {
	ghttp.DefaultHTTPHandlerTask
}

func (t *_ReportTask) Get(ctx channel.HandlerContext, req *ghttp.Request, resp *ghttp.Response, params map[string]any) (errResponse ghttp.ErrorResponse) {
	resp.JsonResponse(_Report{Total: _Money{Amount: 1, Scale: 6}, ID: "not an id"})
	return nil
}

func (t *_ReportTask) MCPGet() (tool Tool) {
	return NewTool[struct{}, _Report](&mcp.Tool{Name: "report", Description: "Read the report."})
}

type _EmptyTask struct {
	ghttp.DefaultHTTPHandlerTask
}

func (t *_EmptyTask) Get(ctx channel.HandlerContext, req *ghttp.Request, resp *ghttp.Response, params map[string]any) (errResponse ghttp.ErrorResponse) {
	return nil
}

func (t *_EmptyTask) MCPGet() (tool Tool) {
	return NewTool[struct{}, *_Money](&mcp.Tool{Name: "empty", Description: "Read nothing."})
}

type _GetTask struct {
	ghttp.DefaultHTTPHandlerTask
	_Tool Tool
}

func (t *_GetTask) MCPGet() (tool Tool) {
	return t._Tool
}

type _IndexTask struct {
	ghttp.DefaultHTTPHandlerTask
	_Tool Tool
}

func (t *_IndexTask) MCPIndex() (tool Tool) {
	return t._Tool
}

type _Envelope struct {
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code int `json:"code"`
	} `json:"error"`
}

type _CallResult struct {
	StructuredContent json.RawMessage `json:"structuredContent"`
	IsError           bool            `json:"isError"`
}

func _ItemsRoute(items *_ItemsTask) (route *ghttp.SimpleRoute) {
	return ghttp.NewSimpleRoute().SetEndpoint("/orgs", &_OrgsTask{}).SetEndpoint("/orgs/items", items)
}

func _BindServer(options Options, route *ghttp.SimpleRoute) (server *Server) {
	server = New(&mcp.Implementation{Name: "test", Version: "1"}, options)
	server.Bind(ghttp.NewDispatchHandler(route), route.RouteEntries())
	return server
}

func _Request(t *testing.T, server *Server, method string, params any) (envelope _Envelope) {
	t.Helper()
	body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
	require.NoError(t, err, "request should encode")
	request := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	request.Header.Set("MCP-Protocol-Version", "2025-11-25")
	request.Header.Set("Authorization", "Bearer token")
	ch := &channel.DefaultChannel{}
	ch.Init()
	ctx := channel.NewMockHandlerContext()
	ctx.On("Channel").Return(ch)
	recorder := httptest.NewRecorder()
	server.Serve(recorder, Caller{Request: ghttp.WrapRequest(ch, request), HandlerContext: ctx, Language: "zh-TW"})
	require.Equal(t, http.StatusOK, recorder.Code, "MCP request should return a JSON-RPC envelope")
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope), "response should be JSON")
	return envelope
}

func _Call(t *testing.T, server *Server, name string, arguments map[string]any) (result _CallResult) {
	t.Helper()
	envelope := _Request(t, server, "tools/call", map[string]any{"name": name, "arguments": arguments})
	require.Nil(t, envelope.Error, "tool call should not be a protocol error")
	require.NoError(t, json.Unmarshal(envelope.Result, &result), "tool result should decode")
	return result
}

func _Tools(t *testing.T, server *Server) (tools map[string]*mcp.Tool, names []string) {
	t.Helper()
	envelope := _Request(t, server, "tools/list", map[string]any{})
	var list mcp.ListToolsResult
	require.NoError(t, json.Unmarshal(envelope.Result, &list), "tool list should decode")
	tools = map[string]*mcp.Tool{}
	for _, tool := range list.Tools {
		tools[tool.Name] = tool
		names = append(names, tool.Name)
	}

	return tools, names
}

func _JSON(t *testing.T, value any) (text string) {
	t.Helper()
	data, err := json.Marshal(value)
	require.NoError(t, err, "value should encode")
	return string(data)
}

func TestServerBindDerivesTools(t *testing.T) {
	t.Parallel()

	server := _BindServer(Options{}, _ItemsRoute(&_ItemsTask{}))
	tools, names := _Tools(t, server)

	assert.ElementsMatch(t, []string{"items_list", "items_get", "items_update"}, names, "every declared function should become a tool")
	assert.JSONEq(t, `{"readOnlyHint":true,"destructiveHint":false,"idempotentHint":false,"openWorldHint":false}`, _JSON(t, tools["items_list"].Annotations), "Index should be read-only")
	assert.JSONEq(t, `{"readOnlyHint":false,"destructiveHint":true,"idempotentHint":false,"openWorldHint":false}`, _JSON(t, tools["items_update"].Annotations), "Patch should be a destructive write")
	assert.JSONEq(t, fmt.Sprintf(`{
		"type":"object","additionalProperties":false,"required":["org"],
		"properties":{
			"org":{"type":"string","minLength":1,"pattern":%q,"description":"Organization."},
			"limit":{"type":"integer","minimum":1,"maximum":100,"examples":[20]},
			"since":{"type":"string","pattern":"^\\d{4}-\\d{2}-\\d{2}$","description":"Date in YYYY-MM-DD form.","deprecated":true},
			"name":{"type":"string","maxLength":20,"pattern":"^[a-z]+$","format":"hostname","examples":["abc"]},
			"tags":{"type":"array","minItems":1,"uniqueItems":true,"items":{"type":"string","enum":["a","b"],"examples":["a"]}}
		}}`, DefaultIDPattern), _JSON(t, tools["items_list"].InputSchema), "list schema should come from the input tags")
	label := `{"type":"object","additionalProperties":false,"required":["key"],
		"properties":{"key":{"type":"string","description":"Label key."},"value":{"type":"string"}}}`
	assert.JSONEq(t, `{
		"type":"object","additionalProperties":false,
		"properties":{
			"name":{"type":"string","description":"New name."},
			"notes":{"type":"array","items":{"type":"string"}},
			"labels":{"type":"array","items":{"type":"object","description":"One label.","additionalProperties":false,"required":["key"],
				"properties":{"key":{"type":"string","description":"Label key."},"value":{"type":"string"}}}},
			"parent":`+label+`,
			"by_key":{"type":"object","additionalProperties":`+label+`}
		}}`, _JSON(t, tools["items_update"].InputSchema.(map[string]any)["properties"].(map[string]any)["body"]), "nested body fields should be optional without null and keep their tags")
}

func TestServerBindDerivesOutputSchema(t *testing.T) {
	t.Parallel()

	route := _ItemsRoute(&_ItemsTask{}).SetEndpoint("/report", &_ReportTask{}).SetEndpoint("/empty", &_EmptyTask{}).
		SetEndpoint("/preset", &_GetTask{_Tool: NewTool[struct{}, _Echo](&mcp.Tool{Name: "preset", OutputSchema: &jsonschema.Schema{Type: "object"}})})
	server := _BindServer(Options{}, route)
	tools, _ := _Tools(t, server)
	plain, err := jsonschema.ForType(reflect.TypeFor[_Echo](), &jsonschema.ForOptions{})
	require.NoError(t, err, "plain output schema should infer")
	want, err := jsonschema.ForType(reflect.TypeFor[_Report](), &jsonschema.ForOptions{})
	require.NoError(t, err, "report output schema should infer")
	money := func(schema *jsonschema.Schema) {
		schema.Properties["amount"].Description, schema.Properties["amount"].Examples = "Fixed-point amount.", []any{2864000000}
	}

	rows := want.Properties["rows"]
	rows.Description, rows.Deprecated = "Report rows.", true
	rows.Items.Properties["code"].Description = "Platform code."
	money(rows.Items.Properties["money"])
	money(want.Properties["by_code"].AdditionalProperties)
	want.Properties["hints"].Items.Description, want.Properties["hints"].Items.Examples = "Hint code.", []any{"partial_data"}
	money(want.Properties["total"])

	assert.JSONEq(t, _JSON(t, plain), _JSON(t, tools["items_get"].OutputSchema), "an output type without gmcp tags should keep the inferred schema")
	assert.JSONEq(t, _JSON(t, want), _JSON(t, tools["report"].OutputSchema), "output tags should only add descriptions to the inferred schema")
	assert.JSONEq(t, `{"type":"object"}`, _JSON(t, tools["preset"].OutputSchema), "a preset output schema should be kept")
	assert.JSONEq(t, `{"rows":null,"total":{"amount":1,"scale":6},"id":"not an id"}`, string(_Call(t, server, "report", map[string]any{}).StructuredContent), "null values and unchecked IDs should pass output validation")
	assert.JSONEq(t, `{"amount":0,"scale":0}`, string(_Call(t, server, "empty", map[string]any{}).StructuredContent), "an empty body should give the zero value of a pointer output")
}

func TestServerForwardsCalls(t *testing.T) {
	t.Parallel()

	server := _BindServer(Options{ToolParam: "tool"}, _ItemsRoute(&_ItemsTask{}))
	tests := []struct {
		name      string
		tool      string
		arguments map[string]any
		want      string
	}{
		{"index with query", "items_list", map[string]any{"org": "o1", "limit": 5, "since": "2026-01-02", "tags": []string{"a", "b"}},
			`{"function":"Index","org":"o1","query":"l=5&since=2026-01-02&tags=a%2Cb","tool":"items_list","language":"zh-TW"}`},
		{"zero query values are not sent", "items_list", map[string]any{"org": "o1"},
			`{"function":"Index","org":"o1","tool":"items_list","language":"zh-TW"}`},
		{"get carries its own id", "items_get", map[string]any{"org": "o1", "item": "i-1.x"},
			`{"function":"Get","org":"o1","id":"i-1.x","tool":"items_get","language":"zh-TW"}`},
		{"patch sends a v1 json body", "items_update", map[string]any{"org": "o1", "item": "i1", "body": map[string]any{"name": "n"}},
			`{"function":"Patch","org":"o1","id":"i1","tool":"items_update","language":"zh-TW","body":"{\"name\":\"n\"}"}`},
		{"handler errors become public codes", "items_get", map[string]any{"org": "o1", "item": "missing"},
			`{"error":{"code":"not_found","message":"Resource not found."}}`},
		{"schema rejects an unsafe id", "items_get", map[string]any{"org": "../x", "item": "i1"},
			`{"error":{"code":"invalid_argument","message":"Invalid arguments."}}`},
		{"path ids are checked at runtime", "items_get", map[string]any{"org": "o1", "item": "a b"},
			`{"error":{"code":"invalid_argument","message":"Invalid arguments."}}`},
		{"dot segments are rejected", "items_get", map[string]any{"org": "o1", "item": ".."},
			`{"error":{"code":"invalid_argument","message":"Invalid arguments."}}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := _Call(t, server, tt.tool, tt.arguments)
			assert.JSONEq(t, tt.want, string(result.StructuredContent), "structured content should match the case")
		})
	}
}

func TestServerPollsOnlyGet(t *testing.T) {
	t.Parallel()

	items := &_ItemsTask{_Accepted: true}
	server := _BindServer(Options{}, _ItemsRoute(items))

	get := _Call(t, server, "items_get", map[string]any{"org": "o1", "item": "i1"})
	update := _Call(t, server, "items_update", map[string]any{"org": "o1", "item": "i1", "body": map[string]any{}})

	assert.False(t, get.IsError, "the last 202 response should be the result")
	assert.EqualValues(t, _MaxForwardAttempts, items._GetCalls.Load(), "GET should poll while the handler answers 202")
	assert.False(t, update.IsError, "a 202 write should be the result")
	assert.EqualValues(t, 1, items._PatchCalls.Load(), "writes should never be resent")
}

func TestServerIDPattern(t *testing.T) {
	t.Parallel()

	server := _BindServer(Options{IDPattern: `^[0-9A-Za-z]+$`}, _ItemsRoute(&_ItemsTask{}))
	tools, _ := _Tools(t, server)

	assert.Equal(t, `^[0-9A-Za-z]+$`, tools["items_get"].InputSchema.(map[string]any)["properties"].(map[string]any)["org"].(map[string]any)["pattern"], "ID schema should use the configured pattern")
	for _, arguments := range []map[string]any{{"org": "a-b", "item": "i1"}, {"org": "o1", "item": "a-b"}} {
		result := _Call(t, server, "items_get", arguments)
		assert.JSONEq(t, `{"error":{"code":"invalid_argument","message":"Invalid arguments."}}`, string(result.StructuredContent), "IDs outside the pattern should be rejected: %v", arguments)
	}
}

func TestServerBindRejectsInvalidDeclarations(t *testing.T) {
	t.Parallel()

	type _UnknownKey struct {
		A string `json:"a" gmcp:"query=a;typo"`
	}

	type _Unplaced struct {
		A string `json:"a"`
	}

	type _OptionalPath struct {
		A string `json:"a,omitempty" gmcp:"path"`
	}

	type _UnknownAncestor struct {
		A ID `json:"a" gmcp:"path=teams"`
	}

	type _OwnID struct {
		A ID `json:"a" gmcp:"path"`
	}

	type _GetBody struct {
		Body struct{} `json:"body" gmcp:"body"`
	}

	type _NestedPlacement struct {
		Body struct {
			Rows []struct {
				A string `json:"a" gmcp:"query=a"`
			} `json:"rows"`
		} `json:"body" gmcp:"body"`
	}

	type _DefaultKey struct {
		A string `json:"a" gmcp:"query=a;default=x"`
	}

	type _BadExample struct {
		A int `json:"a" gmcp:"query=a;example=abc"`
	}

	type _BadPattern struct {
		A string `json:"a" gmcp:"query=a;pattern=["`
	}

	type _OutputValidation struct {
		Rows []struct {
			A string `json:"a" gmcp:"minLength=1"`
		} `json:"rows"`
	}

	type _OutputPlacement struct {
		A string `json:"a" gmcp:"query=a"`
	}

	tool := func(input Tool) (route *ghttp.SimpleRoute) {
		return ghttp.NewSimpleRoute().SetEndpoint("/orgs", &_GetTask{_Tool: input})
	}

	tests := []struct {
		name    string
		options Options
		bind    func(options Options)
		want    string
	}{
		{"unknown tag key", Options{}, func(options Options) {
			_BindServer(options, tool(NewTool[_UnknownKey, _Echo](&mcp.Tool{Name: "x"})))
		}, `invalid gmcp tag key "typo"`},
		{"unplaced field", Options{}, func(options Options) {
			_BindServer(options, tool(NewTool[_Unplaced, _Echo](&mcp.Tool{Name: "x"})))
		}, "needs exactly one valid path, query, or body placement"},
		{"optional own path", Options{}, func(options Options) {
			_BindServer(options, tool(NewTool[_OptionalPath, _Echo](&mcp.Tool{Name: "x"})))
		}, "needs exactly one valid path, query, or body placement"},
		{"unknown ancestor", Options{}, func(options Options) {
			_BindServer(options, tool(NewTool[_UnknownAncestor, _Echo](&mcp.Tool{Name: "x"})))
		}, `path ID "teams" is not an ancestor node`},
		{"index with own id", Options{}, func(options Options) {
			_BindServer(options, ghttp.NewSimpleRoute().SetEndpoint("/orgs", &_IndexTask{_Tool: NewTool[_OwnID, _Echo](&mcp.Tool{Name: "x"})}))
		}, "declared by MCPIndex and cannot carry the ID"},
		{"get with body", Options{}, func(options Options) {
			_BindServer(options, tool(NewTool[_GetBody, _Echo](&mcp.Tool{Name: "x"})))
		}, "forwards GET and cannot carry a body"},
		{"placement inside a nested body", Options{}, func(options Options) {
			_BindServer(options, tool(NewTool[_NestedPlacement, _Echo](&mcp.Tool{Name: "x"})))
		}, "is inside a body and cannot be placed"},
		{"input default key", Options{}, func(options Options) {
			_BindServer(options, tool(NewTool[_DefaultKey, _Echo](&mcp.Tool{Name: "x"})))
		}, `invalid gmcp tag key "default"`},
		{"unparsable example", Options{}, func(options Options) {
			_BindServer(options, tool(NewTool[_BadExample, _Echo](&mcp.Tool{Name: "x"})))
		}, `invalid gmcp tag example "abc"`},
		{"invalid pattern", Options{}, func(options Options) {
			_BindServer(options, tool(NewTool[_BadPattern, _Echo](&mcp.Tool{Name: "x"})))
		}, `invalid gmcp tag pattern "["`},
		{"nested output validation key", Options{}, func(options Options) {
			_BindServer(options, tool(NewTool[struct{}, _OutputValidation](&mcp.Tool{Name: "x"})))
		}, `invalid gmcp tag key "minLength"`},
		{"output placement key", Options{}, func(options Options) {
			_BindServer(options, tool(NewTool[struct{}, _OutputPlacement](&mcp.Tool{Name: "x"})))
		}, `invalid gmcp tag key "query"`},
		{"tool not built by NewTool", Options{}, func(options Options) {
			_BindServer(options, tool(Tool{}))
		}, "declares a tool that NewTool did not build"},
		{"duplicate name", Options{}, func(options Options) {
			_BindServer(options, ghttp.NewSimpleRoute().
				SetEndpoint("/a", &_GetTask{_Tool: NewTool[struct{}, _Echo](&mcp.Tool{Name: "x"})}).
				SetEndpoint("/b", &_GetTask{_Tool: NewTool[struct{}, _Echo](&mcp.Tool{Name: "x"})}))
		}, `tool "x" is declared by /a and /b`},
		{"scope rejects tool", Options{Allows: func(path string, method string) (allowed bool) { return method != http.MethodPatch }}, func(options Options) {
			_BindServer(options, _ItemsRoute(&_ItemsTask{}))
		}, `tool "items_update" cannot PATCH /orgs/items`},
		{"preset input schema", Options{}, func(options Options) {
			NewTool[struct{}, _Echo](&mcp.Tool{Name: "x", InputSchema: map[string]any{"type": "object"}})
		}, "definition without InputSchema"},
		{"unbound public tool", Options{Gateway: &Gateway{Public: []string{"missing"}}}, func(options Options) {
			_BindServer(options, _ItemsRoute(&_ItemsTask{}))
		}, `gateway public tool "missing" is not bound`},
		{"reserved gateway name", Options{Gateway: &Gateway{}}, func(options Options) {
			_BindServer(options, tool(NewTool[struct{}, _Echo](&mcp.Tool{Name: "query"})))
		}, `tool name "query" is reserved`},
		{"bind twice", Options{}, func(options Options) {
			route := _ItemsRoute(&_ItemsTask{})
			server := _BindServer(options, route)
			server.Bind(ghttp.NewDispatchHandler(route), route.RouteEntries())
		}, "Bind called more than once"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var recovered any
			func() {
				defer func() { recovered = recover() }()
				tt.bind(tt.options)
			}()

			require.NotNil(t, recovered, "invalid declaration should panic")
			assert.Contains(t, fmt.Sprint(recovered), tt.want, "panic should name the problem")
		})
	}
}

func TestServerGateway(t *testing.T) {
	t.Parallel()

	server := _BindServer(Options{ToolParam: "tool", Gateway: &Gateway{Public: []string{"items_list"}}}, _ItemsRoute(&_ItemsTask{}))
	tools, names := _Tools(t, server)

	assert.Equal(t, []string{"items_list", "tools", "query"}, names, "only public, discovery, and query tools should be listed")
	assert.False(t, tools["query"].Annotations.ReadOnlyHint, "query should not claim read-only while it can reach a write")
	assert.Equal(t, -32602, _Request(t, server, "tools/call", map[string]any{"name": "items_get", "arguments": map[string]any{"org": "o1", "item": "i1"}}).Error.Code, "hidden tools should not be callable directly")

	discovered := _Call(t, server, "tools", map[string]any{})
	detail := _Call(t, server, "tools", map[string]any{"search": "items_get"})
	queried := _Call(t, server, "query", map[string]any{"name": "items_get", "arguments": map[string]any{"org": "o1", "item": "i1"}})
	missing := _Call(t, server, "query", map[string]any{"name": "items_get", "arguments": map[string]any{"org": "o1", "item": "missing"}})
	public := _Call(t, server, "query", map[string]any{"name": "items_list", "arguments": map[string]any{"org": "o1"}})

	assert.JSONEq(t, `{"names":["items_get","items_update"]}`, string(discovered.StructuredContent), "discovery should list hidden tools")
	assert.Contains(t, string(detail.StructuredContent), `"input_schema"`, "an exact search should return the input schema")
	assert.Contains(t, string(detail.StructuredContent), `"output_schema"`, "an exact search should return the output schema")
	assert.JSONEq(t, `{"function":"Get","org":"o1","id":"i1","tool":"items_get","language":"zh-TW"}`, string(queried.StructuredContent), "query should forward to the named tool")
	assert.JSONEq(t, `{"error":{"code":"not_found","message":"Resource not found."}}`, string(missing.StructuredContent), "query should keep the public error code of the named tool")
	assert.JSONEq(t, `{"error":{"code":"invalid_argument","message":"Invalid arguments."}}`, string(public.StructuredContent), "query should only reach hidden tools")
}
