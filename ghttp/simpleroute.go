package ghttp

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type _SimpleNode struct {
	_Node
	parentCustomNames map[string]string // Custom node names for this endpoint (ID = name + "_id")
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

type SimpleRoute struct {
	root RouteNode
}

func NewSimpleRoute() *SimpleRoute {
	return &SimpleRoute{root: &_SimpleNode{
		_Node: _Node{
			parent:    nil,
			name:      "",
			resources: map[string]RouteNode{},
			routeType: RouteTypeRootEndPoint,
		},
	}}
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
	
	// Collect custom node name mappings in the path
	customNameMap := make(map[string]string) // originalName -> customName
	var nodePath []string // Track node path
	
	for idx, part := range parts {
		if strings.Index(part, ":") == 0 {
			// Extract custom name from :custom_id format (remove : prefix and _id suffix)
			if len(nodePath) > 0 {
				customIDName := strings.TrimPrefix(part, ":")
				originalName := nodePath[len(nodePath)-1]
				customNameMap[originalName] = strings.TrimSuffix(customIDName, "_id")
			}
			
			current.(*_SimpleNode).routeType = RouteTypeEndPoint
			if idx+1 == partsLen {
				// Store custom node name mapping to the endpoint node
				if len(customNameMap) > 0 {
					current.(*_SimpleNode).parentCustomNames = make(map[string]string)
					for k, v := range customNameMap {
						current.(*_SimpleNode).parentCustomNames[k] = v
					}
				}
				current.(*_SimpleNode).handler = handler
				current.(*_SimpleNode).acceptances = acceptances
			}

			continue
		}

		nodePath = append(nodePath, part)
		
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
				// Create an endpoint node and set its properties
				node.routeType = RouteTypeEndPoint
				node.handler = handler
				node.acceptances = acceptances
				// Store custom node name mapping
				if len(customNameMap) > 0 {
					node.parentCustomNames = make(map[string]string)
					for k, v := range customNameMap {
						node.parentCustomNames[k] = v
					}
				}
			}

			current.Resources()[part] = node
			current = node
		}
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
	
	// Track nodes in the path and their corresponding part values
	type pathItem struct {
		node RouteNode
		part string
	}
	var pathItems []pathItem
	
	for idx, part := range parts {
		next = current.Resources()[part]
		switch current.RouteType() {
		case RouteTypeRootEndPoint, RouteTypeEndPoint:
			if idx+1 == nodeLens {
				if next == nil {
					if current == r.root && part != "" {
						return nil, nil, false
					} else {
						pathItems = append(pathItems, pathItem{node: current, part: part})
						// Use the current endpoint's parentCustomNames mapping
						for _, item := range pathItems {
							// Get custom node name if defined
							nodeName := item.node.Name()
							if currentNode, ok := current.(*_SimpleNode); ok && currentNode.parentCustomNames != nil {
								if customName, exists := currentNode.parentCustomNames[nodeName]; exists {
									nodeName = customName
								}
							}
							
							// Derive ID parameter name from node name
							paramName := fmt.Sprintf("%s_id", nodeName)
							params[fmt.Sprintf("[gone-http]%s", paramName)] = item.part
						}
						return current, params, false
					}
				} else {
					return next, params, true
				}
			} else {
				if next == nil {
					if _, f := current.Resources()[parts[idx+1]]; f {
						pathItems = append(pathItems, pathItem{node: current, part: part})
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

			return current, params, false
		case RouteTypeGroup:
			if next == nil {
				return nil, nil, false
			}

			current = next
		}
	}

	if current.RouteType() == RouteTypeGroup {
		return nil, nil, false
	}

	// Use the current endpoint's parentCustomNames mapping
	for _, item := range pathItems {
		// Get custom node name if defined
		nodeName := item.node.Name()
		if currentNode, ok := current.(*_SimpleNode); ok && currentNode.parentCustomNames != nil {
			if customName, exists := currentNode.parentCustomNames[nodeName]; exists {
				nodeName = customName
			}
		}
		
		// Derive ID parameter name from node name
		paramName := fmt.Sprintf("%s_id", nodeName)
		params[fmt.Sprintf("[gone-http]%s", paramName)] = item.part
	}

	return current, params, current == next
}
