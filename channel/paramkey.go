package channel

import (
	"sync"
)

type ParamKey string

const ParamAcceptTimeout = ParamKey("accept_timeout")
const ParamReadBufferSize = ParamKey("read_buffer_size")
const ParamReadTimeout = ParamKey("read_timeout")
const ParamWriteTimeout = ParamKey("write_timeout")

// ParamMaxInProgressBytes bounds shared incomplete input per server in bytes. Nonpositive values use the default.
const ParamMaxInProgressBytes = ParamKey("max_in_progress_bytes")

// ParamFrameAssemblyTimeout bounds one incomplete frame/message in milliseconds, regardless of incoming traffic.
// Nonpositive values use the default.
const ParamFrameAssemblyTimeout = ParamKey("frame_assembly_timeout")

// ParamMaxQueuedWriteBytes includes in-flight and pending payloads plus per-entry overhead.
// Nonpositive values use the default.
const ParamMaxQueuedWriteBytes = ParamKey("max_queued_write_bytes")

const DefaultMaxInProgressBytes = 64 << 20
const DefaultFrameAssemblyTimeout = 30000
const DefaultMaxQueuedWriteBytes = 32 << 20

// ParamReadIdleTimeout closes a connection that receives nothing for this many milliseconds, so idle peers
// do not hold descriptors forever. Connections a server accepts default to DefaultReadIdleTimeout; client
// connections default to off. A net channel checks it each ParamReadTimeout; a WebSocket must receive each
// whole message, ping or pong within it. A gudp child uses gudp.ParamPeerIdleTimeout instead. Nonpositive
// values keep idle connections open.
const ParamReadIdleTimeout = ParamKey("read_idle_timeout")

// ParamMaxConnections limits the connections a server holds open at once (default: DefaultMaxConnections);
// further connections wait to be accepted. Nonpositive values disable the limit.
const ParamMaxConnections = ParamKey("max_connections")

const DefaultReadIdleTimeout = 300000
const DefaultMaxConnections = 10000

func GetParamIntDefault(ch Channel, key ParamKey, defaultValue int) int {
	switch v := ch.Param(key).(type) {
	case int8:
		return int(v)
	case uint8:
		return int(v)
	case int16:
		return int(v)
	case uint16:
		return int(v)
	case int32:
		return int(v)
	case int:
		return v
	}

	return defaultValue
}

func GetParamInt64Default(ch Channel, key ParamKey, defaultValue int64) int64 {
	switch v := ch.Param(key).(type) {
	case int8:
		return int64(v)
	case uint8:
		return int64(v)
	case int16:
		return int64(v)
	case uint16:
		return int64(v)
	case int32:
		return int64(v)
	case uint32:
		return int64(v)
	case int:
		return int64(v)
	case uint:
		return int64(v)
	case int64:
		return v
	}

	return defaultValue
}

func GetParamStringDefault(ch Channel, key ParamKey, defaultValue string) string {
	switch v := ch.Param(key).(type) {
	case string:
		return v
	}

	return defaultValue
}

func GetParamBoolDefault(ch Channel, key ParamKey, defaultValue bool) bool {
	switch v := ch.Param(key).(type) {
	case bool:
		return v
	}

	return defaultValue
}

type Params struct {
	sync.Map
}

func (p *Params) Load(key ParamKey) (value any, ok bool) {
	return p.Map.Load(key)
}

func (p *Params) Store(key ParamKey, value any) {
	p.Map.Store(key, value)
}
func (p *Params) Range(f func(key ParamKey, value any) bool) {
	p.Map.Range(func(k, v any) bool {
		return f(k.(ParamKey), v)
	})
}
func (p *Params) Delete(key ParamKey) {
	p.Map.Delete(key)
}
func (p *Params) LoadOrStore(key ParamKey, value any) (actual any, loaded bool) {
	return p.Map.LoadOrStore(key, value)
}
