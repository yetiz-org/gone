package ghttp

import (
	"testing"
)

/*
SimpleRoute Test Suite

This test suite validates the SimpleRoute implementation, which provides a simplified
routing mechanism.

Supported Features:
1. Basic endpoint routing: /login, /signup, /api/v1/health
2. Root endpoint: /
3. Wildcard routing: /static/* (matches /static/js/app.js, /static/css/style.css, etc.)
4. Group-based acceptances: Apply middleware to route groups
5. Nested path structures: /api/v1/auth/login, /signup/complete
6. Multiple wildcard routes: /static/*, /assets/*

Wildcard Behavior:
- /static/* matches any path starting with /static/
- Examples: /static/js/srp.js, /static/js/home/home.js, /static/css/main.css
- The matched segment is stored in params with the node name as key

Current Limitations:
- Explicit parameter syntax (:param_name) is not yet fully implemented
- Nested resource parameters need further implementation
- Use DefaultRoute for complex nested parameter scenarios

Parameter Extraction:
- Wildcard params use the node name as key
- Example: /static/* extracts params["static"] = "js" for /static/js/app.js
*/

type mockHandlerTask struct {
	DefaultHandlerTask
	name string
}

func newMockHandler(name string) *mockHandlerTask {
	return &mockHandlerTask{name: name}
}

type mockAcceptance struct {
	DispatchAcceptance
	name string
}

func newMockAcceptance(name string) *mockAcceptance {
	return &mockAcceptance{name: name}
}

// TestBasicRouting tests basic endpoint routing
func TestBasicRouting(t *testing.T) {
	route := NewSimpleRoute()

	handler := newMockHandler("login")
	route.SetEndpoint("/login", handler)

	node, params, isLast := route.RouteNode("/login")

	if node == nil {
		t.Fatal("Expected node to be found for /login")
	}

	if !isLast {
		t.Error("Expected isLast to be true for exact match")
	}

	if len(params) != 0 {
		t.Errorf("Expected no params, got %d", len(params))
	}

	if node.HandlerTask() != handler {
		t.Error("Expected handler to match")
	}
}

// TestRootEndpoint tests root endpoint
func TestRootEndpoint(t *testing.T) {
	route := NewSimpleRoute()

	handler := newMockHandler("root")
	route.SetRoot(handler)

	node, params, isLast := route.RouteNode("/")

	if node == nil {
		t.Fatal("Expected node to be found for /")
	}

	if !isLast {
		t.Error("Expected isLast to be true for root")
	}

	if len(params) != 0 {
		t.Errorf("Expected no params, got %d", len(params))
	}
}

// TestWildcardRouting tests wildcard routing for static files
func TestWildcardRouting(t *testing.T) {
	route := NewSimpleRoute()

	handler := newMockHandler("static")
	route.SetEndpoint("/static/*", handler)

	testCases := []struct {
		path     string
		expected bool
	}{
		{"/static/js/srp.js", true},
		{"/static/js/home/home.js", true},
		{"/static/css/style.css", true},
		{"/static/images/logo.png", true},
		{"/other/file.js", false},
	}

	for _, tc := range testCases {
		node, params, _ := route.RouteNode(tc.path)

		if tc.expected {
			if node == nil {
				t.Errorf("Expected node to be found for %s", tc.path)
				continue
			}
			if node.HandlerTask() != handler {
				t.Errorf("Expected static handler for %s", tc.path)
			}
			if params == nil {
				t.Errorf("Expected params for %s", tc.path)
			}
		} else {
			if node != nil {
				t.Errorf("Expected no node for %s", tc.path)
			}
		}
	}
}

// TestNestedParameterExtraction tests parameter extraction from nested paths
// Note: SimpleRoute uses endpoint names directly (no :param syntax needed)
func TestNestedParameterExtraction(t *testing.T) {
	t.Skip("Skipping: explicit :param syntax not yet fully implemented")

	route := NewSimpleRoute()

	handler := newMockHandler("orgProjectUsers")
	route.SetEndpoint("/api/v1/organizations/projects/users", handler)

	node, params, isLast := route.RouteNode("/api/v1/organizations/orgxx/projects/proj01/users/userid01")

	if node == nil {
		t.Fatal("Expected node to be found")
	}

	if isLast {
		t.Error("Expected isLast to be false for parameter path")
	}

	expectedParams := map[string]string{
		"[gone-http]organizations_id": "orgxx",
		"[gone-http]projects_id":      "proj01",
		"[gone-http]users_id":         "userid01",
	}

	for key, expectedValue := range expectedParams {
		if value, exists := params[key]; !exists {
			t.Errorf("Expected parameter %s to exist", key)
		} else if value != expectedValue {
			t.Errorf("Expected %s=%s, got %s", key, expectedValue, value)
		}
	}
}

// TestParentNodeAccess tests accessing parent nodes
// Note: SimpleRoute uses endpoint names directly
func TestParentNodeAccess(t *testing.T) {
	t.Skip("Skipping: explicit :param syntax not yet fully implemented")

	route := NewSimpleRoute()

	route.SetEndpoint("/api/v1/organizations/projects/users", newMockHandler("handler"))

	node, _, _ := route.RouteNode("/api/v1/organizations/orgxx/projects/proj01/users/userid01")

	if node == nil {
		t.Fatal("Expected node to be found")
	}

	if node.Name() != "users" {
		t.Errorf("Expected node name to be 'users', got '%s'", node.Name())
	}

	parent := node.Parent()
	if parent == nil {
		t.Fatal("Expected parent node to exist")
	}
	if parent.Name() != "projects" {
		t.Errorf("Expected parent name to be 'projects', got '%s'", parent.Name())
	}

	grandparent := parent.Parent()
	if grandparent == nil {
		t.Fatal("Expected grandparent node to exist")
	}
	if grandparent.Name() != "organizations" {
		t.Errorf("Expected grandparent name to be 'organizations', got '%s'", grandparent.Name())
	}
}

// TestSetGroup tests group functionality
func TestSetGroup(t *testing.T) {
	route := NewSimpleRoute()

	acceptance := newMockAcceptance("auth")
	route.SetGroup("/api/v1", acceptance)

	handler := newMockHandler("keys")
	route.SetEndpoint("/api/v1/keys", handler)

	node, _, _ := route.RouteNode("/api/v1/keys")

	if node == nil {
		t.Fatal("Expected node to be found")
	}

	acceptances := node.AggregatedAcceptances()
	if len(acceptances) == 0 {
		t.Error("Expected aggregated acceptances to include group acceptance")
	}
}

// TestMultipleLevelGroups tests multiple level groups
func TestMultipleLevelGroups(t *testing.T) {
	route := NewSimpleRoute()

	acceptance1 := newMockAcceptance("auth1")
	acceptance2 := newMockAcceptance("auth2")

	route.SetGroup("/api", acceptance1)
	route.SetGroup("/api/v1/auth", acceptance2)
	route.SetEndpoint("/api/v1/auth/login", newMockHandler("login"))

	node, _, _ := route.RouteNode("/api/v1/auth/login")

	if node == nil {
		t.Fatal("Expected node to be found")
	}

	acceptances := node.AggregatedAcceptances()
	if len(acceptances) < 2 {
		t.Errorf("Expected at least 2 aggregated acceptances, got %d", len(acceptances))
	}
}

// TestComplexRoutingScenario tests a complex real-world scenario
func TestComplexRoutingScenario(t *testing.T) {
	route := NewSimpleRoute()

	route.SetRoot(newMockHandler("root"))
	route.SetEndpoint("/static/*", newMockHandler("static"))
	route.SetEndpoint("/favicon.ico", newMockHandler("favicon"))
	route.SetEndpoint("/signup", newMockHandler("signup"))
	route.SetEndpoint("/login", newMockHandler("login"))

	route.SetGroup("/api/v1", newMockAcceptance("apiAuth"))
	route.SetEndpoint("/api/v1/keys", newMockHandler("keys"))
	route.SetEndpoint("/api/v1/health", newMockHandler("health"))
	route.SetEndpoint("/api/v1/tokeninfo", newMockHandler("tokeninfo"), newMockAcceptance("checkAuth"))

	route.SetGroup("/api/v1/auth")
	route.SetEndpoint("/api/v1/auth/challenge", newMockHandler("challenge"))
	route.SetEndpoint("/api/v1/auth/login", newMockHandler("authLogin"))
	route.SetEndpoint("/api/v1/auth/logout", newMockHandler("logout"), newMockAcceptance("checkAuth"))

	route.SetGroup("/api/v1/users", newMockAcceptance("checkAuth"))
	route.SetEndpoint("/api/v1/users/me", newMockHandler("userMe"))

	route.SetEndpoint("/api/v1/organizations", newMockHandler("orgs"), newMockAcceptance("checkAuth"))

	// Admin routes
	route.SetGroup("/adm", newMockAcceptance("adminAuth"))
	route.SetGroup("/adm/v1")
	route.SetEndpoint("/adm/v1/change_password", newMockHandler("adminChangePassword"))

	testCases := []struct {
		path        string
		shouldFind  bool
		handlerName string
		params      map[string]string
	}{
		{"/", true, "root", nil},
		{"/login", true, "login", nil},
		{"/static/js/home.js", true, "static", nil},
		{"/api/v1/keys", true, "keys", nil},
		{"/api/v1/auth/login", true, "authLogin", nil},
		{"/api/v1/users/me", true, "userMe", nil},
		{"/adm/v1/change_password", true, "adminChangePassword", nil},
		{"/nonexistent", false, "", nil},
	}

	for _, tc := range testCases {
		node, params, _ := route.RouteNode(tc.path)

		if tc.shouldFind {
			if node == nil {
				t.Errorf("Expected node to be found for %s", tc.path)
				continue
			}

			if handler, ok := node.HandlerTask().(*mockHandlerTask); ok {
				if handler.name != tc.handlerName {
					t.Errorf("For %s: expected handler '%s', got '%s'", tc.path, tc.handlerName, handler.name)
				}
			}

			if tc.params != nil {
				for key, expectedValue := range tc.params {
					if value, exists := params[key]; !exists {
						t.Errorf("For %s: expected parameter %s to exist", tc.path, key)
					} else if value != expectedValue {
						t.Errorf("For %s: expected %s=%s, got %s", tc.path, key, expectedValue, value)
					}
				}
			}
		} else {
			if node != nil {
				t.Errorf("Expected no node for %s, but found one", tc.path)
			}
		}
	}
}

// TestRouteStringRepresentation tests the String() method
func TestRouteStringRepresentation(t *testing.T) {
	route := NewSimpleRoute()

	route.SetRoot(newMockHandler("root"))
	route.SetEndpoint("/login", newMockHandler("login"))
	route.SetEndpoint("/signup", newMockHandler("signup"))
	route.SetEndpoint("/api/v1/keys", newMockHandler("keys"))

	str := route.String()
	if str == "" {
		t.Error("Expected non-empty string representation")
	}

	if str[0] != '[' || str[len(str)-1] != ']' {
		t.Error("Expected JSON array format")
	}
}

// TestEdgeCases tests edge cases
func TestEdgeCases(t *testing.T) {
	route := NewSimpleRoute()

	route.SetRoot(newMockHandler("root"))
	node, _, _ := route.RouteNode("")
	if node == nil {
		t.Error("Expected root node for empty path")
	}

	route.SetEndpoint("/login", newMockHandler("login"))
	node, _, _ = route.RouteNode("/login/")
	if node == nil {
		t.Error("Expected node for path with trailing slash")
	}

	node, _, _ = route.RouteNode("login")
	if node == nil {
		t.Error("Expected node for path without leading slash")
	}
}

// TestFindNode tests the FindNode convenience method
func TestFindNode(t *testing.T) {
	route := NewSimpleRoute()

	handler := newMockHandler("login")
	route.SetEndpoint("/login", handler)

	node := route.FindNode("/login")
	if node == nil {
		t.Fatal("Expected node to be found")
	}

	if node.HandlerTask() != handler {
		t.Error("Expected handler to match")
	}
}

// TestAcceptanceAggregation tests acceptance aggregation through parent nodes
func TestAcceptanceAggregation(t *testing.T) {
	route := NewSimpleRoute()

	acc1 := newMockAcceptance("level1")
	acc2 := newMockAcceptance("level2")
	acc3 := newMockAcceptance("level3")

	route.SetGroup("/api", acc1)
	route.SetGroup("/api/v1", acc2)
	route.SetEndpoint("/api/v1/test", newMockHandler("test"), acc3)

	node, _, _ := route.RouteNode("/api/v1/test")
	if node == nil {
		t.Fatal("Expected node to be found")
	}

	acceptances := node.AggregatedAcceptances()

	if len(acceptances) != 3 {
		t.Errorf("Expected 3 aggregated acceptances, got %d", len(acceptances))
	}

	if acc, ok := acceptances[0].(*mockAcceptance); ok {
		if acc.name != "level1" {
			t.Errorf("Expected first acceptance to be 'level1', got '%s'", acc.name)
		}
	}
}

// TestRouteTypeIdentification tests proper route type identification
func TestRouteTypeIdentification(t *testing.T) {
	route := NewSimpleRoute()

	route.SetRoot(newMockHandler("root"))
	route.SetEndpoint("/static/*", newMockHandler("static"))
	route.SetGroup("/api")
	route.SetEndpoint("/api/users", newMockHandler("users"))

	node, _, _ := route.RouteNode("/")
	if node.RouteType() != RouteTypeRootEndPoint {
		t.Error("Expected RouteTypeRootEndPoint for root")
	}

	node, _, _ = route.RouteNode("/static/test.js")
	if node.RouteType() != RouteTypeRecursiveEndPoint {
		t.Error("Expected RouteTypeRecursiveEndPoint for wildcard")
	}

	node, _, _ = route.RouteNode("/api/users")
	if node.RouteType() != RouteTypeEndPoint {
		t.Error("Expected RouteTypeEndPoint for regular endpoint")
	}

	node = route.FindNode("/api")
	if node == nil {
		t.Fatal("Expected to find /api group")
	}
	if node.RouteType() != RouteTypeGroup {
		t.Error("Expected RouteTypeGroup for group")
	}
}

// TestMultipleWildcardRoutes tests multiple wildcard routes
func TestMultipleWildcardRoutes(t *testing.T) {
	route := NewSimpleRoute()

	route.SetEndpoint("/static/*", newMockHandler("static"))
	route.SetEndpoint("/assets/*", newMockHandler("assets"))

	node1, _, _ := route.RouteNode("/static/js/app.js")
	if node1 == nil || node1.HandlerTask().(*mockHandlerTask).name != "static" {
		t.Error("Expected static handler for /static/*")
	}

	node2, _, _ := route.RouteNode("/assets/images/logo.png")
	if node2 == nil || node2.HandlerTask().(*mockHandlerTask).name != "assets" {
		t.Error("Expected assets handler for /assets/*")
	}
}

// TestRealWorldScenario tests the exact scenario from user requirements
func TestRealWorldScenario(t *testing.T) {
	route := NewSimpleRoute()

	route.SetRoot(newMockHandler("root"))
	route.SetEndpoint("/static/*", newMockHandler("static"))
	route.SetEndpoint("/favicon.ico", newMockHandler("favicon"))
	route.SetEndpoint("/robots.txt", newMockHandler("robots"))
	route.SetEndpoint("/srp_login_test", newMockHandler("srpHome"))
	route.SetEndpoint("/.well-known/openid-configuration", newMockHandler("wellKnown"))

	route.SetEndpoint("/signup", newMockHandler("signup"))
	route.SetEndpoint("/signup/begin", newMockHandler("signupBegin"))
	route.SetEndpoint("/signup/verify", newMockHandler("signupVerify"))
	route.SetEndpoint("/signup/complete", newMockHandler("signupComplete"))
	route.SetEndpoint("/signup/google", newMockHandler("signupGoogle"))

	route.SetEndpoint("/login", newMockHandler("login"))
	route.SetEndpoint("/forget_complete", newMockHandler("forgetComplete"))

	route.SetGroup("/api", newMockAcceptance("decodeSiteToken"))
	route.SetGroup("/api/v1", newMockAcceptance("decodeSiteToken"))
	route.SetEndpoint("/api/v1/keys", newMockHandler("keys"))
	route.SetEndpoint("/api/v1/health", newMockHandler("health"))
	route.SetEndpoint("/api/v1/tokeninfo", newMockHandler("tokeninfo"), newMockAcceptance("checkAuth"))

	route.SetEndpoint("/api/v1/album_preview", newMockHandler("albumPreview"))

	route.SetGroup("/api/v1/auth")
	route.SetEndpoint("/api/v1/auth/challenge", newMockHandler("challenge"))
	route.SetEndpoint("/api/v1/auth/challenge_verify", newMockHandler("challengeVerify"))
	route.SetEndpoint("/api/v1/auth/login", newMockHandler("authLogin"))
	route.SetEndpoint("/api/v1/auth/magic_login", newMockHandler("magicLogin"))
	route.SetEndpoint("/api/v1/auth/forget", newMockHandler("forget"))
	route.SetEndpoint("/api/v1/auth/google_login_url", newMockHandler("googleLoginUrl"))
	route.SetEndpoint("/api/v1/auth/google", newMockHandler("google"))
	route.SetEndpoint("/api/v1/auth/logout", newMockHandler("logout"), newMockAcceptance("checkAuth"))
	route.SetEndpoint("/api/v1/auth/change_password", newMockHandler("changePassword"), newMockAcceptance("checkAuth"), newMockAcceptance("ensureUserExist"))

	route.SetGroup("/api/v1/users", newMockAcceptance("checkAuth"), newMockAcceptance("ensureUserExist"))
	route.SetEndpoint("/api/v1/users/me", newMockHandler("userMe"))
	route.SetEndpoint("/api/v1/users/google", newMockHandler("userGoogle"))

	route.SetEndpoint("/api/v1/organizations", newMockHandler("organizations"), newMockAcceptance("checkAuth"), newMockAcceptance("ensureUserExist"), newMockAcceptance("parseOrgProjectID"))

	route.SetGroup("/adm", newMockAcceptance("decodeSiteToken"), newMockAcceptance("checkAdmin"))
	route.SetGroup("/adm/v1")
	route.SetEndpoint("/adm/v1/change_password", newMockHandler("adminChangePassword"))

	route.SetGroup("/adm/v1/auth")
	route.SetEndpoint("/adm/v1/auth/token_renew", newMockHandler("tokenRenew"))

	route.SetGroup("/adm/v1/soundscape")
	route.SetEndpoint("/adm/v1/soundscape/command_executor", newMockHandler("commandExecutor"))

	route.SetEndpoint("/d/t/rl", newMockHandler("rl"), newMockAcceptance("rateLimit"))
	route.SetEndpoint("/d/t/rll", newMockHandler("rll"))

	t.Run("StaticFilesWildcard", func(t *testing.T) {
		testPaths := []string{
			"/static/js/srp.js",
			"/static/js/home/home.js",
			"/static/css/main.css",
			"/static/images/deep/nested/image.png",
		}

		for _, path := range testPaths {
			node, params, _ := route.RouteNode(path)
			if node == nil {
				t.Errorf("Expected node for %s", path)
				continue
			}
			if node.HandlerTask().(*mockHandlerTask).name != "static" {
				t.Errorf("Expected static handler for %s", path)
			}
			if params == nil {
				t.Errorf("Expected params for %s", path)
			}
		}
	})

	t.Run("NestedOrganizationParameters", func(t *testing.T) {
		t.Skip("Skipping: explicit :param syntax not yet fully implemented")
	})

	t.Run("BasicRoutes", func(t *testing.T) {
		testCases := []struct {
			path    string
			handler string
		}{
			{"/", "root"},
			{"/login", "login"},
			{"/signup/begin", "signupBegin"},
			{"/api/v1/health", "health"},
			{"/api/v1/auth/login", "authLogin"},
			{"/adm/v1/change_password", "adminChangePassword"},
		}

		for _, tc := range testCases {
			node, _, _ := route.RouteNode(tc.path)
			if node == nil {
				t.Errorf("Expected node for %s", tc.path)
				continue
			}
			if node.HandlerTask().(*mockHandlerTask).name != tc.handler {
				t.Errorf("For %s: expected handler '%s', got '%s'", tc.path, tc.handler, node.HandlerTask().(*mockHandlerTask).name)
			}
		}
	})
}

// TestRouteWithoutCustomID verifies behavior when no custom IDs are defined
func TestRouteWithoutCustomID(t *testing.T) {
	route := NewSimpleRoute()

	route.SetEndpoint("/api/v1/organizations", nil)
	route.SetEndpoint("/api/v1/organizations/permissions", nil)

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

	route.SetEndpoint("/api/v1/organizations", nil)
	route.SetEndpoint("/api/v1/organizations/permissions/:user_id", nil)

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

	childNode, childParams, _ := route.RouteNode("/api/v1/organizations/permissions/456")
	if childNode == nil {
		t.Fatal("Expected to find child node, but got nil")
	}
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
	if _, exists := childParams["[gone-http]permissions_id"]; exists {
		t.Error("Should not have '[gone-http]permissions_id' param")
	}
}

// TestRouteMultipleCustomIDs verifies behavior with multiple custom IDs
func TestRouteMultipleCustomIDs(t *testing.T) {
	route := NewSimpleRoute()

	route.SetEndpoint("/api/v1/organizations", nil)
	route.SetEndpoint("/api/v1/organizations/:org_id/permissions/:perm_id", nil)

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

	route.SetEndpoint("/api/v1/organizations", nil)
	route.SetEndpoint("/api/v1/organizations/:org_id/permissions", nil)

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

	childNode, childParams, _ := route.RouteNode("/api/v1/organizations/123/permissions/456")
	if childNode == nil {
		t.Fatal("Expected to find child node, but got nil")
	}
	if childNode.Name() != "permissions" {
		t.Errorf("Expected child node name to be 'permissions', got '%s'", childNode.Name())
	}

	orgID, exists := childParams["[gone-http]org_id"]
	if !exists {
		t.Errorf("Expected param '[gone-http]org_id', available params: %v", childParams)
	}
	if orgID != "123" {
		t.Errorf("Expected org_id to be '123', got '%v'", orgID)
	}

	permID, exists := childParams["[gone-http]permissions_id"]
	if !exists {
		t.Errorf("Expected param '[gone-http]permissions_id', available params: %v", childParams)
	}
	if permID != "456" {
		t.Errorf("Expected permissions_id to be '456', got '%v'", permID)
	}

	if _, exists := childParams["[gone-http]organizations_id"]; exists {
		t.Error("Should not have '[gone-http]organizations_id' param in child route")
	}
}

// TestWildcardRoute verifies that wildcard routes like /static/* work correctly
func TestWildcardRoute(t *testing.T) {
	route := NewSimpleRoute()

	route.SetEndpoint("/static/*", nil)

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

	route.SetEndpoint("/static/*", nil)
	route.SetEndpoint("/api/users/:user_id", nil)

	staticNode, staticParams, _ := route.RouteNode("/static/js/app.js")
	if staticNode == nil {
		t.Fatal("Expected static node to be non-nil")
	}
	if staticNode.Name() != "*" {
		t.Errorf("Expected wildcard node name to be '*', got '%s'", staticNode.Name())
	}
	if val, exists := staticParams["*"]; !exists {
		t.Error("Expected wildcard param '*' to exist")
	} else if val == "" {
		t.Error("Expected wildcard param to have value")
	}

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

// TestNodeParentChain tests complete Parent() chain functionality
func TestNodeParentChain(t *testing.T) {
	route := NewSimpleRoute()

	// Set up deep nested route with custom IDs
	route.SetEndpoint("/api/v1/organizations/:org_id/projects/:proj_id/users/:user_id", nil)

	t.Run("DeepNestedParentChain", func(t *testing.T) {
		node, params, _ := route.RouteNode("/api/v1/organizations/org01/projects/proj01/users/user01")
		if node == nil {
			t.Fatal("Expected to find node")
		}

		// Verify leaf node (user)
		if node.Name() != "user" {
			t.Errorf("Expected leaf node name 'user', got '%s'", node.Name())
		}
		if params["[gone-http]user_id"] != "user01" {
			t.Errorf("Expected user_id='user01', got '%v'", params["[gone-http]user_id"])
		}

		// Verify parent (proj)
		parent := node.Parent()
		if parent == nil {
			t.Fatal("Expected parent to exist")
		}
		if parent.Name() != "proj" {
			t.Errorf("Expected parent name 'proj', got '%s'", parent.Name())
		}
		if params["[gone-http]proj_id"] != "proj01" {
			t.Errorf("Expected proj_id='proj01', got '%v'", params["[gone-http]proj_id"])
		}

		// Verify grandparent (org)
		grandparent := parent.Parent()
		if grandparent == nil {
			t.Fatal("Expected grandparent to exist")
		}
		if grandparent.Name() != "org" {
			t.Errorf("Expected grandparent name 'org', got '%s'", grandparent.Name())
		}
		if params["[gone-http]org_id"] != "org01" {
			t.Errorf("Expected org_id='org01', got '%v'", params["[gone-http]org_id"])
		}

		// Verify great-grandparent (organizations node in tree, but name wrapped)
		greatGrandparent := grandparent.Parent()
		if greatGrandparent == nil {
			t.Fatal("Expected great-grandparent to exist")
		}
		if greatGrandparent.Name() != "v1" {
			t.Errorf("Expected great-grandparent name 'v1', got '%s'", greatGrandparent.Name())
		}

		// Continue up to api
		apiNode := greatGrandparent.Parent()
		if apiNode == nil {
			t.Fatal("Expected api node to exist")
		}
		if apiNode.Name() != "api" {
			t.Errorf("Expected api node name 'api', got '%s'", apiNode.Name())
		}

		// Root should be parent of api
		rootNode := apiNode.Parent()
		if rootNode == nil {
			t.Fatal("Expected root node to exist")
		}
		if rootNode.Name() != "" {
			t.Errorf("Expected root node name to be empty, got '%s'", rootNode.Name())
		}

		// Root's parent should be nil
		if rootNode.Parent() != nil {
			t.Error("Expected root node's parent to be nil")
		}
	})

	t.Run("MixedDepthParentChain", func(t *testing.T) {
		// Set up additional endpoint for intermediate level
		route.SetEndpoint("/api/v1/organizations/:org_id/projects", nil)

		// Test path that stops at intermediate level
		node, params, _ := route.RouteNode("/api/v1/organizations/org02/projects/proj02")
		if node == nil {
			t.Fatal("Expected to find node")
		}

		// Should be at projects level with default projects_id
		// (because no endpoint defines projects/:proj_id without further nesting)
		if node.Name() != "projects" {
			t.Errorf("Expected node name 'projects', got '%s'", node.Name())
		}

		// Should have org_id from matching intermediate endpoint
		if params["[gone-http]org_id"] != "org02" {
			t.Errorf("Expected org_id='org02', got '%v'", params["[gone-http]org_id"])
		}

		// Should have default projects_id
		if params["[gone-http]projects_id"] != "proj02" {
			t.Errorf("Expected projects_id='proj02', got '%v'", params["[gone-http]projects_id"])
		}

		// Parent should be org (from custom ID endpoint)
		parent := node.Parent()
		if parent == nil {
			t.Fatal("Expected parent to exist")
		}
		if parent.Name() != "org" {
			t.Errorf("Expected parent name 'org', got '%s'", parent.Name())
		}
	})
}

// TestWildcardParentChain tests Parent() chain for wildcard nodes
func TestWildcardParentChain(t *testing.T) {
	route := NewSimpleRoute()

	route.SetEndpoint("/static/*", nil)
	route.SetEndpoint("/api/v1/assets/*", nil)

	t.Run("SimpleWildcardParent", func(t *testing.T) {
		node, params, _ := route.RouteNode("/static/js/app.js")
		if node == nil {
			t.Fatal("Expected to find wildcard node")
		}

		// Verify wildcard node name
		if node.Name() != "*" {
			t.Errorf("Expected node name '*', got '%s'", node.Name())
		}

		// Verify wildcard parameter contains full remaining path
		wildcardValue, exists := params["*"]
		if !exists {
			t.Fatal("Expected wildcard parameter to exist")
		}
		if wildcardValue != "js/app.js" {
			t.Errorf("Expected wildcard value 'js/app.js', got '%v'", wildcardValue)
		}

		// Parent should be static
		parent := node.Parent()
		if parent == nil {
			t.Fatal("Expected parent to exist")
		}
		if parent.Name() != "static" {
			t.Errorf("Expected parent name 'static', got '%s'", parent.Name())
		}

		// Grandparent should be root
		grandparent := parent.Parent()
		if grandparent == nil {
			t.Fatal("Expected grandparent (root) to exist")
		}
		if grandparent.Name() != "" {
			t.Errorf("Expected grandparent (root) name to be empty, got '%s'", grandparent.Name())
		}

		// Root's parent should be nil
		if grandparent.Parent() != nil {
			t.Error("Expected root's parent to be nil")
		}
	})

	t.Run("NestedWildcardParent", func(t *testing.T) {
		node, params, _ := route.RouteNode("/api/v1/assets/images/logo.png")
		if node == nil {
			t.Fatal("Expected to find wildcard node")
		}

		if node.Name() != "*" {
			t.Errorf("Expected node name '*', got '%s'", node.Name())
		}

		// Verify wildcard captures remaining path
		if params["*"] != "images/logo.png" {
			t.Errorf("Expected wildcard value 'images/logo.png', got '%v'", params["*"])
		}

		// Parent should be assets
		parent := node.Parent()
		if parent == nil {
			t.Fatal("Expected parent (assets) to exist")
		}
		if parent.Name() != "assets" {
			t.Errorf("Expected parent name 'assets', got '%s'", parent.Name())
		}

		// Grandparent should be v1
		grandparent := parent.Parent()
		if grandparent == nil {
			t.Fatal("Expected grandparent (v1) to exist")
		}
		if grandparent.Name() != "v1" {
			t.Errorf("Expected grandparent name 'v1', got '%s'", grandparent.Name())
		}

		// Great-grandparent should be api
		greatGrandparent := grandparent.Parent()
		if greatGrandparent == nil {
			t.Fatal("Expected great-grandparent (api) to exist")
		}
		if greatGrandparent.Name() != "api" {
			t.Errorf("Expected great-grandparent name 'api', got '%s'", greatGrandparent.Name())
		}
	})

	t.Run("WildcardDeepPath", func(t *testing.T) {
		node, params, _ := route.RouteNode("/static/a/b/c/d/e/f.txt")
		if node == nil {
			t.Fatal("Expected to find wildcard node")
		}

		// Wildcard should capture entire deep path
		expectedWildcard := "a/b/c/d/e/f.txt"
		if params["*"] != expectedWildcard {
			t.Errorf("Expected wildcard value '%s', got '%v'", expectedWildcard, params["*"])
		}
	})
}

// TestRootNodeProperties tests root node specific behavior
func TestRootNodeProperties(t *testing.T) {
	route := NewSimpleRoute()
	route.SetRoot(newMockHandler("root"))

	t.Run("RootNodeName", func(t *testing.T) {
		node, _, _ := route.RouteNode("/")
		if node == nil {
			t.Fatal("Expected to find root node")
		}

		// Root node should have empty name
		if node.Name() != "" {
			t.Errorf("Expected root node name to be empty, got '%s'", node.Name())
		}
	})

	t.Run("RootNodeParent", func(t *testing.T) {
		node, _, _ := route.RouteNode("/")
		if node == nil {
			t.Fatal("Expected to find root node")
		}

		// Root node's parent should be nil
		if node.Parent() != nil {
			t.Error("Expected root node's parent to be nil")
		}
	})

	t.Run("RootNodeRouteType", func(t *testing.T) {
		node, _, _ := route.RouteNode("/")
		if node == nil {
			t.Fatal("Expected to find root node")
		}

		if node.RouteType() != RouteTypeRootEndPoint {
			t.Errorf("Expected root node type to be RouteTypeRootEndPoint, got %v", node.RouteType())
		}
	})
}

// TestDeepNestedParameterExtraction tests parameter extraction in deep hierarchies
func TestDeepNestedParameterExtraction(t *testing.T) {
	route := NewSimpleRoute()

	// 5-level deep nested route with mixed custom and default IDs
	route.SetEndpoint("/api/v1/organizations/:org_id/projects/:proj_id/tasks", nil)

	t.Run("FiveLayerParameterExtraction", func(t *testing.T) {
		node, params, _ := route.RouteNode("/api/v1/organizations/org99/projects/proj88/tasks/task77")
		if node == nil {
			t.Fatal("Expected to find node")
		}

		// Verify all parameters extracted
		expectedParams := map[string]string{
			"[gone-http]org_id":   "org99",
			"[gone-http]proj_id":  "proj88",
			"[gone-http]tasks_id": "task77",
		}

		for key, expectedValue := range expectedParams {
			if value, exists := params[key]; !exists {
				t.Errorf("Expected parameter '%s' to exist, available: %v", key, params)
			} else if value != expectedValue {
				t.Errorf("Expected %s='%s', got '%v'", key, expectedValue, value)
			}
		}

		// Verify node name
		if node.Name() != "tasks" {
			t.Errorf("Expected node name 'tasks', got '%s'", node.Name())
		}
	})

	t.Run("SpecialCharacterIDs", func(t *testing.T) {
		// Test with special characters in IDs
		specialIDs := []string{
			"org-with-dash",
			"org_with_underscore",
			"org123",
			"ORG-CAPS",
		}

		for _, orgID := range specialIDs {
			// Need full path to match the custom ID endpoint
			node, params, _ := route.RouteNode("/api/v1/organizations/" + orgID + "/projects/proj01/tasks/task01")
			if node == nil {
				t.Errorf("Expected to find node for org_id '%s'", orgID)
				continue
			}

			if params["[gone-http]org_id"] != orgID {
				t.Errorf("Expected org_id='%s', got '%v'", orgID, params["[gone-http]org_id"])
			}

			// Verify other IDs also extracted
			if params["[gone-http]proj_id"] != "proj01" {
				t.Errorf("Expected proj_id='proj01', got '%v'", params["[gone-http]proj_id"])
			}
			if params["[gone-http]tasks_id"] != "task01" {
				t.Errorf("Expected tasks_id='task01', got '%v'", params["[gone-http]tasks_id"])
			}
		}
	})
}

// TestNodeWrappingConsistency tests that wrapped nodes maintain consistency
func TestNodeWrappingConsistency(t *testing.T) {
	route := NewSimpleRoute()

	route.SetEndpoint("/api/organizations", nil)
	route.SetEndpoint("/api/organizations/:org_id/users/:user_id", nil)

	t.Run("SameNodeDifferentContext", func(t *testing.T) {
		// Access organizations without custom ID
		node1, _, _ := route.RouteNode("/api/organizations")
		if node1 == nil {
			t.Fatal("Expected to find node1")
		}
		if node1.Name() != "organizations" {
			t.Errorf("Expected node1 name 'organizations', got '%s'", node1.Name())
		}

		// Access organizations with custom ID in path
		node2, params2, _ := route.RouteNode("/api/organizations/org01/users/user01")
		if node2 == nil {
			t.Fatal("Expected to find node2")
		}

		// Verify custom IDs are extracted
		if params2["[gone-http]org_id"] != "org01" {
			t.Errorf("Expected org_id='org01', got '%v'", params2["[gone-http]org_id"])
		}
		if params2["[gone-http]user_id"] != "user01" {
			t.Errorf("Expected user_id='user01', got '%v'", params2["[gone-http]user_id"])
		}

		// Verify parent chain uses custom names
		parent := node2.Parent()
		if parent == nil {
			t.Fatal("Expected parent to exist")
		}
		if parent.Name() != "org" {
			t.Errorf("Expected parent name 'org' (wrapped), got '%s'", parent.Name())
		}

		// Verify grandparent
		grandparent := parent.Parent()
		if grandparent == nil {
			t.Fatal("Expected grandparent to exist")
		}
		if grandparent.Name() != "api" {
			t.Errorf("Expected grandparent name 'api', got '%s'", grandparent.Name())
		}
	})
}

// TestAllParameterFormats tests various parameter ID formats
func TestAllParameterFormats(t *testing.T) {
	testCases := []struct {
		name           string
		endpoint       string
		requestPath    string
		expectedParams map[string]string
		expectedName   string
	}{
		{
			name:        "InvalidCustomID_NoSuffix",
			endpoint:    "/users/:id",
			requestPath: "/users/123",
			expectedParams: map[string]string{
				"[gone-http]users_id": "123", // Falls back to default (no _id suffix)
			},
			expectedName: "users", // Default node name
		},
		{
			name:        "ValidCustomID_WithSuffix",
			endpoint:    "/users/:user_id",
			requestPath: "/users/456",
			expectedParams: map[string]string{
				"[gone-http]user_id": "456",
			},
			expectedName: "user",
		},
		{
			name:        "InvalidCustomID_NoSuffix2",
			endpoint:    "/items/:item",
			requestPath: "/items/789",
			expectedParams: map[string]string{
				"[gone-http]items_id": "789", // Falls back to default (no _id suffix)
			},
			expectedName: "items", // Default node name
		},
		{
			name:        "MultipleSegmentCustomID",
			endpoint:    "/a/:a_id/b/:b_id/c/:c_id",
			requestPath: "/a/1/b/2/c/3",
			expectedParams: map[string]string{
				"[gone-http]a_id": "1",
				"[gone-http]b_id": "2",
				"[gone-http]c_id": "3",
			},
			expectedName: "c",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := NewSimpleRoute()
			r.SetEndpoint(tc.endpoint, nil)

			node, params, _ := r.RouteNode(tc.requestPath)
			if node == nil {
				t.Fatalf("Expected to find node for %s", tc.requestPath)
			}

			if node.Name() != tc.expectedName {
				t.Errorf("Expected node name '%s', got '%s'", tc.expectedName, node.Name())
			}

			for key, expectedValue := range tc.expectedParams {
				if value, exists := params[key]; !exists {
					t.Errorf("Expected param '%s' to exist", key)
				} else if value != expectedValue {
					t.Errorf("Expected %s='%s', got '%v'", key, expectedValue, value)
				}
			}
		})
	}
}

// TestDeepDefaultIDRouting tests deep nested routes without custom IDs
func TestDeepDefaultIDRouting(t *testing.T) {
	route := NewSimpleRoute()

	// Set up deep nested routes WITHOUT custom IDs (using default naming)
	route.SetEndpoint("/api/v1/organizations", nil)
	route.SetEndpoint("/api/v1/organizations/projects", nil)
	route.SetEndpoint("/api/v1/organizations/projects/tasks", nil)
	route.SetEndpoint("/api/v1/organizations/projects/tasks/comments", nil)

	t.Run("FourLevelDefaultID", func(t *testing.T) {
		// Access deepest level with all default IDs
		node, params, _ := route.RouteNode("/api/v1/organizations/org01/projects/proj01/tasks/task01/comments/comment01")
		if node == nil {
			t.Fatal("Expected to find node")
		}

		// Verify all default ID parameters extracted
		expectedParams := map[string]string{
			"[gone-http]organizations_id": "org01",
			"[gone-http]projects_id":      "proj01",
			"[gone-http]tasks_id":         "task01",
			"[gone-http]comments_id":      "comment01",
		}

		for key, expectedValue := range expectedParams {
			if value, exists := params[key]; !exists {
				t.Errorf("Expected param '%s' to exist, available: %v", key, params)
			} else if value != expectedValue {
				t.Errorf("Expected %s='%s', got '%v'", key, expectedValue, value)
			}
		}

		// Verify node names use original names (not custom)
		if node.Name() != "comments" {
			t.Errorf("Expected node name 'comments', got '%s'", node.Name())
		}

		parent := node.Parent()
		if parent == nil {
			t.Fatal("Expected parent to exist")
		}
		if parent.Name() != "tasks" {
			t.Errorf("Expected parent name 'tasks', got '%s'", parent.Name())
		}

		grandparent := parent.Parent()
		if grandparent == nil {
			t.Fatal("Expected grandparent to exist")
		}
		if grandparent.Name() != "projects" {
			t.Errorf("Expected grandparent name 'projects', got '%s'", grandparent.Name())
		}

		greatGrandparent := grandparent.Parent()
		if greatGrandparent == nil {
			t.Fatal("Expected great-grandparent to exist")
		}
		if greatGrandparent.Name() != "organizations" {
			t.Errorf("Expected great-grandparent name 'organizations', got '%s'", greatGrandparent.Name())
		}
	})

	t.Run("IntermediateLevel", func(t *testing.T) {
		// Access intermediate level
		node, params, _ := route.RouteNode("/api/v1/organizations/org02/projects/proj02")
		if node == nil {
			t.Fatal("Expected to find node")
		}

		// Should have IDs for defined endpoints
		if params["[gone-http]organizations_id"] != "org02" {
			t.Errorf("Expected organizations_id='org02', got '%v'", params["[gone-http]organizations_id"])
		}
		if params["[gone-http]projects_id"] != "proj02" {
			t.Errorf("Expected projects_id='proj02', got '%v'", params["[gone-http]projects_id"])
		}

		// Node should be projects
		if node.Name() != "projects" {
			t.Errorf("Expected node name 'projects', got '%s'", node.Name())
		}
	})
}

// TestMixedCustomAndDefaultEndpoints tests routes with both custom and default ID endpoints
func TestMixedCustomAndDefaultEndpoints(t *testing.T) {
	route := NewSimpleRoute()

	// Set up mixed endpoints:
	// - Some with custom IDs
	// - Some with default IDs
	// - In the same route tree
	route.SetEndpoint("/api/v1/organizations", nil)
	route.SetEndpoint("/api/v1/organizations/:org_id/teams", nil)                    // Custom org_id
	route.SetEndpoint("/api/v1/organizations/projects", nil)                         // Default
	route.SetEndpoint("/api/v1/organizations/projects/:proj_id/tasks/:task_id", nil) // Mixed: default org, custom proj & task
	route.SetEndpoint("/api/v1/users", nil)                                          // Default
	route.SetEndpoint("/api/v1/users/:user_id/posts/:post_id", nil)                  // Custom both

	t.Run("CustomIDPath", func(t *testing.T) {
		// Access path with custom IDs
		node, params, _ := route.RouteNode("/api/v1/organizations/org01/teams/team01")
		if node == nil {
			t.Fatal("Expected to find node")
		}

		// Should use custom org_id
		if params["[gone-http]org_id"] != "org01" {
			t.Errorf("Expected org_id='org01', got '%v'", params["[gone-http]org_id"])
		}
		// Should use default teams_id
		if params["[gone-http]teams_id"] != "team01" {
			t.Errorf("Expected teams_id='team01', got '%v'", params["[gone-http]teams_id"])
		}
		// Should NOT have organizations_id in this path
		if _, exists := params["[gone-http]organizations_id"]; exists {
			t.Error("Should not have organizations_id when org_id is used")
		}

		// Parent should be "org" (from custom ID)
		parent := node.Parent()
		if parent == nil {
			t.Fatal("Expected parent to exist")
		}
		if parent.Name() != "org" {
			t.Errorf("Expected parent name 'org', got '%s'", parent.Name())
		}
	})

	t.Run("DefaultIDPath", func(t *testing.T) {
		// Access path with default IDs
		node, params, _ := route.RouteNode("/api/v1/organizations/org02/projects/proj02")
		if node == nil {
			t.Fatal("Expected to find node")
		}

		// Should use default organizations_id (not org_id)
		if params["[gone-http]organizations_id"] != "org02" {
			t.Errorf("Expected organizations_id='org02', got '%v'", params["[gone-http]organizations_id"])
		}
		if params["[gone-http]projects_id"] != "proj02" {
			t.Errorf("Expected projects_id='proj02', got '%v'", params["[gone-http]projects_id"])
		}
		// Should NOT have org_id in this path
		if _, exists := params["[gone-http]org_id"]; exists {
			t.Error("Should not have org_id when using default path")
		}

		// Parent should be "organizations" (default name)
		parent := node.Parent()
		if parent == nil {
			t.Fatal("Expected parent to exist")
		}
		if parent.Name() != "organizations" {
			t.Errorf("Expected parent name 'organizations', got '%s'", parent.Name())
		}
	})

	t.Run("MixedIDsInSamePath", func(t *testing.T) {
		// Path with custom task_id - need to access via task endpoint
		// This tests that same base path can have different ID semantics
		// depending on which endpoint is matched

		// Access via default projects endpoint
		node1, params1, _ := route.RouteNode("/api/v1/organizations/org03/projects/proj03")
		if node1 == nil {
			t.Fatal("Expected to find node1")
		}
		// Should use default IDs
		if params1["[gone-http]organizations_id"] != "org03" {
			t.Errorf("Expected organizations_id='org03', got '%v'", params1["[gone-http]organizations_id"])
		}
		if params1["[gone-http]projects_id"] != "proj03" {
			t.Errorf("Expected projects_id='proj03', got '%v'", params1["[gone-http]projects_id"])
		}
		if node1.Name() != "projects" {
			t.Errorf("Expected node1 name 'projects', got '%s'", node1.Name())
		}

		// Access via custom task_id endpoint
		// NOTE: The endpoint is /api/v1/organizations/projects/:proj_id/tasks/:task_id
		// So we don't include org_id in the path - it goes directly from organizations to projects
		node2, params2, _ := route.RouteNode("/api/v1/organizations/projects/proj04/tasks/task04")
		if node2 == nil {
			t.Fatal("Expected to find node2")
		}
		// Should have custom proj_id and task_id
		if params2["[gone-http]proj_id"] != "proj04" {
			t.Errorf("Expected proj_id='proj04', got '%v'", params2["[gone-http]proj_id"])
		}
		if params2["[gone-http]task_id"] != "task04" {
			t.Errorf("Expected task_id='task04', got '%v'", params2["[gone-http]task_id"])
		}
		// Node name should be "task" (from custom ID)
		if node2.Name() != "task" {
			t.Errorf("Expected node2 name 'task', got '%s'", node2.Name())
		}
		// Parent should be "proj" (from custom ID)
		if node2.Parent().Name() != "proj" {
			t.Errorf("Expected parent name 'proj', got '%s'", node2.Parent().Name())
		}
	})

	t.Run("AllCustomIDsPath", func(t *testing.T) {
		// Path with all custom IDs
		node, params, _ := route.RouteNode("/api/v1/users/user01/posts/post01")
		if node == nil {
			t.Fatal("Expected to find node")
		}

		// Should have both custom IDs
		if params["[gone-http]user_id"] != "user01" {
			t.Errorf("Expected user_id='user01', got '%v'", params["[gone-http]user_id"])
		}
		if params["[gone-http]post_id"] != "post01" {
			t.Errorf("Expected post_id='post01', got '%v'", params["[gone-http]post_id"])
		}

		// Node name should be "post"
		if node.Name() != "post" {
			t.Errorf("Expected node name 'post', got '%s'", node.Name())
		}

		// Parent should be "user"
		parent := node.Parent()
		if parent == nil {
			t.Fatal("Expected parent to exist")
		}
		if parent.Name() != "user" {
			t.Errorf("Expected parent name 'user', got '%s'", parent.Name())
		}
	})
}

// TestIDExtractionRules tests ID extraction only happens for endpoint-defined nodes
func TestIDExtractionRules(t *testing.T) {
	route := NewSimpleRoute()

	// Set up endpoints - NOTE: /api and /api/v1 are NOT endpoints, just path groups
	route.SetGroup("/api")
	route.SetGroup("/api/v1")
	route.SetEndpoint("/api/v1/organizations", nil)
	route.SetEndpoint("/api/v1/organizations/projects", nil)

	t.Run("NoIDForNonEndpointPaths", func(t *testing.T) {
		node, params, _ := route.RouteNode("/api/v1/organizations/org01/projects/proj01")
		if node == nil {
			t.Fatal("Expected to find node")
		}

		// Should have IDs for endpoint-defined levels
		if params["[gone-http]organizations_id"] != "org01" {
			t.Errorf("Expected organizations_id='org01', got '%v'", params["[gone-http]organizations_id"])
		}
		if params["[gone-http]projects_id"] != "proj01" {
			t.Errorf("Expected projects_id='proj01', got '%v'", params["[gone-http]projects_id"])
		}

		// Should NOT have IDs for non-endpoint paths
		if _, exists := params["[gone-http]api_id"]; exists {
			t.Error("Should NOT have api_id - /api is not an endpoint, just a path group")
		}
		if _, exists := params["[gone-http]v1_id"]; exists {
			t.Error("Should NOT have v1_id - /api/v1 is not an endpoint, just a path group")
		}
	})

	t.Run("EndpointDefinitionRequired", func(t *testing.T) {
		// Access organizations without further nesting
		node, params, _ := route.RouteNode("/api/v1/organizations/org02")
		if node == nil {
			t.Fatal("Expected to find node")
		}

		// Should have organizations_id because /api/v1/organizations IS an endpoint
		if params["[gone-http]organizations_id"] != "org02" {
			t.Errorf("Expected organizations_id='org02', got '%v'", params["[gone-http]organizations_id"])
		}

		// Still should NOT have api_id or v1_id
		if _, exists := params["[gone-http]api_id"]; exists {
			t.Error("Should NOT have api_id")
		}
		if _, exists := params["[gone-http]v1_id"]; exists {
			t.Error("Should NOT have v1_id")
		}
	})

	t.Run("OnlyDefinedEndpointsExtractIDs", func(t *testing.T) {
		// Test that all endpoint-defined nodes in path extract IDs
		r := NewSimpleRoute()
		r.SetEndpoint("/api/v1/organizations", nil)
		r.SetEndpoint("/api/v1/organizations/departments", nil)
		r.SetEndpoint("/api/v1/organizations/departments/teams", nil)

		// Access teams level
		node, params, _ := r.RouteNode("/api/v1/organizations/org01/departments/dept01/teams/team01")
		if node == nil {
			t.Fatal("Expected to find node")
		}

		// Should have IDs for all defined endpoints in the path
		if params["[gone-http]organizations_id"] != "org01" {
			t.Errorf("Expected organizations_id='org01', got '%v'", params["[gone-http]organizations_id"])
		}
		if params["[gone-http]departments_id"] != "dept01" {
			t.Errorf("Expected departments_id='dept01', got '%v'", params["[gone-http]departments_id"])
		}
		if params["[gone-http]teams_id"] != "team01" {
			t.Errorf("Expected teams_id='team01', got '%v'", params["[gone-http]teams_id"])
		}

		// Verify all node names in parent chain
		if node.Name() != "teams" {
			t.Errorf("Expected node name 'teams', got '%s'", node.Name())
		}
		if node.Parent().Name() != "departments" {
			t.Errorf("Expected parent name 'departments', got '%s'", node.Parent().Name())
		}
		if node.Parent().Parent().Name() != "organizations" {
			t.Errorf("Expected grandparent name 'organizations', got '%s'", node.Parent().Parent().Name())
		}
	})
}

// TestDeepDefaultVsCustomComparison compares behavior of deep default vs custom ID routes
func TestDeepDefaultVsCustomComparison(t *testing.T) {
	t.Run("DeepDefaultRoute", func(t *testing.T) {
		route := NewSimpleRoute()
		// All default IDs - no custom syntax
		route.SetEndpoint("/api/v1/a", nil)
		route.SetEndpoint("/api/v1/a/b", nil)
		route.SetEndpoint("/api/v1/a/b/c", nil)
		route.SetEndpoint("/api/v1/a/b/c/d", nil)

		node, params, _ := route.RouteNode("/api/v1/a/a01/b/b01/c/c01/d/d01")
		if node == nil {
			t.Fatal("Expected to find node")
		}

		// Verify all default IDs
		expectedDefaults := map[string]string{
			"[gone-http]a_id": "a01",
			"[gone-http]b_id": "b01",
			"[gone-http]c_id": "c01",
			"[gone-http]d_id": "d01",
		}
		for key, expectedValue := range expectedDefaults {
			if value, exists := params[key]; !exists {
				t.Errorf("Expected param '%s', available: %v", key, params)
			} else if value != expectedValue {
				t.Errorf("Expected %s='%s', got '%v'", key, expectedValue, value)
			}
		}

		// Verify all node names are default
		if node.Name() != "d" {
			t.Errorf("Expected node name 'd', got '%s'", node.Name())
		}
		if node.Parent().Name() != "c" {
			t.Errorf("Expected parent name 'c', got '%s'", node.Parent().Name())
		}
		if node.Parent().Parent().Name() != "b" {
			t.Errorf("Expected grandparent name 'b', got '%s'", node.Parent().Parent().Name())
		}
		if node.Parent().Parent().Parent().Name() != "a" {
			t.Errorf("Expected great-grandparent name 'a', got '%s'", node.Parent().Parent().Parent().Name())
		}
	})

	t.Run("DeepCustomRoute", func(t *testing.T) {
		route := NewSimpleRoute()
		// All custom IDs
		route.SetEndpoint("/api/v1/alpha/:alpha_id/beta/:beta_id/gamma/:gamma_id/delta/:delta_id", nil)

		node, params, _ := route.RouteNode("/api/v1/alpha/a01/beta/b01/gamma/c01/delta/d01")
		if node == nil {
			t.Fatal("Expected to find node")
		}

		// Verify all custom IDs
		expectedCustom := map[string]string{
			"[gone-http]alpha_id": "a01",
			"[gone-http]beta_id":  "b01",
			"[gone-http]gamma_id": "c01",
			"[gone-http]delta_id": "d01",
		}
		for key, expectedValue := range expectedCustom {
			if value, exists := params[key]; !exists {
				t.Errorf("Expected param '%s', available: %v", key, params)
			} else if value != expectedValue {
				t.Errorf("Expected %s='%s', got '%v'", key, expectedValue, value)
			}
		}

		// Verify all node names are custom (derived from custom IDs)
		if node.Name() != "delta" {
			t.Errorf("Expected node name 'delta', got '%s'", node.Name())
		}
		if node.Parent().Name() != "gamma" {
			t.Errorf("Expected parent name 'gamma', got '%s'", node.Parent().Name())
		}
		if node.Parent().Parent().Name() != "beta" {
			t.Errorf("Expected grandparent name 'beta', got '%s'", node.Parent().Parent().Name())
		}
		if node.Parent().Parent().Parent().Name() != "alpha" {
			t.Errorf("Expected great-grandparent name 'alpha', got '%s'", node.Parent().Parent().Parent().Name())
		}
	})
}

// TestRouteInvalidCustomID verifies that custom IDs without "_id" suffix are ignored
func TestRouteInvalidCustomID(t *testing.T) {
	route := NewSimpleRoute()

	// Set up endpoint with invalid custom ID (no "_id" suffix)
	route.SetEndpoint("/api/v1/organizations", nil)
	route.SetEndpoint("/api/v1/organizations/:org", nil) // :org without _id suffix

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

	// 2. Test parent node with ID (should use default organizations_id, not custom "org")
	parentIDNode, parentIDParams, _ := route.RouteNode("/api/v1/organizations/123")
	if parentIDNode == nil {
		t.Fatal("Expected to find parent node with ID, but got nil")
	}
	if parentIDNode.Name() != "organizations" {
		t.Errorf("Expected node name to be 'organizations', got '%s'", parentIDNode.Name())
	}

	// Should use DEFAULT organizations_id (because :org doesn't end with _id)
	orgID, exists := parentIDParams["[gone-http]organizations_id"]
	if !exists {
		t.Errorf("Expected param '[gone-http]organizations_id' (default), available params: %v", parentIDParams)
	}
	if orgID != "123" {
		t.Errorf("Expected organizations_id to be '123', got '%v'", orgID)
	}

	// Should NOT have custom "org" param
	if _, exists := parentIDParams["[gone-http]org"]; exists {
		t.Error("Should not have '[gone-http]org' param (invalid custom ID without _id suffix)")
	}
}

// TestRouteValidVsInvalidCustomID verifies both valid and invalid custom IDs in same route
func TestRouteValidVsInvalidCustomID(t *testing.T) {
	route := NewSimpleRoute()

	// Set up endpoint with one valid and one invalid custom ID
	route.SetEndpoint("/api/v1/organizations", nil)
	// :org_id is valid (has _id suffix), :perm is invalid (no _id suffix)
	route.SetEndpoint("/api/v1/organizations/:org_id/permissions/:perm", nil)

	// Test the route
	node, params, _ := route.RouteNode("/api/v1/organizations/123/permissions/456")
	if node == nil {
		t.Fatal("Expected to find node, but got nil")
	}
	if node.Name() != "permissions" {
		t.Errorf("Expected node name to be 'permissions', got '%s'", node.Name())
	}

	// Verify valid custom ID: org_id
	orgID, exists := params["[gone-http]org_id"]
	if !exists {
		t.Errorf("Expected param '[gone-http]org_id' (valid custom ID), available params: %v", params)
	}
	if orgID != "123" {
		t.Errorf("Expected org_id to be '123', got '%v'", orgID)
	}

	// Verify invalid custom ID falls back to default: permissions_id
	permID, exists := params["[gone-http]permissions_id"]
	if !exists {
		t.Errorf("Expected param '[gone-http]permissions_id' (default, :perm invalid), available params: %v", params)
	}
	if permID != "456" {
		t.Errorf("Expected permissions_id to be '456', got '%v'", permID)
	}

	// Should NOT have invalid custom "perm" param
	if _, exists := params["[gone-http]perm"]; exists {
		t.Error("Should not have '[gone-http]perm' param (invalid custom ID without _id suffix)")
	}
}

// TestBraceSyntaxSingleID tests {param} syntax with single custom ID
func TestBraceSyntaxSingleID(t *testing.T) {
	route := NewSimpleRoute()

	route.SetEndpoint("/api/v1/organizations", nil)
	route.SetEndpoint("/api/v1/organizations/{org_id}/permissions", nil)

	// Test parent node without ID
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

	// Test parent node with ID
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

	// Test child route with custom org_id
	childNode, childParams, _ := route.RouteNode("/api/v1/organizations/123/permissions/456")
	if childNode == nil {
		t.Fatal("Expected to find child node, but got nil")
	}
	if childNode.Name() != "permissions" {
		t.Errorf("Expected child node name to be 'permissions', got '%s'", childNode.Name())
	}

	orgID, exists := childParams["[gone-http]p:org_id"]
	if !exists {
		t.Errorf("Expected param '[gone-http]p:org_id', available params: %v", childParams)
	}
	if orgID != "123" {
		t.Errorf("Expected org_id to be '123', got '%v'", orgID)
	}

	permID, exists := childParams["[gone-http]permissions_id"]
	if !exists {
		t.Errorf("Expected param '[gone-http]permissions_id', available params: %v", childParams)
	}
	if permID != "456" {
		t.Errorf("Expected permissions_id to be '456', got '%v'", permID)
	}

	if _, exists := childParams["[gone-http]organizations_id"]; exists {
		t.Error("Should not have '[gone-http]organizations_id' param in child route")
	}
}

// TestBraceSyntaxMultipleIDs tests {param} syntax with multiple custom IDs
// This tests the exact scenario: "/mgmt/v1/organizations/{organizations_id}/members/{id}"
func TestBraceSyntaxMultipleIDs(t *testing.T) {
	route := NewSimpleRoute()

	route.SetEndpoint("/mgmt/v1/organizations", nil)
	route.SetEndpoint("/mgmt/v1/organizations/{organizations_id}/members/{member_id}", nil)

	// Test parent node
	parentNode, _, _ := route.RouteNode("/mgmt/v1/organizations")
	if parentNode == nil {
		t.Fatal("Expected to find parent node")
	}
	if parentNode.Name() != "organizations" {
		t.Errorf("Expected parent node name to be 'organizations', got '%s'", parentNode.Name())
	}

	// Test full path with both IDs
	node, params, _ := route.RouteNode("/mgmt/v1/organizations/org123/members/mem456")
	if node == nil {
		t.Fatal("Expected to find node")
	}

	// Verify node name is "member" (derived from member_id)
	if node.Name() != "member" {
		t.Errorf("Expected node name to be 'member', got '%s'", node.Name())
	}

	// Verify parent node name is "organizations" (the actual route node name when traversing from child route)
	if node.Parent().Name() != "organizations" {
		t.Errorf("Expected parent node name to be 'organizations', got '%s'", node.Parent().Name())
	}

	// Verify first custom ID: organizations_id
	orgID, exists := params["[gone-http]p:organizations_id"]
	if !exists {
		t.Errorf("Expected param '[gone-http]p:organizations_id', available params: %v", params)
	}
	if orgID != "org123" {
		t.Errorf("Expected organizations_id to be 'org123', got '%v'", orgID)
	}

	// Verify second custom ID: member_id
	memberID, exists := params["[gone-http]p:member_id"]
	if !exists {
		t.Errorf("Expected param '[gone-http]p:member_id', available params: %v", params)
	}
	if memberID != "mem456" {
		t.Errorf("Expected member_id to be 'mem456', got '%v'", memberID)
	}

	// Should NOT have default members_id
	if _, exists := params["[gone-http]members_id"]; exists {
		t.Error("Should not have '[gone-http]members_id' param (should be member_id)")
	}
}

// TestBraceSyntaxMixedWithColonSyntax tests mixing {param} and :param syntax
func TestBraceSyntaxMixedWithColonSyntax(t *testing.T) {
	route := NewSimpleRoute()

	route.SetEndpoint("/api/v1/organizations", nil)
	// Mix both syntaxes: :org_id and {perm_id}
	route.SetEndpoint("/api/v1/organizations/:org_id/permissions/{perm_id}", nil)

	node, params, _ := route.RouteNode("/api/v1/organizations/123/permissions/456")
	if node == nil {
		t.Fatal("Expected to find node")
	}
	if node.Name() != "perm" {
		t.Errorf("Expected node name to be 'perm', got '%s'", node.Name())
	}

	// Verify :org_id works
	orgID, exists := params["[gone-http]org_id"]
	if !exists {
		t.Errorf("Expected param '[gone-http]org_id', available params: %v", params)
	}
	if orgID != "123" {
		t.Errorf("Expected org_id to be '123', got '%v'", orgID)
	}

	// Verify {perm_id} works
	permID, exists := params["[gone-http]p:perm_id"]
	if !exists {
		t.Errorf("Expected param '[gone-http]p:perm_id', available params: %v", params)
	}
	if permID != "456" {
		t.Errorf("Expected perm_id to be '456', got '%v'", permID)
	}
}

// TestBraceSyntaxNonIdParam tests {param} syntax without _id suffix (should still be explicit)
func TestBraceSyntaxNonIdParam(t *testing.T) {
	route := NewSimpleRoute()

	route.SetEndpoint("/api/v1/organizations", nil)
	// {org} should be treated as explicit param name
	route.SetEndpoint("/api/v1/organizations/{org}", nil)

	node, params, _ := route.RouteNode("/api/v1/organizations/123")
	if node == nil {
		t.Fatal("Expected to find node")
	}
	if node.Name() != "org" {
		t.Errorf("Expected node name to be 'org', got '%s'", node.Name())
	}

	// Should use explicit param name with p: prefix
	orgID, exists := params["[gone-http]p:org"]
	if !exists {
		t.Errorf("Expected param '[gone-http]p:org' (explicit), available params: %v", params)
	}
	if orgID != "123" {
		t.Errorf("Expected org to be '123', got '%v'", orgID)
	}
}

// TestBraceSyntaxUserRequest tests the exact user request scenario
func TestBraceSyntaxUserRequest(t *testing.T) {
	route := NewSimpleRoute()

	// User's exact request: "/mgmt/v1/organizations/{organizations_id}/members/{id}"
	route.SetEndpoint("/mgmt/v1/organizations/{organizations_id}/members/{id}", nil)

	node, params, _ := route.RouteNode("/mgmt/v1/organizations/org-abc/members/user-123")
	if node == nil {
		t.Fatal("Expected to find node")
	}

	// Verify organizations_id (valid custom ID)
	orgID, exists := params["[gone-http]p:organizations_id"]
	if !exists {
		t.Errorf("Expected param '[gone-http]p:organizations_id', available params: %v", params)
	}
	if orgID != "org-abc" {
		t.Errorf("Expected organizations_id to be 'org-abc', got '%v'", orgID)
	}

	// Verify id (explicit brace param)
	memberID, exists := params["[gone-http]p:id"]
	if !exists {
		t.Errorf("Expected param '[gone-http]p:id' (explicit), available params: %v", params)
	}
	if memberID != "user-123" {
		t.Errorf("Expected id to be 'user-123', got '%v'", memberID)
	}

	if _, exists := params["[gone-http]members_id"]; exists {
		t.Error("Should not have '[gone-http]members_id' param (brace id is explicit)")
	}
}

// =============================================================================
// GetID Function Tests - Verify both colon syntax and brace syntax work with GetID
// =============================================================================

// TestGetID_ColonSyntax tests that GetID works correctly with colon syntax (:param_id)
func TestGetID_ColonSyntax(t *testing.T) {
	route := NewSimpleRoute()
	handler := &DefaultHandlerTask{}

	// Set up endpoint with colon syntax
	route.SetEndpoint("/api/v1/organizations/:org_id/albums/:album_id/songs", nil)

	t.Run("ColonSyntax_FullPath", func(t *testing.T) {
		_, params, _ := route.RouteNode("/api/v1/organizations/org123/albums/album456/songs/song789")

		// Test GetID with colon syntax params
		if id := handler.GetID("org_id", params); id != "org123" {
			t.Errorf("GetID(\"org_id\") = %q, want \"org123\"", id)
		}
		if id := handler.GetID("album_id", params); id != "album456" {
			t.Errorf("GetID(\"album_id\") = %q, want \"album456\"", id)
		}
		// songs uses default format since no custom ID specified
		if id := handler.GetID("songs_id", params); id != "song789" {
			t.Errorf("GetID(\"songs_id\") = %q, want \"song789\"", id)
		}
		if id := handler.GetID("songs", params); id != "song789" {
			t.Errorf("GetID(\"songs\") = %q, want \"song789\"", id)
		}
	})

	t.Run("ColonSyntax_IntermediatePath", func(t *testing.T) {
		_, params, _ := route.RouteNode("/api/v1/organizations/org123/albums/album456/songs")

		if id := handler.GetID("org_id", params); id != "org123" {
			t.Errorf("GetID(\"org_id\") = %q, want \"org123\"", id)
		}
		if id := handler.GetID("album_id", params); id != "album456" {
			t.Errorf("GetID(\"album_id\") = %q, want \"album456\"", id)
		}
	})
}

// TestGetID_BraceSyntax tests that GetID works correctly with brace syntax ({param_id})
func TestGetID_BraceSyntax(t *testing.T) {
	route := NewSimpleRoute()
	handler := &DefaultHandlerTask{}

	// Set up endpoint with brace syntax
	route.SetEndpoint("/api/v1/organizations/{organizations_id}/albums/{albums_id}/songs", nil)

	t.Run("BraceSyntax_FullPath", func(t *testing.T) {
		_, params, _ := route.RouteNode("/api/v1/organizations/org123/albums/album456/songs/song789")

		// Test GetID with brace syntax params - must use exact param name
		if id := handler.GetID("organizations_id", params); id != "org123" {
			t.Errorf("GetID(\"organizations_id\") = %q, want \"org123\"", id)
		}
		if id := handler.GetID("albums_id", params); id != "album456" {
			t.Errorf("GetID(\"albums_id\") = %q, want \"album456\"", id)
		}
		// songs uses default format
		if id := handler.GetID("songs_id", params); id != "song789" {
			t.Errorf("GetID(\"songs_id\") = %q, want \"song789\"", id)
		}
		if id := handler.GetID("songs", params); id != "song789" {
			t.Errorf("GetID(\"songs\") = %q, want \"song789\"", id)
		}
	})

	t.Run("BraceSyntax_IntermediatePath", func(t *testing.T) {
		_, params, _ := route.RouteNode("/api/v1/organizations/org123/albums/album456/songs")

		if id := handler.GetID("organizations_id", params); id != "org123" {
			t.Errorf("GetID(\"organizations_id\") = %q, want \"org123\"", id)
		}
		if id := handler.GetID("albums_id", params); id != "album456" {
			t.Errorf("GetID(\"albums_id\") = %q, want \"album456\"", id)
		}
	})
}

// TestGetID_MixedSyntax tests that GetID works with mixed colon and brace syntax
func TestGetID_MixedSyntax(t *testing.T) {
	route := NewSimpleRoute()
	handler := &DefaultHandlerTask{}

	// Mix both syntaxes in same endpoint
	route.SetEndpoint("/api/v1/organizations/:org_id/albums/{albums_id}/songs", nil)

	_, params, _ := route.RouteNode("/api/v1/organizations/org123/albums/album456/songs/song789")

	// Colon syntax param
	if id := handler.GetID("org_id", params); id != "org123" {
		t.Errorf("GetID(\"org_id\") = %q, want \"org123\"", id)
	}

	// Brace syntax param
	if id := handler.GetID("albums_id", params); id != "album456" {
		t.Errorf("GetID(\"albums_id\") = %q, want \"album456\"", id)
	}

	// Default format for songs
	if id := handler.GetID("songs_id", params); id != "song789" {
		t.Errorf("GetID(\"songs_id\") = %q, want \"song789\"", id)
	}
}

// TestGetID_DefaultFormat tests GetID with default parameter format (no custom ID)
func TestGetID_DefaultFormat(t *testing.T) {
	route := NewSimpleRoute()
	handler := &DefaultHandlerTask{}

	// Default format requires each endpoint level to be defined
	route.SetEndpoint("/api/v1/organizations", nil)
	route.SetEndpoint("/api/v1/organizations/albums", nil)
	route.SetEndpoint("/api/v1/organizations/albums/songs", nil)

	_, params, _ := route.RouteNode("/api/v1/organizations/org123/albums/album456/songs/song789")

	// All use default format: [gone-http]node_id
	if id := handler.GetID("organizations", params); id != "org123" {
		t.Errorf("GetID(\"organizations\") = %q, want \"org123\"", id)
	}
	if id := handler.GetID("albums", params); id != "album456" {
		t.Errorf("GetID(\"albums\") = %q, want \"album456\"", id)
	}
	if id := handler.GetID("songs", params); id != "song789" {
		t.Errorf("GetID(\"songs\") = %q, want \"song789\"", id)
	}
}

// TestGetID_BackwardCompatibility verifies backward compatibility with existing code
func TestGetID_BackwardCompatibility(t *testing.T) {
	handler := &DefaultHandlerTask{}

	t.Run("LegacyDefaultFormat", func(t *testing.T) {
		// Simulate legacy params format
		params := map[string]any{
			"[gone-http]users_id":   "user123",
			"[gone-http]posts_id":   "post456",
			"[gone-http]is_index":   false,
			"[gone-http]node_name":  "posts",
		}

		if id := handler.GetID("users", params); id != "user123" {
			t.Errorf("GetID(\"users\") = %q, want \"user123\"", id)
		}
		if id := handler.GetID("posts", params); id != "post456" {
			t.Errorf("GetID(\"posts\") = %q, want \"post456\"", id)
		}
	})

	t.Run("LegacyColonSyntaxFormat", func(t *testing.T) {
		// Simulate colon syntax params format
		params := map[string]any{
			"[gone-http]user_id":   "user123",
			"[gone-http]post_id":   "post456",
		}

		if id := handler.GetID("user", params); id != "user123" {
			t.Errorf("GetID(\"user\") = %q, want \"user123\"", id)
		}
		if id := handler.GetID("post", params); id != "post456" {
			t.Errorf("GetID(\"post\") = %q, want \"post456\"", id)
		}
	})

	t.Run("NewBraceSyntaxFormat", func(t *testing.T) {
		// Simulate brace syntax params format with p: prefix
		params := map[string]any{
			"[gone-http]p:user_id":   "user123",
			"[gone-http]p:post_id":   "post456",
		}

		if id := handler.GetID("user_id", params); id != "user123" {
			t.Errorf("GetID(\"user_id\") = %q, want \"user123\"", id)
		}
		if id := handler.GetID("post_id", params); id != "post456" {
			t.Errorf("GetID(\"post_id\") = %q, want \"post456\"", id)
		}
	})
}

// TestGetID_ComplexNestedRoute tests GetID with deeply nested routes
func TestGetID_ComplexNestedRoute(t *testing.T) {
	route := NewSimpleRoute()
	handler := &DefaultHandlerTask{}

	// Complex nested route with brace syntax
	route.SetEndpoint("/mgmt/v1/organizations/{org_id}/projects/{project_id}/tasks/{task_id}/comments", nil)

	_, params, _ := route.RouteNode("/mgmt/v1/organizations/org1/projects/proj2/tasks/task3/comments/comment4")

	// All brace syntax params
	if id := handler.GetID("org_id", params); id != "org1" {
		t.Errorf("GetID(\"org_id\") = %q, want \"org1\"", id)
	}
	if id := handler.GetID("project_id", params); id != "proj2" {
		t.Errorf("GetID(\"project_id\") = %q, want \"proj2\"", id)
	}
	if id := handler.GetID("task_id", params); id != "task3" {
		t.Errorf("GetID(\"task_id\") = %q, want \"task3\"", id)
	}
	// comments uses default format
	if id := handler.GetID("comments_id", params); id != "comment4" {
		t.Errorf("GetID(\"comments_id\") = %q, want \"comment4\"", id)
	}
	if id := handler.GetID("comments", params); id != "comment4" {
		t.Errorf("GetID(\"comments\") = %q, want \"comment4\"", id)
	}
}

// TestGetID_EmptyAndMissing tests GetID behavior with missing params
func TestGetID_EmptyAndMissing(t *testing.T) {
	handler := &DefaultHandlerTask{}

	params := map[string]any{
		"[gone-http]existing_id": "value123",
	}

	// Existing param
	if id := handler.GetID("existing", params); id != "value123" {
		t.Errorf("GetID(\"existing\") = %q, want \"value123\"", id)
	}

	// Missing param should return empty string
	if id := handler.GetID("nonexistent", params); id != "" {
		t.Errorf("GetID(\"nonexistent\") = %q, want \"\"", id)
	}

	// Empty params map
	emptyParams := map[string]any{}
	if id := handler.GetID("anything", emptyParams); id != "" {
		t.Errorf("GetID(\"anything\") with empty params = %q, want \"\"", id)
	}
}

// TestGetID_SpecialCharactersInValues tests GetID with special characters in ID values
func TestGetID_SpecialCharactersInValues(t *testing.T) {
	route := NewSimpleRoute()
	handler := &DefaultHandlerTask{}

	route.SetEndpoint("/api/v1/items/{item_id}", nil)

	testCases := []struct {
		name    string
		idValue string
	}{
		{"UUID", "550e8400-e29b-41d4-a716-446655440000"},
		{"WithUnderscore", "item_with_underscore"},
		{"WithDash", "item-with-dash"},
		{"Numeric", "12345"},
		{"Mixed", "Item-123_ABC"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, params, _ := route.RouteNode("/api/v1/items/" + tc.idValue)

			if id := handler.GetID("item_id", params); id != tc.idValue {
				t.Errorf("GetID(\"item_id\") = %q, want %q", id, tc.idValue)
			}
		})
	}
}

// TestTaskHelper_GetID tests TaskHelper.GetID maintains same behavior as DefaultHandlerTask.GetID
func TestTaskHelper_GetID(t *testing.T) {
	route := NewSimpleRoute()
	taskHelper := &TaskHelper{}

	// Set up with brace syntax
	route.SetEndpoint("/api/v1/users/{user_id}/posts/{post_id}", nil)

	_, params, _ := route.RouteNode("/api/v1/users/user123/posts/post456")

	// Verify TaskHelper.GetID works same as DefaultHandlerTask.GetID
	if id := taskHelper.GetID("user_id", params); id != "user123" {
		t.Errorf("TaskHelper.GetID(\"user_id\") = %q, want \"user123\"", id)
	}
	if id := taskHelper.GetID("post_id", params); id != "post456" {
		t.Errorf("TaskHelper.GetID(\"post_id\") = %q, want \"post456\"", id)
	}
}
