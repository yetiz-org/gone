package ghttp

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type _SimpleNode struct {
	_Node
}

// _SimpleNodeWrapper wraps a node with custom parameter name for specific route context
type _SimpleNodeWrapper struct {
	RouteNode
	customName    string
	wrappedParent RouteNode
}

func (w *_SimpleNodeWrapper) Name() string {
	return w.customName
}

func (w *_SimpleNodeWrapper) Parent() RouteNode {
	if w.wrappedParent != nil {
		return w.wrappedParent
	}
	return w.RouteNode.Parent()
}

func (n *_SimpleNode) path() string {
	rtn := ""
	var current RouteNode = n
	if current.RouteType() == RouteTypeRootEndPoint {
		return "/"
	}

	for {
		switch current.RouteType() {
		case RouteTypeRootEndPoint:
			rtn = fmt.Sprintf("/%s", rtn)
		case RouteTypeEndPoint:
			rtn = fmt.Sprintf("%s/:%s/%s", current.Name(), current.Name(), rtn)
		case RouteTypeRecursiveEndPoint:
			rtn = fmt.Sprintf("%s/%s*", current.Name(), rtn)
		case RouteTypeGroup:
			rtn = fmt.Sprintf("%s/%s", current.Name(), rtn)
		}

		if current.Parent() == nil {
			break
		}

		current = current.Parent()
	}

	return strings.TrimRight(rtn, "/")
}

// endpointParamMapping stores custom parameter names for a specific endpoint path
type endpointParamMapping struct {
	// Map from node name to custom param name (e.g., "organizations" -> "org_id")
	nodeToParamName map[string]string
}

type SimpleRoute struct {
	root             RouteNode
	endpointMappings map[string]*endpointParamMapping // Key is the normalized endpoint path
}

func NewSimpleRoute() *SimpleRoute {
	return &SimpleRoute{
		root: &_SimpleNode{
			_Node: _Node{
				parent:    nil,
				name:      "",
				resources: map[string]RouteNode{},
				routeType: RouteTypeRootEndPoint,
			},
		},
		endpointMappings: make(map[string]*endpointParamMapping),
	}
}

func (r *SimpleRoute) traverse(node RouteNode, result map[string]int) {
	if len(node.Resources()) > 0 {
		for _, n := range node.Resources() {
			r.traverse(n, result)
		}
	}

	switch node.RouteType() {
	case RouteTypeRootEndPoint, RouteTypeEndPoint:
		result[node.(*_SimpleNode).path()] = 1
	case RouteTypeRecursiveEndPoint:
		result[node.(*_SimpleNode).path()] = 1
	}
}

func (r *SimpleRoute) String() string {
	traverse := map[string]int{}
	r.traverse(r.root, traverse)
	var paths []string
	for path := range traverse {
		paths = append(paths, path)
	}

	sort.Strings(paths)
	marshal, _ := json.Marshal(paths)
	return string(marshal)
}

func (r *SimpleRoute) SetRoot(handler HandlerTask, acceptances ...Acceptance) *SimpleRoute {
	r.root.(*_SimpleNode).handler = handler
	r.root.(*_SimpleNode).acceptances = acceptances
	return r
}

func (r *SimpleRoute) SetGroup(path string, acceptances ...Acceptance) *SimpleRoute {
	path = strings.TrimLeft(strings.TrimRight(path, "/"), "/")
	if path == "" {
		r.root.(*_SimpleNode).acceptances = acceptances
		return r
	}

	current := r.root
	parts := strings.Split(path, "/")
	partsLen := len(parts)
	for idx, part := range parts {
		if strings.Index(part, ":") == 0 {
			continue
		}

		if v, f := current.Resources()[part]; f {
			current = v
		} else {
			node := &_SimpleNode{
				_Node: _Node{
					parent:    current,
					name:      part,
					resources: map[string]RouteNode{},
					routeType: RouteTypeGroup,
				},
			}

			if idx+1 == partsLen {
				node.acceptances = acceptances
			}

			current.Resources()[part] = node
			current = node
		}
	}

	return r
}

func (r *SimpleRoute) SetEndpoint(path string, handler HandlerTask, acceptances ...Acceptance) *SimpleRoute {
	path = strings.TrimLeft(strings.TrimRight(path, "/"), "/")
	if path == "" {
		r.root.(*_SimpleNode).handler = handler
		r.root.(*_SimpleNode).acceptances = acceptances
		return r
	}

	current := r.root
	parts := strings.Split(path, "/")
	partsLen := len(parts)

	mapping := &endpointParamMapping{
		nodeToParamName: make(map[string]string),
	}
	hasCustomParams := false

	for idx, part := range parts {
		if strings.Index(part, ":") == 0 {
			paramName := strings.TrimPrefix(part, ":")
			// Only recognize custom ID if it ends with "_id"
			if strings.HasSuffix(paramName, "_id") {
				mapping.nodeToParamName[current.(*_SimpleNode).name] = paramName
				hasCustomParams = true
			}

			current.(*_SimpleNode).routeType = RouteTypeEndPoint
			if idx+1 == partsLen {
				current.(*_SimpleNode).handler = handler
				current.(*_SimpleNode).acceptances = acceptances
			}

			continue
		}

		if part == "*" {
			wildcardNode := &_SimpleNode{
				_Node: _Node{
					parent:      current,
					name:        "*",
					resources:   map[string]RouteNode{},
					routeType:   RouteTypeRecursiveEndPoint,
					handler:     handler,
					acceptances: acceptances,
				},
			}
			current.Resources()["*"] = wildcardNode
			return r
		}

		if v, f := current.Resources()[part]; f {
			current = v
		} else {
			node := &_SimpleNode{
				_Node: _Node{
					parent:    current,
					name:      part,
					resources: map[string]RouteNode{},
					routeType: RouteTypeGroup,
				},
			}

			if idx+1 == partsLen {
				node.routeType = RouteTypeEndPoint
				node.handler = handler
				node.acceptances = acceptances
			}

			current.Resources()[part] = node
			current = node
		}
	}

	if hasCustomParams {
		r.endpointMappings[path] = mapping
	}

	if handler != nil {
		handler.Register()
	}

	return r
}

func (r *SimpleRoute) FindNode(path string) RouteNode {
	routeNode, _, _ := r.RouteNode(path)
	return routeNode
}

func (r *SimpleRoute) RouteNode(path string) (node RouteNode, parameters map[string]any, isLast bool) {
	path = strings.TrimLeft(strings.TrimRight(path, "/"), "/")
	params := map[string]any{}
	if path == "" {
		return r.root, nil, true
	}

	parts := strings.Split(path, "/")
	nodeLens := len(parts)
	current := r.root
	next := r.root
	var matchedNodes []RouteNode

	for idx, part := range parts {
		next = current.Resources()[part]
		matchedNodes = append(matchedNodes, current)

		switch current.RouteType() {
		case RouteTypeRootEndPoint, RouteTypeEndPoint:
			if idx+1 == nodeLens {
				if next == nil {
					if current == r.root && part != "" {
						return nil, nil, false
					} else {
						paramKey := r.getParamKeyForPath(current, matchedNodes, parts)
						params[paramKey] = part
						returnNode := r.wrapNodeChainIfNeeded(current, matchedNodes, parts)
						return returnNode, params, false
					}
				} else {
					returnNode := r.wrapNodeChainIfNeeded(next, append(matchedNodes, next), parts)
					return returnNode, params, true
				}
			} else {
				if next == nil {
					if _, f := current.Resources()[parts[idx+1]]; f {
						paramKey := r.getParamKeyForPath(current, matchedNodes, parts)
						params[paramKey] = part
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
				params[current.Name()] = part
			}

			returnNode := r.wrapNodeChainIfNeeded(current, matchedNodes, parts)
			return returnNode, params, false
		case RouteTypeGroup:
			if next == nil {
				if wildcardNode, hasWildcard := current.Resources()["*"]; hasWildcard {
					wildcardValue := strings.Join(parts[idx:], "/")
					params["*"] = wildcardValue
					return wildcardNode, params, false
				}
				return nil, nil, false
			}

			current = next
		}
	}

	if current.RouteType() == RouteTypeGroup {
		return nil, nil, false
	}

	finalNode := r.wrapNodeChainIfNeeded(current, matchedNodes, parts)
	return finalNode, params, current == next
}

func (r *SimpleRoute) findBestMatchingMapping(matchedNodes []RouteNode, pathParts []string) *endpointParamMapping {
	if len(r.endpointMappings) == 0 {
		return nil
	}

	for endpointPath, mapping := range r.endpointMappings {
		if r.pathMatchesEndpoint(matchedNodes, pathParts, endpointPath) {
			return mapping
		}
	}

	return nil
}

func (r *SimpleRoute) pathMatchesEndpoint(matchedNodes []RouteNode, pathParts []string, endpointPath string) bool {
	patternParts := strings.Split(strings.TrimLeft(endpointPath, "/"), "/")

	if len(patternParts) > len(pathParts) {
		return false
	}

	for i, patternPart := range patternParts {
		if strings.HasPrefix(patternPart, ":") {
			continue
		}

		if i >= len(pathParts) || pathParts[i] != patternPart {
			return false
		}
	}

	return true
}

func (r *SimpleRoute) getParamKeyForPath(node RouteNode, matchedNodes []RouteNode, pathParts []string) string {
	mapping := r.findBestMatchingMapping(matchedNodes, pathParts)
	if mapping != nil {
		if paramName, ok := mapping.nodeToParamName[node.Name()]; ok {
			return fmt.Sprintf("[gone-http]%s", paramName)
		}
	}
	return fmt.Sprintf("[gone-http]%s_id", node.Name())
}

func (r *SimpleRoute) wrapNodeChainIfNeeded(node RouteNode, matchedNodes []RouteNode, pathParts []string) RouteNode {
	mapping := r.findBestMatchingMapping(matchedNodes, pathParts)
	if mapping == nil || len(mapping.nodeToParamName) == 0 {
		return node
	}

	var wrappedParent RouteNode
	if node.Parent() != nil {
		wrappedParent = r.wrapNodeChainIfNeeded(node.Parent(), matchedNodes, pathParts)
	}

	if paramName, ok := mapping.nodeToParamName[node.Name()]; ok {
		customName := paramName
		if strings.HasSuffix(paramName, "_id") {
			customName = strings.TrimSuffix(paramName, "_id")
		}

		return &_SimpleNodeWrapper{
			RouteNode:     node,
			customName:    customName,
			wrappedParent: wrappedParent,
		}
	}

	if wrappedParent != node.Parent() {
		return &_SimpleNodeWrapper{
			RouteNode:     node,
			customName:    node.Name(),
			wrappedParent: wrappedParent,
		}
	}

	return node
}
