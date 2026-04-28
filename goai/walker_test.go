package goai

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yetiz-org/gone/channel"
	"github.com/yetiz-org/gone/ghttp"
)

// _WalkerItemHandler is a collection-style handler used by walker tests:
// implements Index (collection list) and Get (item-level), so the walker
// emits two candidates — one bare-path and one item-level path with an
// auto-appended `{<last_segment>_id}` placeholder.
type _WalkerItemHandler struct {
	ghttp.DefaultHTTPHandlerTask
}

func (h *_WalkerItemHandler) Index(ctx channel.HandlerContext, req *ghttp.Request, resp *ghttp.Response, params map[string]any) ghttp.ErrorResponse {
	return nil
}

func (h *_WalkerItemHandler) Get(ctx channel.HandlerContext, req *ghttp.Request, resp *ghttp.Response, params map[string]any) ghttp.ErrorResponse {
	return nil
}

// _WalkerAnchorAfterInjector is an Acceptance that contributes one PathParam
// whose AnchorAfter equals the *last* segment of the route the handler is
// mounted on. This mirrors a common production pattern where a permission
// helper declares the resource id placeholder for the collection's
// item-level routes — the placeholder is meant for `<collection>/{xxx_id}`,
// not for the bare collection path.
//
// expandPathWithParams drops the placeholder from the bare collection path
// (anchor is the last segment), but the param entry remains in the
// pathParams slice. Without dedup, the walker's item-level branch then
// auto-appends a second PathParam with the same name.
type _WalkerAnchorAfterInjector struct {
	ghttp.DispatchAcceptance
	name        string
	anchorAfter string
}

func (a *_WalkerAnchorAfterInjector) InjectedParamsFor(routePath string) []PathParam {
	return []PathParam{
		{
			Name:        a.name,
			In:          "path",
			Required:    true,
			AnchorAfter: a.anchorAfter,
		},
	}
}

func TestWalkDoesNotDuplicatePathParamWhenInjectorAnchorIsLastSegment(t *testing.T) {
	prev := ghttp.SetSkipHandlerRegister(true)
	defer ghttp.SetSkipHandlerRegister(prev)

	route := ghttp.NewSimpleRoute()
	route.SetEndpoint("/things", &_WalkerItemHandler{}, &_WalkerAnchorAfterInjector{
		name:        "things_id",
		anchorAfter: "things",
	})

	candidates := Walk(route)

	var itemCandidate *OperationCandidate
	for i := range candidates {
		if candidates[i].Method == "GET" && candidates[i].HandlerMethod == "Get" {
			itemCandidate = &candidates[i]
			break
		}
	}

	require.NotNil(t, itemCandidate, "expected item-level GET candidate")
	assert.Equal(t, "/things/{things_id}", itemCandidate.Path)

	names := make(map[string]int, len(itemCandidate.PathParams))
	for _, p := range itemCandidate.PathParams {
		names[p.Name]++
	}

	assert.Equal(t, 1, names["things_id"], "expected one PathParam named things_id, got %d (params: %+v)", names["things_id"], itemCandidate.PathParams)
	assert.Len(t, itemCandidate.PathParams, 1)
}
