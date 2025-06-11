package example

import (
	"github.com/yetiz-org/gone/http"
)

var homepage = new(DefaultHomeTask)
var longTask = new(LongTask)

type Route struct {
	http.DefaultRoute
}

func NewRoute() *Route {
	route := Route{DefaultRoute: *http.NewRoute()}
	route.
		SetRoot(http.NewEndPoint("", new(DefaultTask), nil)).
		AddRecursivePoint(http.NewEndPoint("static", new(DefaultTask), nil)).
		AddEndPoint(http.NewEndPoint("home", homepage, nil)).
		AddEndPoint(http.NewEndPoint("routine", new(Routine), nil)).
		AddEndPoint(http.NewEndPoint("long", longTask, nil)).
		AddEndPoint(http.NewEndPoint("close", new(CloseTask), nil)).
		AddEndPoint(http.NewEndPoint("400", new(Routine), []http.Acceptance{new(Acceptance400)})).
		AddEndPoint(http.NewEndPoint("sse", new(SSE), nil)).
		AddGroup(http.NewGroup("v1", []http.Acceptance{}).
			AddEndPoint(http.NewEndPoint("home", homepage, nil)),
		)
	return &route
}
