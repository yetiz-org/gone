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
		SetRoot(gtp.NewEndPoint("", new(ServerHandlerTask), nil)).
		AddRecursivePoint(gtp.NewEndPoint("static", new(ServerHandlerTask), nil)).
		AddEndPoint(gtp.NewEndPoint("echo", new(ServerHandlerTask), nil)).
		AddEndPoint(gtp.NewEndPoint("home", new(ServerHandlerTask), nil)).
		AddGroup(gtp.NewGroup("v1", nil).
			AddEndPoint(gtp.NewEndPoint("home", new(ServerHandlerTask), nil)),
		)

	return &route
}
