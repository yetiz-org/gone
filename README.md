# Gone

Gone is a Go networking toolkit built around a `channel` pipeline model. It provides reusable building blocks for HTTP, TCP, UDP, and WebSocket applications.

This README focuses on how to use the project. If you want to start a service, begin with `ghttp`, `gtcp/simpletcp`, `gudp/simpleudp`, or `gws`. Use the lower-level `channel` package only when you need custom protocol handling, custom codecs, or direct lifecycle control.

## Installation

```bash
go get github.com/yetiz-org/gone
```

The project currently targets Go 1.27.

## Package Layout

```text
channel             Core Channel, Pipeline, Handler, Future, and I/O lifecycle
ghttp               HTTP server, routing, handler tasks, gzip, logging, SSE, static files
gws                 WebSocket channel, upgrade processor, and message handler tasks
gmcp                MCP server that exposes ghttp handler functions as tools
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

### Graceful Shutdown

`server.Close()` drains before it closes the child channels: the listener stops accepting at once, idle keep-alive connections close, and requests in flight keep running and write their responses. After `ghttp.ParamShutdownTimeout` seconds (default 5; a nonpositive value uses the default), the connections still serving are closed and their responses are lost. Long streams such as SSE keep the drain open until that timeout, and `net/http` keeps a connection that has not sent its first request for up to 5 seconds, so either can delay Close. Close waits for HTTP responses only: goroutines a handler starts on its own are not waited for, and hijacked connections such as upgraded WebSockets are closed when the drain ends. Set the timeout below the time your platform allows between its stop signal and a forced kill.

### Request Bodies

`ghttp.ParamMaxBodyBytes` limits each request body (default 4 MiB; a nonpositive value disables the limit). The body is not read until it is needed. `DispatchHandler` reads it into memory after routing and before the acceptances run, and answers `413` when it exceeds the limit or `400` when it cannot be read; a declared `Content-Length` over the limit is rejected without reading the body, and unknown routes are answered without reading it. `Request.Body()` and `Request().Body` then share one buffered copy, so treat `Body()` as read-only: do not write, `Reset`, `Clear` or `Compact` it, or change the slice from `Body().Bytes()` in place. A pipeline without `DispatchHandler` keeps the earlier behavior: the server reads the body before the pipeline and answers `413` or `400` itself.

A task that implements `ghttp.RequestBodyStreamer` and returns true reads `Request().Body` as a stream instead, for example to spool a large upload to disk without holding it in memory. The limit still applies, so the task handles the `*http.MaxBytesError`; `Request.ContentLength()` is the declared size, and a chunked body's size is known only after reading it. Calling `Request.Body()` still buffers the body on demand: after part of the stream was read it returns only the rest, and a printing `LogHandler` (`NewLogHandler(true)`) buffers every body before routing. When buffering fails, `Request().Body` returns the same error on every read.

Buffered bodies hold up to the limit per request, as Netty's `HttpObjectAggregator` does per message, so peak memory is about the number of concurrent buffered requests times `ParamMaxBodyBytes`; size the limit and the server's concurrency together. A handler placed before `DispatchHandler` may read `Request().Body`: the first read buffers the body, so `Body()` still returns all of it, and a streaming task then reads that buffered copy.

### Sessions, Client Address and Logs

- Session cookies are `HttpOnly` by default (`ghttp.SessionHttpOnly`). They are `Secure` when `ghttp.SessionSecure` is set, the request arrived over TLS, or a peer accepted by `ghttp.TrustForwardedFor` forwarded it with `X-Forwarded-Proto: https`.
- `Request.RenewSession()` deletes the previous session from the store. With the memory provider, a concurrent request still holding the previous session cannot store it again. A provider that loads a separate copy per request, such as Redis, can still have a concurrent request store its copy under the previous ID again, including any credentials that copy already held; preventing that needs the provider to refuse saves for deleted IDs.
- `DefaultSession` is safe for concurrent requests sharing one session. `Data()` returns the live map, which is not synchronized; when other requests may write the same session, for example while rendering a template, read `Snapshot()` instead, which returns a copy taken under the session lock. The memory provider keeps at most `MaxSessions` sessions (default 100,000); going over it drops expired sessions and then the least recently saved ones until a tenth of the cap is free. Saves running concurrently can briefly exceed the cap until the next save evicts again. Traffic that keeps creating sessions can therefore end idle sessions early; use a shared store such as Redis, or rate-limit session creation, when that matters.
- `WrapRequest` takes the client address from `X-Forwarded-For` only when `ghttp.TrustForwardedFor` accepts the direct peer; by default, loopback, private and other non-public peers such as a load balancer in the same network. Any host in those networks can then set the header, and so can outside clients that reach the server through NAT, such as Docker port publishing or a Kubernetes SNAT, because they arrive from a private address. Replace it at startup with your proxies' addresses for a stricter boundary, or to trust a public proxy.
- `LogHandler` logs the headers in `RedactHeaders` (by default `Authorization`, `Proxy-Authorization`, `Cookie` and `Set-Cookie`) as `[REDACTED]`; clear it to log every value.

### Connection Limits and Static Files

- `gtcp` and `ghttp` servers hold at most `channel.ParamMaxConnections` connections at once (default 10,000; nonpositive disables it), upgraded WebSockets included. Further connections wait in the listen backlog until one closes. Keep the process descriptor limit (`ulimit -n`) above it.
- Connections a server accepts close after receiving nothing for `channel.ParamReadIdleTimeout` milliseconds (default 300,000; nonpositive disables it). Client connections default to off. A `gtcp` channel checks it every `channel.ParamReadTimeout`; a WebSocket must receive each whole message, ping or pong within it, so a server that only pushes should ping its clients more often than that. A gUDP child ignores it and uses `gudp.ParamPeerIdleTimeout`. HTTP keep-alive connections use `ghttp.ParamIdleTimeout` instead.
- `StaticFilesHandlerTask` rejects paths that leave `FolderPath` but follows symlinks inside it, so keep untrusted writers out of that folder.

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

### HTTP RawMode

`RawMode` returns the underlying `http.ResponseWriter` and, once it returns `ok`, ghttp writes nothing else for that response, so gzip and response logging do not run. `RawMode` does not combine with `SSEMode`: it returns `ok == false` after `SSEMode` starts, `SSEMode` must not be called after `RawMode`, and long-lived streams should extend the write deadline with `http.NewResponseController(writer).SetWriteDeadline`.

```go
writer, ok := t.RawMode(req, resp, params)
if ok {
	t.handler.ServeHTTP(writer, req.Request()) // any net/http handler
}
```

### HTTP Internal Dispatch

`DispatchHandler.Dispatch` serves an `*http.Request` in process through the same route, acceptances and handler task, and returns the `*Pack` instead of writing to the network. `seed` is copied into the params before acceptances run, so callers can pass trusted values that a network request cannot set. The pack has no writer: no cookie is issued, the session stays in a private store, `RawMode` and `SSEMode` are refused, and `CORSHelper` and `DefaultStatusResponse` are skipped. The caller bounds the request body.

```go
pack := dispatcher.Dispatch(ctx, httpRequest, map[string]any{"origin": "internal"})
status, body := pack.Response.StatusCode(), pack.Response.Body().Bytes()
```

## MCP

`gmcp` serves Model Context Protocol tools that forward to `ghttp` handler functions. A handler task declares one tool per REST function with `MCPIndex`, `MCPGet`, `MCPPost`, `MCPPut`, `MCPPatch`, or `MCPDelete`. Each call is dispatched in process (see HTTP Internal Dispatch) as the request a REST client would send, so routes, acceptances, and handlers keep owning permissions, validation, and responses.

Declare the tool input once with `json` and `gmcp` tags. Each top-level field has exactly one placement: `path` (the endpoint's own ID), `path=<ancestor node>`, `query=<name>`, `body`, or `file=<form field>`. The one `body` field is a struct, slice, or array, sent as the JSON request body; for a JSON array body, the element fields take the same tags as other body fields. Input fields, including those nested in the body, also accept the schema keys `description`, `example`, `format`, `deprecated`, `enum`, `minLength`, `maxLength`, `pattern`, `minimum`, `maximum`, `minItems`, `maxItems`, and `uniqueItems`; `items.` applies the value keys to array elements. A field is optional when its json tag has `omitempty` or `omitzero` or it is a pointer. `gmcp.ID` values must match `Options.IDPattern`, and `gmcp.Date` is `YYYY-MM-DD`. A type that implements `gmcp.JSONSchemaProvider` (`JSONSchema() *jsonschema.Schema`, returning a new schema on every call) describes its own input schema, such as an optional value whose schema is the value schema plus `null`: every input field of the type, in the body too, takes that schema as is, keeping `null` without describing the type's fields, while the field's tags and the optional rule still apply. Output schemas do not use it.

The output schema comes from `Out`. Its fields accept only `description`, `example`, `deprecated`, `items.description`, and `items.example`, because every result is validated against the output schema: null and the inferred required list are kept, and `gmcp.ID` is not pattern-checked. Without `gmcp` tags the output schema is the one the SDK infers. To return something the REST response does not carry, pass `gmcp.WithOutputAdjuster(adjust)` to `NewTool`, where `adjust` is a `gmcp.OutputAdjuster[In, Out]`: `func(ctx context.Context, input In, output Out) (Out, error)`. It runs only after a successful REST call, on the output decoded from the response, so routes, acceptances, and handlers still decide whether the call succeeds; several adjusters run in the order given, the result is validated against the output schema of `Out`, and a returned error fails the call with `internal_error`. A handler usually passes one of its own methods, so the adjuster can compute the value from the input and the handler's dependencies, as `Items.MCPIndex` in the example below does.

Binary content travels as `gmcp.DataURL`, one RFC 2397 data URL string in base64 form such as `data:image/png;base64,iVBORw0KGgo=`. Its media type needs a type and subtype, may carry parameters (`data:text/plain;charset=utf-8;base64,...`), and is kept as written. Only lowercase `data:` and `;base64` are accepted, the base64 needs its padding, and percent-encoded data URLs are rejected. A `file=<form field>` field is a required `gmcp.File` or an optional `*gmcp.File`, sent as `{"filename": "cover.png", "data": "data:image/png;base64,..."}`. A tool with file fields forwards `multipart/form-data` with one part per file field, named by the tag, carrying the filename and the data URL media type as its `Content-Type`. File fields cannot be used by GET tools, together with `body`, or inside the body. A call fails with `invalid_argument` when a filename is empty, `.`, or `..`, or contains `/`, `\`, or a control character, or when the data URL is malformed or empty; the REST handler still validates size and type. `Options.MaxBodyBytes` limits the encoded multipart body as it limits JSON bodies, and `Options.MaxRequestBodyBytes` limits each MCP request (zero keeps the SDK default of 4 MiB, a negative value disables the limit); base64 makes a request about 4/3 of the file size.

When `Out` is `gmcp.Blob`, the forwarded request accepts `*/*` and a successful response body is returned as is: `{"filename": "report.pdf", "data": "data:application/pdf;base64,..."}`. The data URL takes the response `Content-Type`, or `application/octet-stream` when it is missing or invalid, and `filename` comes from a `Content-Disposition` file name that passes the same file name rules, and is omitted otherwise. The result content is one text summary, `{"media_type": ..., "filename": ...}`, so the base64 data is sent only once, in the structured content; output adjusters run before that summary is built. Any other `Out` still fails with `internal_error` on a non-JSON response.

Tags follow the goai tag syntax, so values cannot contain `;`. An `example` is parsed as the field type (JSON for numbers, booleans, arrays, and objects). `default` is not accepted because the SDK would write it into arguments and results.

```go
type ItemGetToolRequest struct {
	Org  gmcp.ID  `json:"org" gmcp:"path=orgs;description=Organization ID."`
	Item gmcp.ID  `json:"item" gmcp:"path;description=Item ID."`
	Tags []string `json:"tags,omitempty" gmcp:"query=tags;maxItems=5;uniqueItems;items.example=new"`
}

type ItemGetResponse struct {
	Name  string `json:"name"`
	Price int64  `json:"price" gmcp:"description=Price in cents.;example=1250"`
}

func (h *Items) MCPGet() gmcp.Tool {
	return gmcp.NewTool[ItemGetToolRequest, ItemGetResponse](&mcp.Tool{Name: "item_get", Description: "Read one item."})
}

type ItemsLinkResponse struct {
	URL string `json:"url" gmcp:"description=Web page that lists the items."`
}

func (h *Items) MCPIndex() gmcp.Tool {
	return gmcp.NewTool[struct{}, ItemsLinkResponse](&mcp.Tool{Name: "items_link", Description: "Get the items page link."},
		gmcp.WithOutputAdjuster(h.itemsLink))
}

// itemsLink runs after GET /items succeeds and replaces its body with the page link.
func (h *Items) itemsLink(ctx context.Context, input struct{}, output ItemsLinkResponse) (ItemsLinkResponse, error) {
	output.URL = h.siteURL + "/items"
	return output, nil
}

type CoverPostToolRequest struct {
	Org   gmcp.ID   `json:"org" gmcp:"path=orgs;description=Organization ID."`
	Cover gmcp.File `json:"cover" gmcp:"file=file;description=Cover image, JPEG or PNG."`
}

func (h *Cover) MCPPost() gmcp.Tool {
	return gmcp.NewTool[CoverPostToolRequest, CoverPostResponse](&mcp.Tool{Name: "cover_upload", Description: "Upload a cover image."})
}

func (h *Cover) MCPGet() gmcp.Tool {
	return gmcp.NewTool[CoverGetToolRequest, gmcp.Blob](&mcp.Tool{Name: "cover_download", Description: "Download a cover image."})
}

server := gmcp.New(&mcp.Implementation{Name: "app", Version: "1.0.0"}, gmcp.Options{ToolParam: "mcp_tool"})
server.Bind(dispatcher, route.RouteEntries()) // once, after the route is complete

// In the authenticated /mcp handler:
if writer, ok := h.RawMode(req, resp, params); ok {
	server.Serve(writer, gmcp.Caller{Request: req, HandlerContext: ctx})
}
```

`Bind` panics on an invalid declaration or tag, a duplicate name, or a tool that `Options.Allows` rejects. The HTTP method decides the tool annotations: GET is read-only, writes are destructive, and PUT and DELETE are idempotent. Every tool is closed-world unless its definition sets `Annotations.OpenWorldHint`, which Bind keeps for a tool whose effects reach external systems; Bind replaces every other annotation. Only GET polls again on `202 Accepted`. A failed call is a result with `isError` whose only content is the text of one error object, without `structuredContent`, because a client may validate structured content against the output schema even for an error. The error object always has a `category` from the HTTP status: `permission_denied` (401, 403), `not_found` (404), `conflict` (409), `temporarily_unavailable` (429, 503, 529), `invalid_argument` (other 4xx and input validation), or `internal_error`. When the error response body is a JSON object, the object also keeps its six-digit string `error_code` as `code` and its string `title` and `detail`, the fields of `erresponse.DefaultErrorResponse` that identify the error to a client and an end user, such as `{"error":{"category":"invalid_argument","code":"400103","title":"...","detail":"..."}}`; nothing else of the body is returned. JSON-RPC errors keep their code with a fixed message and carry only `{"category":...}` as data. The MCP header mismatch (-32020), missing client capabilities (-32021), and unsupported protocol version (-32022) errors are `invalid_argument`; the last two also keep the `requiredCapabilities` object or the `supported` and `requested` versions the specification requires, so a client can retry. `Options.Gateway` lists only its public tools plus `tools` (discovery, including one tool's input and output schemas) and `query` (call by name); `query` is open-world when any hidden tool is. `Caller.ToolVisible`, when set, filters `tools/list` per request to the bound tools it shows and marks the list cache scope `private`; with `Options.Gateway` it filters the listed public tools and the tools found by `tools`. It does not authorize calls: `tools/call` and `query` still forward every bound tool for the REST route to authorize. Path IDs use the default node-name parameter keys; routes with custom `{param}` or `:param_id` names are not supported.

## TCP

For framed messages, prefer `gtcp/simpletcp`. It includes a varint length-prefixed codec: outbound `buf.ByteBuf` values are framed automatically, and inbound frames are decoded back to complete messages.

`simpletcp.MaxFrameLength` limits each payload to 16 MiB. Oversized declarations are rejected before reading the body; invalid framing or exhausted decoder input budgets clear the buffered input and close the channel. Oversized writes fail their future. Valid frames may span reads or share a read, and zero-length keepalives remain supported.

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

`gudp/simpleudp` provides a similar client/server wrapper for UDP. The UDP server is the only reader of its socket and serves each source address through one child channel, so UDP packets can use the same handler pipeline model. `gudp.ParamMaxPeers` limits the addresses served at once (default 4096; datagrams from new addresses are dropped at the limit), and `gudp.ParamPeerIdleTimeout` closes a child after that many milliseconds without a datagram in either direction (default 60,000); the address's next datagram opens a new child. Each child queues at most 32 datagrams and all children of a server at most 64 MiB; datagrams beyond that are dropped. Each written `ByteBuf` leaves as one datagram.

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

Each `simpleudp` datagram must contain complete frames. `simpleudp.MaxFrameLength` limits each payload to 65,504 bytes, leaving room for its three-byte prefix within the 65,507-byte datagram budget. Oversized writes fail their future. Incomplete or oversized incoming datagrams report an error and are discarded with decoder state reset; later datagrams are decoded independently. Set `channel.ParamReadBufferSize` large enough for the datagrams your application accepts, because UDP reads discard bytes beyond the read buffer.

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

In stream mode, `channel.ReplayDecoder` closes the channel and reports `channel.ErrDecoderBufferLimit` when more than `MaxBufferedBytes` (default 16 MiB; non-positive values select the default) stay unread after decoding, so one read can still carry complete messages larger in total. A decode error other than incomplete input also closes it. `Datagram` mode reports these errors without closing the channel, resets framing state after each read, and rejects incomplete input instead of joining packets.

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
- `channel.ParamReadIdleTimeout`: close an accepted connection after receiving nothing for this many milliseconds (default 300,000; nonpositive disables it).
- `channel.ParamMaxConnections`: connections a `gtcp` or `ghttp` server holds open at once (default 10,000; nonpositive disables it).
- `gws.ParamCheckOrigin`: whether WebSocket upgrade checks request origin.
- `gws.ParamWSReadLimit`: largest incoming WebSocket message in bytes (default 16 MiB; nonpositive disables it). Each connection can hold one such message, so lower it when many connections are open. The default ping handler waits for each pong to be written, so a peer that does not read pongs stalls only its own reads.
- `ghttp.ParamMaxBodyBytes`: largest HTTP request body in bytes (default 4 MiB; nonpositive disables it).
- `ghttp.ParamShutdownTimeout`: seconds closing an HTTP server waits for requests in flight before closing their connections (default 5; nonpositive uses the default).
- `gudp.ParamMaxPeers` and `gudp.ParamPeerIdleTimeout`: UDP server address limit and idle timeout in milliseconds.

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
