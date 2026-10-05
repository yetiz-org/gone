package gmcp

import (
	"context"
	"encoding/json/v2"
	"errors"
	"net/http"
	"regexp"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// _Category is the public kind of an error; a client that does not know the error code acts on it.
type _Category string

const (
	_InvalidArgument        _Category = "invalid_argument"
	_PermissionDenied       _Category = "permission_denied"
	_NotFound               _Category = "not_found"
	_Conflict               _Category = "conflict"
	_TemporarilyUnavailable _Category = "temporarily_unavailable"
	_InternalError          _Category = "internal_error"
)

// _StatusOverloaded is the non-standard status of an overloaded upstream service.
const _StatusOverloaded = 529

// _CodePattern matches the six-digit error_code of a REST error response.
var _CodePattern = regexp.MustCompile(`^[0-9]{6}$`)

// _PublicError is the public error object. Category is always set; a forwarded REST error also carries the six-digit
// error code and the end-user title and detail of its response when it has them.
type _PublicError struct {
	Category _Category `json:"category"`
	Code     string    `json:"code,omitempty"`
	Title    string    `json:"title,omitempty"`
	Detail   string    `json:"detail,omitempty"`
}

// _ErrorContent is the {"error":{...}} object of a sanitized error.
type _ErrorContent struct {
	Error _PublicError `json:"error"`
}

// _ToolError is an application failure of one category. A forwarded REST error also keeps the six-digit error_code
// and the string title and detail of its response body; the title and detail are the end-user fields of
// erresponse.DefaultErrorResponse.
type _ToolError struct {
	_Category _Category
	_Code     string
	_Title    string
	_Detail   string
}

// Error returns the fixed message of the category.
func (e *_ToolError) Error() (message string) {
	return e._Category._Message()
}

// _Public returns the public error object of e.
func (e *_ToolError) _Public() (public _PublicError) {
	return _PublicError{Category: e._Category, Code: e._Code, Title: e._Title, Detail: e._Detail}
}

// _JSON encodes e as a {"error":{...}} object.
func (e _PublicError) _JSON() (data []byte) {
	// The category is a constant and the other fields were decoded from JSON strings, so they are valid UTF-8 and the
	// object always encodes.
	data, _ = json.Marshal(_ErrorContent{Error: e})
	return data
}

// _StatusCategory maps an HTTP error status to a category; transport rejections and forwarded responses share it.
func _StatusCategory(status int) (category _Category) {
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return _PermissionDenied

	case status == http.StatusNotFound:
		return _NotFound

	case status == http.StatusConflict:
		return _Conflict

	case status == http.StatusTooManyRequests || status == http.StatusServiceUnavailable || status == _StatusOverloaded:
		return _TemporarilyUnavailable

	case status >= http.StatusBadRequest && status < http.StatusInternalServerError:
		return _InvalidArgument

	default:
		return _InternalError
	}
}

// _Message returns the fixed English message of the category, used where a protocol requires a message.
func (c _Category) _Message() (message string) {
	switch c {
	case _InvalidArgument:
		return "Invalid arguments."

	case _PermissionDenied:
		return "Permission denied."

	case _NotFound:
		return "Resource not found."

	case _Conflict:
		return "The resource state conflicts with the request."

	case _TemporarilyUnavailable:
		return "Temporarily unavailable. Please retry later."

	default:
		return "Unable to complete the request."
	}
}

// _ErrorMiddleware rebuilds failed tool results from their tool errors, dropping SDK and handler texts, metadata, and
// output. The public error object is only the text content: a client may validate structuredContent against the
// output schema of the tool even for an error result. The rebuilt result keeps its tool error, so rebuilding a result
// that is passed through again, as a gateway query does, gives the same content.
func (s *Server) _ErrorMiddleware(next mcp.MethodHandler) (handler mcp.MethodHandler) {
	return func(ctx context.Context, method string, request mcp.Request) (result mcp.Result, err error) {
		result, err = next(ctx, method, request)
		toolResult, ok := result.(*mcp.CallToolResult)
		if err != nil || !ok || toolResult == nil || !toolResult.IsError {
			return result, err
		}

		failure := &_ToolError{_Category: _InternalError}
		if toolError, found := errors.AsType[*_ToolError](toolResult.GetError()); found {
			failure = toolError
		} else if toolResult.GetError() != nil {
			// Registered handlers mark every application failure, so a remaining error comes from SDK input validation.
			failure._Category = _InvalidArgument
		}

		sanitized := &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(failure._Public()._JSON())}}}
		sanitized.SetError(failure)
		return sanitized, nil
	}
}
