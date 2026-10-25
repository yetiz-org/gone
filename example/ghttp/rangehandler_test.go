package example

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yetiz-org/gone/ghttp"
)

func TestCustomRangeTask_Response(t *testing.T) {
	for _, tt := range []struct {
		name, rangeHeader, data string
		status                  int
		contentRange, body      string
	}{
		{"partial", "bytes=1-2", "abcd", 206, "bytes 1-2/4", "bc"},
		{"unsatisfiable", "bytes=4-5", "abcd", 416, "bytes */4", ""},
		{"full", "", "abcd", 200, "", "abcd"},
		{"empty_suffix", "bytes=-1", "", 416, "bytes */0", ""},
		{"empty_full", "", "", 200, "", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			raw := httptest.NewRequest("GET", "/video", nil)
			raw.Header.Set("Range", tt.rangeHeader)
			req := ghttp.WrapRequest(nil, raw)
			resp := ghttp.NewResponse(req)
			task := &CustomRangeTask{data: []byte(tt.data)}
			require.Nil(t, task.Get(nil, req, resp, nil))
			assert.Equal(t, tt.status, resp.StatusCode())
			assert.Equal(t, tt.contentRange, resp.GetHeader("Content-Range"))
			assert.Equal(t, tt.body, string(resp.Body().Bytes()))
		})
	}
}
