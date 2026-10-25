package gws

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/yetiz-org/gone/channel"
)

func BenchmarkWebSocketReadOneByte(b *testing.B) {
	done := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			select {
			case <-done:
				return
			default:
			}
			if err := conn.WriteMessage(websocket.BinaryMessage, []byte{1}); err != nil {
				return
			}
		}
	}))
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		b.Fatal(err)
	}
	client := &Channel{DefaultNetChannel: &channel.DefaultNetChannel{}, wsConn: conn}
	b.Cleanup(func() { close(done); conn.Close(); server.Close() })
	b.ReportAllocs()
	for b.Loop() {
		_, data, err := client._ReadMessage()
		if err != nil {
			b.Fatal(err)
		}
		if len(data) != 1 || data[0] != 1 {
			b.Fatal("message changed")
		}
	}
}
