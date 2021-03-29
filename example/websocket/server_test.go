package websocket

import (
	"encoding/binary"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/kklab-com/goth-kklogger"
)

var server = Server{}

func TestServer_Start(t *testing.T) {
	kklogger.SetLogLevel("TRACE")
	go func() {
		time.Sleep(time.Second)
		c, resp, err := websocket.DefaultDialer.Dial("ws://localhost:18081/echo", nil)
		if err != nil {
			kklogger.Error(fmt.Sprintf("dial: %s, %v", err.Error(), resp))
			return
		}

		var m []byte
		buf := make([]byte, 2+len(m))
		binary.BigEndian.PutUint16(buf, uint16(websocket.CloseNormalClosure))
		copy(buf[2:], m)
		c.WriteMessage(websocket.TextMessage, []byte("write data"))
		c.WriteControl(websocket.CloseMessage, buf, time.Now().Add(time.Second))
	}()

	server.Start(&net.TCPAddr{IP: nil, Port: 18081})
}
