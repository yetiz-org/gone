package http

import (
	"net"
	"testing"

	"github.com/kklab-com/goth-kklogger"
)

func TestServer_Start(t *testing.T) {
	kklogger.SetLogLevel("TRACE")
	server := Server{}
	server.Start(&net.TCPAddr{IP: nil, Port: 18080})
}
