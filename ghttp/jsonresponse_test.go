package ghttp

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

type jsonResponseSample struct {
	URL string `json:"url"`
}

func newTestResponse() *Response {
	return NewResponse(&Request{request: httptest.NewRequest("GET", "/", nil)})
}

// TestResponse_JsonResponse_DefaultNoHTMLEscape verifies the package default keeps
// &, <, > literal, emits no trailing newline, and stays valid JSON.
func TestResponse_JsonResponse_DefaultNoHTMLEscape(t *testing.T) {
	url := "https://x/a.wav?Expires=1&Signature=a<b>&Key-Pair-Id=K"
	resp := newTestResponse()
	resp.JsonResponse(jsonResponseSample{URL: url})
	body := string(resp.Body().Bytes())

	// Literal &, <, > preserved; they would be escaped to \uXXXX otherwise.
	assert.Contains(t, body, url)
	// No trailing newline: byte-for-byte compatible with the previous json.Marshal output.
	assert.False(t, strings.HasSuffix(body, "\n"))

	var got jsonResponseSample
	assert.NoError(t, json.Unmarshal([]byte(body), &got))
	assert.Equal(t, url, got.URL)
}

// TestResponse_JsonResponse_PerResponseOverride verifies SetJsonEncoderConfig overrides
// the package default for one response, without mutating the global default.
func TestResponse_JsonResponse_PerResponseOverride(t *testing.T) {
	url := "https://x/a.wav?Expires=1&Signature=a<b>&Key-Pair-Id=K"
	resp := newTestResponse()
	resp.SetJsonEncoderConfig(func(enc *json.Encoder) { enc.SetEscapeHTML(true) })
	resp.JsonResponse(jsonResponseSample{URL: url})
	body := string(resp.Body().Bytes())

	// Escaping back on: the verbatim URL (with &, <, >) no longer appears in the body.
	assert.NotContains(t, body, url)

	var got jsonResponseSample
	assert.NoError(t, json.Unmarshal([]byte(body), &got))
	assert.Equal(t, url, got.URL)
}
