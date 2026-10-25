package gudp

import "github.com/yetiz-org/gone/channel"

// ParamMaxPeers limits how many remote addresses a ServerChannel serves at once (default: DefaultMaxPeers).
// Datagrams from new addresses are dropped while the limit is reached. Nonpositive values disable the limit.
var ParamMaxPeers channel.ParamKey = "udp_max_peers"

// ParamMaxPeersPerIP limits active remote addresses from one source IP (default: DefaultMaxPeersPerIP).
// IPv4-mapped IPv6 addresses share the IPv4 quota. Nonpositive values use the default.
var ParamMaxPeersPerIP channel.ParamKey = "udp_max_peers_per_ip"

// ParamNewPeersPerSecond limits admission of new remote addresses across the server
// (default: DefaultNewPeersPerSecond). Existing peers are unaffected. Nonpositive values use the default.
var ParamNewPeersPerSecond channel.ParamKey = "udp_new_peers_per_second"

// ParamNewPeerBurst is the maximum accumulated new-peer admission allowance
// (default: DefaultNewPeerBurst). Nonpositive values use the default.
var ParamNewPeerBurst channel.ParamKey = "udp_new_peer_burst"

// ParamPeerIdleTimeout closes a peer's child channel after no datagram is received from or sent to the peer for this many milliseconds
// (default: DefaultPeerIdleTimeout). Its next datagram opens a new child. Nonpositive values disable it.
var ParamPeerIdleTimeout channel.ParamKey = "udp_peer_idle_timeout"

const DefaultMaxPeers = 4096
const DefaultMaxPeersPerIP = 128
const DefaultNewPeersPerSecond = 256
const DefaultNewPeerBurst = 256
const DefaultPeerIdleTimeout = 60000
