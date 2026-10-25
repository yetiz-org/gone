package ghttp

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/yetiz-org/gone/channel"
)

type waitingResourceBody struct {
	reader      *bytes.Reader
	blocked     chan struct{}
	release     chan struct{}
	blockedOnce sync.Once
}

func (b *waitingResourceBody) Read(p []byte) (n int, rtErr error) {
	if b.reader.Len() > 0 {
		return b.reader.Read(p)
	}
	b.blockedOnce.Do(func() { close(b.blocked) })
	<-b.release
	return 0, io.EOF
}

func (b *waitingResourceBody) Close() (rtErr error) {
	return nil
}

func TestHTTPSharedInputBudgetAndRelease(t *testing.T) {
	server := &ServerChannel{maxBodyBytes: 1024}
	server.SetParam(channel.ParamKey("max_in_progress_bytes"), 1024)
	serve := func(body io.ReadCloser) *httptest.ResponseRecorder {
		left, right := net.Pipe()
		defer left.Close()
		defer right.Close()
		child := server.DeriveNetChildChannel(&Channel{}, server, left).(*Channel)
		child.Pipeline().AddLast("answer", channel.NewRWHandler(func(ctx channel.HandlerContext, obj any) {
			obj.(*Pack).Writer.WriteHeader(http.StatusOK)
		}, nil))
		request := httptest.NewRequest(http.MethodPost, "/upload", body)
		request.ContentLength = -1
		ctx := context.WithValue(request.Context(), ConnCtx, left)
		ctx = context.WithValue(ctx, ConnChCtx, child)
		recorder := httptest.NewRecorder()
		server.ServeHTTP(recorder, request.WithContext(ctx))
		return recorder
	}
	firstBody := &waitingResourceBody{reader: bytes.NewReader(make([]byte, 800)), blocked: make(chan struct{}), release: make(chan struct{})}
	var releaseOnce sync.Once
	defer releaseOnce.Do(func() { close(firstBody.release) })
	firstDone := make(chan *httptest.ResponseRecorder, 1)
	go func() { firstDone <- serve(firstBody) }()
	select {
	case <-firstBody.blocked:
	case <-time.After(time.Second):
		t.Fatal("first body was not retained")
	}
	second := serve(io.NopCloser(bytes.NewReader(make([]byte, 800))))
	require.NotEqual(t, http.StatusOK, second.Code, "another request exceeded the shared incomplete-body budget")
	releaseOnce.Do(func() { close(firstBody.release) })
	select {
	case first := <-firstDone:
		require.Equal(t, http.StatusOK, first.Code)
	case <-time.After(time.Second):
		t.Fatal("first request did not complete")
	}
	third := serve(io.NopCloser(bytes.NewReader(make([]byte, 800))))
	require.Equal(t, http.StatusOK, third.Code, "completion/error must return the reservation")
}
