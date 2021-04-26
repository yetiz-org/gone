package http

import "github.com/kklab-com/gone/channel"

var ParamIdleTimeout channel.ParamKey = "idle_timeout"
var ParamReadTimeout channel.ParamKey = "read_timeout"
var ParamReadHeaderTimeout channel.ParamKey = "read_header_timeout"
var ParamWriteTimeout channel.ParamKey = "write_timeout"
var ParamAcceptWaitCount channel.ParamKey = "accept_wait_count"
var ParamMaxHeaderBytes channel.ParamKey = "max_header_bytes"
var ParamMaxMultiPartMemory channel.ParamKey = "max_multi_part_memory"
