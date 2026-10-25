package gmcp

import (
	"bytes"
	"context"
	"encoding/base64"
	jsonv1 "encoding/json"
	"encoding/json/v2"
	"errors"
	"fmt"
	"reflect"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var _ErrOutputTooLarge = errors.New("gmcp: output exceeds its budget")

// _OutputCapture discards partial JSON on failure; no caller can publish a prefix.
type _OutputCapture struct {
	_Body  bytes.Buffer
	_Limit int64
}

func (b *_OutputCapture) Write(data []byte) (n int, rtErr error) {
	if int64(len(data)) > b._Limit-int64(b._Body.Len()) {
		b._Body.Reset()
		return 0, _ErrOutputTooLarge
	}

	required := b._Body.Len() + len(data)
	if required > b._Body.Cap() {
		capacity := min(b._Limit, max(int64(required), int64(b._Body.Cap())*2))
		grown := make([]byte, b._Body.Len(), int(capacity))
		copy(grown, b._Body.Bytes())
		b._Body = *bytes.NewBuffer(grown)
	}

	return b._Body.Write(data)
}

func (s *Server) _EncodeOutput(value any, limit int64) (encoded []byte, rtErr error) {
	capture := &_OutputCapture{_Limit: limit}
	if blob, ok := value.(Blob); ok {
		length := int64(len(blob.Data.Data))
		if length > (limit/4)*3 {
			return nil, _ErrOutputTooLarge
		}

		if _, rtErr = capture.Write([]byte{'{'}); rtErr != nil {
			return nil, rtErr
		}

		if blob.Filename != "" {
			if _, rtErr = capture.Write([]byte(`"filename":`)); rtErr != nil {
				return nil, rtErr
			}

			if rtErr = json.MarshalWrite(capture, blob.Filename, jsonv1.DefaultOptionsV1()); rtErr != nil {
				return nil, rtErr
			}

			if _, rtErr = capture.Write([]byte{','}); rtErr != nil {
				return nil, rtErr
			}
		}

		if _, rtErr = capture.Write([]byte(`"data":`)); rtErr != nil {
			return nil, rtErr
		}

		if rtErr = blob.Data._WriteDataURL(capture); rtErr != nil {
			return nil, rtErr
		}

		if _, rtErr = capture.Write([]byte{'}'}); rtErr != nil {
			return nil, rtErr
		}

		return capture._Body.Bytes(), nil
	}

	if rtErr = json.MarshalWrite(capture, value, jsonv1.DefaultOptionsV1()); rtErr != nil {
		return nil, rtErr
	}

	return capture._Body.Bytes(), nil
}

func (s *Server) _ResolveOutput(schema any) (resolved *jsonschema.Resolved, defaults bool, rtErr error) {
	if schema == nil {
		return nil, false, nil
	}

	encoded, rtErr := s._EncodeOutput(schema, s._Options.MaxSchemaBytes)
	if rtErr != nil {
		return nil, false, rtErr
	}

	var internal *jsonschema.Schema
	if rtErr = json.Unmarshal(encoded, &internal, jsonv1.DefaultOptionsV1()); rtErr != nil {
		return nil, false, rtErr
	}

	if internal == nil {
		return nil, false, fmt.Errorf("gmcp: nil output schema")
	}

	var hasDefaults func(*jsonschema.Schema) bool
	hasDefaults = func(item *jsonschema.Schema) bool {
		if item.Default != nil {
			return true
		}

		for _, property := range item.Properties {
			if hasDefaults(property) {
				return true
			}
		}

		return false
	}

	resolved, rtErr = internal.Resolve(&jsonschema.ResolveOptions{ValidateDefaults: true})
	return resolved, hasDefaults(internal), rtErr
}

func (r *_Route) _Output(value any) (result *mcp.CallToolResult, rtErr error) {
	if value == nil {
		return &mcp.CallToolResult{}, nil
	}

	// Like the SDK, a nil result of a declared pointer Out is its element's zero value.
	if r._OutputPointer && reflect.ValueOf(value).IsNil() {
		value = reflect.Zero(reflect.TypeOf(value).Elem()).Interface()
	}

	encoded, rtErr := r._Server._EncodeOutput(value, r._Server._Options.MaxOutputBytes)
	if rtErr != nil {
		return nil, rtErr
	}

	if r._ResolvedOutput != nil {
		var plain any
		if rtErr = json.Unmarshal(encoded, &plain, jsonv1.DefaultOptionsV1()); rtErr != nil {
			return nil, rtErr
		}

		_, object := plain.(map[string]any)
		defaults := object && r._OutputDefaults || plain == nil && r._ResolvedOutput.Schema().Type == "object"
		if defaults {
			if plain == nil {
				plain = map[string]any{}
			}

			encoded = nil
			if rtErr = r._ResolvedOutput.ApplyDefaults(&plain); rtErr != nil {
				return nil, rtErr
			}
		}

		if rtErr = r._ResolvedOutput.Validate(&plain); rtErr != nil {
			return nil, rtErr
		}

		if defaults {
			if encoded, rtErr = r._Server._EncodeOutput(plain, r._Server._Options.MaxOutputBytes); rtErr != nil {
				return nil, rtErr
			}
		}
	}

	result = &mcp.CallToolResult{StructuredContent: jsonv1.RawMessage(encoded)}
	if blob, ok := value.(Blob); ok {
		result, rtErr = blob._Result()
		if rtErr != nil {
			return nil, rtErr
		}

		result.StructuredContent = jsonv1.RawMessage(encoded)
	} else {
		result.Content = []mcp.Content{&mcp.TextContent{Text: string(encoded)}}
	}

	return result, nil
}

func (s *Server) _BudgetMiddleware(next mcp.MethodHandler) (handler mcp.MethodHandler) {
	type callBudgetKey struct{}

	return func(ctx context.Context, method string, request mcp.Request) (result mcp.Result, rtErr error) {
		if method == "tools/call" && ctx.Value(callBudgetKey{}) == nil {
			select {
			case s._InFlight <- struct{}{}:
				defer func() { <-s._InFlight }()
				ctx = context.WithValue(ctx, callBudgetKey{}, true)
			default:
				failure := &_ToolError{_Category: _TemporarilyUnavailable}
				toolResult := &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(failure._Public()._JSON())}}}
				toolResult.SetError(failure)
				return toolResult, nil
			}
		}

		return next(ctx, method, request)
	}
}

// _WriteDataURL streams the base64 representation after its complete size has passed admission.
func (d DataURL) _WriteDataURL(writer *_OutputCapture) (rtErr error) {
	if !_ValidMediaType(d.MediaType) {
		return _ErrInvalidDataURL
	}

	prefix, rtErr := json.Marshal("data:"+d.MediaType+";base64,", jsonv1.DefaultOptionsV1())
	if rtErr != nil {
		return rtErr
	}

	if _, rtErr = writer.Write(prefix[:len(prefix)-1]); rtErr != nil {
		return rtErr
	}

	encoder := base64.NewEncoder(base64.StdEncoding, writer)
	if _, rtErr = encoder.Write(d.Data); rtErr != nil {
		return rtErr
	}

	if rtErr = encoder.Close(); rtErr != nil {
		return rtErr
	}

	_, rtErr = writer.Write([]byte{'"'})
	return rtErr
}
