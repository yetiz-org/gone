package channel

import (
	"io"
	"net"
	"testing"
	"time"

	buf "github.com/yetiz-org/goth-bytebuf"
)

// benchDialPair is the bench analogue of dialLoopbackPair without t.Helper.
func benchDialPair(b *testing.B) (net.Conn, net.Conn) {
	b.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		b.Fatal(err)
	}
	done := make(chan net.Conn, 1)
	go func() {
		c, _ := lis.Accept()
		done <- c
	}()
	client, err := net.Dial("tcp", lis.Addr().String())
	if err != nil {
		b.Fatal(err)
	}
	server := <-done
	_ = lis.Close()
	b.Cleanup(func() {
		_ = client.Close()
		_ = server.Close()
	})
	return client, server
}

func benchNewChannel(b *testing.B, c net.Conn) *DefaultNetChannel {
	b.Helper()
	ch := &DefaultNetChannel{BufferSize: 65536, WriteTimeout: 5 * time.Second, ReadTimeout: 5 * time.Second}
	ch.setConn(c)
	return ch
}

// startDrainer consumes bytes from server in the background until closed, so
// the TCP receive window never stalls the writer under measurement.
func startDrainer(b *testing.B, server net.Conn) {
	b.Helper()
	go func() {
		tmp := make([]byte, 64*1024)
		for {
			if _, err := server.Read(tmp); err != nil {
				return
			}
		}
	}()
}

// payloadFragments is a realistic header+body mix used to simulate protocol
// frames: 4B length prefix + three body fragments of progressively larger
// size. Total is 1 + 16 + 256 + 4096 = 4369 bytes across 4 components.
func payloadFragments() [][]byte {
	return [][]byte{
		{0x10},
		make4K(16),
		make4K(256),
		make4K(4096),
	}
}

func make4K(n int) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = byte(i)
	}
	return out
}

// BenchmarkUnsafeWrite_DefaultByteBuf measures the cost of coalescing header
// and body into one DefaultByteBuf and sending via DefaultConn.Write.
func BenchmarkUnsafeWrite_DefaultByteBuf(b *testing.B) {
	client, server := benchDialPair(b)
	startDrainer(b, server)
	ch := benchNewChannel(b, client)

	frags := payloadFragments()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		merged := buf.EmptyByteBuf()
		for _, f := range frags {
			merged.WriteBytes(f)
		}
		if err := ch.UnsafeWrite(merged); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkUnsafeWrite_CompositeWritev measures the same workload but via
// CompositeByteBuf + io.WriterTo (writev(2) on *net.TCPConn).
func BenchmarkUnsafeWrite_CompositeWritev(b *testing.B) {
	client, server := benchDialPair(b)
	startDrainer(b, server)
	ch := benchNewChannel(b, client)

	frags := payloadFragments()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		comps := make([]buf.ByteBuf, len(frags))
		for i, f := range frags {
			comps[i] = buf.NewSharedByteBuf(f)
		}
		composite := buf.NewCompositeByteBuf(comps...)
		if err := ch.UnsafeWrite(composite); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkUnsafeWrite_CompositeCompileAssertion keeps the linker from
// eliding io as unused when this file is compiled in isolation.
var _ io.Writer = (*net.TCPConn)(nil)
