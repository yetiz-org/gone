package ghttp

import (
	"testing"
)

// TestRouteWithoutCustomID verifies behavior when no custom IDs are defined
func TestRouteWithoutCustomID(t *testing.T) {
	route := NewSimpleRoute()

	// Set up parent and child endpoints without custom IDs
	route.SetEndpoint("/api/v1/organizations", nil)
	route.SetEndpoint("/api/v1/organizations/permissions", nil)

	// 1. Test parent node itself (no ID)
	parentNode, parentParams, _ := route.RouteNode("/api/v1/organizations")
	if parentNode == nil {
		t.Fatal("Expected to find parent node /api/v1/organizations, but got nil")
	}
	if parentNode.Name() != "organizations" {
		t.Errorf("Expected parent node name to be 'organizations', got '%s'", parentNode.Name())
	}
	if len(parentParams) != 0 {
		t.Errorf("Expected no params for parent node, got: %v", parentParams)
	}

	// 2. Test parent node with ID
	parentIDNode, parentIDParams, _ := route.RouteNode("/api/v1/organizations/123")
	if parentIDNode == nil {
		t.Fatal("Expected to find parent node with ID, but got nil")
	}
	if parentIDNode.Name() != "organizations" {
		t.Errorf("Expected node name to be 'organizations', got '%s'", parentIDNode.Name())
	}
	orgID, exists := parentIDParams["[gone-http]organizations_id"]
	if !exists {
		t.Errorf("Expected param '[gone-http]organizations_id', available params: %v", parentIDParams)
	}
	if orgID != "123" {
		t.Errorf("Expected organizations_id to be '123', got '%v'", orgID)
	}

	// 3. Test child route with ID
	childNode, childParams, _ := route.RouteNode("/api/v1/organizations/permissions/456")
	if childNode == nil {
		t.Fatal("Expected to find child node, but got nil")
	}
	if childNode.Name() != "permissions" {
		t.Errorf("Expected child node name to be 'permissions', got '%s'", childNode.Name())
	}
	permID, exists := childParams["[gone-http]permissions_id"]
	if !exists {
		t.Errorf("Expected param '[gone-http]permissions_id', available params: %v", childParams)
	}
	if permID != "456" {
		t.Errorf("Expected permissions_id to be '456', got '%v'", permID)
	}
}

// TestRouteSingleCustomID verifies behavior with one custom ID
func TestRouteSingleCustomID(t *testing.T) {
	route := NewSimpleRoute()

	// Set up parent endpoint and child with custom ID :user_id
	route.SetEndpoint("/api/v1/organizations", nil)
	route.SetEndpoint("/api/v1/organizations/permissions/:user_id", nil)

	// 1. Test parent node itself (no ID)
	parentNode, parentParams, _ := route.RouteNode("/api/v1/organizations")
	if parentNode == nil {
		t.Fatal("Expected to find parent node /api/v1/organizations, but got nil")
	}
	if parentNode.Name() != "organizations" {
		t.Errorf("Expected parent node name to be 'organizations', got '%s'", parentNode.Name())
	}
	if len(parentParams) != 0 {
		t.Errorf("Expected no params for parent node, got: %v", parentParams)
	}

	// 2. Test parent node with ID (should use default organizations_id)
	parentIDNode, parentIDParams, _ := route.RouteNode("/api/v1/organizations/123")
	if parentIDNode == nil {
		t.Fatal("Expected to find parent node with ID, but got nil")
	}
	if parentIDNode.Name() != "organizations" {
		t.Errorf("Expected node name to be 'organizations', got '%s'", parentIDNode.Name())
	}
	orgID, exists := parentIDParams["[gone-http]organizations_id"]
	if !exists {
		t.Errorf("Expected param '[gone-http]organizations_id', available params: %v", parentIDParams)
	}
	if orgID != "123" {
		t.Errorf("Expected organizations_id to be '123', got '%v'", orgID)
	}

	// 3. Test child route with custom ID (should use user_id, node name = "user")
	childNode, childParams, _ := route.RouteNode("/api/v1/organizations/permissions/456")
	if childNode == nil {
		t.Fatal("Expected to find child node, but got nil")
	}
	// Node name should be "user" (derived from :user_id)
	if childNode.Name() != "user" {
		t.Errorf("Expected child node name to be 'user', got '%s'", childNode.Name())
	}
	userID, exists := childParams["[gone-http]user_id"]
	if !exists {
		t.Errorf("Expected param '[gone-http]user_id', available params: %v", childParams)
	}
	if userID != "456" {
		t.Errorf("Expected user_id to be '456', got '%v'", userID)
	}
	// Should NOT have permissions_id
	if _, exists := childParams["[gone-http]permissions_id"]; exists {
		t.Error("Should not have '[gone-http]permissions_id' param")
	}
}

// TestRouteMultipleCustomIDs verifies behavior with multiple custom IDs
func TestRouteMultipleCustomIDs(t *testing.T) {
	route := NewSimpleRoute()

	// Set up parent endpoint and child with two custom IDs
	route.SetEndpoint("/api/v1/organizations", nil)
	route.SetEndpoint("/api/v1/organizations/:org_id/permissions/:perm_id", nil)

	// 1. Test parent node itself (no ID)
	parentNode, parentParams, _ := route.RouteNode("/api/v1/organizations")
	if parentNode == nil {
		t.Fatal("Expected to find parent node /api/v1/organizations, but got nil")
	}
	if parentNode.Name() != "organizations" {
		t.Errorf("Expected parent node name to be 'organizations', got '%s'", parentNode.Name())
	}
	if len(parentParams) != 0 {
		t.Errorf("Expected no params for parent node, got: %v", parentParams)
	}

	// 2. Test parent node with ID (should use default organizations_id)
	parentIDNode, parentIDParams, _ := route.RouteNode("/api/v1/organizations/999")
	if parentIDNode == nil {
		t.Fatal("Expected to find parent node with ID, but got nil")
	}
	if parentIDNode.Name() != "organizations" {
		t.Errorf("Expected node name to be 'organizations', got '%s'", parentIDNode.Name())
	}
	orgIDFromParent, exists := parentIDParams["[gone-http]organizations_id"]
	if !exists {
		t.Errorf("Expected param '[gone-http]organizations_id', available params: %v", parentIDParams)
	}
	if orgIDFromParent != "999" {
		t.Errorf("Expected organizations_id to be '999', got '%v'", orgIDFromParent)
	}

	// 3. Test child route with both custom IDs
	// When matching /api/v1/organizations/:org_id/permissions/:perm_id:
	// - organizations node's effective name is "org" (evidenced by org_id parameter)
	// - permissions node's effective name is "perm" (evidenced by perm_id parameter)
	childNode, childParams, _ := route.RouteNode("/api/v1/organizations/123/permissions/456")
	if childNode == nil {
		t.Fatal("Expected to find child node, but got nil")
	}
	// Node name should be "perm" (derived from :perm_id)
	if childNode.Name() != "perm" {
		t.Errorf("Expected child node name to be 'perm', got '%s'", childNode.Name())
	}

	if childNode.Parent().Name() != "org" {
		t.Errorf("Expected parent node name to be 'org', got '%s'", childNode.Parent().Name())
	}

	// Verify first custom ID: org_id (not organizations_id)
	// This proves organizations node's effective name is "org"
	orgID, exists := childParams["[gone-http]org_id"]
	if !exists {
		t.Errorf("Expected param '[gone-http]org_id', available params: %v", childParams)
	}
	if orgID != "123" {
		t.Errorf("Expected org_id to be '123', got '%v'", orgID)
	}

	// Verify second custom ID: perm_id (not permissions_id)
	// This proves permissions node's effective name is "perm"
	permID, exists := childParams["[gone-http]perm_id"]
	if !exists {
		t.Errorf("Expected param '[gone-http]perm_id', available params: %v", childParams)
	}
	if permID != "456" {
		t.Errorf("Expected perm_id to be '456', got '%v'", permID)
	}

	// Should NOT have default names - confirms custom names are used for all nodes
	if _, exists := childParams["[gone-http]organizations_id"]; exists {
		t.Error("Should not have '[gone-http]organizations_id' param in child route (should be org_id)")
	}
	if _, exists := childParams["[gone-http]permissions_id"]; exists {
		t.Error("Should not have '[gone-http]permissions_id' param in child route (should be perm_id)")
	}
}

// TestRouteMixedCustomAndDefaultIDs verifies mixed custom and default IDs
func TestRouteMixedCustomAndDefaultIDs(t *testing.T) {
	route := NewSimpleRoute()

	// Set up with one custom ID and one default
	route.SetEndpoint("/api/v1/organizations", nil)
	route.SetEndpoint("/api/v1/organizations/:org_id/permissions", nil)

	// 1. Test parent node itself (no ID)
	parentNode, parentParams, _ := route.RouteNode("/api/v1/organizations")
	if parentNode == nil {
		t.Fatal("Expected to find parent node /api/v1/organizations, but got nil")
	}
	if parentNode.Name() != "organizations" {
		t.Errorf("Expected parent node name to be 'organizations', got '%s'", parentNode.Name())
	}
	if len(parentParams) != 0 {
		t.Errorf("Expected no params for parent node, got: %v", parentParams)
	}

	// 2. Test parent node with ID (should use default organizations_id)
	parentIDNode, parentIDParams, _ := route.RouteNode("/api/v1/organizations/999")
	if parentIDNode == nil {
		t.Fatal("Expected to find parent node with ID, but got nil")
	}
	if parentIDNode.Name() != "organizations" {
		t.Errorf("Expected node name to be 'organizations', got '%s'", parentIDNode.Name())
	}
	orgIDFromParent, exists := parentIDParams["[gone-http]organizations_id"]
	if !exists {
		t.Errorf("Expected param '[gone-http]organizations_id', available params: %v", parentIDParams)
	}
	if orgIDFromParent != "999" {
		t.Errorf("Expected organizations_id to be '999', got '%v'", orgIDFromParent)
	}

	// 3. Test child route with mixed IDs
	childNode, childParams, _ := route.RouteNode("/api/v1/organizations/123/permissions/456")
	if childNode == nil {
		t.Fatal("Expected to find child node, but got nil")
	}
	// Node name should be "permissions" (default)
	if childNode.Name() != "permissions" {
		t.Errorf("Expected child node name to be 'permissions', got '%s'", childNode.Name())
	}

	// Verify custom ID: org_id
	orgID, exists := childParams["[gone-http]org_id"]
	if !exists {
		t.Errorf("Expected param '[gone-http]org_id', available params: %v", childParams)
	}
	if orgID != "123" {
		t.Errorf("Expected org_id to be '123', got '%v'", orgID)
	}

	// Verify default ID: permissions_id
	permID, exists := childParams["[gone-http]permissions_id"]
	if !exists {
		t.Errorf("Expected param '[gone-http]permissions_id', available params: %v", childParams)
	}
	if permID != "456" {
		t.Errorf("Expected permissions_id to be '456', got '%v'", permID)
	}

	// Should NOT have organizations_id in child route
	if _, exists := childParams["[gone-http]organizations_id"]; exists {
		t.Error("Should not have '[gone-http]organizations_id' param in child route")
	}
}
