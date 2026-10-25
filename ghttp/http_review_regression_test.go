package ghttp

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/yetiz-org/gone/channel"
	"github.com/yetiz-org/gone/ghttp/httpsession"
	"github.com/yetiz-org/gone/ghttp/httpsession/memory"
	"github.com/yetiz-org/goth-util/hash"
)

type boundaryRenewalSession struct {
	httpsession.Session
	onExpire func()
}

func (s *boundaryRenewalSession) IsExpire() (expired bool) {
	s.onExpire()
	return true
}

func TestExpiredLookupPreservesRenewal(t *testing.T) {
	previous := SessionProvider()
	t.Cleanup(func() { defaultSessionProvider = previous })
	for _, replacement := range []bool{false, true} {
		t.Run("replacement="+strconv.FormatBool(replacement), func(t *testing.T) {
			provider := &boundaryLookupProvider{SessionProvider: memory.NewSessionProvider()}
			held := httpsession.NewDefaultSession(provider)
			held.SetExpire(time.Now().Add(-time.Hour))
			var live httpsession.Session = held
			if replacement {
				data, err := held.MarshalJSON()
				require.NoError(t, err)
				current := httpsession.NewDefaultSession(provider)
				require.NoError(t, current.UnmarshalJSON(data))
				live = current
			}
			provider.session = &boundaryRenewalSession{Session: held, onExpire: func() {
				live.SetExpire(time.Now().Add(time.Hour))
				provider.session = live
			}}
			defaultSessionProvider = provider
			req := WrapRequest(channel.NewMockChannel(), httptest.NewRequest(MethodGet, "/session", nil))
			req.AddCookie(&http.Cookie{Name: SessionKey, Value: hash.TimeHash([]byte(held.Id()), time.Now().Add(time.Hour).Unix())})
			require.Nil(t, _StoredSession(req), "an expired observation must reject the current request")
			require.NotNil(t, provider.Session(held.Id()), "a later valid generation must survive")
			require.Same(t, live, provider.Session(held.Id()))
			require.False(t, live.IsExpire())
		})
	}
}

func TestSSEHeaderFailureDoesNotEncodeBatch(t *testing.T) {
	message := SSEMessage{Data: []string{strings.Repeat("x", 4096)}}
	allocations := func(count int) float64 {
		messages := make([]SSEMessage, count)
		for i := range messages {
			messages[i] = message
		}
		return testing.AllocsPerRun(3, func() {
			ctx, _, params, _ := newSSEFramingContext()
			ctx.failAt, ctx.writeErr = 1, errors.New("header failed")
			future := _DefaultSSEOperation.WriteMessages(ctx, messages, params)
			if !future.IsFail() || ctx.writes != 1 {
				t.Fatal("header failure must stop all events")
			}
		})
	}
	one, batch := allocations(1), allocations(16)
	require.LessOrEqual(t, batch, one+1, "never-written events must not allocate encoded buffers")
}

type budgetStreamTask struct {
	DefaultHTTPHandlerTask
	read int
	err  error
}

func (t *budgetStreamTask) StreamRequestBody() (stream bool) {
	return true
}

func (t *budgetStreamTask) Post(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) (errResponse ErrorResponse) {
	var scratch [64]byte
	for {
		n, err := req.Request().Body.Read(scratch[:])
		t.read += n
		if err != nil {
			if !errors.Is(err, io.EOF) {
				t.err = err
				resp.SetStatusCode(http.StatusBadRequest)
			}
			return nil
		}
	}
}

func TestHTTPStreamDoesNotRetainConsumedBytes(t *testing.T) {
	server := &ServerChannel{maxBodyBytes: 4096}
	server.SetParam(channel.ParamMaxInProgressBytes, 1024)
	task := &budgetStreamTask{}
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()
	child := server.DeriveNetChildChannel(&Channel{}, server, left).(*Channel)
	child.Pipeline().AddLast("dispatch", NewDispatchHandler(NewSimpleRoute().SetEndpoint("/stream", task)))
	request := httptest.NewRequest(http.MethodPost, "/stream", strings.NewReader(strings.Repeat("x", 2048)))
	ctx := context.WithValue(request.Context(), ConnCtx, left)
	ctx = context.WithValue(ctx, ConnChCtx, child)
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request.WithContext(ctx))
	require.NoError(t, task.err)
	require.Equal(t, 2048, task.read)
	require.Equal(t, http.StatusOK, recorder.Code)
}

func TestHTTPBufferedBodyStaysChargedUntilHandlerReturns(t *testing.T) {
	server := &ServerChannel{maxBodyBytes: 1024}
	server.SetParam(channel.ParamMaxInProgressBytes, 1024)
	entered, proceed := make(chan struct{}), make(chan struct{})
	var unblock sync.Once
	t.Cleanup(func() { unblock.Do(func() { close(proceed) }) })
	serve := func(block bool) *httptest.ResponseRecorder {
		left, right := net.Pipe()
		defer left.Close()
		defer right.Close()
		child := server.DeriveNetChildChannel(&Channel{}, server, left).(*Channel)
		child.Pipeline().AddLast("answer", channel.NewRWHandler(func(ctx channel.HandlerContext, obj any) {
			pack := obj.(*Pack)
			if block {
				close(entered)
				<-proceed
			}
			if pack.Request.Body().ReadableBytes() == 800 {
				pack.Writer.WriteHeader(http.StatusOK)
			}
		}, nil))
		request := httptest.NewRequest(http.MethodPost, "/upload", bytes.NewReader(make([]byte, 800)))
		ctx := context.WithValue(request.Context(), ConnCtx, left)
		ctx = context.WithValue(ctx, ConnChCtx, child)
		recorder := httptest.NewRecorder()
		server.ServeHTTP(recorder, request.WithContext(ctx))
		return recorder
	}
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() { done <- serve(true) }()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("first handler did not receive its completed buffer")
	}
	second := serve(false)
	unblock.Do(func() { close(proceed) })
	select {
	case first := <-done:
		require.Equal(t, http.StatusOK, first.Code)
	case <-time.After(3 * time.Second):
		t.Fatal("first handler did not return")
	}
	require.Equal(t, http.StatusBadRequest, second.Code, "a retained Request buffer still occupies the shared budget")
	require.Equal(t, http.StatusOK, serve(false).Code, "returning the handler releases its buffer reservation")
}

func BenchmarkSSEFailedHeaderBatch(b *testing.B) {
	for _, count := range []int{1, 16, 256} {
		b.Run(strconv.Itoa(count), func(b *testing.B) {
			messages := make([]SSEMessage, count)
			for i := range messages {
				messages[i] = SSEMessage{Data: []string{strings.Repeat("x", 64<<10)}}
			}
			b.ReportAllocs()
			for b.Loop() {
				ctx, _, params, _ := newSSEFramingContext()
				ctx.failAt, ctx.writeErr = 1, io.ErrClosedPipe
				_DefaultSSEOperation.WriteMessages(ctx, messages, params)
			}
		})
	}
}

func BenchmarkHTTPBudgetStream(b *testing.B) {
	server := &ServerChannel{maxBodyBytes: 4096}
	server.SetParam(channel.ParamMaxInProgressBytes, 4096)
	task := &budgetStreamTask{}
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()
	child := server.DeriveNetChildChannel(&Channel{}, server, left).(*Channel)
	child.Pipeline().AddLast("dispatch", NewDispatchHandler(NewSimpleRoute().SetEndpoint("/stream", task)))
	data := strings.Repeat("x", 2048)
	b.ReportAllocs()
	for b.Loop() {
		request := httptest.NewRequest(http.MethodPost, "/stream", strings.NewReader(data))
		ctx := context.WithValue(request.Context(), ConnCtx, left)
		ctx = context.WithValue(ctx, ConnChCtx, child)
		server.ServeHTTP(httptest.NewRecorder(), request.WithContext(ctx))
	}
}
