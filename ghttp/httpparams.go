package ghttp

import "github.com/yetiz-org/gone/channel"

var ParamIdleTimeout channel.ParamKey = "idle_timeout"
var ParamReadTimeout channel.ParamKey = "read_timeout"
var ParamReadHeaderTimeout channel.ParamKey = "read_header_timeout"
var ParamWriteTimeout channel.ParamKey = "write_timeout"
var ParamAcceptWaitCount channel.ParamKey = "accept_wait_count"
var ParamMaxHeaderBytes channel.ParamKey = "max_header_bytes"

// var ParamMaxMultiPartMemory channel.ParamKey = "max_multi_part_memory"

// ParamMaxBodyBytes limits buffered HTTP request bodies in bytes (default: 4 MiB).
// Nonpositive values disable the limit.
const ParamMaxBodyBytes = channel.ParamKey("max_body_bytes")

// ParamMaxInflightBodyBytes bounds the request body bytes that the requests a server is serving may hold at
// once (default: 0). Nonpositive values disable the bound. Each request with a body reserves its
// Content-Length until the server finishes serving it; a body of unknown length reserves ParamMaxBodyBytes,
// or the bound itself when that limit is disabled or larger, and is then limited to what it reserved. A body
// over ParamMaxBodyBytes reserves nothing, as it is rejected unread. Without reading the body, a server answers
// 413 when a Content-Length exceeds the bound and 503 with "Retry-After: 1" when the bound has no room left,
// adding "Connection: close" over HTTP/1. The bound covers every body the server receives, streamed ones
// included; it does not cover DispatchHandler.Dispatch, nor a body a request upgraded to a WebSocket keeps
// after the upgrade.
const ParamMaxInflightBodyBytes = channel.ParamKey("max_inflight_body_bytes")

// ParamLogURI is a child parameter holding a func(req *Request) string whose result is the request URI gone
// writes in its logs, in place of Request.RequestURI, so an application can keep credentials carried in a path
// or query out of logs. Unset logs Request.RequestURI.
const ParamLogURI = channel.ParamKey("log_uri")

// ParamShutdownTimeout is how many seconds closing a server waits for requests in flight to finish before
// it closes the remaining connections (default: DefaultShutdownTimeout). Nonpositive values use the default.
const ParamShutdownTimeout = channel.ParamKey("shutdown_timeout")

// DefaultShutdownTimeout is the ParamShutdownTimeout used when it is unset or nonpositive.
const DefaultShutdownTimeout = 5
