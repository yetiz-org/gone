package ghttp

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestResponseBoundedEncoderConfig(t *testing.T) {
	original := DefaultJsonEncoderConfig
	t.Cleanup(func() { DefaultJsonEncoderConfig = original })
	for _, standard := range []bool{false, true} {
		DefaultJsonEncoderConfig = original
		if standard {
			DefaultJsonEncoderConfig = nil
		}

		response := newTestResponse()
		response.bodyLimit = 512
		response.JsonResponse(jsonResponseSample{URL: "<ok>&"})
		require.NoError(t, response.BodyError())
		if standard {
			require.Contains(t, string(response.Body().Bytes()), `\u003cok\u003e\u0026`)
		} else {
			require.Contains(t, string(response.Body().Bytes()), `<ok>&`)
		}
	}

	for _, global := range []bool{false, true} {
		for _, escape := range []bool{false, true} {
			config := func(enc *json.Encoder) { enc.SetEscapeHTML(escape); enc.SetIndent("", "  ") }
			var expected bytes.Buffer
			encoder := json.NewEncoder(&expected)
			config(encoder)
			value := jsonResponseSample{URL: "<ok>&"}
			require.NoError(t, encoder.Encode(value))
			wire := bytes.TrimRight(expected.Bytes(), "\n")
			for _, limit := range []int64{int64(len(wire)) - 1, int64(len(wire)), int64(len(wire)) + 1} {
				DefaultJsonEncoderConfig = original
				response := newTestResponse()
				response.bodyLimit = limit
				if global {
					DefaultJsonEncoderConfig = config
				} else {
					response.SetJsonEncoderConfig(config)
				}
				response.JsonResponse(value)
				if limit < int64(len(wire)) {
					require.ErrorIs(t, response.BodyError(), ErrResponseBodyTooLarge)
					require.Empty(t, response.Body().Bytes())
				} else {
					require.NoError(t, response.BodyError())
					require.Equal(t, string(wire), string(response.Body().Bytes()))
				}
			}
		}
	}
}
