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

// ParamShutdownTimeout is how many seconds closing a server waits for requests in flight to finish before
// it closes the remaining connections (default: DefaultShutdownTimeout). Nonpositive values use the default.
const ParamShutdownTimeout = channel.ParamKey("shutdown_timeout")

// DefaultShutdownTimeout is the ParamShutdownTimeout used when it is unset or nonpositive.
const DefaultShutdownTimeout = 5
