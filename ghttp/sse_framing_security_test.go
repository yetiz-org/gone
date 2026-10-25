package ghttp

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yetiz-org/gone/channel"
)

type sseFramingContext struct {
	channel.HandlerContext
	transport  *Channel
	writes     int
	failAt     int
	writeErr   error
	lastFuture channel.Future
}

func (c *sseFramingContext) Channel() (ch channel.Channel) {
	return c.transport
}

func (c *sseFramingContext) Write(obj any, future channel.Future) (written channel.Future) {
	c.writes++
	c.lastFuture = future
	if c.failAt == c.writes {
		future.Completable().Fail(c.writeErr)
	} else if err := c.transport.UnsafeWrite(obj); err != nil {
		future.Completable().Fail(err)
	} else {
		future.Completable().Complete(obj)
	}

	return future
}

func newSSEFramingContext() (ctx *sseFramingContext, pack *Pack, params map[string]any, recorder *httptest.ResponseRecorder) {
	recorder = httptest.NewRecorder()
	recorder.Code = 0
	ctx = &sseFramingContext{transport: &Channel{}}
	pack = &Pack{Response: EmptyResponse(), Writer: recorder, writeSeparateMode: true}
	pack.Response.SetHeader("Content-Type", "text/event-stream")
	params = map[string]any{
		"[gone-http]context_pack": pack,
		"[gone-http]dispatcher":   NewDispatchHandler(nil),
	}
	return ctx, pack, params, recorder
}

func TestSSEFramingRejectsBeforeIO(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		message SSEMessage
	}{
		{name: "comment CR", message: SSEMessage{Comment: "private\rinjected"}},
		{name: "comment LF", message: SSEMessage{Comment: "private\ninjected"}},
		{name: "comment CRLF", message: SSEMessage{Comment: "private\r\ninjected"}},
		{name: "event CR", message: SSEMessage{Event: "private\rinjected"}},
		{name: "event LF", message: SSEMessage{Event: "private\ninjected"}},
		{name: "event CRLF", message: SSEMessage{Event: "private\r\ninjected"}},
		{name: "id CR", message: SSEMessage{Id: "private\rinjected"}},
		{name: "id LF", message: SSEMessage{Id: "private\ninjected"}},
		{name: "id CRLF", message: SSEMessage{Id: "private\r\ninjected"}},
		{name: "id NUL", message: SSEMessage{Id: "private\x00injected"}},
		{name: "empty", message: SSEMessage{}},
		{name: "negative retry only", message: SSEMessage{Retry: -1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			for _, batch := range []bool{false, true} {
				t.Run("batch="+strconv.FormatBool(batch), func(t *testing.T) {
					ctx, pack, params, recorder := newSSEFramingContext()
					var future channel.Future
					if batch {
						future = _DefaultSSEOperation.WriteMessages(ctx, []SSEMessage{{Data: []string{"valid"}}, tc.message}, params)
					} else {
						future = _DefaultSSEOperation.WriteMessage(ctx, tc.message, params)
					}

					require.True(t, future.IsDone())
					require.True(t, future.IsFail())
					require.Error(t, future.Error())
					assert.NotContains(t, future.Error().Error(), "private", "errors must not echo rejected fields")
					assert.Zero(t, ctx.writes, "must not invoke transport")
					assert.Zero(t, recorder.Code, "must not commit a status")
					assert.Empty(t, recorder.Body.Bytes())
					assert.Empty(t, recorder.Header(), "must not write headers")
					assert.False(t, recorder.Flushed)
					assert.False(t, pack.Response.headerWritten)
				})
			}
		})
	}

	t.Run("empty batch", func(t *testing.T) {
		ctx, _, params, recorder := newSSEFramingContext()
		future := _DefaultSSEOperation.WriteMessages(ctx, nil, params)
		require.True(t, future.IsFail())
		assert.Zero(t, ctx.writes)
		assert.Zero(t, recorder.Code)
		assert.Empty(t, recorder.Body.Bytes())
	})
}

func TestSSEFramingDataLines(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		message SSEMessage
		wire    string
	}{
		{
			name:    "legacy single line",
			message: SSEMessage{Comment: "comment", Event: "event", Data: []string{"one", "two"}, Id: "id", Retry: 1000},
			wire:    ": comment\nevent: event\ndata: one\ndata: two\nid: id\nretry: 1000\n\n",
		},
		{name: "LF", message: SSEMessage{Data: []string{"one\nevent: injected\n\n"}}, wire: "data: one\ndata: event: injected\ndata: \ndata: \n\n"},
		{name: "CR", message: SSEMessage{Data: []string{"one\revent: injected\r\r"}}, wire: "data: one\ndata: event: injected\ndata: \ndata: \n\n"},
		{name: "CRLF", message: SSEMessage{Data: []string{"one\r\nevent: injected\r\n\r\n"}}, wire: "data: one\ndata: event: injected\ndata: \ndata: \n\n"},
		{name: "mixed separators and empty datum", message: SSEMessage{Data: []string{"one\r\ntwo\rthree\nfour", ""}}, wire: "data: one\ndata: two\ndata: three\ndata: four\ndata: \n\n"},
		{name: "empty datum", message: SSEMessage{Data: []string{""}}, wire: "data: \n\n"},
		{name: "negative retry with data", message: SSEMessage{Data: []string{"one"}, Retry: -1}, wire: "data: one\n\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx, pack, params, recorder := newSSEFramingContext()
			future := _DefaultSSEOperation.WriteMessage(ctx, tc.message, params)

			require.True(t, future.IsSuccess())
			assert.Equal(t, tc.wire, recorder.Body.String())
			assert.Equal(t, http.StatusOK, recorder.Code)
			assert.Equal(t, "text/event-stream", recorder.Header().Get("Content-Type"))
			assert.True(t, recorder.Flushed)
			assert.True(t, pack.Response.headerWritten)
		})
	}
}

func TestSSEWriteBatchOrder(t *testing.T) {
	t.Parallel()

	ctx, _, params, recorder := newSSEFramingContext()
	future := _DefaultSSEOperation.WriteMessages(ctx, []SSEMessage{{Event: "first", Data: []string{"one"}}, {Event: "second", Data: []string{"two"}}}, params)

	require.True(t, future.IsSuccess())
	assert.Equal(t, "event: first\ndata: one\n\nevent: second\ndata: two\n\n", recorder.Body.String())
	assert.Equal(t, 3, ctx.writes, "one header and one write per event")
	assert.Same(t, ctx.lastFuture, future, "the returned Future belongs to the final event")
}

func TestSSEWriteTransportFailure(t *testing.T) {
	t.Parallel()

	for failAt := 1; failAt <= 3; failAt++ {
		t.Run(strconv.Itoa(failAt), func(t *testing.T) {
			t.Parallel()

			ctx, _, params, recorder := newSSEFramingContext()
			ctx.failAt = failAt
			ctx.writeErr = errors.New("transport failed")
			future := _DefaultSSEOperation.WriteMessages(ctx, []SSEMessage{{Data: []string{"one"}}, {Data: []string{"two"}}, {Data: []string{"three"}}}, params)

			require.True(t, future.IsFail())
			assert.Equal(t, failAt, ctx.writes, "later events must not be written")
			if failAt == 1 {
				assert.Zero(t, recorder.Code)
				assert.Empty(t, recorder.Body.Bytes())
			} else {
				assert.ErrorIs(t, future.Error(), ctx.writeErr)
				if failAt == 2 {
					assert.Empty(t, recorder.Body.Bytes())
				} else {
					assert.Equal(t, "data: one\n\n", recorder.Body.String())
				}
			}
		})
	}
}

func TestSSEWriteMissingPack(t *testing.T) {
	t.Parallel()

	ctx, _, _, recorder := newSSEFramingContext()
	future := _DefaultSSEOperation.WriteMessage(ctx, SSEMessage{Data: []string{"one"}}, nil)
	require.True(t, future.IsFail())
	future = _DefaultSSEOperation.WriteMessages(ctx, []SSEMessage{{Data: []string{"one"}}}, nil)
	require.True(t, future.IsFail())
	assert.Zero(t, ctx.writes)
	assert.Zero(t, recorder.Code)
	assert.Empty(t, recorder.Body.Bytes())
}
