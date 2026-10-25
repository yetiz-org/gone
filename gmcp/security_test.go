package gmcp

import (
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yetiz-org/gone/channel"
	"github.com/yetiz-org/gone/ghttp"
)

type _AmbiguousReadTask struct {
	_ItemsTask
	_Tool       Tool
	_IndexCalls atomic.Int32
}

func (t *_AmbiguousReadTask) MCPGet() (tool Tool) {
	return t._Tool
}

func (t *_AmbiguousReadTask) Index(ctx channel.HandlerContext, req *ghttp.Request, resp *ghttp.Response, params map[string]any) (failure ghttp.ErrorResponse) {
	t._IndexCalls.Add(1)
	resp.JsonResponse(t._Echo("Index", req, params))
	return nil
}

type _ReadAcceptance struct {
	ghttp.DispatchAcceptance
	_Calls atomic.Int32
}

func (a *_ReadAcceptance) Do(ctx channel.HandlerContext, req *ghttp.Request, resp *ghttp.Response, params map[string]any) (rtErr error) {
	a._Calls.Add(1)
	return nil
}

type _AmbiguousWriteTask struct {
	ghttp.DefaultHTTPHandlerTask
	_CreateCalls atomic.Int32
	_PostCalls   atomic.Int32
}

func (t *_AmbiguousWriteTask) Create(ctx channel.HandlerContext, req *ghttp.Request, resp *ghttp.Response, params map[string]any) (failure ghttp.ErrorResponse) {
	t._CreateCalls.Add(1)
	resp.JsonResponse(_Echo{Function: "Create"})
	return nil
}

func (t *_AmbiguousWriteTask) Post(ctx channel.HandlerContext, req *ghttp.Request, resp *ghttp.Response, params map[string]any) (failure ghttp.ErrorResponse) {
	t._PostCalls.Add(1)
	resp.JsonResponse(_Echo{Function: "Post"})
	return nil
}

func (t *_AmbiguousWriteTask) MCPPost() (tool Tool) {
	return NewTool[struct{}, _Echo](&mcp.Tool{Name: "collection_post"})
}

type _LargeOutputTask struct {
	ghttp.DefaultHTTPHandlerTask
	_Size int
}

func (t *_LargeOutputTask) Get(ctx channel.HandlerContext, req *ghttp.Request, resp *ghttp.Response, params map[string]any) (failure ghttp.ErrorResponse) {
	resp.JsonResponse(map[string]any{"data": strings.Repeat("x", t._Size)})
	return nil
}

func (t *_LargeOutputTask) MCPGet() (tool Tool) {
	return NewTool[struct{}, map[string]any](&mcp.Tool{Name: "large"})
}

func TestServerSafeIDInvariant(t *testing.T) {
	for _, pattern := range []string{`.+`, `i`} {
		t.Run(pattern, func(t *testing.T) {
			items := &_ItemsTask{}
			server := _BindServer(Options{IDPattern: pattern}, _ItemsRoute(items))
			ids := []string{"i?secret=x", "i#fragment", "i/child", `i\child`, "i%2fchild", ".", "..", "i\nfield"}
			if pattern == "i" {
				ids = append(ids, "prefixi")
			}

			for _, id := range ids {
				for _, arguments := range []map[string]any{{"org": "i", "item": id}, {"org": id, "item": "i"}} {
					result := _Call(t, server, "items_get", arguments)
					assert.True(t, result.IsError, "unsafe own or ancestor ID must fail before dispatch: %q", id)
					assert.Zero(t, items._GetCalls.Load(), "rejected ID must cause no handler side effects: %q", id)
				}
			}

			assert.False(t, _Call(t, server, "items_get", map[string]any{"org": "i", "item": "i"}).IsError, "safe IDs still dispatch")
		})
	}
}

func TestServerExactOperationDeclarations(t *testing.T) {
	type ancestorInput struct {
		Org ID `json:"org" gmcp:"path=orgs"`
	}

	for _, test := range []struct {
		name      string
		tool      Tool
		arguments map[string]any
		want      string
	}{
		{"without IDs", NewTool[struct{}, _Echo](&mcp.Tool{Name: "collection_get"}), map[string]any{}, `{"function":"Get","language":"zh-TW"}`},
		{"ancestor ID", NewTool[ancestorInput, _Echo](&mcp.Tool{Name: "collection_get"}), map[string]any{"org": "o1"}, `{"function":"Get","org":"o1","language":"zh-TW"}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			reads := &_AmbiguousReadTask{_Tool: test.tool}
			acceptance := &_ReadAcceptance{}
			var server *Server
			require.NotPanics(t, func() {
				server = _BindServer(Options{}, ghttp.NewSimpleRoute().SetEndpoint("/orgs", &_OrgsTask{}).SetEndpoint("/orgs/items", reads, acceptance))
			}, "MCPGet selects Get without requiring an endpoint ID")
			if len(test.arguments) > 0 {
				result := _Call(t, server, "collection_get", map[string]any{"org": "items"})
				require.True(t, result.IsError, "an ancestor ID colliding with a child route must fail")
				assert.Zero(t, acceptance._Calls.Load(), "route ID checks must precede acceptances")
				assert.Zero(t, reads._IndexCalls.Load(), "a route collision must not run Index")
				assert.Zero(t, reads._GetCalls.Load(), "a route collision must not run Get")
			}

			result := _Call(t, server, "collection_get", test.arguments)
			require.False(t, result.IsError)
			assert.JSONEq(t, test.want, string(result.StructuredContent))
			assert.EqualValues(t, 1, reads._GetCalls.Load())
			assert.Zero(t, reads._IndexCalls.Load(), "MCPGet must never fall back to Index")
			assert.EqualValues(t, 1, acceptance._Calls.Load(), "valid Get retains route authorization")

			result = _Call(t, server, "items_list", map[string]any{"org": "o1"})
			require.False(t, result.IsError)
			assert.JSONEq(t, `{"function":"Index","org":"o1","language":"zh-TW"}`, string(result.StructuredContent))
			assert.EqualValues(t, 1, reads._GetCalls.Load(), "MCPIndex must not run Get")
			assert.EqualValues(t, 1, reads._IndexCalls.Load())
			assert.EqualValues(t, 2, acceptance._Calls.Load())
		})
	}
	type duplicateID struct {
		First string `json:"first" gmcp:"path"`
		Last  string `json:"last" gmcp:"path"`
	}

	assert.Panics(t, func() {
		_BindServer(Options{}, ghttp.NewSimpleRoute().SetEndpoint("/duplicate", &_GetTask{_Tool: NewTool[duplicateID, _Echo](&mcp.Tool{Name: "duplicate"})}))
	}, "two fields cannot both own the endpoint ID")
	writes := &_AmbiguousWriteTask{}
	server := _BindServer(Options{}, ghttp.NewSimpleRoute().SetEndpoint("/writes", writes))
	result := _Call(t, server, "collection_post", map[string]any{})
	assert.JSONEq(t, `{"function":"Post"}`, string(result.StructuredContent), "the explicit MCPPost declaration selects Post at a collection path")
	assert.Zero(t, writes._CreateCalls.Load(), "the undeclared Create operation must have no effects")
	assert.EqualValues(t, 1, writes._PostCalls.Load(), "exact Post dispatch must retain collection upload compatibility")
}

func TestServerRejectsBatchBeforeHandlers(t *testing.T) {
	call := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"items_get","arguments":{"org":"o1","item":"i1"}}}`
	notification := `{"jsonrpc":"2.0","method":"notifications/initialized"}`
	for _, version := range []string{"", "2025-03-26", "2025-11-25"} {
		for _, body := range []string{"  [" + call + "]", "[" + call + "," + call + "]", "[" + notification + "]"} {
			items := &_ItemsTask{}
			server := _BindServer(Options{}, _ItemsRoute(items))
			response := _ServeBody(t, server, []byte(body), map[string]string{"MCP-Protocol-Version": version})
			assert.Equal(t, http.StatusBadRequest, response.Code, "batch must be rejected for version %q", version)
			assert.Zero(t, items._GetCalls.Load(), "batch admission must precede handler work")
		}
	}
}

func TestServerBoundsDefaultOutput(t *testing.T) {
	server := _BindServer(Options{}, ghttp.NewSimpleRoute().SetEndpoint("/large", &_LargeOutputTask{_Size: 2 << 20}))
	result := _Call(t, server, "large", map[string]any{})
	require.True(t, result.IsError, "oversized output must become a bounded public tool error")
	assert.Less(t, len(_Payload(t, result)), 1024, "rejection must not retain the oversized output")
}
