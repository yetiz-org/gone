package ghttp

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yetiz-org/gone/channel"
	buf "github.com/yetiz-org/goth-bytebuf"
)

// RangeTestTask is a test handler that returns fixed content for Range testing
type RangeTestTask struct {
	DefaultHTTPHandlerTask
	content []byte
}

func NewRangeTestTask(content []byte) *RangeTestTask {
	return &RangeTestTask{content: content}
}

func (h *RangeTestTask) Get(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) ErrorResponse {
	resp.SetHeader("Content-Type", "application/octet-stream")
	resp.SetBody(buf.NewByteBuf(h.content))
	return nil
}

// RangeDisabledTask is a test handler with auto range disabled
type RangeDisabledTask struct {
	DefaultHTTPHandlerTask
	content []byte
}

func NewRangeDisabledTask(content []byte) *RangeDisabledTask {
	return &RangeDisabledTask{content: content}
}

func (h *RangeDisabledTask) EnableAutoRangeSupport() bool {
	return false
}

func (h *RangeDisabledTask) Get(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) ErrorResponse {
	resp.SetHeader("Content-Type", "application/octet-stream")
	resp.SetBody(buf.NewByteBuf(h.content))
	return nil
}

// TestAutoRangeSupporter_Integration tests the AutoRangeSupporter interface with gone server
func TestAutoRangeSupporter_Integration(t *testing.T) {
	testContent := make([]byte, 1000)
	for i := 0; i < 1000; i++ {
		testContent[i] = byte('A' + (i % 26))
	}

	rangeEnabledTask := NewRangeTestTask(testContent)
	rangeDisabledTask := NewRangeDisabledTask(testContent)

	route := NewSimpleRoute()
	route.SetEndpoint("/enabled", rangeEnabledTask)
	route.SetEndpoint("/disabled", rangeDisabledTask)

	bootstrap := channel.NewServerBootstrap()
	bootstrap.ChannelType(&ServerChannel{})
	bootstrap.Handler(channel.NewInitializer(func(ch channel.Channel) {
		ch.Pipeline().AddLast("INDICATE_HANDLER_INBOUND", &channel.IndicateHandlerInbound{})
		ch.Pipeline().AddLast("INDICATE_HANDLER_OUTBOUND", &channel.IndicateHandlerOutbound{})
	}))

	bootstrap.ChildHandler(channel.NewInitializer(func(ch channel.Channel) {
		ch.Pipeline().AddLast("INDICATE_HANDLER_INBOUND", &channel.IndicateHandlerInbound{})
		ch.Pipeline().AddLast("NET_STATUS_INBOUND", &channel.NetStatusInbound{})
		ch.Pipeline().AddLast("LOG_HANDLER", NewLogHandler(false))
		ch.Pipeline().AddLast("DISPATCHER", NewDispatchHandler(route))
		ch.Pipeline().AddLast("NET_STATUS_OUTBOUND", &channel.NetStatusOutbound{})
		ch.Pipeline().AddLast("INDICATE_HANDLER_OUTBOUND", &channel.IndicateHandlerOutbound{})
	}))

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	serverCh := bootstrap.Bind(&net.TCPAddr{IP: nil, Port: port}).Sync().Channel()
	require.NotNil(t, serverCh)

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)

	defer func() {
		serverCh.Close()
		serverCh.CloseFuture().Sync()
	}()

	client := &http.Client{Timeout: 5 * time.Second}
	time.Sleep(100 * time.Millisecond)

	t.Run("Enabled_NormalRequest", func(t *testing.T) {
		resp, err := client.Get(baseURL + "/enabled")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, 200, resp.StatusCode)
		assert.Equal(t, "bytes", resp.Header.Get("Accept-Ranges"))
		assert.Equal(t, "1000", resp.Header.Get("Content-Length"))

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Equal(t, testContent, body)
	})

	t.Run("Enabled_RangeRequest", func(t *testing.T) {
		req, err := http.NewRequest("GET", baseURL+"/enabled", nil)
		require.NoError(t, err)
		req.Header.Set("Range", "bytes=0-99")

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, 206, resp.StatusCode)
		assert.Equal(t, "bytes 0-99/1000", resp.Header.Get("Content-Range"))
		assert.Equal(t, "100", resp.Header.Get("Content-Length"))

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Equal(t, testContent[0:100], body)
	})

	t.Run("Enabled_RangeFromEnd", func(t *testing.T) {
		req, err := http.NewRequest("GET", baseURL+"/enabled", nil)
		require.NoError(t, err)
		req.Header.Set("Range", "bytes=-100")

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, 206, resp.StatusCode)
		assert.Equal(t, "bytes 900-999/1000", resp.Header.Get("Content-Range"))
		assert.Equal(t, "100", resp.Header.Get("Content-Length"))

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Equal(t, testContent[900:1000], body)
	})

	t.Run("Enabled_RangeToEnd", func(t *testing.T) {
		req, err := http.NewRequest("GET", baseURL+"/enabled", nil)
		require.NoError(t, err)
		req.Header.Set("Range", "bytes=500-")

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, 206, resp.StatusCode)
		assert.Equal(t, "bytes 500-999/1000", resp.Header.Get("Content-Range"))
		assert.Equal(t, "500", resp.Header.Get("Content-Length"))

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Equal(t, testContent[500:1000], body)
	})

	t.Run("Enabled_InvalidRange", func(t *testing.T) {
		req, err := http.NewRequest("GET", baseURL+"/enabled", nil)
		require.NoError(t, err)
		req.Header.Set("Range", "bytes=2000-3000")

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, 416, resp.StatusCode)
		assert.Equal(t, "bytes */1000", resp.Header.Get("Content-Range"))
	})

	t.Run("Disabled_NormalRequest", func(t *testing.T) {
		resp, err := client.Get(baseURL + "/disabled")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, 200, resp.StatusCode)
		assert.Empty(t, resp.Header.Get("Accept-Ranges"))
		assert.Equal(t, "1000", resp.Header.Get("Content-Length"))

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Equal(t, testContent, body)
	})

	t.Run("Disabled_RangeRequestIgnored", func(t *testing.T) {
		req, err := http.NewRequest("GET", baseURL+"/disabled", nil)
		require.NoError(t, err)
		req.Header.Set("Range", "bytes=0-99")

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, 200, resp.StatusCode)
		assert.Empty(t, resp.Header.Get("Content-Range"))
		assert.Equal(t, "1000", resp.Header.Get("Content-Length"))

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Equal(t, testContent, body)
	})
}

// TestParseRange tests the ParseRange function directly
func TestParseRange(t *testing.T) {
	t.Run("ValidSpecifiedRange", func(t *testing.T) {
		tests := []struct {
			header    string
			size      int64
			wantStart int64
			wantEnd   int64
			wantValid bool
		}{
			{"bytes=0-99", 1000, 0, 99, true},
			{"bytes=500-599", 1000, 500, 599, true},
			{"bytes=0-0", 1000, 0, 0, true},
			{"bytes=999-999", 1000, 999, 999, true},
			{"bytes=0-1500", 1000, 0, 999, true},
		}

		for _, tt := range tests {
			t.Run(tt.header, func(t *testing.T) {
				start, end, valid := ParseRange(tt.header, tt.size)
				assert.Equal(t, tt.wantValid, valid)
				if valid {
					assert.Equal(t, tt.wantStart, start)
					assert.Equal(t, tt.wantEnd, end)
				}
			})
		}
	})

	t.Run("ValidSuffixRange", func(t *testing.T) {
		tests := []struct {
			header    string
			size      int64
			wantStart int64
			wantEnd   int64
		}{
			{"bytes=-100", 1000, 900, 999},
			{"bytes=-1", 1000, 999, 999},
			{"bytes=-1000", 1000, 0, 999},
			{"bytes=-2000", 1000, 0, 999},
		}

		for _, tt := range tests {
			t.Run(tt.header, func(t *testing.T) {
				start, end, valid := ParseRange(tt.header, tt.size)
				assert.True(t, valid)
				assert.Equal(t, tt.wantStart, start)
				assert.Equal(t, tt.wantEnd, end)
			})
		}
	})

	t.Run("ValidOpenEndRange", func(t *testing.T) {
		tests := []struct {
			header    string
			size      int64
			wantStart int64
			wantEnd   int64
		}{
			{"bytes=0-", 1000, 0, 999},
			{"bytes=500-", 1000, 500, 999},
			{"bytes=999-", 1000, 999, 999},
		}

		for _, tt := range tests {
			t.Run(tt.header, func(t *testing.T) {
				start, end, valid := ParseRange(tt.header, tt.size)
				assert.True(t, valid)
				assert.Equal(t, tt.wantStart, start)
				assert.Equal(t, tt.wantEnd, end)
			})
		}
	})

	t.Run("InvalidRanges", func(t *testing.T) {
		tests := []string{
			"bytes=1000-2000",
			"bytes=500-400",
			"bytes=-0",
			"bytes=-",
			"bytes=0-100,200-300",
			"characters=0-100",
			"0-100",
			"bytes=abc-def",
			"",
		}

		for _, header := range tests {
			t.Run(header, func(t *testing.T) {
				_, _, valid := ParseRange(header, 1000)
				assert.False(t, valid)
			})
		}
	})
}

// TestAutoRangeSupporter_EnabledByDefault verifies DefaultHTTPHandlerTask enables auto range
func TestAutoRangeSupporter_EnabledByDefault(t *testing.T) {
	task := &DefaultHTTPHandlerTask{}
	assert.True(t, task.EnableAutoRangeSupport())
}

// TestAutoRangeSupporter_CanBeDisabled verifies handlers can disable auto range
func TestAutoRangeSupporter_CanBeDisabled(t *testing.T) {
	task := &RangeDisabledTask{}
	assert.False(t, task.EnableAutoRangeSupport())
}
