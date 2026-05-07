package ghttp

import (
	"slices"
	"testing"
)

func TestDefaultRouteEndpointOverridesSameNameGroupAndInheritsAcceptances(t *testing.T) {
	route := NewRoute()

	groupAcceptance := newMockAcceptance("group")
	endpointAcceptance := newMockAcceptance("endpoint")
	handler := newMockHandler("handler")

	route.AddGroup(NewGroup("t", []Acceptance{groupAcceptance}))
	route.AddEndPoint(NewEndPoint("t", handler, []Acceptance{endpointAcceptance}))

	node, params, isLast := route.RouteNode("/t")
	if node == nil {
		t.Fatal("Expected node to be found")
	}
	if !isLast {
		t.Fatal("Expected exact endpoint match")
	}
	if node.RouteType() != RouteTypeEndPoint {
		t.Fatalf("Expected endpoint to replace group, got route type %d", node.RouteType())
	}
	if node.HandlerTask() != handler {
		t.Fatal("Expected endpoint handler to be registered")
	}
	if len(params) != 0 {
		t.Fatalf("Expected no params for exact endpoint, got %v", params)
	}

	names := acceptanceNames(node.AggregatedAcceptances())
	expected := []string{"group", "endpoint"}
	if !slices.Equal(names, expected) {
		t.Fatalf("Expected acceptances %v, got %v", expected, names)
	}

	_, params, _ = route.RouteNode("/t/task-1")
	if id := handler.GetID("t", params); id != "task-1" {
		t.Fatalf("Expected endpoint id %q, got %q", "task-1", id)
	}
}

func TestDefaultRouteSameNameGroupAfterEndpointKeepsEndpointAndAddsAcceptance(t *testing.T) {
	route := NewRoute()

	endpointAcceptance := newMockAcceptance("endpoint")
	groupAcceptance := newMockAcceptance("group")
	handler := newMockHandler("handler")

	route.AddEndPoint(NewEndPoint("t", handler, []Acceptance{endpointAcceptance}))
	route.AddGroup(NewGroup("t", []Acceptance{groupAcceptance}))

	node, _, _ := route.RouteNode("/t")
	if node == nil {
		t.Fatal("Expected node to be found")
	}
	if node.RouteType() != RouteTypeEndPoint {
		t.Fatalf("Expected group declaration to keep endpoint route type, got %d", node.RouteType())
	}
	if node.HandlerTask() != handler {
		t.Fatal("Expected group declaration to keep endpoint handler")
	}

	names := acceptanceNames(node.AggregatedAcceptances())
	expected := []string{"group", "endpoint"}
	if !slices.Equal(names, expected) {
		t.Fatalf("Expected acceptances %v, got %v", expected, names)
	}
}

func TestDefaultRouteDuplicateGroupsMergeAcceptancesForDescendants(t *testing.T) {
	route := NewRoute()

	route.AddGroup(NewGroup("api", []Acceptance{newMockAcceptance("api-1")}).
		AddGroup(NewGroup("v1", []Acceptance{newMockAcceptance("v1")}).
			AddEndPoint(NewEndPoint("tasks", newMockHandler("tasks"), []Acceptance{newMockAcceptance("endpoint")})),
		),
	)
	route.AddGroup(NewGroup("api", []Acceptance{newMockAcceptance("api-2")}))

	node, _, _ := route.RouteNode("/api/v1/tasks")
	if node == nil {
		t.Fatal("Expected node to be found")
	}

	names := acceptanceNames(node.AggregatedAcceptances())
	expected := []string{"api-1", "api-2", "v1", "endpoint"}
	if !slices.Equal(names, expected) {
		t.Fatalf("Expected acceptances %v, got %v", expected, names)
	}
}

func TestDefaultRouteDuplicateGroupsMergeDescendants(t *testing.T) {
	route := NewRoute()

	route.AddGroup(NewGroup("api", []Acceptance{newMockAcceptance("api-1")}))
	route.AddGroup(NewGroup("api", []Acceptance{newMockAcceptance("api-2")}).
		AddEndPoint(NewEndPoint("tasks", newMockHandler("tasks"), []Acceptance{newMockAcceptance("endpoint")})),
	)

	node, _, _ := route.RouteNode("/api/tasks")
	if node == nil {
		t.Fatal("Expected merged descendant endpoint to be found")
	}

	names := acceptanceNames(node.AggregatedAcceptances())
	expected := []string{"api-1", "api-2", "endpoint"}
	if !slices.Equal(names, expected) {
		t.Fatalf("Expected acceptances %v, got %v", expected, names)
	}
}

func TestDefaultRouteGroupAfterEndpointMergesDescendants(t *testing.T) {
	route := NewRoute()

	route.AddEndPoint(NewEndPoint("api", newMockHandler("api"), []Acceptance{newMockAcceptance("endpoint")}))
	route.AddGroup(NewGroup("api", []Acceptance{newMockAcceptance("group")}).
		AddEndPoint(NewEndPoint("tasks", newMockHandler("tasks"), nil)),
	)

	node, _, _ := route.RouteNode("/api/tasks")
	if node == nil {
		t.Fatal("Expected merged descendant endpoint to be found")
	}

	names := acceptanceNames(node.AggregatedAcceptances())
	expected := []string{"group", "endpoint"}
	if !slices.Equal(names, expected) {
		t.Fatalf("Expected acceptances %v, got %v", expected, names)
	}
}

func TestDefaultRouteDuplicateEndpointPanics(t *testing.T) {
	route := NewRoute()

	route.AddEndPoint(NewEndPoint("login", newMockHandler("first"), nil))

	assertPanic(t, func() {
		route.AddEndPoint(NewEndPoint("login", newMockHandler("second"), nil))
	})
}

func TestDefaultRouteDuplicateRecursiveEndpointPanics(t *testing.T) {
	route := NewRoute()

	route.AddRecursivePoint(NewEndPoint("static", newMockHandler("first"), nil))

	assertPanic(t, func() {
		route.AddRecursivePoint(NewEndPoint("static", newMockHandler("second"), nil))
	})
}

func TestDefaultRouteDuplicateRootEndpointPanics(t *testing.T) {
	route := NewRoute()

	route.SetRoot(NewEndPoint("", newMockHandler("first"), nil))

	assertPanic(t, func() {
		route.SetRoot(NewEndPoint("", newMockHandler("second"), nil))
	})
}
