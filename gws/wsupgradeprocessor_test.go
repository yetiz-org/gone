package gws

import (
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/yetiz-org/gone/channel"
	"github.com/yetiz-org/gone/ghttp"
	"github.com/yetiz-org/gone/ghttp/httpstatus"
)

type wsFailingAcceptance struct {
	ghttp.DispatchAcceptance
	err error
}

func (a wsFailingAcceptance) Do(ctx channel.HandlerContext, req *ghttp.Request, resp *ghttp.Response, params map[string]any) error {
	return a.err
}

type rejectingWSTask struct {
	DefaultServerHandlerTask
	status int
}

func (t *rejectingWSTask) WSUpgrade(req *ghttp.Request, resp *ghttp.Response, params map[string]any) bool {
	if t.status != 0 {
		resp.SetStatusCode(t.status)
	}
	return false
}

func newUpgradeTestPack(t *testing.T, task ghttp.HandlerTask, acceptances ...ghttp.Acceptance) (*ghttp.Pack, *channel.DefaultChannel) {
	t.Helper()

	ch := &channel.DefaultChannel{}
	ch.Init()
	req := ghttp.WrapRequest(ch, httptest.NewRequest("GET", "/ws", nil))
	resp := ghttp.NewResponse(req)
	route := ghttp.NewSimpleRoute().SetEndpoint("/ws", task, acceptances...)
	node, params, _ := route.RouteNode("/ws")
	if params == nil {
		params = map[string]any{}
	}

	return &ghttp.Pack{Request: req, Response: resp, RouteNode: node, Params: params}, ch
}

func completedWriteFuture(ch channel.Channel, value any) channel.Future {
	future := channel.NewFuture(ch)
	future.Completable().Complete(value)
	return future
}

func TestUpgradeProcessorPlainAcceptanceErrorReturnsBadRequest(t *testing.T) {
	pack, ch := newUpgradeTestPack(t, &DefaultServerHandlerTask{}, &wsFailingAcceptance{err: errors.New("reject")})
	ctx := channel.NewMockHandlerContext()
	ctx.On("Channel").Return(ch)
	ctx.On("Write", pack, nil).Return(completedWriteFuture(ch, pack))

	(&UpgradeProcessor{}).Read(ctx, pack)

	assert.Equal(t, httpstatus.BadRequest, pack.Response.StatusCode())
	ctx.AssertExpectations(t)
}

func TestUpgradeProcessorRejectedUpgradeSetsBadRequestWhenStatusUnset(t *testing.T) {
	pack, ch := newUpgradeTestPack(t, &DefaultServerHandlerTask{})
	ctx := channel.NewMockHandlerContext()
	ctx.On("Channel").Return(ch).Maybe()
	ctx.On("Write", pack, nil).Return(completedWriteFuture(ch, pack))
	processor := &UpgradeProcessor{UpgradeCheckFunc: func(req *ghttp.Request, resp *ghttp.Response, params map[string]any) bool {
		return false
	}}

	processor.Read(ctx, pack)

	assert.Equal(t, httpstatus.BadRequest, pack.Response.StatusCode())
	ctx.AssertExpectations(t)
}

func TestUpgradeProcessorRejectedUpgradePreservesExplicitStatus(t *testing.T) {
	pack, ch := newUpgradeTestPack(t, &rejectingWSTask{status: httpstatus.Forbidden})
	ctx := channel.NewMockHandlerContext()
	ctx.On("Channel").Return(ch).Maybe()
	ctx.On("Write", pack, nil).Return(completedWriteFuture(ch, pack))

	(&UpgradeProcessor{}).Read(ctx, pack)

	assert.Equal(t, httpstatus.Forbidden, pack.Response.StatusCode())
	ctx.AssertExpectations(t)
}
