package gudp

import "github.com/yetiz-org/gone/channel"

// ParamMaxPeers limits how many remote addresses a ServerChannel serves at once (default: DefaultMaxPeers).
// Datagrams from new addresses are dropped while the limit is reached. Nonpositive values disable the limit.
var ParamMaxPeers channel.ParamKey = "udp_max_peers"

// ParamPeerIdleTimeout closes a peer's child channel after no datagram is received from or sent to the peer for this many milliseconds
// (default: DefaultPeerIdleTimeout). Its next datagram opens a new child. Nonpositive values disable it.
var ParamPeerIdleTimeout channel.ParamKey = "udp_peer_idle_timeout"

const DefaultMaxPeers = 4096
const DefaultPeerIdleTimeout = 60000
