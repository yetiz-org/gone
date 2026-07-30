package ghttp

import (
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/yetiz-org/gone/channel"
	"github.com/yetiz-org/gone/ghttp/httpstatus"
)

type failingAcceptance struct {
	DispatchAcceptance
	err error
}

func (a failingAcceptance) Do(ctx channel.HandlerContext, req *Request, resp *Response, params map[string]any) error {
	return a.err
}

func TestDispatchHandlerPlainAcceptanceErrorReturnsBadRequest(t *testing.T) {
	ch := &channel.DefaultChannel{}
	ch.Init()
	req := &Request{request: httptest.NewRequest(MethodGet, "/probe", nil), channel: ch, trackID: "track"}
	pack := &Pack{Request: req, Response: NewResponse(req), Params: map[string]any{}}
	route := NewSimpleRoute().SetEndpoint("/probe", &DefaultHTTPHandlerTask{}, &failingAcceptance{err: errors.New("reject")})
	handler := NewDispatchHandler(route)
	ctx := channel.NewMockHandlerContext()
	ctx.On("Channel").Return(ch)
	ctx.On("Write", pack, pack.Response.done).Run(func(args mock.Arguments) {
		args.Get(1).(channel.Future).Completable().Complete(args.Get(0))
	}).Return(pack.Response.done)

	handler.Read(ctx, pack)

	assert.Equal(t, httpstatus.BadRequest, pack.Response.StatusCode())
	ctx.AssertExpectations(t)
}
