package gmcp

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	_ToolsName = "tools"
	_QueryName = "query"
)

// Gateway keeps tools/list small: only Public tools are listed and directly callable, and every other bound tool is
// found through the "tools" discovery tool and called through the "query" tool.
type Gateway struct {
	// Public names the bound tools that stay listed and directly callable.
	Public []string
}

type _GatewayNextKey struct{}

// _ToolsInput searches the hidden catalog; an empty search lists names only.
type _ToolsInput struct {
	Search string `json:"search,omitempty" jsonschema:"Optional tool name or description search text."`
}

// _ToolsSummary is one matching tool.
type _ToolsSummary struct {
	Name        string `json:"name" jsonschema:"Tool name."`
	Description string `json:"description" jsonschema:"Tool description."`
}

// _ToolsDetail is the definition of the tool whose name equals the search.
type _ToolsDetail struct {
	Name         string `json:"name" jsonschema:"Tool name."`
	Description  string `json:"description" jsonschema:"Tool description."`
	InputSchema  any    `json:"input_schema" jsonschema:"Tool input schema."`
	OutputSchema any    `json:"output_schema,omitempty" jsonschema:"Tool output schema."`
}

// _ToolsOutput carries only the fields of the answered search mode.
type _ToolsOutput struct {
	Names []string         `json:"names,omitempty" jsonschema:"Available tool names."`
	Tools *[]_ToolsSummary `json:"tools,omitempty" jsonschema:"Matching tools."`
	Tool  *_ToolsDetail    `json:"tool,omitempty" jsonschema:"Exact tool definition."`
}

// _QueryInput names a hidden tool and its raw JSON object arguments.
type _QueryInput struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// _BindGateway checks that every public tool is bound and registers the discovery and query tools over the other
// bound tools. The query tool is read-only only when every hidden tool is, and open-world when any hidden tool is.
func (s *Server) _BindGateway(names map[string]string) {
	public := s._Options.Gateway.Public
	for _, name := range public {
		if _, bound := names[name]; !bound {
			panic(fmt.Sprintf("gmcp: gateway public tool %q is not bound", name))
		}
	}

	catalog := slices.DeleteFunc(slices.Clone(s._Catalog), func(tool *mcp.Tool) bool {
		return slices.Contains(public, tool.Name)
	})

	slices.SortFunc(catalog, func(left, right *mcp.Tool) int {
		return strings.Compare(left.Name, right.Name)
	})

	readOnly, destructive, openWorld := true, false, false
	for _, tool := range catalog {
		readOnly = readOnly && tool.Annotations.ReadOnlyHint
		destructive = destructive || tool.Annotations.DestructiveHint != nil && *tool.Annotations.DestructiveHint
		openWorld = openWorld || tool.Annotations.OpenWorldHint != nil && *tool.Annotations.OpenWorldHint
	}

	_AddTool(s, &mcp.Tool{
		Name: _ToolsName, Title: "Discover Tools", Description: "Discover tools by name or description and inspect the input and output schemas of one tool.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, DestructiveHint: new(false), OpenWorldHint: new(false)},
	}, func(ctx context.Context, request *mcp.CallToolRequest, input _ToolsInput) (result *mcp.CallToolResult, output _ToolsOutput, err error) {
		return nil, s._Discover(catalog, input.Search), nil
	})

	mcp.AddTool(s._SDK, &mcp.Tool{
		Name: _QueryName, Title: "Run Tool", Description: "Run one named tool with its arguments.",
		InputSchema: &jsonschema.Schema{
			Type: "object", Required: []string{"name", "arguments"},
			Properties: map[string]*jsonschema.Schema{
				"name":      {Type: "string", Description: "Exact tool name.", MinLength: new(1)},
				"arguments": {Type: "object", Description: "Arguments for the named tool."},
			},
			AdditionalProperties: &jsonschema.Schema{Not: &jsonschema.Schema{}},
		},
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: readOnly, DestructiveHint: new(destructive), OpenWorldHint: new(openWorld)},
	}, func(ctx context.Context, request *mcp.CallToolRequest, input _QueryInput) (result *mcp.CallToolResult, output any, err error) {
		if !slices.ContainsFunc(catalog, func(tool *mcp.Tool) bool { return tool.Name == input.Name }) {
			return nil, nil, &_ToolError{_Category: _InvalidArgument}
		}

		return s._Query(ctx, request, input)
	})
}

// _Discover answers a search: an exact name returns that definition, other text matches names and descriptions
// case-insensitively, and empty text lists the names.
func (s *Server) _Discover(catalog []*mcp.Tool, search string) (output _ToolsOutput) {
	search = strings.TrimSpace(search)
	if search == "" {
		output.Names = make([]string, 0, len(catalog))
		for _, tool := range catalog {
			output.Names = append(output.Names, tool.Name)
		}

		return output
	}

	for _, tool := range catalog {
		if tool.Name == search {
			output.Tool = &_ToolsDetail{Name: tool.Name, Description: tool.Description, InputSchema: tool.InputSchema, OutputSchema: tool.OutputSchema}
			return output
		}
	}

	needle := strings.ToLower(search)
	matches := make([]_ToolsSummary, 0)
	for _, tool := range catalog {
		if strings.Contains(strings.ToLower(tool.Name), needle) || strings.Contains(strings.ToLower(tool.Description), needle) {
			matches = append(matches, _ToolsSummary{Name: tool.Name, Description: tool.Description})
		}
	}

	output.Tools = &matches
	return output
}

// _Query calls the named tool through the handler chain below the gateway with a copy of the request, and returns
// its result unchanged.
func (s *Server) _Query(ctx context.Context, request *mcp.CallToolRequest, input _QueryInput) (result *mcp.CallToolResult, output any, err error) {
	next, ok := ctx.Value(_GatewayNextKey{}).(mcp.MethodHandler)
	if !ok || request == nil || request.Params == nil {
		return nil, nil, &_ToolError{_Category: _InternalError}
	}

	delegated := *request
	params := *request.Params
	params.Name = input.Name
	params.Arguments = input.Arguments
	delegated.Params = &params
	inner, err := next(ctx, "tools/call", &delegated)
	if err != nil {
		return nil, nil, err
	}

	result, ok = inner.(*mcp.CallToolResult)
	if !ok || result == nil {
		return nil, nil, &_ToolError{_Category: _InternalError}
	}

	return result, nil, nil
}

// _GatewayMiddleware lists only the public, discovery, and query tools, taking their definitions from the SDK
// registry, and rejects direct calls to hidden tools.
func (s *Server) _GatewayMiddleware(next mcp.MethodHandler) (handler mcp.MethodHandler) {
	visible := append(slices.Clone(s._Options.Gateway.Public), _ToolsName, _QueryName)
	return func(ctx context.Context, method string, request mcp.Request) (result mcp.Result, err error) {
		switch method {
		case "tools/list":
			listRequest, ok := request.(*mcp.ListToolsRequest)
			if !ok || listRequest == nil {
				return nil, &jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: "Invalid tool list request."}
			}

			if listRequest.Params != nil && listRequest.Params.Cursor != "" {
				return nil, &jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: "Tool list cursor is not supported."}
			}

			return s._ListVisible(ctx, next, listRequest, visible)

		case "tools/call":
			callRequest, ok := request.(*mcp.CallToolRequest)
			if !ok || callRequest == nil || callRequest.Params == nil {
				return nil, &jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: "Invalid tool call request."}
			}

			switch {
			case callRequest.Params.Name == _QueryName:
				return next(context.WithValue(ctx, _GatewayNextKey{}, next), method, request)

			case slices.Contains(visible, callRequest.Params.Name):
				return next(ctx, method, request)

			default:
				return nil, &jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: "Unknown tool."}
			}

		default:
			return next(ctx, method, request)
		}
	}
}

// _ListVisible pages through the SDK tool list and returns the visible tools in declaration order.
func (s *Server) _ListVisible(ctx context.Context, next mcp.MethodHandler, request *mcp.ListToolsRequest, visible []string) (result mcp.Result, err error) {
	found := make(map[string]*mcp.Tool, len(visible))
	var first *mcp.ListToolsResult
	cursor := ""
	for {
		pageRequest := *request
		pageParams := mcp.ListToolsParams{}
		if request.Params != nil {
			pageParams = *request.Params
		}

		pageParams.Cursor = cursor
		pageRequest.Params = &pageParams
		pageResult, pageErr := next(ctx, "tools/list", &pageRequest)
		if pageErr != nil {
			return nil, pageErr
		}

		page, ok := pageResult.(*mcp.ListToolsResult)
		if !ok || page == nil {
			return nil, &jsonrpc.Error{Code: jsonrpc.CodeInternalError, Message: "Unable to list tools."}
		}

		if first == nil {
			first = page
		}

		for _, tool := range page.Tools {
			if tool != nil && slices.Contains(visible, tool.Name) {
				found[tool.Name] = tool
			}
		}

		if len(found) == len(visible) || page.NextCursor == "" {
			break
		}

		cursor = page.NextCursor
	}

	if len(found) != len(visible) {
		return nil, &jsonrpc.Error{Code: jsonrpc.CodeInternalError, Message: "Unable to list tools."}
	}

	listed := *first
	listed.Tools = make([]*mcp.Tool, 0, len(visible))
	for _, name := range visible {
		listed.Tools = append(listed.Tools, found[name])
	}

	listed.NextCursor = ""
	return &listed, nil
}
