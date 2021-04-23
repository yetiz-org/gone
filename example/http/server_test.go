package http

import (
	"net"
	http2 "net/http"
	"testing"
	"time"

	"github.com/kklab-com/gone/channel"
	"github.com/kklab-com/gone/http"
	"github.com/kklab-com/goth-kklogger"
	"github.com/kklab-com/goth-kkutil/buf"
	"github.com/stretchr/testify/assert"
)

func TestServer_Start(t *testing.T) {
	kklogger.SetLogLevel("TRACE")
	bootstrap := channel.NewServerBootstrap()
	bootstrap.ChannelType(&http.ServerChannel{})
	bootstrap.Handler(channel.NewInitializer(func(ch channel.Channel) {
		ch.Pipeline().AddLast("INDICATE_HANDLER_INBOUND", &channel.IndicateHandlerInbound{})
		ch.Pipeline().AddLast("INDICATE_HANDLER_OUTBOUND", &channel.IndicateHandlerOutbound{})
	}))

	bootstrap.ChildHandler(channel.NewInitializer(func(ch channel.Channel) {
		ch.Pipeline().AddLast("INDICATE_HANDLER_INBOUND", &channel.IndicateHandlerInbound{})
		ch.Pipeline().AddLast("GZIP_HANDLER", new(http.GZipHandler))
		ch.Pipeline().AddLast("LOG_HANDLER", http.NewLogHandler(false))
		ch.Pipeline().AddLast("DISPATCHER", http.NewDispatchHandler(NewRoute()))
		ch.Pipeline().AddLast("INDICATE_HANDLER_OUTBOUND", &channel.IndicateHandlerOutbound{})
	}))

	ch := bootstrap.Bind(&net.TCPAddr{IP: nil, Port: 18080}).Sync().Channel()

	if rtn, err := http2.DefaultClient.Get("http://localhost:18080"); err != nil {
		assert.Fail(t, err.Error())
	} else {
		assert.EqualValues(t, "feeling good", string(buf.EmptyByteBuf().WriteReader(rtn.Body).Bytes()))
	}

	http2.DefaultClient.CloseIdleConnections()
	if rtn, err := http2.DefaultClient.Get("http://localhost:18080/home"); err != nil {
		assert.Fail(t, err.Error())
	} else {
		assert.EqualValues(t, "/home", string(buf.EmptyByteBuf().WriteReader(rtn.Body).Bytes()))
	}

	http2.DefaultClient.CloseIdleConnections()
	if rtn, err := http2.DefaultClient.Get("http://localhost:18080/v1/home"); err != nil {
		assert.Fail(t, err.Error())
	} else {
		assert.EqualValues(t, "/v1/home", string(buf.EmptyByteBuf().WriteReader(rtn.Body).Bytes()))
	}

	http2.DefaultClient.CloseIdleConnections()
	if rtn, err := http2.DefaultClient.Get("http://localhost:18080/homes"); err != nil {
		assert.Fail(t, err.Error())
	} else {
		assert.EqualValues(t, 404, rtn.StatusCode)
	}

	http2.DefaultClient.CloseIdleConnections()
	if rtn, err := http2.DefaultClient.Get("http://localhost:18080/close"); err != nil {
		assert.Fail(t, err.Error())
	} else {
		assert.EqualValues(t, "/close", string(buf.EmptyByteBuf().WriteReader(rtn.Body).Bytes()))
	}

	go func() {
		time.Sleep(time.Minute * 5)
		ch.Close()
	}()

	ch.CloseFuture().Sync()
}
