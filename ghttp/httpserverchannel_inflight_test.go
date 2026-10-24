package ghttp

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/yetiz-org/gone/channel"
	buf "github.com/yetiz-org/goth-bytebuf"
)

const testInflightBound = 100

// holdingTask buffers the body, reports it entered and keeps its reservation until release is closed.
type holdingTask struct {
	DefaultHTTPHandlerTask
	entered chan struct{}
	release chan struct{}
}

func (t *holdingTask) Post(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) (errResponse ErrorResponse) {
	length := req.Body().ReadableBytes()
	t.entered <- struct{}{}
	<-t.release
	resp.TextResponse(buf.NewByteBufString(fmt.Sprint(length)))
	return nil
}

func startInflightTestServer(t *testing.T, maxBodyBytes any, route Route) (addr string) {
	t.Helper()
	return bindTestServer(t, maxBodyBytes, func(ch channel.Channel) {
		ch.Pipeline().AddLast("DISPATCHER", NewDispatchHandler(route))
	}, func(bootstrap channel.ServerBootstrap) {
		bootstrap.SetParams(ParamMaxInflightBodyBytes, testInflightBound)
	})
}

// rawResponse writes a raw HTTP/1.1 request and returns the response without sending more body bytes.
func rawResponse(t *testing.T, addr string, request string) (resp *http.Response) {
	t.Helper()
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })
	require.NoError(t, conn.SetDeadline(time.Now().Add(3*time.Second)))
	_, err = io.WriteString(conn, request)
	require.NoError(t, err)
	resp, err = http.ReadResponse(bufio.NewReader(conn), nil)
	require.NoError(t, err, "the server must answer without waiting for the body")
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

func postBody(t *testing.T, addr string, size int, chunked bool) (status int, body string) {
	t.Helper()
	var reader io.Reader = bytes.NewReader(bytes.Repeat([]byte{'x'}, size))
	if chunked {
		reader = io.MultiReader(reader)
	}

	resp, err := http.Post("http://"+addr+"/upload", "application/octet-stream", reader)
	require.NoError(t, err)
	defer resp.Body.Close()
	got, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, string(got)
}

// holdRequest starts a request that holds its reservation until the returned release func runs.
func holdRequest(t *testing.T, addr string, task *holdingTask, size int, chunked bool) (release func()) {
	t.Helper()
	var reader io.Reader = bytes.NewReader(bytes.Repeat([]byte{'x'}, size))
	if chunked {
		reader = io.MultiReader(reader)
	}

	done := make(chan int, 1)
	go func() {
		resp, err := http.Post("http://"+addr+"/upload", "application/octet-stream", reader)
		if err != nil {
			done <- 0
			return
		}

		resp.Body.Close()
		done <- resp.StatusCode
	}()

	select {
	case <-task.entered:
	case <-time.After(3 * time.Second):
		t.Fatal("the held request did not reach the task")
	}

	return func() {
		close(task.release)
		require.Equal(t, http.StatusOK, <-done, "the held request completes after release")
	}
}

func TestServerChannelInflightBodyBound(t *testing.T) {
	tests := []struct {
		name         string
		maxBodyBytes any
		size         int
		chunked      bool
		wantStatus   int
	}{
		{name: "fixed body within the bound", size: testInflightBound, wantStatus: http.StatusOK},
		{name: "chunked body within the bound", size: testInflightBound, chunked: true, wantStatus: http.StatusOK},
		{name: "chunked body is limited to the bound when the body limit is disabled", maxBodyBytes: int64(0), size: testInflightBound + 1, chunked: true, wantStatus: http.StatusRequestEntityTooLarge},
		{name: "chunked body is limited to the bound when the body limit is larger", maxBodyBytes: int64(testInflightBound * 2), size: testInflightBound + 1, chunked: true, wantStatus: http.StatusRequestEntityTooLarge},
		{name: "empty body reserves nothing", wantStatus: http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr := startInflightTestServer(t, tt.maxBodyBytes, NewSimpleRoute().SetEndpoint("/upload", &bodyEchoTask{}))

			status, body := postBody(t, addr, tt.size, tt.chunked)

			require.Equal(t, tt.wantStatus, status)
			if tt.wantStatus == http.StatusOK {
				require.Equal(t, fmt.Sprint(tt.size), body)
			}
		})
	}
}

func TestServerChannelInflightBodyOverBoundRejectedUnread(t *testing.T) {
	tests := []struct {
		name          string
		maxBodyBytes  any
		contentLength string
	}{
		{name: "length over the bound", contentLength: fmt.Sprint(testInflightBound + 1)},
		{name: "largest length with the body limit disabled", maxBodyBytes: int64(0), contentLength: "9223372036854775807"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &bodyEchoTask{}
			addr := startInflightTestServer(t, tt.maxBodyBytes, NewSimpleRoute().SetEndpoint("/upload", task))

			resp := rawResponse(t, addr, "POST /upload HTTP/1.1\r\nHost: test\r\nContent-Length: "+tt.contentLength+"\r\n\r\n")

			require.Equal(t, http.StatusRequestEntityTooLarge, resp.StatusCode)
			require.True(t, resp.Close, "the connection closes instead of reading the unread body")
			require.Zero(t, task.calls.Load(), "a rejected body must not reach the task")
		})
	}
}

func TestServerChannelInflightBodyBoundRecovers(t *testing.T) {
	task := &holdingTask{entered: make(chan struct{}, 1), release: make(chan struct{})}
	route := NewSimpleRoute().SetEndpoint("/upload", task).SetEndpoint("/echo", &bodyEchoTask{})
	addr := startInflightTestServer(t, nil, route)
	release := holdRequest(t, addr, task, 60, false)

	resp := rawResponse(t, addr, "POST /echo HTTP/1.1\r\nHost: test\r\nContent-Length: 41\r\n\r\n")
	require.Equal(t, http.StatusServiceUnavailable, resp.StatusCode, "a body that does not fit the remaining bound is rejected")
	require.Equal(t, "1", resp.Header.Get("Retry-After"))
	require.True(t, resp.Close, "the connection closes instead of reading the unread body")

	release()
	full, err := http.Post("http://"+addr+"/echo", "text/plain", strings.NewReader(strings.Repeat("x", testInflightBound)))
	require.NoError(t, err)
	defer full.Body.Close()
	require.Equal(t, http.StatusOK, full.StatusCode, "a finished request returns its reservation")
}

func TestServerChannelInflightUnknownLengthReservesBodyLimit(t *testing.T) {
	task := &holdingTask{entered: make(chan struct{}, 1), release: make(chan struct{})}
	route := NewSimpleRoute().SetEndpoint("/upload", task).SetEndpoint("/echo", &bodyEchoTask{})
	addr := startInflightTestServer(t, int64(60), route)
	release := holdRequest(t, addr, task, 10, true)
	defer release()

	over := rawResponse(t, addr, "POST /echo HTTP/1.1\r\nHost: test\r\nContent-Length: 41\r\n\r\n")
	require.Equal(t, http.StatusServiceUnavailable, over.StatusCode, "a chunked body reserves the whole body limit")

	resp, err := http.Post("http://"+addr+"/echo", "text/plain", strings.NewReader(strings.Repeat("x", 40)))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, "the rest of the bound stays available")
}

func TestServerChannelInflightBodyUnsetKeepsBodies(t *testing.T) {
	addr := startTestServer(t, int64(0), NewSimpleRoute().SetEndpoint("/upload", &bodyEchoTask{}), false)

	status, body := postBody(t, addr, testInflightBound*1000, true)

	require.Equal(t, http.StatusOK, status, "without the bound a body is limited only by ParamMaxBodyBytes")
	require.Equal(t, fmt.Sprint(testInflightBound*1000), body)
}

func TestRequestLogURI(t *testing.T) {
	ch := &channel.DefaultChannel{}
	ch.Init()
	raw := httptest.NewRequest(MethodGet, "/reset/secret?token=secret", nil)
	req := WrapRequest(ch, raw)
	resp := NewResponse(req)
	handler := NewLogHandler(false)
	require.Equal(t, "/reset/secret?token=secret", req.LogURI(), "without ParamLogURI the log keeps RequestURI")

	ch.SetParam(ParamLogURI, func(req *Request) string { return req.Url().Path + "?[REDACTED]" })
	require.Equal(t, "/reset/secret?[REDACTED]", req.LogURI())
	require.Equal(t, "/reset/secret?[REDACTED]", handler.constructReq(req).URI)
	require.Equal(t, "/reset/secret?[REDACTED]", handler.constructResp(resp).URI)
	require.Equal(t, "/reset/secret?token=secret", req.RequestURI(), "handlers still see the request URI")
}
