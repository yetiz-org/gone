package gmcp

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yetiz-org/gone/channel"
	"github.com/yetiz-org/gone/erresponse"
	"github.com/yetiz-org/gone/ghttp"
	buf "github.com/yetiz-org/goth-bytebuf"
	kkerror "github.com/yetiz-org/goth-kkerror"
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
	switch t.GetID("items", params) {
	case "missing":
		return erresponse.NotFound

	case "invalid":
		return &erresponse.DefaultErrorResponse{
			StatusCode: http.StatusBadRequest, Name: "invalid_request", Description: "name is longer than 20", Title: "Invalid name",
			Detail: "Use at most 20 characters.", Data: map[string]any{"field": "name"},
		}

	case "untitled":
		resp.SetStatusCode(http.StatusBadRequest)
		resp.JsonResponse(map[string]any{"error_code": 400103, "title": 1, "detail": "Use at most 20 characters.", "error_description": "name is longer than 20"})
		return nil

	case "locked":
		return &erresponse.DefaultErrorResponse{
			StatusCode: http.StatusConflict, Name: "conflict", Description: "item is locked", Title: "Item is busy", Detail: "Retry later.",
			DefaultKKError: kkerror.DefaultKKError{ErrorCode: "409401", ErrorMessage: "item is locked"},
		}

	case "overloaded":
		return &erresponse.DefaultErrorResponse{
			StatusCode: 529, Name: "server_error", Description: "upstream is overloaded",
			DefaultKKError: kkerror.DefaultKKError{ErrorCode: "529401"},
		}

	case "miscoded":
		resp.SetStatusCode(http.StatusBadRequest)
		resp.JsonResponse(map[string]any{"error_code": "4001031"})
		return nil
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
	return NewTool[_PatchInput, _Echo](&mcp.Tool{Name: "items_update", Description: "Update one item.", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: new(true)}})
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

type _PostTask struct {
	ghttp.DefaultHTTPHandlerTask
	_Tool Tool
}

func (t *_PostTask) MCPPost() (tool Tool) {
	return t._Tool
}

type _LabelsInput struct {
	Org    ID       `json:"org" gmcp:"path=orgs"`
	Labels []_Label `json:"labels" gmcp:"body;description=Labels to set.;minItems=1"`
}

type _LabelsTask struct {
	ghttp.DefaultHTTPHandlerTask
}

func (t *_LabelsTask) Put(ctx channel.HandlerContext, req *ghttp.Request, resp *ghttp.Response, params map[string]any) (errResponse ghttp.ErrorResponse) {
	resp.JsonResponse(_Echo{Function: "Put", Org: t.GetID("orgs", params), Body: string(req.Body().Bytes())})
	return nil
}

func (t *_LabelsTask) MCPPut() (tool Tool) {
	return NewTool[_LabelsInput, _Echo](&mcp.Tool{Name: "labels_set", Description: "Replace labels."})
}

type _UploadInput struct {
	Org   ID    `json:"org" gmcp:"path=orgs"`
	Cover File  `json:"cover" gmcp:"file=file;description=Cover image."`
	Proof *File `json:"proof,omitempty" gmcp:"file=proof"`
}

type _UploadPart struct {
	Field       string `json:"field"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Digest      string `json:"digest"`
}

type _UploadEcho struct {
	Org   string        `json:"org"`
	Form  string        `json:"form"`
	Parts []_UploadPart `json:"parts"`
}

type _UploadTask struct {
	ghttp.DefaultHTTPHandlerTask
	_Calls atomic.Int32
}

func (t *_UploadTask) Post(ctx channel.HandlerContext, req *ghttp.Request, resp *ghttp.Response, params map[string]any) (errResponse ghttp.ErrorResponse) {
	t._Calls.Add(1)
	form, _, _ := mime.ParseMediaType(req.Header().Get("Content-Type"))
	echo := _UploadEcho{Org: t.GetID("orgs", params), Form: form}
	for _, field := range []string{"file", "proof"} {
		file, header, err := req.FormFile(field)
		if err != nil {
			continue
		}

		data, err := io.ReadAll(file)
		_ = file.Close()
		if err != nil {
			return erresponse.InvalidRequest
		}

		echo.Parts = append(echo.Parts, _UploadPart{Field: field, Filename: header.Filename, ContentType: header.Header.Get("Content-Type"), Digest: _Digest(data)})
	}

	resp.JsonResponse(echo)
	return nil
}

func (t *_UploadTask) MCPPost() (tool Tool) {
	return NewTool[_UploadInput, _UploadEcho](&mcp.Tool{Name: "upload", Description: "Upload files."})
}

type _Maybe[T any] struct {
	Set   bool
	Value *T
}

func (o _Maybe[T]) IsZero() (zero bool) {
	return !o.Set
}

func (o _Maybe[T]) JSONSchema() (schema *jsonschema.Schema) {
	schema, err := jsonschema.For[T](nil)
	if err != nil {
		panic(err)
	}

	schema.Types, schema.Type = []string{"null", schema.Type}, ""
	return schema
}

func (o _Maybe[T]) MarshalJSON() (data []byte, err error) {
	return json.Marshal(o.Value)
}

func (o *_Maybe[T]) UnmarshalJSON(data []byte) (err error) {
	o.Set, o.Value = true, nil
	if string(data) == "null" {
		return nil
	}

	return json.Unmarshal(data, &o.Value)
}

type _OptionalBody struct {
	Name _Maybe[string] `json:"name,omitzero" gmcp:"description=New name."`
	Mode _Maybe[string] `json:"mode,omitzero" gmcp:"description=New mode.;enum=a,b"`
	Kind _Maybe[string] `json:"kind,omitzero" gmcp:"description=New kind.;enum=c"`
}

type _OptionalInput struct {
	Org  ID            `json:"org" gmcp:"path=orgs"`
	Item ID            `json:"item" gmcp:"path"`
	Body _OptionalBody `json:"body" gmcp:"body"`
}

type _OptionalTask struct {
	ghttp.DefaultHTTPHandlerTask
}

func (t *_OptionalTask) Patch(ctx channel.HandlerContext, req *ghttp.Request, resp *ghttp.Response, params map[string]any) (errResponse ghttp.ErrorResponse) {
	resp.JsonResponse(_Echo{Function: "Patch", Body: string(req.Body().Bytes())})
	return nil
}

func (t *_OptionalTask) MCPPatch() (tool Tool) {
	return NewTool[_OptionalInput, _Echo](&mcp.Tool{Name: "optional_update", Description: "Update optional values."})
}

type _DownloadInput struct {
	File ID `json:"file" gmcp:"path"`
}

type _DownloadTask struct {
	ghttp.DefaultHTTPHandlerTask
	_Tool Tool
}

func (t *_DownloadTask) Get(ctx channel.HandlerContext, req *ghttp.Request, resp *ghttp.Response, params map[string]any) (errResponse ghttp.ErrorResponse) {
	switch t.GetID("files", params) {
	case "report":
		resp.SetHeader("Content-Type", "application/pdf")
		resp.SetHeader("Content-Disposition", `attachment; filename="report.pdf"`)
		resp.SetBody(buf.NewByteBuf(append([]byte{0x00, 0xff}, req.Header().Get("Accept")...)))

	case "empty":
		resp.SetHeader("Content-Type", "text/plain")

	default:
		resp.SetHeader("Content-Disposition", `attachment; filename="../report.pdf"`)
		resp.SetBody(buf.NewByteBuf([]byte{0x01}))
	}

	return nil
}

func (t *_DownloadTask) MCPGet() (tool Tool) {
	return t._Tool
}

type _Link struct {
	Function string `json:"function"`
	ID       string `json:"id"`
	URL      string `json:"url,omitempty" gmcp:"description=Item page."`
}

type _LinkTask struct {
	_ItemsTask
	_Base     string
	_Adjusted atomic.Int32
}

func (t *_LinkTask) MCPGet() (tool Tool) {
	return NewTool[_GetInput, _Link](&mcp.Tool{Name: "items_link", Description: "Read one item link."}, WithOutputAdjuster(t._AddLink), WithOutputAdjuster(t._MarkLink))
}

func (t *_LinkTask) _AddLink(ctx context.Context, input _GetInput, output _Link) (adjusted _Link, err error) {
	t._Adjusted.Add(1)
	if input.Item == "broken" {
		return output, errors.New("link unavailable")
	}

	output.URL = t._Base + "/orgs/" + string(input.Org) + "/items/" + output.ID
	return output, nil
}

func (t *_LinkTask) _MarkLink(ctx context.Context, input _GetInput, output _Link) (adjusted _Link, err error) {
	output.URL += "?from=" + strings.ToLower(output.Function)
	return output, nil
}

type _Envelope struct {
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	} `json:"error"`
}

type _ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type _CallResult struct {
	Content           []_ContentBlock `json:"content"`
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

func _Serve(t *testing.T, server *Server, method string, params any) (recorder *httptest.ResponseRecorder) {
	t.Helper()
	body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
	require.NoError(t, err, "request should encode")
	return _ServeBody(t, server, body, nil)
}

// _ServeBody serves one MCP HTTP request with body and the standard request headers; headers overrides or adds headers.
func _ServeBody(t *testing.T, server *Server, body []byte, headers map[string]string) (recorder *httptest.ResponseRecorder) {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	request.Header.Set("MCP-Protocol-Version", "2025-11-25")
	request.Header.Set("Authorization", "Bearer token")
	for name, value := range headers {
		request.Header.Set(name, value)
	}

	ch := &channel.DefaultChannel{}
	ch.Init()
	ctx := channel.NewMockHandlerContext()
	ctx.On("Channel").Return(ch)
	recorder = httptest.NewRecorder()
	server.Serve(recorder, Caller{Request: ghttp.WrapRequest(ch, request), HandlerContext: ctx, Language: "zh-TW"})
	return recorder
}

func _Request(t *testing.T, server *Server, method string, params any) (envelope _Envelope) {
	t.Helper()
	recorder := _Serve(t, server, method, params)
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

// _Payload returns the structured content of a successful result, or the text content of an error result, which must
// not carry structured content.
func _Payload(t *testing.T, result _CallResult) (payload string) {
	t.Helper()
	if !result.IsError {
		return string(result.StructuredContent)
	}

	assert.Empty(t, result.StructuredContent, "an error result should not carry structured content")
	require.Len(t, result.Content, 1, "an error result should hold one text block")
	return result.Content[0].Text
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

func _DataURL(mediaType string, data []byte) (dataURL string) {
	return "data:" + mediaType + ";base64," + base64.StdEncoding.EncodeToString(data)
}

func _Digest(data []byte) (digest string) {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func _UploadRoute(upload *_UploadTask) (route *ghttp.SimpleRoute) {
	return ghttp.NewSimpleRoute().SetEndpoint("/orgs", &_OrgsTask{}).SetEndpoint("/orgs/uploads", upload)
}

func TestServerBindDerivesTools(t *testing.T) {
	t.Parallel()

	server := _BindServer(Options{}, _ItemsRoute(&_ItemsTask{}))
	tools, names := _Tools(t, server)

	assert.ElementsMatch(t, []string{"items_list", "items_get", "items_update"}, names, "every declared function should become a tool")
	assert.JSONEq(t, `{"readOnlyHint":true,"destructiveHint":false,"idempotentHint":false,"openWorldHint":false}`, _JSON(t, tools["items_list"].Annotations), "Index should be read-only")
	assert.JSONEq(t, `{"readOnlyHint":false,"destructiveHint":true,"idempotentHint":false,"openWorldHint":true}`, _JSON(t, tools["items_update"].Annotations), "Patch should be a destructive write that keeps only its declared openWorld hint")
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

func TestServerToolsReportsBoundTools(t *testing.T) {
	t.Parallel()

	route := _ItemsRoute(&_ItemsTask{}).SetEndpoint("/orgs/labels", &_LabelsTask{})
	server := New(&mcp.Implementation{Name: "test", Version: "1"}, Options{Gateway: &Gateway{Public: []string{"items_list"}}})
	assert.Nil(t, server.Tools(), "no tool should be reported before Bind")

	server.Bind(ghttp.NewDispatchHandler(route), route.RouteEntries())
	want := []BoundTool{
		{Name: "items_list", Method: http.MethodGet, Path: "/orgs/items"},
		{Name: "items_get", Method: http.MethodGet, Path: "/orgs/items"},
		{Name: "items_update", Method: http.MethodPatch, Path: "/orgs/items"},
		{Name: "labels_set", Method: http.MethodPut, Path: "/orgs/labels"},
	}
	tools := server.Tools()
	assert.Equal(t, want, tools, "every forwarding tool should be reported in registration order without the gateway tools")

	tools[0].Name = "changed"
	assert.Equal(t, want, server.Tools(), "changing the returned slice should not change the reported tools")

	empty := _BindServer(Options{}, ghttp.NewSimpleRoute().SetEndpoint("/orgs", &_OrgsTask{}))
	assert.Equal(t, []BoundTool{}, empty.Tools(), "a bound server without tools should report an empty slice")
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
		{"handler errors keep their category and code", "items_get", map[string]any{"org": "o1", "item": "missing"},
			`{"error":{"category":"not_found","code":"404001"}}`},
		{"handler error title and detail are kept", "items_get", map[string]any{"org": "o1", "item": "invalid"},
			`{"error":{"category":"invalid_argument","title":"Invalid name","detail":"Use at most 20 characters."}}`},
		{"a conflict keeps its code, title, and detail", "items_get", map[string]any{"org": "o1", "item": "locked"},
			`{"error":{"category":"conflict","code":"409401","title":"Item is busy","detail":"Retry later."}}`},
		{"an overloaded upstream is temporarily unavailable", "items_get", map[string]any{"org": "o1", "item": "overloaded"},
			`{"error":{"category":"temporarily_unavailable","code":"529401"}}`},
		{"non-string error fields are dropped", "items_get", map[string]any{"org": "o1", "item": "untitled"},
			`{"error":{"category":"invalid_argument","detail":"Use at most 20 characters."}}`},
		{"a code that is not six digits is dropped", "items_get", map[string]any{"org": "o1", "item": "miscoded"},
			`{"error":{"category":"invalid_argument"}}`},
		{"schema rejects an unsafe id", "items_get", map[string]any{"org": "../x", "item": "i1"},
			`{"error":{"category":"invalid_argument"}}`},
		{"path ids are checked at runtime", "items_get", map[string]any{"org": "o1", "item": "a b"},
			`{"error":{"category":"invalid_argument"}}`},
		{"dot segments are rejected", "items_get", map[string]any{"org": "o1", "item": ".."},
			`{"error":{"category":"invalid_argument"}}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := _Call(t, server, tt.tool, tt.arguments)
			assert.JSONEq(t, tt.want, _Payload(t, result), "the payload should match the case")
			require.Len(t, result.Content, 1, "the content should hold one text block")
			assert.JSONEq(t, tt.want, result.Content[0].Text, "the text content should match the payload")
		})
	}
}

func TestServerForwardsArrayBody(t *testing.T) {
	t.Parallel()

	server := _BindServer(Options{}, ghttp.NewSimpleRoute().SetEndpoint("/orgs", &_OrgsTask{}).SetEndpoint("/orgs/labels", &_LabelsTask{}))
	tools, _ := _Tools(t, server)
	schema := tools["labels_set"].InputSchema.(map[string]any)
	result := _Call(t, server, "labels_set", map[string]any{"org": "o1", "labels": []any{map[string]any{"key": "a", "value": "1"}, map[string]any{"key": "b"}}})

	assert.Equal(t, []any{"org", "labels"}, schema["required"], "an array body should stay required")
	assert.JSONEq(t, `{"type":"array","description":"Labels to set.","minItems":1,
		"items":{"type":"object","additionalProperties":false,"required":["key"],
			"properties":{"key":{"type":"string","description":"Label key."},"value":{"type":"string"}}}}`,
		_JSON(t, schema["properties"].(map[string]any)["labels"]), "an array body should keep its tags and annotate its element fields")
	assert.JSONEq(t, `{"function":"Put","org":"o1","body":"[{\"key\":\"a\",\"value\":\"1\"},{\"key\":\"b\"}]"}`, string(result.StructuredContent), "the array should be the JSON body")
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
		assert.JSONEq(t, `{"error":{"category":"invalid_argument"}}`, _Payload(t, result), "IDs outside the pattern should be rejected: %v", arguments)
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

	type _GetArrayBody struct {
		Body []struct{} `json:"body" gmcp:"body"`
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

	type _GetFile struct {
		A File `json:"a" gmcp:"file=a"`
	}

	type _BodyAndFile struct {
		Body struct{} `json:"body" gmcp:"body"`
		A    File     `json:"a" gmcp:"file=a"`
	}

	type _FileInBody struct {
		Body struct {
			Files []File `json:"files"`
		} `json:"body" gmcp:"body"`
	}

	type _FilePlacedInBody struct {
		Body struct {
			A File `json:"a" gmcp:"file=a"`
		} `json:"body" gmcp:"body"`
	}

	type _FileAsBody struct {
		A File `json:"a" gmcp:"body"`
	}

	type _NotAFile struct {
		A string `json:"a" gmcp:"file=a"`
	}

	type _UnnamedFile struct {
		A File `json:"a" gmcp:"file"`
	}

	type _DuplicateFile struct {
		A File  `json:"a" gmcp:"file=a"`
		B *File `json:"b,omitempty" gmcp:"file=a"`
	}

	type _OptionalFileValue struct {
		A File `json:"a,omitzero" gmcp:"file=a"`
	}

	tool := func(input Tool) (route *ghttp.SimpleRoute) {
		return ghttp.NewSimpleRoute().SetEndpoint("/orgs", &_GetTask{_Tool: input})
	}

	write := func(input Tool) (route *ghttp.SimpleRoute) {
		return ghttp.NewSimpleRoute().SetEndpoint("/orgs", &_PostTask{_Tool: input})
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
		}, "needs exactly one valid path, query, body, or file placement"},
		{"optional own path", Options{}, func(options Options) {
			_BindServer(options, tool(NewTool[_OptionalPath, _Echo](&mcp.Tool{Name: "x"})))
		}, "needs exactly one valid path, query, body, or file placement"},
		{"unknown ancestor", Options{}, func(options Options) {
			_BindServer(options, tool(NewTool[_UnknownAncestor, _Echo](&mcp.Tool{Name: "x"})))
		}, `path ID "teams" is not an ancestor node`},
		{"index with own id", Options{}, func(options Options) {
			_BindServer(options, ghttp.NewSimpleRoute().SetEndpoint("/orgs", &_IndexTask{_Tool: NewTool[_OwnID, _Echo](&mcp.Tool{Name: "x"})}))
		}, "declared by MCPIndex and cannot carry the ID"},
		{"get with body", Options{}, func(options Options) {
			_BindServer(options, tool(NewTool[_GetBody, _Echo](&mcp.Tool{Name: "x"})))
		}, "forwards GET and cannot carry a body"},
		{"get with an array body", Options{}, func(options Options) {
			_BindServer(options, tool(NewTool[_GetArrayBody, _Echo](&mcp.Tool{Name: "x"})))
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
		{"get with file", Options{}, func(options Options) {
			_BindServer(options, tool(NewTool[_GetFile, _Echo](&mcp.Tool{Name: "x"})))
		}, "forwards GET and cannot carry a file"},
		{"body and file", Options{}, func(options Options) {
			_BindServer(options, write(NewTool[_BodyAndFile, _Echo](&mcp.Tool{Name: "x"})))
		}, "cannot carry both a body and a file"},
		{"file type inside a body", Options{}, func(options Options) {
			_BindServer(options, write(NewTool[_FileInBody, _Echo](&mcp.Tool{Name: "x"})))
		}, "gmcp.File is valid only in a top-level file field"},
		{"file placement inside a body", Options{}, func(options Options) {
			_BindServer(options, write(NewTool[_FilePlacedInBody, _Echo](&mcp.Tool{Name: "x"})))
		}, "is inside a body and cannot be placed"},
		{"file as body", Options{}, func(options Options) {
			_BindServer(options, write(NewTool[_FileAsBody, _Echo](&mcp.Tool{Name: "x"})))
		}, "gmcp.File is valid only in a top-level file field"},
		{"file placement on another type", Options{}, func(options Options) {
			_BindServer(options, write(NewTool[_NotAFile, _Echo](&mcp.Tool{Name: "x"})))
		}, "needs exactly one valid path, query, body, or file placement"},
		{"file without a form name", Options{}, func(options Options) {
			_BindServer(options, write(NewTool[_UnnamedFile, _Echo](&mcp.Tool{Name: "x"})))
		}, "needs exactly one valid path, query, body, or file placement"},
		{"duplicate form name", Options{}, func(options Options) {
			_BindServer(options, write(NewTool[_DuplicateFile, _Echo](&mcp.Tool{Name: "x"})))
		}, "needs exactly one valid path, query, body, or file placement"},
		{"optional file value", Options{}, func(options Options) {
			_BindServer(options, write(NewTool[_OptionalFileValue, _Echo](&mcp.Tool{Name: "x"})))
		}, "needs exactly one valid path, query, body, or file placement"},
		{"input of any type", Options{}, func(options Options) {
			_BindServer(options, tool(NewTool[any, _Echo](&mcp.Tool{Name: "x"})))
		}, `tool "x" input interface {} is not a struct; use struct{} for a tool without input`},
		{"pointer input", Options{}, func(options Options) {
			_BindServer(options, tool(NewTool[*_GetInput, _Echo](&mcp.Tool{Name: "x"})))
		}, `tool "x" input *gmcp._GetInput is not a struct`},
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
	assert.Equal(t, new(true), tools["query"].Annotations.OpenWorldHint, "query should be open-world while it can reach an open-world tool")
	assert.Equal(t, new(false), tools["tools"].Annotations.OpenWorldHint, "discovery should stay closed-world")
	hidden := _Request(t, server, "tools/call", map[string]any{"name": "items_get", "arguments": map[string]any{"org": "o1", "item": "i1"}})
	require.NotNil(t, hidden.Error, "hidden tools should not be callable directly")
	assert.Equal(t, -32602, hidden.Error.Code, "a hidden tool call should be invalid params")
	assert.Equal(t, "Invalid arguments.", hidden.Error.Message, "a protocol error should have the fixed message of its category")
	assert.JSONEq(t, `{"category":"invalid_argument"}`, string(hidden.Error.Data), "a protocol error should carry only the public category")

	discovered := _Call(t, server, "tools", map[string]any{})
	detail := _Call(t, server, "tools", map[string]any{"search": "items_get"})
	queried := _Call(t, server, "query", map[string]any{"name": "items_get", "arguments": map[string]any{"org": "o1", "item": "i1"}})
	missing := _Call(t, server, "query", map[string]any{"name": "items_get", "arguments": map[string]any{"org": "o1", "item": "missing"}})
	invalid := _Call(t, server, "query", map[string]any{"name": "items_get", "arguments": map[string]any{"org": "o1", "item": "invalid"}})
	public := _Call(t, server, "query", map[string]any{"name": "items_list", "arguments": map[string]any{"org": "o1"}})

	assert.JSONEq(t, `{"names":["items_get","items_update"]}`, string(discovered.StructuredContent), "discovery should list hidden tools")
	assert.Contains(t, string(detail.StructuredContent), `"input_schema"`, "an exact search should return the input schema")
	assert.Contains(t, string(detail.StructuredContent), `"output_schema"`, "an exact search should return the output schema")
	assert.JSONEq(t, `{"function":"Get","org":"o1","id":"i1","tool":"items_get","language":"zh-TW"}`, string(queried.StructuredContent), "query should forward to the named tool")
	assert.JSONEq(t, `{"error":{"category":"not_found","code":"404001"}}`, _Payload(t, missing), "query should keep the category and code of the named tool")
	assert.JSONEq(t, `{"error":{"category":"invalid_argument","title":"Invalid name","detail":"Use at most 20 characters."}}`, _Payload(t, invalid), "query should keep the title and detail of the named tool")
	assert.JSONEq(t, `{"error":{"category":"invalid_argument"}}`, _Payload(t, public), "query should only reach hidden tools")
}

func TestServerSanitizesMCPProtocolErrors(t *testing.T) {
	t.Parallel()

	server := _BindServer(Options{}, _ItemsRoute(&_ItemsTask{}))
	call := func(version string) (body []byte) {
		return fmt.Appendf(nil, `{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"items_get","arguments":{"org":"o1","item":"i1"},
			"_meta":{"io.modelcontextprotocol/protocolVersion":%q,"io.modelcontextprotocol/clientInfo":{"name":"c","version":"1"},"io.modelcontextprotocol/clientCapabilities":{}}}}`, version)
	}

	mismatch := _ServeBody(t, server, call("2026-07-28"), map[string]string{"MCP-Protocol-Version": "2026-07-28", "Mcp-Method": "tools/call", "Mcp-Name": "items_list"})
	unsupported := _ServeBody(t, server, call("2099-01-01"), map[string]string{"MCP-Protocol-Version": "2099-01-01", "Mcp-Method": "tools/call", "Mcp-Name": "items_get"})
	writer := &_ResponseWriter{}

	assert.Equal(t, http.StatusBadRequest, mismatch.Code, "a header mismatch should be rejected")
	assert.JSONEq(t, `{"jsonrpc":"2.0","id":7,"error":{"code":-32020,"message":"Invalid arguments.","data":{"category":"invalid_argument"}}}`, mismatch.Body.String(), "a header mismatch should be a client error")
	assert.Equal(t, http.StatusBadRequest, unsupported.Code, "an unsupported protocol version should be rejected")
	assert.JSONEq(t, fmt.Sprintf(`{"jsonrpc":"2.0","id":7,"error":{"code":-32022,"message":"Invalid arguments.","data":{"category":"invalid_argument","supported":%s,"requested":"2099-01-01"}}}`, _JSON(t, mcp.SupportedProtocolVersions())),
		unsupported.Body.String(), "an unsupported protocol version should keep the supported and requested versions")
	assert.JSONEq(t, `{"jsonrpc":"2.0","id":2,"error":{"code":-32021,"message":"Invalid arguments.","data":{"category":"invalid_argument","requiredCapabilities":{"elicitation":{}}}}}`,
		string(writer._SanitizeJSON([]byte(`{"jsonrpc":"2.0","id":2,"error":{"code":-32021,"message":"needs elicitation","data":{"requiredCapabilities":{"elicitation":{}},"note":"internal"}}}`))),
		"missing client capabilities should keep only the decoded capability set")
	assert.JSONEq(t, `{"jsonrpc":"2.0","id":3,"error":{"code":-32022,"message":"Invalid arguments.","data":{"category":"invalid_argument"}}}`,
		string(writer._SanitizeJSON([]byte(`{"jsonrpc":"2.0","id":3,"error":{"code":-32022,"message":"bad","data":"2099-01-01"}}`))),
		"version data that is not an object should be dropped")
}

func TestServerForwardsFiles(t *testing.T) {
	t.Parallel()

	upload := &_UploadTask{}
	server := _BindServer(Options{MaxBodyBytes: 1024}, _UploadRoute(upload))
	tools, _ := _Tools(t, server)
	png, text := []byte("\x89PNG\x00\xff"), []byte("hello")
	cover := map[string]any{"filename": "封面 1.png", "data": _DataURL("image/png", png)}
	file := func(filename string, data string) (arguments map[string]any) {
		return map[string]any{"org": "o1", "cover": map[string]any{"filename": filename, "data": data}}
	}

	invalid := `{"error":{"category":"invalid_argument"}}`
	tests := []struct {
		name      string
		arguments map[string]any
		want      string
	}{
		{"one file part", map[string]any{"org": "o1", "cover": cover},
			fmt.Sprintf(`{"org":"o1","form":"multipart/form-data","parts":[{"field":"file","filename":"封面 1.png","content_type":"image/png","digest":%q}]}`, _Digest(png))},
		{"optional part with media type parameters", map[string]any{"org": "o1", "cover": cover, "proof": map[string]any{"filename": "proof.txt", "data": _DataURL("text/plain;charset=utf-8", text)}},
			fmt.Sprintf(`{"org":"o1","form":"multipart/form-data","parts":[
				{"field":"file","filename":"封面 1.png","content_type":"image/png","digest":%q},
				{"field":"proof","filename":"proof.txt","content_type":"text/plain;charset=utf-8","digest":%q}]}`, _Digest(png), _Digest(text))},
		{"empty filename", file("", _DataURL("image/png", png)), invalid},
		{"filename with a slash", file("a/b.png", _DataURL("image/png", png)), invalid},
		{"filename with a backslash", file(`a\b.png`, _DataURL("image/png", png)), invalid},
		{"dot filename", file(".", _DataURL("image/png", png)), invalid},
		{"dot dot filename", file("..", _DataURL("image/png", png)), invalid},
		{"filename with a control character", file("a\nb.png", _DataURL("image/png", png)), invalid},
		{"percent-encoded data URL", file("a.txt", "data:text/plain,hello"), invalid},
		{"data URL without a media type", file("a.png", "data:;base64,AAAA"), invalid},
		{"media type without a subtype", file("a.png", _DataURL("image", png)), invalid},
		{"unparsable media type", file("a.png", _DataURL("image/png;=x", png)), invalid},
		{"plain base64 without a data URL", file("a.png", base64.StdEncoding.EncodeToString(png)), invalid},
		{"broken base64", file("a.png", "data:image/png;base64,iVBOR@@"), invalid},
		{"empty data", file("a.png", "data:image/png;base64,"), invalid},
		{"multipart body over MaxBodyBytes", file("a.png", _DataURL("image/png", bytes.Repeat([]byte{0xff}, 1024))), invalid},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := _Call(t, server, "upload", tt.arguments)
			assert.JSONEq(t, tt.want, _Payload(t, result), "the payload should match the case")
		})
	}

	t.Cleanup(func() {
		assert.EqualValues(t, 2, upload._Calls.Load(), "only valid files should reach the handler")
	})

	fileSchema := func(description string) (schema string) {
		return fmt.Sprintf(`{"type":"object","description":%q,"additionalProperties":false,"required":["filename","data"],
			"properties":{
				"filename":{"type":"string","minLength":1,"description":"File name without a directory, such as cover.png."},
				"data":{"type":"string","pattern":"^data:[^,]+;base64,","description":"Base64 data URL with a media type, such as data:image/png;base64,iVBORw0KGgo=."}
			}}`, description)
	}

	properties := tools["upload"].InputSchema.(map[string]any)["properties"].(map[string]any)
	assert.Equal(t, []any{"org", "cover"}, tools["upload"].InputSchema.(map[string]any)["required"], "a pointer file should be optional")
	assert.JSONEq(t, fileSchema("Cover image."), _JSON(t, properties["cover"]), "a file field should use the fixed File schema with its tag description")
	assert.JSONEq(t, fileSchema("File to upload."), _JSON(t, properties["proof"]), "an optional file should use the fixed File schema without null")
}

func TestServerMaxRequestBodyBytes(t *testing.T) {
	t.Parallel()

	limit := int64(mcp.DefaultMaxRequestBodyBytes + 1<<20)
	upload := &_UploadTask{}
	configured := _BindServer(Options{MaxRequestBodyBytes: limit}, _UploadRoute(upload))
	standard := _BindServer(Options{}, _UploadRoute(&_UploadTask{}))
	call := func(size int64) (params map[string]any) {
		data := _DataURL("application/octet-stream", bytes.Repeat([]byte{0xff}, int(size)))
		return map[string]any{"name": "upload", "arguments": map[string]any{"org": "o1", "cover": map[string]any{"filename": "a.bin", "data": data}}}
	}

	// Base64 makes the request 4/3 of the file size.
	above := call(mcp.DefaultMaxRequestBodyBytes * 3 / 4)
	accepted := _Request(t, configured, "tools/call", above)
	defaulted := _Serve(t, standard, "tools/call", above)
	rejected := _Serve(t, configured, "tools/call", call(limit*3/4))

	var result _CallResult
	require.Nil(t, accepted.Error, "a request within the configured limit should not be a protocol error")
	require.NoError(t, json.Unmarshal(accepted.Result, &result), "tool result should decode")
	assert.False(t, result.IsError, "a request above the SDK default but within the configured limit should be served")
	assert.EqualValues(t, 1, upload._Calls.Load(), "the accepted request should reach the handler")
	assert.Equal(t, http.StatusRequestEntityTooLarge, defaulted.Code, "zero should keep the SDK default limit")
	assert.Equal(t, http.StatusRequestEntityTooLarge, rejected.Code, "a request above the configured limit should be rejected")
	assert.JSONEq(t, `{"error":{"category":"invalid_argument"}}`, rejected.Body.String(), "the rejection should be a public category")
}

func TestServerReturnsBlobs(t *testing.T) {
	t.Parallel()

	route := ghttp.NewSimpleRoute().
		SetEndpoint("/files", &_DownloadTask{_Tool: NewTool[_DownloadInput, Blob](&mcp.Tool{Name: "file_get", Description: "Read one file."})}).
		SetEndpoint("/raw", &_DownloadTask{_Tool: NewTool[struct{}, _Echo](&mcp.Tool{Name: "raw", Description: "Read raw content."})})
	server := _BindServer(Options{}, route)
	tools, _ := _Tools(t, server)
	tests := []struct {
		name    string
		file    string
		want    string
		summary string
	}{
		{"media type and filename", "report", fmt.Sprintf(`{"filename":"report.pdf","data":%q}`, _DataURL("application/pdf", []byte("\x00\xff*/*"))),
			`{"media_type":"application/pdf","filename":"report.pdf"}`},
		{"defaults without headers and drops unsafe filenames", "plain", `{"data":"data:application/octet-stream;base64,AQ=="}`, `{"media_type":"application/octet-stream"}`},
		{"empty body", "empty", `{"data":"data:text/plain;base64,"}`, `{"media_type":"text/plain"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := _Call(t, server, "file_get", map[string]any{"file": tt.file})
			assert.False(t, result.IsError, "a binary response should be a result")
			assert.JSONEq(t, tt.want, string(result.StructuredContent), "the body should be returned as a data URL")
			require.Len(t, result.Content, 1, "the content should hold one summary block")
			assert.Equal(t, "text", result.Content[0].Type, "the summary should be text")
			assert.JSONEq(t, tt.summary, result.Content[0].Text, "the summary should not repeat the data")
		})
	}

	assert.JSONEq(t, `{"error":{"category":"internal_error"}}`, _Payload(t, _Call(t, server, "raw", map[string]any{})), "a binary response should still fail for a JSON output")
	assert.JSONEq(t, `{"type":"object","additionalProperties":false,"required":["data"],
		"properties":{
			"filename":{"type":"string","description":"Suggested file name, when the response names one."},
			"data":{"type":"string","description":"Base64 data URL with the media type of the content."}
		}}`, _JSON(t, tools["file_get"].OutputSchema), "a Blob output should use the fixed Blob schema")
}

func TestServerJSONSchemaProvider(t *testing.T) {
	t.Parallel()

	server := _BindServer(Options{}, ghttp.NewSimpleRoute().SetEndpoint("/orgs", &_OrgsTask{}).SetEndpoint("/orgs/items", &_OptionalTask{}))
	tools, _ := _Tools(t, server)
	invalid := `{"error":{"category":"invalid_argument"}}`
	tests := []struct {
		name string
		body map[string]any
		want string
	}{
		{"omitted values are not sent", map[string]any{}, `{"function":"Patch","body":"{}"}`},
		{"null is sent", map[string]any{"name": nil}, `{"function":"Patch","body":"{\"name\":null}"}`},
		{"values are sent", map[string]any{"name": "n", "mode": "a", "kind": "c"}, `{"function":"Patch","body":"{\"name\":\"n\",\"mode\":\"a\",\"kind\":\"c\"}"}`},
		{"field enum tags still validate", map[string]any{"mode": "c"}, invalid},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := _Call(t, server, "optional_update", map[string]any{"org": "o1", "item": "i1", "body": tt.body})
			assert.JSONEq(t, tt.want, _Payload(t, result), "the payload should match the case")
		})
	}

	assert.JSONEq(t, `{"type":"object","additionalProperties":false,
		"properties":{
			"name":{"type":["null","string"],"description":"New name."},
			"mode":{"type":["null","string"],"description":"New mode.","enum":["a","b"]},
			"kind":{"type":["null","string"],"description":"New kind.","enum":["c"]}
		}}`, _JSON(t, tools["optional_update"].InputSchema.(map[string]any)["properties"].(map[string]any)["body"]), "provided schemas should keep null, stay optional, and take their own field tags")
}

func TestServerAdjustsOutputs(t *testing.T) {
	t.Parallel()

	link := &_LinkTask{_Base: "https://example.com"}
	server := _BindServer(Options{}, ghttp.NewSimpleRoute().SetEndpoint("/orgs", &_OrgsTask{}).SetEndpoint("/orgs/items", link))
	tests := []struct {
		name     string
		item     string
		want     string
		adjusted int32
	}{
		{"adjusters run in order on the decoded output", "i1", `{"function":"Get","id":"i1","url":"https://example.com/orgs/o1/items/i1?from=get"}`, 1},
		{"an adjuster error fails the call", "broken", `{"error":{"category":"internal_error"}}`, 1},
		{"a failed REST call is not adjusted", "missing", `{"error":{"category":"not_found","code":"404001"}}`, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := link._Adjusted.Load()
			result := _Call(t, server, "items_link", map[string]any{"org": "o1", "item": tt.item})
			assert.JSONEq(t, tt.want, _Payload(t, result), "the payload should match the case")
			require.Len(t, result.Content, 1, "the content should hold one text block")
			assert.JSONEq(t, tt.want, result.Content[0].Text, "the text content should match the payload")
			assert.Equal(t, tt.adjusted, link._Adjusted.Load()-before, "adjusters should run only after a successful REST call")
		})
	}

	assert.PanicsWithValue(t, "gmcp: WithOutputAdjuster needs an adjuster", func() {
		WithOutputAdjuster[_GetInput, _Link](nil)
	}, "a nil adjuster should be rejected at declaration")
}
