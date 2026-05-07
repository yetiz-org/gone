# Gone

Gone is a Go networking toolkit built around a `channel` pipeline model. It provides reusable building blocks for HTTP, TCP, UDP, and WebSocket applications.

This README focuses on how to use the project. If you want to start a service, begin with `ghttp`, `gtcp/simpletcp`, `gudp/simpleudp`, or `gws`. Use the lower-level `channel` package only when you need custom protocol handling, custom codecs, or direct lifecycle control.

## Installation

```bash
go get github.com/yetiz-org/gone
```

The project currently targets Go 1.26.

## Package Layout

```text
channel             Core Channel, Pipeline, Handler, Future, and I/O lifecycle
ghttp               HTTP server, routing, handler tasks, gzip, logging, SSE, static files
gws                 WebSocket channel, upgrade processor, and message handler tasks
gtcp                TCP channel and server channel
gtcp/simpletcp      TCP client/server wrapper with a built-in length-prefixed codec
gudp                UDP channel and server channel
gudp/simpleudp      UDP client/server wrapper with a built-in length-prefixed codec
erresponse          HTTP error response definitions
utils               Buffer pools, varint helpers, and shared utilities
mock                Test doubles for channels and handlers
example             HTTP, TCP, UDP, and WebSocket examples
```

## Channel Model

Gone's `channel` package follows a Netty-like pipeline model. Each connection is a `Channel`; each `Channel` owns a `Pipeline`; each `Pipeline` is a linked chain of `Handler` instances.

```text
Inbound events:
head -> handler A -> handler B -> handler C -> tail

Outbound events:
tail -> handler C -> handler B -> handler A -> head -> Unsafe I/O
```

Inbound events are produced by network reads or framework entry points:

- `Registered`: the channel has been registered into its pipeline.
- `Active`: the channel is usable, usually after bind or connect succeeds.
- `Read`: an object was received. The object may be a `buf.ByteBuf`, an HTTP `Pack`, or a WebSocket `Message`.
- `ReadCompleted`: the current read batch has ended. It is not automatically emitted after every `Read`; it must be emitted by the low-level read loop, a decoder, or a protocol-specific channel.
- `Inactive` and `Unregistered`: the channel has closed and left the active lifecycle.

Outbound events usually start from user code:

- `ch.Write(obj)`: writes an object through the outbound pipeline.
- `ch.Connect(local, remote)`: opens a client connection.
- `ch.Disconnect()` or `ch.Close()`: closes a channel or server.

`Added` and `Removed` are pipeline mutation hooks. `Added` runs when a handler is inserted with `AddLast` or `AddBefore`. It is useful for initializing handler-local state, such as default codec functions, gzip thresholds, or a WebSocket upgrader.

## Writing Handlers

Most user handlers should embed `channel.DefaultHandler` and override only the events they need. Events you do not override keep the default pass-through behavior.

```go
package main

import (
	"fmt"

	"github.com/yetiz-org/gone/channel"
	buf "github.com/yetiz-org/goth-bytebuf"
)

type EchoHandler struct {
	channel.DefaultHandler
}

func (h *EchoHandler) Active(ctx channel.HandlerContext) {
	fmt.Println("connected:", ctx.Channel().ID())
	ctx.FireActive()
}

func (h *EchoHandler) Read(ctx channel.HandlerContext, obj any) {
	if b, ok := obj.(buf.ByteBuf); ok {
		ctx.Write(b, nil)
		return
	}
	ctx.FireRead(obj)
}

func (h *EchoHandler) Inactive(ctx channel.HandlerContext) {
	fmt.Println("disconnected:", ctx.Channel().ID())
	ctx.FireInactive()
}
```

Handler rules:

- Call `ctx.FireRead(obj)` when an inbound object should continue to the next handler.
- Call `ctx.Write(obj, future)` when an outbound object should continue toward the transport.
- Stop propagation only when the handler intentionally consumes the event.
- Do not write directly to the socket from a normal handler. Low-level I/O belongs in `UnsafeRead`, `UnsafeWrite`, and channel implementations.

## HTTP Server

HTTP servers use `ghttp.ServerChannel`. Each request is wrapped into a `*ghttp.Pack` and fired into the child channel pipeline. A typical HTTP pipeline ends with `ghttp.DispatchHandler`, which routes the request to a `HandlerTask`.

```go
package main

import (
	"net"

	"github.com/yetiz-org/gone/channel"
	"github.com/yetiz-org/gone/ghttp"
	"github.com/yetiz-org/gone/ghttp/httpstatus"
	buf "github.com/yetiz-org/goth-bytebuf"
)

type HelloTask struct {
	ghttp.DefaultHTTPHandlerTask
}

func (h *HelloTask) Get(ctx channel.HandlerContext, req *ghttp.Request, resp *ghttp.Response, params map[string]any) ghttp.ErrorResponse {
	resp.SetStatusCode(httpstatus.OK)
	resp.TextResponse(buf.NewByteBufString("hello gone"))
	return nil
}

func main() {
	route := ghttp.NewRoute()
	route.SetRoot(ghttp.NewEndPoint("", &HelloTask{}, nil))

	bootstrap := channel.NewServerBootstrap()
	bootstrap.ChannelType(&ghttp.ServerChannel{})
	bootstrap.ChildHandler(channel.NewInitializer(func(ch channel.Channel) {
		ch.Pipeline().
			AddLast("GZIP", &ghttp.GZipHandler{}).
			AddLast("LOG", ghttp.NewLogHandler(false)).
			AddLast("DISPATCH", ghttp.NewDispatchHandler(route))
	}))

	server := bootstrap.Bind(&net.TCPAddr{Port: 8080}).Sync().Channel()
	server.CloseFuture().Sync()
}
```

## HTTP Routing

`ghttp.Route` maps a URL path to a `HandlerTask`. Gone provides two routing styles:

- `ghttp.NewRoute()` uses explicit node builders such as `NewEndPoint` and `NewGroup`.
- `ghttp.NewSimpleRoute()` uses path strings and is usually easier for application code.

### Builder Route

Use the builder route when you want explicit route nodes and nested route construction.

```go
route := ghttp.NewRoute()
route.
	SetRoot(ghttp.NewEndPoint("", &HomeTask{}, nil)).
	AddEndPoint(ghttp.NewEndPoint("users", &UsersTask{}, nil)).
	AddGroup(ghttp.NewGroup("v1", nil).
		AddEndPoint(ghttp.NewEndPoint("status", &StatusTask{}, nil)))
```

### SimpleRoute

Use `SimpleRoute` when you prefer registering routes by path.

```go
route := ghttp.NewSimpleRoute()
route.
	SetRoot(&HomeTask{}).
	SetEndpoint("/users", &UsersTask{}).
	SetEndpoint("/users/{user_id}", &UserDetailTask{}).
	SetEndpoint("/assets/*", &AssetTask{}).
	SetGroup("/v1").
	SetEndpoint("/v1/status", &StatusTask{})
```

Parameter access is handled through `ghttp.DefaultHandlerTask` helpers:

```go
type UserDetailTask struct {
	ghttp.DefaultHTTPHandlerTask
}

func (h *UserDetailTask) Get(ctx channel.HandlerContext, req *ghttp.Request, resp *ghttp.Response, params map[string]any) ghttp.ErrorResponse {
	userID := h.GetID("user_id", params)
	resp.TextResponse(buf.NewByteBufString(userID))
	return nil
}
```

`SimpleRoute` supports these path forms:

- `/users`: fixed path.
- `/users/{user_id}`: named parameter stored for `GetID("user_id", params)`.
- `/users/:user_id`: colon parameter form.
- `/assets/*`: wildcard recursive endpoint, with the wildcard value stored under `"*"`.

Common HTTP task methods:

- `Get`: handles normal GET requests.
- `Head`: handles HEAD requests.
- `Index`: for GET requests that hit the final index node. If it returns `NotImplemented`, dispatch falls back to `Get`.
- `Post` and `Create`: for POST requests. On the final index node, `Create` is tried before `Post`.
- `Put`, `Patch`, `Delete`, `Options`, `Trace`, and `Connect`: handle their matching HTTP methods.
- `PreCheck`, `Before`, `After`, and `ErrorCaught`: request lifecycle hooks.

### HTTP Gzip

`ghttp.GZipHandler` is an outbound handler. Put it before `DispatchHandler` so the response can be compressed before it is written.

It compresses only when the client accepts gzip, the response body reaches the threshold, and the response content type is suitable. It skips already encoded responses, range responses, and SSE separate-write mode.

```go
ch.Pipeline().
	AddLast("GZIP", &ghttp.GZipHandler{CompressThreshold: 1024}).
	AddLast("DISPATCH", ghttp.NewDispatchHandler(route))
```

## TCP

For framed messages, prefer `gtcp/simpletcp`. It includes a varint length-prefixed codec: outbound `buf.ByteBuf` values are framed automatically, and inbound frames are decoded back to complete messages.

```go
package main

import (
	"fmt"
	"net"

	"github.com/yetiz-org/gone/channel"
	"github.com/yetiz-org/gone/gtcp/simpletcp"
	buf "github.com/yetiz-org/goth-bytebuf"
)

type ServerHandler struct {
	channel.DefaultHandler
}

func (h *ServerHandler) Read(ctx channel.HandlerContext, obj any) {
	msg := obj.(buf.ByteBuf)
	fmt.Println("server received:", string(msg.Bytes()))
	ctx.Write(buf.NewByteBufString("pong"), nil)
}

func main() {
	server := simpletcp.NewServer(&ServerHandler{})
	ch := server.Start(&net.TCPAddr{Port: 9000})
	ch.CloseFuture().Sync()
}
```

Client:

```go
client := simpletcp.NewClient(&ClientHandler{})
ch := client.Start(&net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 9000})
ch.Write(buf.NewByteBufString("ping")).Sync()
```

Use `channel.NewBootstrap()` with `gtcp.Channel` directly only when you need a fully custom pipeline.

## UDP

`gudp/simpleudp` provides a similar client/server wrapper for UDP. The UDP server creates a virtual child channel for each source address so UDP packets can use the same handler pipeline model.

```go
server := simpleudp.NewServer(&ServerHandler{})
ch := server.Start(&net.UDPAddr{Port: 9001})
defer server.Stop().Sync()

client := simpleudp.NewClient(&ClientHandler{})
client.Start(&net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 9001})
client.Write(buf.NewByteBufString("hello udp")).Sync()

ch.CloseFuture().Sync()
```

UDP is connectionless. The `simpleudp` virtual channel exists to reuse the pipeline abstraction; it does not provide TCP-style delivery guarantees.

## WebSocket

WebSocket servers are usually built on top of `ghttp.ServerChannel`. The HTTP pipeline first dispatches the route, then `gws.UpgradeProcessor` upgrades the connection into a `gws.Channel`.

```go
package main

import (
	"net"

	"github.com/yetiz-org/gone/channel"
	"github.com/yetiz-org/gone/ghttp"
	"github.com/yetiz-org/gone/gws"
)

type WSTask struct {
	gws.DefaultServerHandlerTask
}

func (h *WSTask) WSText(ctx channel.HandlerContext, message *gws.DefaultMessage, params map[string]any) {
	ctx.Write(h.Builder.Text("echo: "+message.StringMessage()), nil)
}

func main() {
	route := ghttp.NewSimpleRoute()
	route.
		SetRoot(ghttp.NewDefaultHandlerTask()).
		SetEndpoint("/ws", &WSTask{})

	bootstrap := channel.NewServerBootstrap()
	bootstrap.
		ChannelType(&ghttp.ServerChannel{}).
		SetParams(gws.ParamCheckOrigin, false)
	bootstrap.ChildHandler(channel.NewInitializer(func(ch channel.Channel) {
		ch.Pipeline().
			AddLast("DISPATCH", ghttp.NewDispatchHandler(route)).
			AddLast("WS_UPGRADE", &gws.UpgradeProcessor{})
	}))

	server := bootstrap.Bind(&net.TCPAddr{Port: 8081}).Sync().Channel()
	server.CloseFuture().Sync()
}
```

WebSocket client:

```go
bootstrap := channel.NewBootstrap()
bootstrap.ChannelType(&gws.Channel{})
bootstrap.Handler(channel.NewInitializer(func(ch channel.Channel) {
	ch.Pipeline().AddLast("WS_HANDLER", gws.NewInvokeHandler(&ClientWSTask{}, nil))
}))

ch := bootstrap.Connect(nil, &gws.WSCustomConnectConfig{
	Url: "ws://localhost:8081/ws",
}).Sync().Channel()

ch.Write((&gws.DefaultMessageBuilder{}).Text("hello")).Sync()
```

`gws.HandlerTask` can override:

- `WSText`
- `WSBinary`
- `WSPing`
- `WSPong`
- `WSClose`
- `WSConnected`
- `WSDisconnected`
- `WSErrorCaught`

## Buffers and Write Performance

Gone uses `github.com/yetiz-org/goth-bytebuf` as its main buffer abstraction.

Common buffer types:

- `buf.ByteBuf`: regular byte buffer.
- `buf.NewByteBufString("...")` and `buf.NewByteBuf([]byte{...})`: payload constructors.
- `buf.CompositeByteBuf`: combines multiple buffers without merging them into one slice first.

In `DefaultNetChannel.UnsafeWrite`, `ByteBuf` values that implement `io.WriterTo` write directly to the underlying `net.Conn`. `CompositeByteBuf` can use the underlying `net.Buffers.WriteTo` path on TCP and Unix sockets, which is useful for header + body writes.

```go
header := buf.NewByteBufString("header:")
body := buf.NewByteBufString("body")
ctx.Write(buf.NewCompositeByteBuf(header, body), nil)
```

## Common Parameters

Parameters can be set on bootstrap or channel objects. Channel implementations and handlers read them during initialization and execution.

```go
bootstrap.SetParams(channel.ParamReadBufferSize, 4096)
bootstrap.SetParams(channel.ParamReadTimeout, 1000)
bootstrap.SetParams(channel.ParamWriteTimeout, 100)
```

Common parameters:

- `channel.ParamReadBufferSize`: net channel read buffer size.
- `channel.ParamReadTimeout`: read timeout in milliseconds.
- `channel.ParamWriteTimeout`: write timeout in milliseconds.
- `channel.ParamAcceptTimeout`: server child-channel accept timeout.
- `gws.ParamCheckOrigin`: whether WebSocket upgrade checks request origin.

## Testing

Run the full suite:

```bash
go test ./...
```

Run race checks for the core packages:

```bash
go test -race ./channel ./ghttp ./gtcp ./gtcp/simpletcp ./gudp ./gudp/simpleudp ./gws ./utils
```

Run examples:

```bash
go test ./example/...
```

## Usage Guidelines

- Start with the highest-level package that fits the protocol: `ghttp` for HTTP, `gws` for WebSocket, and `simpletcp` or `simpleudp` for framed TCP/UDP.
- Build directly on `channel.NewBootstrap`, `ByteToMessageDecoder`, `ReplayDecoder`, or `MessageToByteEncoder` only when you need a custom protocol.
- Decide explicitly whether each handler should continue event propagation. Forgetting `ctx.FireRead` or `ctx.Write` stops the event at that handler.
- Do not assume `ReadCompleted` fires for every message. If your logic depends on batch completion, confirm that the preceding decoder or channel emits it.
- Put outbound HTTP handlers that modify response headers or bodies before `DispatchHandler`.
- For WebSocket upgrade, the usual order is `DispatchHandler` followed by `UpgradeProcessor`.

## License

See `LICENSE`, `LICENSE.KKLAB`, and `NOTICE`.
