package gws

import "github.com/yetiz-org/gone/channel"

var ParamCheckOrigin channel.ParamKey = "check_origin"

// ParamWSReadLimit limits each incoming WebSocket message in bytes (default: DefaultWSReadLimit).
// Nonpositive values disable the limit.
var ParamWSReadLimit channel.ParamKey = "ws_read_limit"

// DefaultWSReadLimit is the message size limit used when ParamWSReadLimit is not set.
const DefaultWSReadLimit = 16 << 20
