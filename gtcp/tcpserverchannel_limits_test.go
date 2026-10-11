package gtcp

import (
	"fmt"
	"io"
	"net"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yetiz-org/gone/channel"
)

func startLimitServer(t *testing.T, params map[channel.ParamKey]any, childParams map[channel.ParamKey]any) (addr *net.TCPAddr, children chan *Channel) {
	children = make(chan *Channel, 4)
	bootstrap := channel.NewServerBootstrap()
	bootstrap.ChannelType(&ServerChannel{})
	for k, v := range params {
		bootstrap.SetParams(k, v)
	}

	for k, v := range childParams {
		bootstrap.SetChildParams(k, v)
	}

	bootstrap.ChildHandler(channel.NewInitializer(func(ch channel.Channel) { children <- ch.(*Channel) }))
	server := bootstrap.Bind(&net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)}).Sync().Channel().(*ServerChannel)
	t.Cleanup(func() { server.Close().Sync() })
	return server.listen.Addr().(*net.TCPAddr), children
}

func TestServerChannel_ClosesIdleAcceptedConnections(t *testing.T) {
	for _, readTimeout := range []int{20, 0} {
		t.Run(fmt.Sprintf("read timeout %d", readTimeout), func(t *testing.T) {
			addr, _ := startLimitServer(t, nil, map[channel.ParamKey]any{channel.ParamReadTimeout: readTimeout, channel.ParamReadIdleTimeout: 200})
			silent, err := net.Dial("tcp", addr.String())
			require.NoError(t, err)
			defer silent.Close()
			busy, err := net.Dial("tcp", addr.String())
			require.NoError(t, err)
			defer busy.Close()

			start := time.Now()
			stop := make(chan struct{})
			defer close(stop)
			go func() {
				for {
					select {
					case <-stop:
						return
					case <-time.After(50 * time.Millisecond):
						busy.Write([]byte("x"))
					}
				}
			}()

			silent.SetReadDeadline(time.Now().Add(3 * time.Second))
			_, err = silent.Read(make([]byte, 1))
			require.ErrorIs(t, err, io.EOF, "a silent connection must be closed")
			assert.GreaterOrEqual(t, time.Since(start), 200*time.Millisecond)

			time.Sleep(time.Until(start.Add(500 * time.Millisecond)))
			busy.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
			_, err = busy.Read(make([]byte, 1))
			assert.ErrorIs(t, err, os.ErrDeadlineExceeded, "a connection that keeps sending must stay open")
		})
	}
}

func TestServerChannel_ReadIdleTimeoutDefaults(t *testing.T) {
	addr, children := startLimitServer(t, nil, nil)
	bootstrap := channel.NewBootstrap()
	bootstrap.ChannelType(&Channel{})
	bootstrap.Handler(channel.NewInitializer(func(channel.Channel) {}))
	client := bootstrap.Connect(nil, addr).Sync().Channel().(*Channel)
	defer client.Close()

	select {
	case child := <-children:
		assert.Equal(t, channel.DefaultReadIdleTimeout*time.Millisecond, child.ReadIdleTimeout, "accepted connections close when idle by default")
	case <-time.After(time.Second):
		t.Fatal("connection was not accepted")
	}

	assert.Zero(t, client.ReadIdleTimeout, "client connections stay open by default")
}

func TestServerChannel_MaxConnections(t *testing.T) {
	addr, children := startLimitServer(t, map[channel.ParamKey]any{channel.ParamMaxConnections: 1}, nil)
	first, err := net.Dial("tcp", addr.String())
	require.NoError(t, err)
	<-children
	second, err := net.Dial("tcp", addr.String())
	require.NoError(t, err)
	defer second.Close()

	select {
	case <-children:
		t.Fatal("a second connection was accepted over the limit")
	case <-time.After(200 * time.Millisecond):
	}

	first.Close()
	select {
	case <-children:
	case <-time.After(2 * time.Second):
		t.Fatal("closing a connection did not let the next one in")
	}
}
