package gmcp

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// _ErrorCode is a public tool error code.
type _ErrorCode string

const (
	_InvalidArgument        _ErrorCode = "invalid_argument"
	_PermissionDenied       _ErrorCode = "permission_denied"
	_NotFound               _ErrorCode = "not_found"
	_TemporarilyUnavailable _ErrorCode = "temporarily_unavailable"
	_InternalError          _ErrorCode = "internal_error"
)

// _PublicError carries only a fixed public code and message.
type _PublicError struct {
	Code    _ErrorCode `json:"code"`
	Message string     `json:"message"`
}

// _ErrorContent is the structured content of a sanitized tool error.
type _ErrorContent struct {
	Error _PublicError `json:"error"`
}

// _ToolError is an application failure that maps to one public error code.
type _ToolError struct {
	_Code _ErrorCode
}

// Error returns the public message of the code.
func (e *_ToolError) Error() (message string) {
	return e._Code._Public().Message
}

// _StatusErrorCode maps an HTTP error status to a public code; transport rejections and forwarded responses share it.
func _StatusErrorCode(status int) (code _ErrorCode) {
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return _PermissionDenied

	case status == http.StatusNotFound:
		return _NotFound

	case status == http.StatusTooManyRequests || status == http.StatusServiceUnavailable:
		return _TemporarilyUnavailable

	case status >= http.StatusBadRequest && status < http.StatusInternalServerError:
		return _InvalidArgument

	default:
		return _InternalError
	}
}

// _Public returns the fixed message of the code; unknown codes become internal_error.
func (c _ErrorCode) _Public() (public _PublicError) {
	switch c {
	case _InvalidArgument:
		return _PublicError{Code: c, Message: "Invalid arguments."}

	case _PermissionDenied:
		return _PublicError{Code: c, Message: "Permission denied."}

	case _NotFound:
		return _PublicError{Code: c, Message: "Resource not found."}

	case _TemporarilyUnavailable:
		return _PublicError{Code: c, Message: "Temporarily unavailable. Please retry later."}

	default:
		return _PublicError{Code: _InternalError, Message: "Unable to complete the request."}
	}
}

// _JSON encodes the code as a {"error":{...}} object.
func (c _ErrorCode) _JSON() (data []byte) {
	public := c._Public()
	return fmt.Appendf(nil, `{"error":{"code":%q,"message":%q}}`, public.Code, public.Message)
}

// _ErrorMiddleware rebuilds failed tool results from public codes, dropping SDK and handler texts, metadata, and
// output. Results it already sanitized, such as a gateway query passing one through, are kept.
func (s *Server) _ErrorMiddleware(next mcp.MethodHandler) (handler mcp.MethodHandler) {
	return func(ctx context.Context, method string, request mcp.Request) (result mcp.Result, err error) {
		result, err = next(ctx, method, request)
		toolResult, ok := result.(*mcp.CallToolResult)
		if err != nil || !ok || toolResult == nil || !toolResult.IsError {
			return result, err
		}

		if _, sanitized := toolResult.StructuredContent.(_ErrorContent); sanitized {
			return result, nil
		}

		code := _InternalError
		if toolError, found := errors.AsType[*_ToolError](toolResult.GetError()); found {
			code = toolError._Code
		} else if toolResult.GetError() != nil {
			// Registered handlers mark every application failure, so a remaining error comes from SDK input validation.
			code = _InvalidArgument
		}

		return &mcp.CallToolResult{
			IsError:           true,
			Content:           []mcp.Content{&mcp.TextContent{Text: string(code._JSON())}},
			StructuredContent: _ErrorContent{Error: code._Public()},
		}, nil
	}
}
