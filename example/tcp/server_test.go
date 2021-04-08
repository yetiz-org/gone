package tcp

import (
	"net"
	"testing"
	"time"

	"github.com/kklab-com/goth-kklogger"
)

func TestServer_Start(t *testing.T) {
	kklogger.SetLogLevel("TRACE")
	go func() {
		time.Sleep(time.Millisecond * 500)
		if conn, err := net.Dial("tcp4", "localhost:18080"); err == nil {
			conn.Write([]byte("o12b32c49"))
			time.Sleep(time.Second)
			conn.Write([]byte("a42d22e41"))
			time.Sleep(time.Second)
			conn.Close()
		}
	}()

	(&Server{}).Start(&net.TCPAddr{IP: nil, Port: 18080})
}
