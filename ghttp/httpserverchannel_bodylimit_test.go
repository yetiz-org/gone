package ghttp

import (
	"bufio"
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/yetiz-org/gone/channel"
	buf "github.com/yetiz-org/goth-bytebuf"
)

const testDefaultBodyLimit = 4 << 20

type bodyEchoTask struct {
	DefaultHTTPHandlerTask
	calls atomic.Int32
}

func (t *bodyEchoTask) Post(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) (errResponse ErrorResponse) {
	t.calls.Add(1)
	buffered := req.Body().Bytes()
	replayed, err := io.ReadAll(req.Request().Body)
	if err != nil || !bytes.Equal(buffered, replayed) {
		resp.SetStatusCode(http.StatusConflict)
		return nil
	}

	resp.TextResponse(buf.NewByteBufString(fmt.Sprint(len(buffered))))
	return nil
}

type bodyStreamTask struct {
	DefaultHTTPHandlerTask
	entered chan struct{}
}

func (t *bodyStreamTask) StreamRequestBody() (stream bool) {
	return true
}

func (t *bodyStreamTask) Post(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) (errResponse ErrorResponse) {
	// Reading the first byte before the client sends the rest proves the body is not buffered first.
	first, err := io.ReadFull(req.Request().Body, make([]byte, 1))
	close(t.entered)
	n, copyErr := io.Copy(io.Discard, req.Request().Body)
	n, err = n+int64(first), cmp.Or(err, copyErr)
	if _, tooLarge := errors.AsType[*http.MaxBytesError](err); tooLarge {
		resp.SetStatusCode(http.StatusRequestEntityTooLarge)
		return nil
	}

	resp.TextResponse(buf.NewByteBufString(fmt.Sprint(n)))
	return nil
}

func startTestServer(t *testing.T, maxBodyBytes any, route Route, printBody bool) (addr string) {
	t.Helper()
	return bindTestServer(t, maxBodyBytes, func(ch channel.Channel) {
		ch.Pipeline().AddLast("LOG_HANDLER", NewLogHandler(printBody))
		ch.Pipeline().AddLast("DISPATCHER", NewDispatchHandler(route))
	})
}

func bindTestServer(t *testing.T, maxBodyBytes any, childInit func(ch channel.Channel)) (addr string) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	require.NoError(t, listener.Close())

	bootstrap := channel.NewServerBootstrap()
	bootstrap.ChildHandler(channel.NewInitializer(childInit))
	bootstrap.ChannelType(&ServerChannel{})
	if maxBodyBytes != nil {
		bootstrap.SetParams(ParamMaxBodyBytes, maxBodyBytes)
	}

	serverCh := bootstrap.Bind(&net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: port}).Sync().Channel()
	t.Cleanup(func() { serverCh.Close().Sync() })
	return fmt.Sprintf("127.0.0.1:%d", port)
}

// rawExchange writes a raw HTTP/1.1 request and returns the response status without sending more body bytes.
func rawExchange(t *testing.T, addr string, request string) (status int) {
	t.Helper()
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	require.NoError(t, err)
	defer conn.Close()

	require.NoError(t, conn.SetDeadline(time.Now().Add(3*time.Second)))
	_, err = io.WriteString(conn, request)
	require.NoError(t, err)
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	require.NoError(t, err, "the server must answer without waiting for the rest of the body")
	defer resp.Body.Close()
	return resp.StatusCode
}

func TestServerChannelBodyLimit(t *testing.T) {
	tests := []struct {
		name       string
		param      any
		size       int
		chunked    bool
		wantStatus int
		wantBody   string
	}{
		{name: "default bounds chunked overflow", size: testDefaultBodyLimit + 1, chunked: true, wantStatus: http.StatusRequestEntityTooLarge},
		{name: "default accepts exact fixed limit", size: testDefaultBodyLimit, wantStatus: http.StatusOK, wantBody: fmt.Sprint(testDefaultBodyLimit)},
		{name: "default accepts exact chunked limit", size: testDefaultBodyLimit, chunked: true, wantStatus: http.StatusOK, wantBody: fmt.Sprint(testDefaultBodyLimit)},
		{name: "configured bounds fixed overflow", param: 10, size: 11, wantStatus: http.StatusRequestEntityTooLarge},
		{name: "configured accepts exact limit", param: 10, size: 10, wantStatus: http.StatusOK, wantBody: "10"},
		{name: "larger configured limit is honored", param: int64(8 << 20), size: testDefaultBodyLimit + 1, wantStatus: http.StatusOK, wantBody: fmt.Sprint(testDefaultBodyLimit + 1)},
		{name: "explicit zero preserves optout", param: int64(0), size: testDefaultBodyLimit + 1, chunked: true, wantStatus: http.StatusOK, wantBody: fmt.Sprint(testDefaultBodyLimit + 1)},
		{name: "empty request reaches handler", wantStatus: http.StatusOK, wantBody: "0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &bodyEchoTask{}
			addr := startTestServer(t, tt.param, NewSimpleRoute().SetEndpoint("/upload", task), false)
			var body io.Reader = bytes.NewReader(bytes.Repeat([]byte{'x'}, tt.size))
			if tt.chunked {
				body = io.MultiReader(body)
			}

			resp, err := http.Post("http://"+addr+"/upload", "application/octet-stream", body)
			require.NoError(t, err)
			defer resp.Body.Close()
			got, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			require.Equal(t, tt.wantStatus, resp.StatusCode)
			if tt.wantStatus != http.StatusOK {
				require.Zero(t, task.calls.Load(), "rejected bodies must not reach the task")
				return
			}

			require.Equal(t, tt.wantBody, string(got), "Body and Request().Body must both see the whole body")
		})
	}
}

func TestServerChannelBodyRejectedBeforeReading(t *testing.T) {
	task := &bodyEchoTask{}
	addr := startTestServer(t, 10, NewSimpleRoute().SetEndpoint("/upload", task), false)

	status := rawExchange(t, addr, "POST /upload HTTP/1.1\r\nHost: test\r\nContent-Length: 1000000\r\n\r\n")

	require.Equal(t, http.StatusRequestEntityTooLarge, status, "a declared overflow is rejected without reading the body")
	require.Zero(t, task.calls.Load())
}

func TestServerChannelUnreadableBodyIsBadRequest(t *testing.T) {
	task := &bodyEchoTask{}
	addr := startTestServer(t, nil, NewSimpleRoute().SetEndpoint("/upload", task), false)

	status := rawExchange(t, addr, "POST /upload HTTP/1.1\r\nHost: test\r\nTransfer-Encoding: chunked\r\n\r\nzz\r\n")

	require.Equal(t, http.StatusBadRequest, status)
	require.Zero(t, task.calls.Load())
}

type countingHandler struct {
	channel.DefaultHandler
	reads atomic.Int32
}

func (h *countingHandler) Read(ctx channel.HandlerContext, obj any) {
	h.reads.Add(1)
	obj.(*Pack).Response.SetStatusCode(http.StatusOK)
	ctx.Write(obj, nil).Sync()
}

// TestServerChannelRejectsUnreadableBodyBeforeOtherHandlers covers handlers that read the pack themselves:
// a pipeline without DispatchHandler, and one where DispatchHandler passes a non-HTTP task on.
func TestServerChannelRejectsUnreadableBodyBeforeOtherHandlers(t *testing.T) {
	pipelines := map[string]func(ch channel.Channel, handler channel.Handler){
		"without DispatchHandler": func(ch channel.Channel, handler channel.Handler) {
			ch.Pipeline().AddLast("HANDLER", handler)
		},
		"after DispatchHandler": func(ch channel.Channel, handler channel.Handler) {
			ch.Pipeline().AddLast("DISPATCHER", NewDispatchHandler(NewSimpleRoute().SetEndpoint("/", NewDefaultHandlerTask())))
			ch.Pipeline().AddLast("HANDLER", handler)
		},
	}

	tests := []struct {
		name       string
		request    string
		wantStatus int
		wantReads  int32
	}{
		{name: "declared overflow", request: "POST / HTTP/1.1\r\nHost: test\r\nContent-Length: 11\r\n\r\n01234567890", wantStatus: http.StatusRequestEntityTooLarge},
		{name: "malformed chunked body", request: "POST / HTTP/1.1\r\nHost: test\r\nTransfer-Encoding: chunked\r\n\r\nzz\r\n", wantStatus: http.StatusBadRequest},
		{name: "readable body", request: "POST / HTTP/1.1\r\nHost: test\r\nContent-Length: 2\r\n\r\nok", wantStatus: http.StatusOK, wantReads: 1},
	}

	for pipelineName, build := range pipelines {
		for _, tt := range tests {
			t.Run(pipelineName+"/"+tt.name, func(t *testing.T) {
				handler := &countingHandler{}
				addr := bindTestServer(t, 10, func(ch channel.Channel) { build(ch, handler) })

				status := rawExchange(t, addr, tt.request)

				require.Equal(t, tt.wantStatus, status)
				require.Equal(t, tt.wantReads, handler.reads.Load(), "only a readable body reaches the handler")
			})
		}
	}
}

type bodyReadingHandler struct {
	channel.DefaultHandler
	read atomic.Int64
}

func (h *bodyReadingHandler) Read(ctx channel.HandlerContext, obj any) {
	n, _ := io.Copy(io.Discard, obj.(*Pack).Request.Request().Body)
	h.read.Store(n)
	ctx.FireRead(obj)
}

type bodyLengthTask struct {
	DefaultHTTPHandlerTask
}

func (t *bodyLengthTask) Post(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) (errResponse ErrorResponse) {
	resp.TextResponse(buf.NewByteBufString(fmt.Sprint(req.Body().ReadableBytes())))
	return nil
}

func TestServerChannelBodyReadBeforeDispatcherStaysReadable(t *testing.T) {
	reader := &bodyReadingHandler{}
	addr := bindTestServer(t, nil, func(ch channel.Channel) {
		ch.Pipeline().AddLast("BODY_READER", reader)
		ch.Pipeline().AddLast("DISPATCHER", NewDispatchHandler(NewSimpleRoute().SetEndpoint("/upload", &bodyLengthTask{})))
	})

	resp, err := http.Post("http://"+addr+"/upload", "text/plain", strings.NewReader("hello"))
	require.NoError(t, err)
	defer resp.Body.Close()
	got, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	require.Equal(t, int64(5), reader.read.Load())
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "5", string(got), "the task must still see the whole body")
}

func TestServerChannelRoutesBeforeReadingBody(t *testing.T) {
	addr := startTestServer(t, nil, NewSimpleRoute().SetEndpoint("/upload", &bodyEchoTask{}), false)

	status := rawExchange(t, addr, "POST /missing HTTP/1.1\r\nHost: test\r\nContent-Length: 1000000\r\n\r\n")

	require.Equal(t, http.StatusNotFound, status, "unknown routes are answered without buffering the body")
}

func TestServerChannelStreamingTaskReadsUnbufferedBody(t *testing.T) {
	tests := []struct {
		name       string
		size       int
		printBody  bool
		wantStatus int
	}{
		{name: "streams a body within the limit", size: testDefaultBodyLimit, wantStatus: http.StatusOK},
		{name: "limit still applies to streamed bodies", size: testDefaultBodyLimit + 1, wantStatus: http.StatusRequestEntityTooLarge},
		{name: "limit survives a body buffered before the task", size: testDefaultBodyLimit + 1, printBody: true, wantStatus: http.StatusRequestEntityTooLarge},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &bodyStreamTask{entered: make(chan struct{})}
			addr := startTestServer(t, nil, NewSimpleRoute().SetEndpoint("/stream", task), tt.printBody)
			reader, writer := io.Pipe()
			go func() {
				_, _ = writer.Write([]byte{'x'})
				// A printing LogHandler buffers the body before routing, so only the unbuffered path can wait.
				if !tt.printBody {
					select {
					case <-task.entered:
					case <-time.After(3 * time.Second):
						writer.CloseWithError(errors.New("task did not start before the body was complete"))
						return
					}
				}

				_, _ = io.Copy(writer, strings.NewReader(strings.Repeat("x", tt.size-1)))
				writer.Close()
			}()

			resp, err := http.Post("http://"+addr+"/stream", "application/octet-stream", reader)
			require.NoError(t, err)
			defer resp.Body.Close()
			got, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			require.Equal(t, tt.wantStatus, resp.StatusCode)
			if tt.wantStatus == http.StatusOK {
				require.Equal(t, fmt.Sprint(tt.size), string(got))
			}
		})
	}
}
