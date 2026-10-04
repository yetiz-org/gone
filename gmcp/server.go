// Package gmcp serves Model Context Protocol tools that forward to ghttp handler functions.
//
// A handler task declares one tool per REST function through MCPIndex, MCPGet, MCPPost, MCPPut, MCPPatch, or
// MCPDelete. Server.Bind derives each tool's input schema and REST mapping from its input type, and every tool call is
// dispatched in process as the request a REST client would send, so routes, acceptances, and handlers stay the only
// owners of permissions, validation, and responses.
package gmcp

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"sync/atomic"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yetiz-org/gone/channel"
	"github.com/yetiz-org/gone/ghttp"
)

// DefaultIDPattern accepts URL unreserved characters and cannot form a "." or ".." path segment.
const DefaultIDPattern = `^[0-9A-Za-z_~-][0-9A-Za-z._~-]*$`

// Options configures a Server.
type Options struct {
	// Instructions is returned to clients in the initialize result.
	Instructions string
	// Allows reports whether an MCP access token may call method on the canonical route path. Bind panics for a tool
	// it rejects, so a misconfigured token scope fails at startup. Nil skips the check.
	Allows func(path string, method string) (allowed bool)
	// ToolParam, when set, is the params key under which every forwarded request carries its tool name, so
	// acceptances, handlers, and audit can tell MCP calls apart. Network requests cannot set params.
	ToolParam string
	// IDPattern restricts ID values and every path ID; it must be valid in both RE2 and ECMA-262. Empty uses
	// DefaultIDPattern.
	IDPattern string
	// MaxBodyBytes limits encoded request bodies of forwarded calls, JSON or multipart/form-data; zero disables the
	// limit.
	MaxBodyBytes int64
	// MaxRequestBodyBytes limits the body of one MCP HTTP request. Zero uses mcp.DefaultMaxRequestBodyBytes (4 MiB),
	// and a negative value disables the limit. A larger request is rejected with 413 and invalid_argument. Base64 makes
	// a file argument about 4/3 of the file size.
	MaxRequestBodyBytes int64
	// Gateway, when set, lists only its public tools plus the "tools" discovery and "query" call tools.
	Gateway *Gateway
}

// Caller is the authenticated MCP HTTP request whose credentials forwarded tools reuse.
type Caller struct {
	// Request is the outer request; its Authorization and X-Forwarded-For headers, Host, and remote address are
	// forwarded.
	Request *ghttp.Request
	// HandlerContext is the outer handler context passed to ghttp.DispatchHandler.Dispatch.
	HandlerContext channel.HandlerContext
	// Language, when set, is sent as Accept-Language on forwarded requests.
	Language string
}

// Server owns one MCP server, its stateless Streamable HTTP transport, and the tools bound from a ghttp route.
type Server struct {
	_Options     Options
	_IDPattern   *regexp.Regexp
	_TypeSchemas map[reflect.Type]*jsonschema.Schema
	_SDK         *mcp.Server
	_Handler     *mcp.StreamableHTTPHandler
	_Catalog     []*mcp.Tool
	_Bound       atomic.Bool
}

type _CallerKey struct{}

// New creates a Server identified by implementation. It panics when options.IDPattern is not a valid regular
// expression.
func New(implementation *mcp.Implementation, options Options) (server *Server) {
	if options.IDPattern == "" {
		options.IDPattern = DefaultIDPattern
	}

	dataURL := &jsonschema.Schema{Type: "string", Pattern: _DataURLPattern, Description: _InputDataURLDescription}
	server = &Server{
		_Options:   options,
		_IDPattern: regexp.MustCompile(options.IDPattern),
		_TypeSchemas: map[reflect.Type]*jsonschema.Schema{
			reflect.TypeFor[ID]():      {Type: "string", MinLength: new(1), Pattern: options.IDPattern},
			reflect.TypeFor[Date]():    {Type: "string", Description: "Date in YYYY-MM-DD form.", Pattern: `^\d{4}-\d{2}-\d{2}$`},
			reflect.TypeFor[DataURL](): dataURL,
			reflect.TypeFor[File](): {
				Type: "object", Description: "File to upload.", Required: []string{"filename", "data"},
				Properties: map[string]*jsonschema.Schema{
					"filename": {Type: "string", MinLength: new(1), Description: "File name without a directory, such as cover.png."},
					"data":     dataURL,
				},
				AdditionalProperties: &jsonschema.Schema{Not: &jsonschema.Schema{}},
			},
		},
		_SDK: mcp.NewServer(implementation, &mcp.ServerOptions{Instructions: options.Instructions}),
	}

	server._SDK.AddReceivingMiddleware(server._ErrorMiddleware)
	if options.Gateway != nil {
		server._SDK.AddReceivingMiddleware(server._GatewayMiddleware)
	}

	server._Handler = mcp.NewStreamableHTTPHandler(func(request *http.Request) *mcp.Server {
		return server._SDK
	}, &mcp.StreamableHTTPOptions{
		Stateless: true, JSONResponse: true, PropagateRequestCancellation: true, MaxRequestBodyBytes: options.MaxRequestBodyBytes,
	})
	return server
}

// Bind registers the tools declared by the handler tasks of entries and forwards their calls through dispatcher.
// Call it once, after the route is complete and before Serve. It panics on an invalid declaration, a duplicate tool
// name, a tool that Options.Allows rejects, or an unknown gateway public tool, so misconfiguration fails at startup.
func (s *Server) Bind(dispatcher *ghttp.DispatchHandler, entries []ghttp.RouteEntry) {
	if !s._Bound.CompareAndSwap(false, true) {
		panic("gmcp: Bind called more than once")
	}

	names := map[string]string{}
	register := func(entry ghttp.RouteEntry, tool Tool, method string, index bool) {
		if tool._Definition == nil || tool._Register == nil {
			panic(fmt.Sprintf("gmcp: %s declares a tool that NewTool did not build", entry.Path))
		}

		name := tool._Definition.Name
		if declared, exists := names[name]; exists {
			panic(fmt.Sprintf("gmcp: tool %q is declared by %s and %s", name, declared, entry.Path))
		}

		if s._Options.Gateway != nil && (name == _ToolsName || name == _QueryName) {
			panic(fmt.Sprintf("gmcp: tool name %q is reserved by the gateway", name))
		}

		if s._Options.Allows != nil && !s._Options.Allows(entry.Path, method) {
			panic(fmt.Sprintf("gmcp: tool %q cannot %s %s with the MCP token scope", name, method, entry.Path))
		}

		input := s._NewInput(tool._InputType)
		input._Check(name, entry.Path, method, index)
		names[name] = entry.Path
		route := &_Route{
			_Server: s, _Dispatcher: dispatcher, _Node: entry.Node, _Path: entry.Path, _Name: name,
			_Method: method, _Index: index, _Input: input,
		}

		tool._Definition.InputSchema = input._Schema
		if tool._Definition.OutputSchema == nil {
			tool._Definition.OutputSchema = _NewOutput(tool._OutputType)
		}

		tool._Definition.Annotations = route._Annotations()
		tool._Register(s, route)
		s._Catalog = append(s._Catalog, tool._Definition)
	}

	for _, entry := range entries {
		task := entry.Node.HandlerTask()
		if declarer, ok := task.(IndexTool); ok {
			register(entry, declarer.MCPIndex(), http.MethodGet, true)
		}

		if declarer, ok := task.(GetTool); ok {
			register(entry, declarer.MCPGet(), http.MethodGet, false)
		}

		if declarer, ok := task.(PostTool); ok {
			register(entry, declarer.MCPPost(), http.MethodPost, false)
		}

		if declarer, ok := task.(PutTool); ok {
			register(entry, declarer.MCPPut(), http.MethodPut, false)
		}

		if declarer, ok := task.(PatchTool); ok {
			register(entry, declarer.MCPPatch(), http.MethodPatch, false)
		}

		if declarer, ok := task.(DeleteTool); ok {
			register(entry, declarer.MCPDelete(), http.MethodDelete, false)
		}
	}

	if s._Options.Gateway != nil {
		s._BindGateway(names)
	}
}

// Serve handles one MCP HTTP request with the stateless Streamable HTTP transport. The caller must already have
// authenticated the request, and writer must be the raw response writer (see ghttp RawMode). SDK error texts are
// replaced with public error codes before they reach the client.
func (s *Server) Serve(writer http.ResponseWriter, caller Caller) {
	if caller.Request == nil || caller.HandlerContext == nil {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusInternalServerError)
		_, _ = writer.Write(_InternalError._JSON())
		return
	}

	request := caller.Request.Request()
	request = request.WithContext(context.WithValue(request.Context(), _CallerKey{}, &caller))
	response := &_ResponseWriter{_Writer: writer}
	defer response._Finish()
	s._Handler.ServeHTTP(response, request)
}

// _AddTool registers handler on s and marks every failure it returns as a tool error, so the error middleware can tell
// application failures from SDK input validation errors. It is a function rather than a generic method because Go
// 1.27.1 fails to link a generic method instantiated with a struct type whose tag contains "[".
func _AddTool[In, Out any](s *Server, tool *mcp.Tool, handler mcp.ToolHandlerFor[In, Out]) {
	mcp.AddTool(s._SDK, tool, func(ctx context.Context, request *mcp.CallToolRequest, input In) (result *mcp.CallToolResult, output Out, err error) {
		result, output, err = handler(ctx, request, input)
		if err == nil && (result == nil || !result.IsError) {
			return result, output, nil
		}

		if err == nil {
			err = result.GetError()
		}

		if toolError, ok := errors.AsType[*_ToolError](err); ok {
			return nil, output, toolError
		}

		return nil, output, &_ToolError{_Code: _InternalError}
	})
}
