package gmcp

import (
	"bytes"
	"encoding/base64"
	"encoding/json/v2"
	"errors"
	"fmt"
	"mime"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yetiz-org/gone/ghttp"
	"github.com/yetiz-org/gone/ghttp/httpheadername"
	kklogger "github.com/yetiz-org/goth-kklogger"
)

// _DataURLPattern is the prefix every input data URL matches; it is valid in both RE2 and ECMA-262.
const _DataURLPattern = `^data:[^,]+;base64,`

// _InputDataURLDescription and _OutputDataURLDescription describe a DataURL in input and output schemas.
const (
	_InputDataURLDescription  = "Base64 data URL with a media type, such as data:image/png;base64,iVBORw0KGgo=."
	_OutputDataURLDescription = "Base64 data URL with the media type of the content."
)

// _ErrInvalidDataURL is returned for every malformed data URL; the error middleware turns it into invalid_argument.
var _ErrInvalidDataURL = errors.New("gmcp: invalid data URL")

// DataURL is binary content with its media type. Its JSON form is one RFC 2397 data URL string in base64 form, such
// as "data:image/png;base64,iVBORw0KGgo=". MediaType needs a type and subtype that mime.ParseMediaType accepts and may
// carry parameters, such as "text/plain;charset=utf-8", which are kept as written.
type DataURL struct {
	MediaType string
	Data      []byte
}

// File is a file that a tool input forwards as one multipart/form-data part; see the file placement. The part takes
// its Content-Type from Data.MediaType.
type File struct {
	Filename string  `json:"filename"`
	Data     DataURL `json:"data"`
}

// Blob is the output of a tool that returns the REST response body as is. Data carries the body with the response
// Content-Type, or application/octet-stream when the response has no valid one, and Filename is the file name the
// response Content-Disposition suggests when it is a valid file name.
type Blob struct {
	Filename string  `json:"filename,omitempty"`
	Data     DataURL `json:"data"`
}

// MarshalText encodes d as a base64 data URL. It fails when MediaType is not a valid media type.
func (d DataURL) MarshalText() (text []byte, err error) {
	if !_ValidMediaType(d.MediaType) {
		return nil, _ErrInvalidDataURL
	}

	text = make([]byte, 0, len("data:;base64,")+len(d.MediaType)+base64.StdEncoding.EncodedLen(len(d.Data)))
	text = fmt.Appendf(text, "data:%s;base64,", d.MediaType)
	return base64.StdEncoding.AppendEncode(text, d.Data), nil
}

// UnmarshalText decodes a base64 data URL with a valid media type; the base64 needs its padding. Percent-encoded data
// URLs and data URLs without a media type are rejected.
func (d *DataURL) UnmarshalText(text []byte) (err error) {
	rest, prefixed := bytes.CutPrefix(text, []byte("data:"))
	header, payload, separated := bytes.Cut(rest, []byte(","))
	mediaType, encoded := bytes.CutSuffix(header, []byte(";base64"))
	if !prefixed || !separated || !encoded || !_ValidMediaType(string(mediaType)) {
		return _ErrInvalidDataURL
	}

	data, err := base64.StdEncoding.AppendDecode(nil, payload)
	if err != nil {
		return _ErrInvalidDataURL
	}

	d.MediaType, d.Data = string(mediaType), data
	return nil
}

// _Read sets b from a successful response: Data holds the body and its Content-Type, and Filename the
// Content-Disposition filename when it is a valid file name.
func (b *Blob) _Read(response *ghttp.Response) {
	b.Data = DataURL{MediaType: "application/octet-stream", Data: response.Body().Bytes()}
	if mediaType := response.GetHeader(httpheadername.ContentType); _ValidMediaType(mediaType) {
		b.Data.MediaType = mediaType
	}

	if _, params, err := mime.ParseMediaType(response.GetHeader(httpheadername.ContentDisposition)); err == nil && _ValidFilename(params["filename"]) {
		b.Filename = params["filename"]
	}
}

// _Result is the result content of b: one text summary of its media type and filename. Without content, the SDK
// would add the whole structured output as text, sending the base64 data twice.
func (b Blob) _Result() (result *mcp.CallToolResult, err error) {
	summary, err := json.Marshal(struct {
		MediaType string `json:"media_type"`
		Filename  string `json:"filename,omitempty"`
	}{MediaType: b.Data.MediaType, Filename: b.Filename})
	if err != nil {
		kklogger.ErrorJ("gmcp:Blob.Result#summary!encode_failed", map[string]any{"error": err.Error()})
		return nil, &_ToolError{_Code: _InternalError}
	}

	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(summary)}}}, nil
}

// _ValidMediaType reports whether value is a type/subtype media type, with optional parameters, that
// mime.ParseMediaType accepts and that fits in a data URL and a header: it is UTF-8 with no comma and no control
// character.
func _ValidMediaType(value string) (valid bool) {
	mediaType, _, err := mime.ParseMediaType(value)
	return err == nil && strings.Contains(mediaType, "/") && utf8.ValidString(value) && !strings.ContainsFunc(value, func(r rune) bool {
		return r == ',' || unicode.IsControl(r)
	})
}

// _ValidFilename reports whether name is a UTF-8 file name without a directory: it is not empty, ".", or "..", and has
// no slash, backslash, or control character.
func _ValidFilename(name string) (valid bool) {
	return name != "" && name != "." && name != ".." && utf8.ValidString(name) && !strings.ContainsFunc(name, func(r rune) bool {
		return r == '/' || r == '\\' || unicode.IsControl(r)
	})
}
