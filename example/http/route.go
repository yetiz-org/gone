package http

import (
	"github.com/kklab-com/gone/http"
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
		AddGroup(http.NewGroup("v1", []http.Acceptance{}).
			AddEndPoint(http.NewEndPoint("home", homepage, nil)),
		)
	return &route
}
