package websocket

import (
	gtp "github.com/kklab-com/gone/http"
)

type Route struct {
	gtp.DefaultRoute
}

func NewRoute() *Route {
	route := Route{DefaultRoute: *gtp.NewRoute()}
	route.
		SetRoot(gtp.NewEndPoint("", new(DefaultTask), nil)).
		AddRecursivePoint(gtp.NewEndPoint("static", new(DefaultHomeTask), nil)).
		AddEndPoint(gtp.NewEndPoint("echo", new(ServerHandlerTask), nil)).
		AddEndPoint(gtp.NewEndPoint("home", new(DefaultHomeTask), nil)).
		AddGroup(gtp.NewGroup("v1", nil).
			AddEndPoint(gtp.NewEndPoint("home", new(DefaultHomeTask), nil)),
		)

	return &route
}
