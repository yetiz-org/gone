package ghttp

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/yetiz-org/gone/channel"
	buf "github.com/yetiz-org/goth-bytebuf"
)

// slowTask answers "done" once release is closed, after reporting that the request reached the handler.
type slowTask struct {
	DefaultHTTPHandlerTask
	entered chan struct{}
	release chan struct{}
}

func (t *slowTask) Get(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) (errResponse ErrorResponse) {
	close(t.entered)
	<-t.release
	resp.TextResponse(buf.NewByteBufString("done"))
	return nil
}

type shutdownResult struct {
	status int
	body   string
	err    error
}

func bindShutdownTestServer(t *testing.T, task HandlerTask, shutdownTimeout any) (serverCh channel.Channel, addr string) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	require.NoError(t, listener.Close())

	route := NewSimpleRoute()
	route.SetEndpoint("/slow", task)
	dispatcher := NewDispatchHandler(route)
	bootstrap := channel.NewServerBootstrap()
	bootstrap.ChildHandler(channel.NewInitializer(func(ch channel.Channel) { ch.Pipeline().AddLast("DISPATCHER", dispatcher) }))
	bootstrap.ChannelType(&ServerChannel{})
	if shutdownTimeout != nil {
		bootstrap.SetParams(ParamShutdownTimeout, shutdownTimeout)
	}

	serverCh = bootstrap.Bind(&net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: port}).Sync().Channel()
	t.Cleanup(func() { serverCh.Close().Sync() })
	return serverCh, fmt.Sprintf("127.0.0.1:%d", port)
}

func getSlow(addr string, results chan<- shutdownResult) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get("http://" + addr + "/slow")
	if err != nil {
		results <- shutdownResult{err: err}
		return
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	results <- shutdownResult{status: resp.StatusCode, body: string(body), err: err}
}

func TestServerChannelCloseDrainsRequestsInFlight(t *testing.T) {
	task := &slowTask{entered: make(chan struct{}), release: make(chan struct{})}
	serverCh, addr := bindShutdownTestServer(t, task, nil)
	results := make(chan shutdownResult, 1)
	go getSlow(addr, results)
	<-task.entered
	closed := make(chan struct{})
	go func() {
		serverCh.Close().Sync()
		close(closed)
	}()

	// The listener closes at once, so new connections are refused while the request in flight is served.
	require.Eventually(t, func() bool {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err != nil {
			return true
		}

		conn.Close()
		return false
	}, 2*time.Second, 20*time.Millisecond)
	select {
	case <-closed:
		t.Fatal("Close returned before the request in flight finished")
	case result := <-results:
		t.Fatalf("request ended before it was released: %+v", result)
	default:
	}

	close(task.release)
	result := <-results
	require.NoError(t, result.err)
	require.Equal(t, http.StatusOK, result.status)
	require.Equal(t, "done", result.body)
	select {
	case <-closed:
	case <-time.After(3 * time.Second):
		t.Fatal("Close did not return after the request finished")
	}
}

func TestServerChannelCloseEndsRequestsAfterShutdownTimeout(t *testing.T) {
	task := &slowTask{entered: make(chan struct{}), release: make(chan struct{})}
	t.Cleanup(func() { close(task.release) })
	serverCh, addr := bindShutdownTestServer(t, task, 1)
	results := make(chan shutdownResult, 1)
	go getSlow(addr, results)
	<-task.entered
	start := time.Now()
	serverCh.Close().Sync()
	elapsed := time.Since(start)
	require.GreaterOrEqual(t, elapsed, 900*time.Millisecond, "Close must wait for ParamShutdownTimeout")
	require.Less(t, elapsed, 3*time.Second, "Close must not wait past ParamShutdownTimeout")
	select {
	case result := <-results:
		require.Error(t, result.err, "a request still running after the timeout loses its connection")
	case <-time.After(3 * time.Second):
		t.Fatal("the connection stayed open after the shutdown timeout")
	}
}
