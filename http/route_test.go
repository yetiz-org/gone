package http

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"
)

func TestRestRoute_RouteEndPoint(t *testing.T) {
	req := Request{
		request: &http.Request{},
	}

	req.Request().URL = &url.URL{}
	req.Request().URL.Path = "/auth/group/user/123/book/newbook/name"
	route := NewRoute()
	route.AddGroup(
		NewGroup("auth", nil).
			AddGroup(
				NewGroup("group", nil).
					AddEndPoint(
						NewEndPoint("user", new(DefaultHandlerTask), nil).
							AddEndPoint(NewEndPoint("book", new(DefaultHandlerTask), nil).
								AddEndPoint(NewEndPoint("name", new(DefaultHandlerTask), nil))).
							AddGroup(NewGroup("profile", nil).
								AddEndPoint(NewEndPoint("info", new(DefaultHandlerTask), nil))))),
	)

	point, m, isLast := route.RouteNode(req.Url().Path)
	println(point.Name())
	for k, v := range m {
		println(fmt.Sprintf("%s: %s", k, v))
	}

	if !isLast {
		t.Error("")
	}

	println("----")
	req.Request().URL.Path = "/auth/group/user/123"
	point, m, isLast = route.RouteNode(req.Url().Path)
	println(point.Name())
	for k, v := range m {
		println(fmt.Sprintf("%s: %s", k, v))
	}

	if isLast {
		t.Error("")
	}

	println("----")
	req.Request().URL.Path = "/auth/group/user/123/book"
	point, m, isLast = route.RouteNode(req.Url().Path)
	println(point.Name())
	for k, v := range m {
		println(fmt.Sprintf("%s: %s", k, v))
	}

	if !isLast {
		t.Error("")
	}

	println("----")
	req.Request().URL.Path = "/auth/group/user/123/book/newbook"
	point, m, isLast = route.RouteNode(req.Url().Path)
	println(point.Name())
	for k, v := range m {
		println(fmt.Sprintf("%s: %s", k, v))
	}

	if isLast {
		t.Error("")
	}

	println("----")
	req.Request().URL.Path = "/auth/group/user"
	point, m, isLast = route.RouteNode(req.Url().Path)
	println(point.Name())
	for k, v := range m {
		println(fmt.Sprintf("%s: %s", k, v))
	}

	if !isLast {
		t.Error("")
	}

	println("----")
	req.Request().URL.Path = "/auth/group/user/123/profile/info/myname"
	point, m, isLast = route.RouteNode(req.Url().Path)
	println(point.Name())
	for k, v := range m {
		println(fmt.Sprintf("%s: %s", k, v))
	}

	if isLast {
		t.Error("")
	}

	println("----")
	req.Request().URL.Path = "/auth/group/user/123/book/newbook/dasdqwe"
	point, m, isLast = route.RouteNode(req.Url().Path)
}
