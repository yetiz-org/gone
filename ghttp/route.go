package ghttp

import (
	"fmt"
	"strings"

	"github.com/yetiz-org/goth-kklogger"
)

type Route interface {
	RouteNode(path string) (node RouteNode, nodeParams map[string]any, isLast bool)
}

type DefaultRoute struct {
	root RouteNode
}

func NewRoute() *DefaultRoute {
	n := NewEndPoint("", NewDefaultHandlerTask(), nil)
	n.routeType = RouteTypeRootEndPoint
	return &DefaultRoute{
		root: n,
	}
}

func (r *DefaultRoute) RouteNode(path string) (node RouteNode, nodeParams map[string]any, isLast bool) {
	path = strings.TrimLeft(strings.TrimRight(path, "/"), "/")
	params := map[string]any{}
	if path == "" {
		return r.root, nil, true
	}

	resources := strings.Split(path, "/")
	nodeLens := len(resources)
	current := r.root
	next := r.root
	for idx, resourceID := range resources {
		next = current.Resources()[resourceID]
		switch current.RouteType() {
		case RouteTypeEndPoint, RouteTypeRootEndPoint:
			if idx+1 == nodeLens {
				if next == nil {
					if current == r.root && resourceID != "" {
						return nil, nil, false
					} else {
						params[fmt.Sprintf("[gone-http]%s_id", current.Name())] = resourceID
						return current, params, false
					}
				} else {
					return next, params, true
				}
			} else {
				if next == nil {
					if _, f := current.Resources()[resources[idx+1]]; f {
						params[fmt.Sprintf("[gone-http]%s_id", current.Name())] = resourceID
						continue
					} else {
						return nil, nil, false
					}
				} else {
					current = next
				}
			}
		case RouteTypeRecursiveEndPoint:
			if next == nil {
				params[fmt.Sprintf("[gone-http]%s_id", current.Name())] = resourceID
			}

			return current, params, false
		case RouteTypeGroup:
			if next == nil {
				return nil, nil, false
			}

			current = next
		}
	}

	return current, params, current == next
}

func (r *DefaultRoute) SetRoot(point *_EndPoint) *DefaultRoute {
	point.routeType = RouteTypeRootEndPoint
	r.root = point
	return r
}

func (r *DefaultRoute) AddRecursivePoint(point *_EndPoint) *DefaultRoute {
	if point == nil {
		kklogger.ErrorJ("ghttp:DefaultRoute.AddRecursivePoint#add_recursive!nil_point", "add nil point")
		return nil
	}

	if r.root.Resources()[point.Name()] != nil {
		kklogger.ErrorJ("ghttp:DefaultRoute.AddRecursivePoint#add_recursive!duplicate_name", "add same name endpoint")
		return nil
	}

	r.root.Resources()[point.Name()] = point
	point.parent = r.root
	point.routeType = RouteTypeRecursiveEndPoint
	if point.handler != nil {
		point.handler.Register()
	}

	return r
}

func (r *DefaultRoute) AddGroup(group *_RouteGroup) *DefaultRoute {
	if group == nil {
		kklogger.ErrorJ("ghttp:DefaultRoute.AddGroup#add_group!nil_group", "add nil group")
		return nil
	}

	if r.root.Resources()[group.Name()] != nil {
		kklogger.ErrorJ("ghttp:DefaultRoute.AddGroup#add_group!duplicate_name", "add same name group")
		return nil
	}

	r.root.Resources()[group.Name()] = group
	group.parent = r.root
	return r
}

func (r *DefaultRoute) AddEndPoint(point *_EndPoint) *DefaultRoute {
	if point == nil {
		kklogger.ErrorJ("ghttp:DefaultRoute.AddEndPoint#add_endpoint!nil_point", "add nil point")
		return nil
	}

	if r.root.Resources()[point.Name()] != nil {
		kklogger.ErrorJ("ghttp:DefaultRoute.AddEndPoint#add_endpoint!duplicate_name", "add same name endpoint")
		return nil
	}

	r.root.Resources()[point.Name()] = point
	point.parent = r.root
	if point.handler != nil {
		point.handler.Register()
	}

	return r
}

type RouteNode interface {
	Parent() RouteNode
	HandlerTask() HandlerTask
	Name() string
	AggregatedAcceptances() []Acceptance
	Acceptances() []Acceptance
	Resources() map[string]RouteNode
	RouteType() RouteType
}

type _Node struct {
	parent      RouteNode
	handler     HandlerTask
	name        string
	acceptances []Acceptance
	resources   map[string]RouteNode
	routeType   RouteType
}

func (n *_Node) Parent() RouteNode {
	return n.parent
}

func (n *_Node) HandlerTask() HandlerTask {
	return n.handler
}

func (n *_Node) Name() string {
	return n.name
}

func (n *_Node) AggregatedAcceptances() []Acceptance {
	var acceptances []Acceptance
	var node RouteNode = n
	for ; node != nil; node = node.Parent() {
		if node.Acceptances() != nil && len(node.Acceptances()) > 0 {
			acceptances = append(node.Acceptances(), acceptances...)
		}
	}

	return acceptances
}

func (n *_Node) Acceptances() []Acceptance {
	return n.acceptances
}

func (n *_Node) Resources() map[string]RouteNode {
	return n.resources
}

func (n *_Node) RouteType() RouteType {
	return n.routeType
}

type RouteType int

const (
	RouteTypeEndPoint RouteType = iota
	RouteTypeGroup
	RouteTypeRecursiveEndPoint
	RouteTypeRootEndPoint
)

type _EndPoint struct {
	_Node
}

func NewEndPoint(name string, task HandlerTask, acceptances []Acceptance) *_EndPoint {
	if task == nil {
		return nil
	}

	point := _EndPoint{
		_Node: _Node{
			handler:     task,
			name:        name,
			acceptances: []Acceptance{},
			resources:   map[string]RouteNode{},
			routeType:   RouteTypeEndPoint,
		},
	}

	if acceptances != nil {
		point.acceptances = acceptances
	}

	return &point
}

func (ep *_EndPoint) AddEndPoint(point *_EndPoint) *_EndPoint {
	if point == nil {
		kklogger.ErrorJ("ghttp:_EndPoint.AddEndPoint#add_endpoint!nil_task", "add nil task")
		return nil
	}

	if ep.resources[point.Name()] != nil {
		kklogger.ErrorJ("ghttp:_EndPoint.AddEndPoint#add_endpoint!duplicate_name", "add same name endpoint")
		return nil
	}

	point.parent = ep
	ep.resources[point.Name()] = point
	if point.handler != nil {
		point.handler.Register()
	}

	return ep
}

func (ep *_EndPoint) AddGroup(group *_RouteGroup) *_EndPoint {
	if group == nil {
		kklogger.ErrorJ("ghttp:_EndPoint.AddGroup#add_group!nil_group", "add nil group")
		return nil
	}

	if ep.resources[group.Name()] != nil {
		kklogger.ErrorJ("ghttp:_EndPoint.AddGroup#add_group!duplicate_name", "add same name group")
		return nil
	}

	group.parent = ep
	ep.resources[group.Name()] = group
	return ep
}

func (ep *_EndPoint) AddRecursiveEndPoint(point *_EndPoint) *_EndPoint {
	if point == nil {
		kklogger.ErrorJ("ghttp:_EndPoint.AddRecursiveEndPoint#add_recursive!nil_task", "add nil task")
		return nil
	}

	if ep.resources[point.Name()] != nil {
		kklogger.ErrorJ("ghttp:_EndPoint.AddRecursiveEndPoint#add_recursive!duplicate_name", "add same name endpoint")
		return nil
	}

	point.parent = ep
	point.routeType = RouteTypeRecursiveEndPoint
	ep.resources[point.Name()] = point
	if point.handler != nil {
		point.handler.Register()
	}

	return ep
}

type _RouteGroup struct {
	_Node
}

func NewGroup(name string, acceptances []Acceptance) *_RouteGroup {
	group := _RouteGroup{
		_Node: _Node{
			name:        name,
			acceptances: []Acceptance{},
			resources:   map[string]RouteNode{},
			routeType:   RouteTypeGroup,
		},
	}

	if acceptances != nil {
		group.acceptances = acceptances
	}

	return &group
}

func (rg *_RouteGroup) AddGroup(group *_RouteGroup) *_RouteGroup {
	if group == nil {
		kklogger.ErrorJ("ghttp:_RouteGroup.AddGroup#add_group!nil_group", "add nil group")
		return nil
	}

	if rg.resources[group.Name()] != nil {
		kklogger.ErrorJ("ghttp:_RouteGroup.AddGroup#add_group!duplicate_name", "add same name group")
		return nil
	}

	group.parent = rg
	rg.resources[group.Name()] = group
	return rg
}

func (rg *_RouteGroup) AddEndPoint(point *_EndPoint) *_RouteGroup {
	if point == nil {
		kklogger.ErrorJ("ghttp:_RouteGroup.AddEndPoint#add_endpoint!nil_task", "add nil task")
		return nil
	}

	if rg.resources[point.Name()] != nil {
		kklogger.ErrorJ("ghttp:_RouteGroup.AddEndPoint#add_endpoint!duplicate_name", "add same name endpoint")
		return nil
	}

	point.parent = rg
	rg.resources[point.Name()] = point
	if point.handler != nil {
		point.handler.Register()
	}

	return rg
}

func (rg *_RouteGroup) AddRecursiveEndPoint(point *_EndPoint) *_RouteGroup {
	if point == nil {
		kklogger.ErrorJ("ghttp:_RouteGroup.AddRecursiveEndPoint#add_recursive!nil_task", "add nil task")
		return nil
	}

	if rg.resources[point.Name()] != nil {
		kklogger.ErrorJ("ghttp:_RouteGroup.AddRecursiveEndPoint#add_recursive!duplicate_name", "add same name endpoint")
		return nil
	}

	point.parent = rg
	point.routeType = RouteTypeRecursiveEndPoint
	rg.resources[point.Name()] = point
	if point.handler != nil {
		point.handler.Register()
	}

	return rg
}
