package gmcp

import (
	"bytes"
	"context"
	jsonv1 "encoding/json"
	"encoding/json/v2"
	"maps"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yetiz-org/gone/ghttp"
	"github.com/yetiz-org/gone/ghttp/httpheadername"
	kklogger "github.com/yetiz-org/goth-kklogger"
)

// _MaxForwardAttempts is how many requests one GET tool call sends while the handler answers 202 Accepted.
const _MaxForwardAttempts = 3

// _MaxForwardWait caps the wait between 202 polls so Retry-After cannot stretch one tool call.
const _MaxForwardWait = 3 * time.Second

// IndexTool is implemented by a handler task that exposes its Index function. The forwarded GET never carries the
// endpoint's own ID, otherwise ghttp would run Get instead.
type IndexTool interface {
	MCPIndex() (tool Tool)
}

// GetTool is implemented by a handler task that exposes its Get function. When the endpoint also implements Index,
// the input must declare the endpoint's own ID so the request cannot fall back to Index.
type GetTool interface {
	MCPGet() (tool Tool)
}

// PostTool is implemented by a handler task that exposes its Post function.
type PostTool interface {
	MCPPost() (tool Tool)
}

// PutTool is implemented by a handler task that exposes its Put function.
type PutTool interface {
	MCPPut() (tool Tool)
}

// PatchTool is implemented by a handler task that exposes its Patch function.
type PatchTool interface {
	MCPPatch() (tool Tool)
}

// DeleteTool is implemented by a handler task that exposes its Delete function.
type DeleteTool interface {
	MCPDelete() (tool Tool)
}

// BindingAdjuster is implemented by an input type that needs a fixed value no field can carry, such as a constant
// path ID; it receives the binding derived from the gmcp tags and returns the adjusted one.
type BindingAdjuster interface {
	AdjustBinding(binding Binding) (adjusted Binding)
}

// Tool is a tool declaration returned by a handler's MCP method; Bind derives its schemas, method, and annotations.
type Tool struct {
	_Definition *mcp.Tool
	_InputType  reflect.Type
	_OutputType reflect.Type
	_Register   func(server *Server, route *_Route)
}

// Binding is the REST request a tool call forwards: ID follows the endpoint segment, IDs are keyed by ancestor route
// node names, Body, when not nil, is sent as JSON, and Files, when not empty, are sent instead as multipart/form-data
// parts keyed by form field name.
type Binding struct {
	ID    string
	IDs   map[string]string
	Query url.Values
	Body  any
	Files map[string]File
}

// _Route is one bound tool: the dispatcher, endpoint node, canonical path, HTTP method, and derived input mapping.
type _Route struct {
	_Server     *Server
	_Dispatcher *ghttp.DispatchHandler
	_Node       ghttp.RouteNode
	_Path       string
	_Name       string
	_Method     string
	_Index      bool
	_Input      *_Input
}

// NewTool declares a tool whose input is In and whose structured output is Out, decoded from the REST response; when
// Out is Blob, the response body is returned as is. In must be a struct; use struct{} for a tool without input. The
// input schema and REST mapping come from the json and gmcp tags of In, so definition must not set InputSchema. Bind
// sets InputSchema and Annotations, keeping only the OpenWorldHint that definition sets, and derives OutputSchema
// from the json and gmcp tags of Out unless definition sets it.
func NewTool[In, Out any](definition *mcp.Tool) (tool Tool) {
	if definition == nil || definition.InputSchema != nil {
		panic("gmcp: NewTool needs a definition without InputSchema; the schema is derived from the input type")
	}

	return Tool{_Definition: definition, _InputType: reflect.TypeFor[In](), _OutputType: reflect.TypeFor[Out](), _Register: func(server *Server, route *_Route) {
		_AddTool(server, definition, func(ctx context.Context, request *mcp.CallToolRequest, value In) (result *mcp.CallToolResult, output Out, err error) {
			binding, ok := route._Input._Binding(value)
			if !ok {
				return nil, output, &_ToolError{_Category: _InvalidArgument}
			}

			if err = route._Forward(ctx, binding, &output); err != nil {
				return nil, output, err
			}

			if blob, ok := any(output).(Blob); ok {
				result, err = blob._Result()
			}

			return result, output, err
		})
	}}
}

// _Annotations derives tool hints from HTTP method semantics: GET is read-only, every write may be destructive, PUT
// and DELETE are idempotent, and openWorld is false because a forwarded call reaches only this server. The
// OpenWorldHint of declared, when set, replaces that default for a tool whose effects reach external systems.
func (r *_Route) _Annotations(declared *mcp.ToolAnnotations) (annotations *mcp.ToolAnnotations) {
	readOnly := r._Method == http.MethodGet
	annotations = &mcp.ToolAnnotations{
		ReadOnlyHint:    readOnly,
		DestructiveHint: new(!readOnly),
		IdempotentHint:  r._Method == http.MethodPut || r._Method == http.MethodDelete,
		OpenWorldHint:   new(false),
	}
	if declared != nil && declared.OpenWorldHint != nil {
		annotations.OpenWorldHint = new(*declared.OpenWorldHint)
	}

	return annotations
}

// _Forward dispatches binding in process with the caller's credentials and decodes a successful response into
// output. The body is encoded with the encoding/json v1 semantics REST handlers decode with, or as multipart/form-data
// when the binding has files. When output is a *Blob, the request accepts any media type and the response body is
// kept as is; otherwise a success without a body leaves output zero. Only GET polls again on 202 Accepted, waiting
// Retry-After up to 3 seconds, and the last 202 response is the result; writes are never resent. Failures surface
// only a public category, plus the six-digit error_code and the string title and detail of a non-2xx response whose
// body is a JSON object, the fields of erresponse.DefaultErrorResponse that identify the error to a client and an end
// user; nothing else of the body is kept.
func (r *_Route) _Forward(ctx context.Context, binding Binding, output any) (rtErr error) {
	caller, ok := ctx.Value(_CallerKey{}).(*Caller)
	if !ok || caller.Request == nil || caller.HandlerContext == nil {
		return &_ToolError{_Category: _InternalError}
	}

	if r._Index && binding.ID != "" {
		kklogger.ErrorJ("gmcp:Route.Forward#binding!index_id", map[string]any{"tool": r._Name, "path": r._Path})
		return &_ToolError{_Category: _InternalError}
	}

	target, ok := r._Target(binding)
	if !ok {
		return &_ToolError{_Category: _InvalidArgument}
	}

	var body []byte
	contentType := "application/json"
	switch {
	case len(binding.Files) > 0 && binding.Body != nil:
		kklogger.ErrorJ("gmcp:Route.Forward#binding!body_and_files", map[string]any{"tool": r._Name, "path": r._Path})
		return &_ToolError{_Category: _InternalError}

	case len(binding.Files) > 0:
		if body, contentType, ok = binding._Multipart(); !ok {
			return &_ToolError{_Category: _InvalidArgument}
		}

	case binding.Body != nil:
		encoded, err := json.Marshal(binding.Body, jsonv1.DefaultOptionsV1())
		if err != nil {
			kklogger.ErrorJ("gmcp:Route.Forward#body!encode_failed", map[string]any{"tool": r._Name, "error": err.Error()})
			return &_ToolError{_Category: _InternalError}
		}

		body = encoded
	}

	if limit := r._Server._Options.MaxBodyBytes; limit > 0 && int64(len(body)) > limit {
		return &_ToolError{_Category: _InvalidArgument}
	}

	accept := "application/json"
	blob, binary := output.(*Blob)
	if binary {
		accept = "*/*"
	}

	seed := map[string]any{}
	if key := r._Server._Options.ToolParam; key != "" {
		seed[key] = r._Name
	}

	outer := caller.Request
	for attempt := 1; ; attempt++ {
		request, err := http.NewRequestWithContext(ctx, r._Method, target, bytes.NewReader(body))
		if err != nil {
			return &_ToolError{_Category: _InternalError}
		}

		request.Host = outer.Host()
		request.RemoteAddr = outer.Request().RemoteAddr
		request.Header.Set(httpheadername.Authorization, outer.Header().Get(httpheadername.Authorization))
		request.Header.Set(httpheadername.Accept, accept)
		if body != nil {
			request.Header.Set(httpheadername.ContentType, contentType)
		}

		if forwardedFor := outer.Header().Get(httpheadername.XForwardedFor); forwardedFor != "" {
			request.Header.Set(httpheadername.XForwardedFor, forwardedFor)
		}

		if caller.Language != "" {
			request.Header.Set(httpheadername.AcceptLanguage, caller.Language)
		}

		pack := r._Dispatcher.Dispatch(caller.HandlerContext, request, seed)
		if pack == nil || !r._Matches(pack, binding) {
			kklogger.WarnJ("gmcp:Route.Forward#route!mismatch", map[string]any{"tool": r._Name, "path": r._Path})
			return &_ToolError{_Category: _InternalError}
		}

		status := pack.Response.StatusCode()
		if status == http.StatusAccepted && r._Method == http.MethodGet && attempt < _MaxForwardAttempts {
			select {
			case <-ctx.Done():
				return ctx.Err()

			case <-time.After(r._RetryAfter(pack.Response.GetHeader(httpheadername.RetryAfter))):
			}

			continue
		}

		if status < http.StatusOK || status >= http.StatusMultipleChoices {
			failure := &_ToolError{_Category: _StatusCategory(status)}
			var fields map[string]any
			if json.Unmarshal(pack.Response.Body().Bytes(), &fields) == nil {
				if code, _ := fields["error_code"].(string); _CodePattern.MatchString(code) {
					failure._Code = code
				}

				failure._Title, _ = fields["title"].(string)
				failure._Detail, _ = fields["detail"].(string)
			}

			return failure
		}

		if binary {
			blob._Read(pack.Response)
			return nil
		}

		raw := pack.Response.Body().Bytes()
		if len(raw) == 0 {
			return nil
		}

		if err := json.Unmarshal(raw, output); err != nil {
			kklogger.ErrorJ("gmcp:Route.Forward#response!decode_failed", map[string]any{"tool": r._Name, "error": err.Error()})
			return &_ToolError{_Category: _InternalError}
		}

		return nil
	}
}

// _Target builds the request URI: IDs follow their ancestor segments, ID follows the endpoint, and the query is
// appended. Every path ID must match Options.IDPattern and every IDs key must name an ancestor segment.
func (r *_Route) _Target(binding Binding) (target string, ok bool) {
	segments := strings.Split(strings.TrimPrefix(r._Path, "/"), "/")
	path := make([]string, 0, len(segments)+len(binding.IDs)+1)
	used := 0
	for _, segment := range segments {
		path = append(path, segment)
		id, found := binding.IDs[segment]
		if !found {
			continue
		}

		if !r._Server._IDPattern.MatchString(id) {
			return "", false
		}

		path = append(path, id)
		used++
	}

	if used != len(binding.IDs) || binding.ID != "" && !r._Server._IDPattern.MatchString(binding.ID) {
		return "", false
	}

	if binding.ID != "" {
		path = append(path, binding.ID)
	}

	target = "/" + strings.Join(path, "/")
	if len(binding.Query) > 0 {
		target += "?" + binding.Query.Encode()
	}

	return target, true
}

// _Matches confirms the request reached the bound endpoint node and that the route resolved the same endpoint ID
// and ancestor IDs, so ghttp ran the function the tool declares.
func (r *_Route) _Matches(pack *ghttp.Pack, binding Binding) (ok bool) {
	if pack.RouteNode != r._Node {
		return false
	}

	task := &ghttp.DefaultHandlerTask{}
	if task.GetID(r._Node.Name(), pack.Params) != binding.ID {
		return false
	}

	for name, id := range binding.IDs {
		if task.GetID(name, pack.Params) != id {
			return false
		}
	}

	return true
}

// _RetryAfter parses Retry-After seconds, using the cap when the value is missing or invalid.
func (r *_Route) _RetryAfter(value string) (wait time.Duration) {
	seconds, err := strconv.Atoi(value)
	if err != nil || seconds < 0 {
		return _MaxForwardWait
	}

	return min(time.Duration(seconds)*time.Second, _MaxForwardWait)
}

// _Multipart encodes Files as a multipart/form-data body with one part per form field, in name order. Each part takes
// its Content-Type from the file's data URL. It reports false when a file has an invalid filename or media type, or no
// data.
func (b Binding) _Multipart() (body []byte, contentType string, ok bool) {
	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)
	for _, name := range slices.Sorted(maps.Keys(b.Files)) {
		file := b.Files[name]
		if !_ValidFilename(file.Filename) || !_ValidMediaType(file.Data.MediaType) || len(file.Data.Data) == 0 {
			return nil, "", false
		}

		header := textproto.MIMEHeader{}
		header.Set(httpheadername.ContentDisposition, multipart.FileContentDisposition(name, file.Filename))
		header.Set(httpheadername.ContentType, file.Data.MediaType)

		// Writes to a bytes.Buffer cannot fail.
		part, _ := writer.CreatePart(header)
		_, _ = part.Write(file.Data.Data)
	}

	_ = writer.Close()
	return buffer.Bytes(), writer.FormDataContentType(), true
}
