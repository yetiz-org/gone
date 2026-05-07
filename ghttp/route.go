package ghttp

import (
	"fmt"
	"slices"
	"strings"

	"github.com/yetiz-org/goth-kklogger"
)

type Route interface {
	RouteNode(path string) (node RouteNode, nodeParams map[string]any, isLast bool)
}

// RouteEntry describes one concrete route node in a route tree.
type RouteEntry struct {
	Path string
	Node RouteNode
}

// RouteEntriesProvider reports whether a route can enumerate its registered nodes.
type RouteEntriesProvider interface {
	RouteEntries() []RouteEntry
}

type DefaultRoute struct {
	root            RouteNode
	rootEndpointSet bool
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
						params[current.(*_EndPoint).paramKey] = resourceID
						return current, params, false
					}
				} else {
					return next, params, true
				}
			} else {
				if next == nil {
					if _, f := current.Resources()[resources[idx+1]]; f {
						params[current.(*_EndPoint).paramKey] = resourceID
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
				params[current.(*_EndPoint).paramKey] = resourceID
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

// RouteEntries returns the endpoint nodes registered in the route tree.
func (r *DefaultRoute) RouteEntries() []RouteEntry {
	return collectRouteEntries(r.root)
}

func (r *DefaultRoute) SetRoot(point *_EndPoint) *DefaultRoute {
	if r.rootEndpointSet {
		panic("ghttp: duplicate root endpoint")
	}

	point.routeType = RouteTypeRootEndPoint
	r.root = point
	r.rootEndpointSet = true
	return r
}

func (r *DefaultRoute) AddRecursivePoint(point *_EndPoint) *DefaultRoute {
	if point == nil {
		kklogger.ErrorJ("ghttp:DefaultRoute.AddRecursivePoint#add_recursive!nil_point", "add nil point")
		return nil
	}

	if r.root.Resources()[point.Name()] != nil {
		if group, ok := r.root.Resources()[point.Name()].(*_RouteGroup); ok {
			r.root.Resources()[point.Name()] = point
			point.parent = r.root
			point.routeType = RouteTypeRecursiveEndPoint
			point.setEndpointAcceptances(point.acceptances)
			point.inheritGroup(group)
			if point.handler != nil && !SkipHandlerRegister() {
				point.handler.Register()
			}
			return r
		}
		panic(fmt.Sprintf("ghttp: duplicate endpoint %q", point.Name()))
	}

	r.root.Resources()[point.Name()] = point
	point.parent = r.root
	point.routeType = RouteTypeRecursiveEndPoint
	if point.handler != nil && !SkipHandlerRegister() {
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
		mergeGroupIntoNode(r.root.Resources()[group.Name()], group)
		return r
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
		if group, ok := r.root.Resources()[point.Name()].(*_RouteGroup); ok {
			r.root.Resources()[point.Name()] = point
			point.parent = r.root
			point.setEndpointAcceptances(point.acceptances)
			point.inheritGroup(group)
			if point.handler != nil && !SkipHandlerRegister() {
				point.handler.Register()
			}
			return r
		}
		panic(fmt.Sprintf("ghttp: duplicate endpoint %q", point.Name()))
	}

	r.root.Resources()[point.Name()] = point
	point.parent = r.root
	if point.handler != nil && !SkipHandlerRegister() {
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
	parent               RouteNode
	handler              HandlerTask
	name                 string
	paramKey             string
	acceptances          []Acceptance
	groupAcceptanceCount int
	resources            map[string]RouteNode
	routeType            RouteType
}

// defaultParamKey is the "[gone-http]<name>_id" key used when no custom
// parameter mapping applies. Computed once at node construction so the
// router hot path avoids fmt.Sprintf per request.
func defaultParamKey(name string) string {
	return "[gone-http]" + name + "_id"
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

func (n *_Node) appendGroupAcceptances(acceptances []Acceptance) {
	if len(acceptances) == 0 {
		return
	}

	merged := make([]Acceptance, 0, len(n.acceptances)+len(acceptances))
	merged = append(merged, n.acceptances[:n.groupAcceptanceCount]...)
	merged = append(merged, acceptances...)
	merged = append(merged, n.acceptances[n.groupAcceptanceCount:]...)
	n.acceptances = merged
	n.groupAcceptanceCount += len(acceptances)
}

func (n *_Node) setEndpointAcceptances(acceptances []Acceptance) {
	merged := make([]Acceptance, 0, n.groupAcceptanceCount+len(acceptances))
	merged = append(merged, n.acceptances[:n.groupAcceptanceCount]...)
	merged = append(merged, acceptances...)
	n.acceptances = merged
}

type RouteType int

const (
	RouteTypeEndPoint RouteType = iota
	RouteTypeGroup
	RouteTypeRecursiveEndPoint
	RouteTypeRootEndPoint
)

func collectRouteEntries(root RouteNode) []RouteEntry {
	if root == nil {
		return nil
	}

	var entries []RouteEntry
	var walk func(node RouteNode)
	walk = func(node RouteNode) {
		switch node.RouteType() {
		case RouteTypeRootEndPoint, RouteTypeEndPoint, RouteTypeRecursiveEndPoint:
			entries = append(entries, RouteEntry{
				Path: routeNodePath(node),
				Node: node,
			})
		}

		names := make([]string, 0, len(node.Resources()))
		for name := range node.Resources() {
			names = append(names, name)
		}
		slices.Sort(names)
		for _, name := range names {
			walk(node.Resources()[name])
		}
	}

	walk(root)
	slices.SortFunc(entries, func(a RouteEntry, b RouteEntry) int {
		return strings.Compare(a.Path, b.Path)
	})
	return entries
}

func routeNodePath(node RouteNode) string {
	if node == nil || node.RouteType() == RouteTypeRootEndPoint {
		return "/"
	}

	var parts []string
	for current := node; current != nil && current.RouteType() != RouteTypeRootEndPoint; current = current.Parent() {
		if current.Name() == "" {
			continue
		}
		if current.RouteType() == RouteTypeRecursiveEndPoint || current.Name() == "*" {
			parts = append([]string{"*"}, parts...)
			if current.Name() != "*" {
				parts = append([]string{current.Name()}, parts...)
			}
			continue
		}
		parts = append([]string{current.Name()}, parts...)
	}

	if len(parts) == 0 {
		return "/"
	}
	return "/" + strings.Join(parts, "/")
}

func routeNodeCore(node RouteNode) *_Node {
	switch typed := node.(type) {
	case *_EndPoint:
		return &typed._Node
	case *_RouteGroup:
		return &typed._Node
	case *_SimpleNode:
		return &typed._Node
	default:
		panic(fmt.Sprintf("ghttp: unsupported route node %T", node))
	}
}

func mergeGroupIntoNode(node RouteNode, group *_RouteGroup) {
	switch existing := node.(type) {
	case *_EndPoint:
		existing.inheritGroup(group)
	case *_RouteGroup:
		existing.appendGroupAcceptances(group.acceptances[:group.groupAcceptanceCount])
		mergeRouteResources(existing, group.resources)
	default:
		panic(fmt.Sprintf("ghttp: unsupported route node %T", node))
	}
}

func mergeRouteResources(parent RouteNode, resources map[string]RouteNode) {
	target := parent.Resources()
	for name, incoming := range resources {
		if existing := target[name]; existing != nil {
			target[name] = mergeRouteNode(parent, existing, incoming)
			continue
		}

		routeNodeCore(incoming).parent = parent
		target[name] = incoming
	}
}

func mergeRouteNode(parent RouteNode, existing RouteNode, incoming RouteNode) RouteNode {
	switch incomingNode := incoming.(type) {
	case *_RouteGroup:
		mergeGroupIntoNode(existing, incomingNode)
		return existing
	case *_EndPoint:
		if existingGroup, ok := existing.(*_RouteGroup); ok {
			incomingNode.parent = parent
			incomingNode.setEndpointAcceptances(incomingNode.acceptances)
			incomingNode.inheritGroup(existingGroup)
			return incomingNode
		}
		panic(fmt.Sprintf("ghttp: duplicate endpoint %q", incomingNode.Name()))
	default:
		panic(fmt.Sprintf("ghttp: unsupported route node %T", incoming))
	}
}

type _EndPoint struct {
	_Node
}

func (ep *_EndPoint) inheritGroup(group *_RouteGroup) {
	ep.appendGroupAcceptances(group.acceptances[:group.groupAcceptanceCount])
	mergeRouteResources(ep, group.resources)
}

func NewEndPoint(name string, task HandlerTask, acceptances []Acceptance) *_EndPoint {
	if task == nil {
		return nil
	}

	point := _EndPoint{
		_Node: _Node{
			handler:     task,
			name:        name,
			paramKey:    defaultParamKey(name),
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
		if group, ok := ep.resources[point.Name()].(*_RouteGroup); ok {
			ep.resources[point.Name()] = point
			point.parent = ep
			point.setEndpointAcceptances(point.acceptances)
			point.inheritGroup(group)
			if point.handler != nil && !SkipHandlerRegister() {
				point.handler.Register()
			}
			return ep
		}
		panic(fmt.Sprintf("ghttp: duplicate endpoint %q", point.Name()))
	}

	point.parent = ep
	ep.resources[point.Name()] = point
	if point.handler != nil && !SkipHandlerRegister() {
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
		mergeGroupIntoNode(ep.resources[group.Name()], group)
		return ep
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
		if group, ok := ep.resources[point.Name()].(*_RouteGroup); ok {
			ep.resources[point.Name()] = point
			point.parent = ep
			point.routeType = RouteTypeRecursiveEndPoint
			point.setEndpointAcceptances(point.acceptances)
			point.inheritGroup(group)
			if point.handler != nil && !SkipHandlerRegister() {
				point.handler.Register()
			}
			return ep
		}
		panic(fmt.Sprintf("ghttp: duplicate endpoint %q", point.Name()))
	}

	point.parent = ep
	point.routeType = RouteTypeRecursiveEndPoint
	ep.resources[point.Name()] = point
	if point.handler != nil && !SkipHandlerRegister() {
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
		group.groupAcceptanceCount = len(acceptances)
	}

	return &group
}

func (rg *_RouteGroup) AddGroup(group *_RouteGroup) *_RouteGroup {
	if group == nil {
		kklogger.ErrorJ("ghttp:_RouteGroup.AddGroup#add_group!nil_group", "add nil group")
		return nil
	}

	if rg.resources[group.Name()] != nil {
		mergeGroupIntoNode(rg.resources[group.Name()], group)
		return rg
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
		if group, ok := rg.resources[point.Name()].(*_RouteGroup); ok {
			rg.resources[point.Name()] = point
			point.parent = rg
			point.setEndpointAcceptances(point.acceptances)
			point.inheritGroup(group)
			if point.handler != nil && !SkipHandlerRegister() {
				point.handler.Register()
			}
			return rg
		}
		panic(fmt.Sprintf("ghttp: duplicate endpoint %q", point.Name()))
	}

	point.parent = rg
	rg.resources[point.Name()] = point
	if point.handler != nil && !SkipHandlerRegister() {
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
		if group, ok := rg.resources[point.Name()].(*_RouteGroup); ok {
			rg.resources[point.Name()] = point
			point.parent = rg
			point.routeType = RouteTypeRecursiveEndPoint
			point.setEndpointAcceptances(point.acceptances)
			point.inheritGroup(group)
			if point.handler != nil && !SkipHandlerRegister() {
				point.handler.Register()
			}
			return rg
		}
		panic(fmt.Sprintf("ghttp: duplicate endpoint %q", point.Name()))
	}

	point.parent = rg
	point.routeType = RouteTypeRecursiveEndPoint
	rg.resources[point.Name()] = point
	if point.handler != nil && !SkipHandlerRegister() {
		point.handler.Register()
	}

	return rg
}
