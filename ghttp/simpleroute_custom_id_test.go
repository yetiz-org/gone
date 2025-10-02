package ghttp

import (
	"testing"
)

func TestSimpleRouteCustomIDNames(t *testing.T) {
	route := NewSimpleRoute()

	// First, set up the parent node as an endpoint
	route.SetEndpoint("/api/v1/organizations", nil)
	// Define child route: /api/v1/organizations/:org_id/permissions/:user_id
	route.SetGroup("/api/v1")
	route.SetEndpoint("/api/v1/organizations/:org_id/permissions/:user_id", nil)

	// 1. Test parent node itself can be accessed (without ID)
	parentNode, parentParams, _ := route.RouteNode("/api/v1/organizations")
	if parentNode == nil {
		t.Fatal("Expected to find parent node /api/v1/organizations, but got nil")
	}
	if len(parentParams) != 0 {
		t.Errorf("Expected no params for parent node, got: %v", parentParams)
	}

	// 2. Test accessing parent node ID: /api/v1/organizations/123
	// When accessing parent node's own ID, should use default organizations_id (parent has no custom ID defined)
	parentIDNode, parentIDParams, _ := route.RouteNode("/api/v1/organizations/123")
	if parentIDNode == nil {
		t.Fatal("Expected to find parent node with ID, but got nil")
	}
	orgIDFromParent, exists := parentIDParams["[gone-http]organizations_id"]
	if !exists {
		t.Errorf("Expected param '[gone-http]organizations_id' when accessing parent ID, available params: %v", parentIDParams)
	}
	if orgIDFromParent != "123" {
		t.Errorf("Expected organizations_id to be '123', got '%v'", orgIDFromParent)
	}

	// 3. Test full child path: /api/v1/organizations/123/permissions/456
	node, params, _ := route.RouteNode("/api/v1/organizations/123/permissions/456")

	if node == nil {
		t.Fatal("Expected to find route node, but got nil")
	}

	// Verify parameter name uses custom org_id instead of organizations_id
	orgID, exists := params["[gone-http]org_id"]
	if !exists {
		t.Errorf("Expected param '[gone-http]org_id' to exist, available params: %v", params)
	}
	if orgID != "123" {
		t.Errorf("Expected org_id to be '123', got '%v'", orgID)
	}

	// Verify parameter name uses custom user_id instead of permissions_id
	userID, exists := params["[gone-http]user_id"]
	if !exists {
		t.Errorf("Expected param '[gone-http]user_id' to exist, available params: %v", params)
	}
	if userID != "456" {
		t.Errorf("Expected user_id to be '456', got '%v'", userID)
	}

	// Confirm old parameter names should not exist
	if _, exists := params["[gone-http]organizations_id"]; exists {
		t.Error("Should not have '[gone-http]organizations_id' param in child route")
	}
	if _, exists := params["[gone-http]permissions_id"]; exists {
		t.Error("Should not have '[gone-http]permissions_id' param")
	}
}

func TestSimpleRouteDefaultIDNames(t *testing.T) {
	route := NewSimpleRoute()

	// First, set up the parent node as an endpoint
	route.SetEndpoint("/api/v1/organizations", nil)
	// Test case without custom ID names, should use default {name}_id format
	route.SetEndpoint("/api/v1/organizations/users", nil)

	// 1. Test parent node itself can be accessed (without ID)
	parentNode, parentParams, _ := route.RouteNode("/api/v1/organizations")
	if parentNode == nil {
		t.Fatal("Expected to find parent node /api/v1/organizations, but got nil")
	}
	if len(parentParams) != 0 {
		t.Errorf("Expected no params for parent node, got: %v", parentParams)
	}

	// 2. Test accessing parent node ID: /api/v1/organizations/123
	// Since no custom ID defined for organizations itself, should use default organizations_id
	parentIDNode, parentIDParams, _ := route.RouteNode("/api/v1/organizations/123")
	if parentIDNode == nil {
		t.Fatal("Expected to find parent node with ID, but got nil")
	}
	orgIDFromParent, exists := parentIDParams["[gone-http]organizations_id"]
	if !exists {
		t.Errorf("Expected param '[gone-http]organizations_id' when accessing parent ID, available params: %v", parentIDParams)
	}
	if orgIDFromParent != "123" {
		t.Errorf("Expected organizations_id to be '123', got '%v'", orgIDFromParent)
	}

	// 3. Test child route: /api/v1/organizations/users/789
	node, params, _ := route.RouteNode("/api/v1/organizations/users/789")

	if node == nil {
		t.Fatal("Expected to find route node, but got nil")
	}

	// Should use default users_id
	userID, exists := params["[gone-http]users_id"]
	if !exists {
		t.Errorf("Expected param '[gone-http]users_id' to exist, available params: %v", params)
	}
	if userID != "789" {
		t.Errorf("Expected users_id to be '789', got '%v'", userID)
	}
}

func TestSimpleRouteMixedIDNames(t *testing.T) {
	route := NewSimpleRoute()

	// First, set up the parent node as an endpoint
	route.SetEndpoint("/api/v1/organizations", nil)
	// Test mixed case: organizations uses custom :org_id, permissions uses default ID name
	route.SetEndpoint("/api/v1/organizations/:org_id/permissions", nil)

	// 1. Test parent node itself can be accessed (without ID)
	parentNode, parentParams, _ := route.RouteNode("/api/v1/organizations")
	if parentNode == nil {
		t.Fatal("Expected to find parent node /api/v1/organizations, but got nil")
	}
	if len(parentParams) != 0 {
		t.Errorf("Expected no params for parent node, got: %v", parentParams)
	}

	// 2. Test accessing parent node ID: /api/v1/organizations/123
	// When accessing parent node's own ID, should use default organizations_id (parent has no custom ID defined)
	parentIDNode, parentIDParams, _ := route.RouteNode("/api/v1/organizations/123")
	if parentIDNode == nil {
		t.Fatal("Expected to find parent node with ID, but got nil")
	}
	orgIDFromParent, exists := parentIDParams["[gone-http]organizations_id"]
	if !exists {
		t.Errorf("Expected param '[gone-http]organizations_id' when accessing parent ID, available params: %v", parentIDParams)
	}
	if orgIDFromParent != "123" {
		t.Errorf("Expected organizations_id to be '123', got '%v'", orgIDFromParent)
	}

	// 3. Test path: /api/v1/organizations/123/permissions/456
	node, params, _ := route.RouteNode("/api/v1/organizations/123/permissions/456")

	if node == nil {
		t.Fatal("Expected to find route node, but got nil")
	}

	// Verify organizations uses custom org_id
	orgID, exists := params["[gone-http]org_id"]
	if !exists {
		t.Errorf("Expected param '[gone-http]org_id' to exist, available params: %v", params)
	}
	if orgID != "123" {
		t.Errorf("Expected org_id to be '123', got '%v'", orgID)
	}

	// Verify permissions uses default permissions_id (since no custom ID was specified)
	permID, exists := params["[gone-http]permissions_id"]
	if !exists {
		t.Errorf("Expected param '[gone-http]permissions_id' to exist, available params: %v", params)
	}
	if permID != "456" {
		t.Errorf("Expected permissions_id to be '456', got '%v'", permID)
	}

	// Confirm incorrect parameter names should not exist
	if _, exists := params["[gone-http]organizations_id"]; exists {
		t.Error("Should not have '[gone-http]organizations_id' param in child route")
	}
}

func TestSimpleRouteWithParentEndpoint(t *testing.T) {
	route := NewSimpleRoute()

	// First, set up the parent node as an endpoint
	route.SetEndpoint("/api/v1/organizations", nil)
	// Then set up child route, permissions uses custom :user_id
	route.SetEndpoint("/api/v1/organizations/permissions/:user_id", nil)

	// 1. Test parent node itself can be accessed (without ID)
	parentNode, parentParams, _ := route.RouteNode("/api/v1/organizations")
	if parentNode == nil {
		t.Fatal("Expected to find parent node /api/v1/organizations, but got nil")
	}
	if len(parentParams) != 0 {
		t.Errorf("Expected no params for parent node, got: %v", parentParams)
	}

	// 2. Test accessing parent node ID: /api/v1/organizations/123
	// Since no custom ID defined for organizations itself, should use default organizations_id
	parentIDNode, parentIDParams, _ := route.RouteNode("/api/v1/organizations/123")
	if parentIDNode == nil {
		t.Fatal("Expected to find parent node with ID, but got nil")
	}
	orgIDFromParent, exists := parentIDParams["[gone-http]organizations_id"]
	if !exists {
		t.Errorf("Expected param '[gone-http]organizations_id' when accessing parent ID, available params: %v", parentIDParams)
	}
	if orgIDFromParent != "123" {
		t.Errorf("Expected organizations_id to be '123', got '%v'", orgIDFromParent)
	}

	// 3. Test path: /api/v1/organizations/123/permissions/456
	node, params, _ := route.RouteNode("/api/v1/organizations/123/permissions/456")

	if node == nil {
		t.Fatal("Expected to find route node, but got nil")
	}

	// Verify organizations uses default organizations_id (since no custom ID was specified)
	orgID, exists := params["[gone-http]organizations_id"]
	if !exists {
		t.Errorf("Expected param '[gone-http]organizations_id' to exist, available params: %v", params)
	}
	if orgID != "123" {
		t.Errorf("Expected organizations_id to be '123', got '%v'", orgID)
	}

	// Verify permissions uses custom user_id
	userID, exists := params["[gone-http]user_id"]
	if !exists {
		t.Errorf("Expected param '[gone-http]user_id' to exist, available params: %v", params)
	}
	if userID != "456" {
		t.Errorf("Expected user_id to be '456', got '%v'", userID)
	}

	// Confirm no incorrect parameter names
	if _, exists := params["[gone-http]permissions_id"]; exists {
		t.Error("Should not have '[gone-http]permissions_id' param")
	}
}
