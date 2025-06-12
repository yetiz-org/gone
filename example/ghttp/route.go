package example

import (
	"github.com/yetiz-org/gone/ghttp"
)

var homepage = new(DefaultHomeTask)
var longTask = new(LongTask)

type Route struct {
	ghttp.DefaultRoute
}

func NewRoute() *Route {
	route := Route{DefaultRoute: *ghttp.NewRoute()}
	route.
		SetRoot(ghttp.NewEndPoint("", new(DefaultTask), nil)).
		AddRecursivePoint(ghttp.NewEndPoint("static", new(DefaultTask), nil)).
		AddEndPoint(ghttp.NewEndPoint("home", homepage, nil)).
		AddEndPoint(ghttp.NewEndPoint("routine", new(Routine), nil)).
		AddEndPoint(ghttp.NewEndPoint("long", longTask, nil)).
		AddEndPoint(ghttp.NewEndPoint("close", new(CloseTask), nil)).
		AddEndPoint(ghttp.NewEndPoint("400", new(Routine), []ghttp.Acceptance{new(Acceptance400)})).
		AddEndPoint(ghttp.NewEndPoint("sse", new(SSE), nil)).
		AddGroup(ghttp.NewGroup("v1", []ghttp.Acceptance{}).
			AddEndPoint(ghttp.NewEndPoint("home", homepage, nil)),
		)
	return &route
}
