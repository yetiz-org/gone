package ghttp

import (
	"github.com/stretchr/testify/mock"
)

// MockRoute is a mock implementation of Route interface
// It provides complete testify/mock integration for testing route behaviors
type MockRoute struct {
	mock.Mock
}

// NewMockRoute creates a new MockRoute instance
func NewMockRoute() *MockRoute {
	return &MockRoute{}
}

// RouteNode returns a route node for the given path
func (m *MockRoute) RouteNode(path string) (node RouteNode, nodeParams map[string]any, isLast bool) {
	args := m.Called(path)
	var routeNode RouteNode
	if args.Get(0) != nil {
		routeNode = args.Get(0).(RouteNode)
	}
	var params map[string]any
	if args.Get(1) != nil {
		params = args.Get(1).(map[string]any)
	}
	return routeNode, params, args.Bool(2)
}

// MockRouteNode is a mock implementation of RouteNode interface
// It provides complete testify/mock integration for testing route node behaviors
type MockRouteNode struct {
	mock.Mock
}

// NewMockRouteNode creates a new MockRouteNode instance
func NewMockRouteNode() *MockRouteNode {
	return &MockRouteNode{}
}

// Parent returns the parent route node
func (m *MockRouteNode) Parent() RouteNode {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(RouteNode)
}

// HandlerTask returns the handler task for this node
func (m *MockRouteNode) HandlerTask() HandlerTask {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(HandlerTask)
}

// Name returns the name of this route node
func (m *MockRouteNode) Name() string {
	args := m.Called()
	return args.String(0)
}

// AggregatedAcceptances returns the aggregated acceptances for this node
func (m *MockRouteNode) AggregatedAcceptances() []Acceptance {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).([]Acceptance)
}

// Acceptances returns the acceptances for this node
func (m *MockRouteNode) Acceptances() []Acceptance {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).([]Acceptance)
}

// Resources returns the child resources for this node
func (m *MockRouteNode) Resources() map[string]RouteNode {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(map[string]RouteNode)
}

// RouteType returns the type of this route node
func (m *MockRouteNode) RouteType() RouteType {
	args := m.Called()
	return RouteType(args.Int(0))
}

// Ensure interface compliance
var _ Route = (*MockRoute)(nil)
var _ RouteNode = (*MockRouteNode)(nil)
