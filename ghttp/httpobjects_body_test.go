package ghttp

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yetiz-org/gone/channel"
)

func TestWrapRequestPreservesBodyReadError(t *testing.T) {
	ch := &channel.DefaultChannel{}
	ch.Init()
	req := httptest.NewRequest("POST", "/upload", nil)
	req.Body = errorReadCloser{}

	wrapped := WrapRequest(ch, req)

	require.NotNil(t, wrapped)
	require.ErrorIs(t, wrapped.BodyReadError(), errBodyRead)
	require.Equal(t, 0, wrapped.Body().ReadableBytes())
}
