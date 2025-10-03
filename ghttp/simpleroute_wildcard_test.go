package ghttp

import (
	"testing"
)

// TestWildcardRoute verifies that wildcard routes like /static/* work correctly
func TestWildcardRoute(t *testing.T) {
	route := NewSimpleRoute()

	// Set up wildcard route
	route.SetEndpoint("/static/*", nil)

	// Test accessing files under static path
	// Wildcard routes should return the wildcard node (name = "*")
	tests := []struct {
		path     string
		expected string
	}{
		{"/static/js/srp.js", "*"},
		{"/static/css/style.css", "*"},
		{"/static/images/logo.png", "*"},
		{"/static/deep/nested/file.txt", "*"},
	}

	for _, tt := range tests {
		node, params, _ := route.RouteNode(tt.path)
		if node == nil {
			t.Errorf("Path %s: expected to find node, but got nil", tt.path)
			continue
		}
		if node.Name() != tt.expected {
			t.Errorf("Path %s: expected node name '%s', got '%s'", tt.path, tt.expected, node.Name())
		}
		// Verify that the wildcard parameter is captured
		if val, exists := params["*"]; !exists {
			t.Errorf("Path %s: expected wildcard param '*' to exist", tt.path)
		} else if val == "" {
			t.Errorf("Path %s: expected wildcard param to have value", tt.path)
		}
	}
}

// TestWildcardWithCustomID verifies wildcard routes work alongside custom ID routes
func TestWildcardWithCustomID(t *testing.T) {
	route := NewSimpleRoute()

	// Set up both wildcard and custom ID routes
	route.SetEndpoint("/static/*", nil)
	route.SetEndpoint("/api/users/:user_id", nil)

	// Test wildcard route
	staticNode, staticParams, _ := route.RouteNode("/static/js/app.js")
	if staticNode == nil {
		t.Fatal("Expected static node to be non-nil")
	}
	// Wildcard routes return the wildcard node with name "*"
	if staticNode.Name() != "*" {
		t.Errorf("Expected wildcard node name to be '*', got '%s'", staticNode.Name())
	}
	// Verify wildcard parameter exists
	if val, exists := staticParams["*"]; !exists {
		t.Error("Expected wildcard param '*' to exist")
	} else if val == "" {
		t.Error("Expected wildcard param to have value")
	}

	// Test custom ID route
	apiNode, apiParams, _ := route.RouteNode("/api/users/123")
	if apiNode == nil {
		t.Fatal("Expected api node to be non-nil")
	}
	if apiNode.Name() != "user" {
		t.Errorf("Expected api node name to be 'user', got '%s'", apiNode.Name())
	}
	if userID, exists := apiParams["[gone-http]user_id"]; !exists {
		t.Error("Expected user_id parameter to exist")
	} else if userID != "123" {
		t.Errorf("Expected user_id to be '123', got '%v'", userID)
	}
}
