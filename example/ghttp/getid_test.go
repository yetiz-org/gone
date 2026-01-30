package example

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	http2 "net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/yetiz-org/gone/channel"
	"github.com/yetiz-org/gone/ghttp"
	"github.com/yetiz-org/goth-kklogger"
)

// =============================================================================
// Handler Tasks for GetID Integration Tests
// =============================================================================

// BraceSyntaxHandler tests {param} syntax
type BraceSyntaxHandler struct {
	ghttp.DefaultHTTPHandlerTask
}

func (h *BraceSyntaxHandler) Get(ctx channel.HandlerContext, req *ghttp.Request, resp *ghttp.Response, params map[string]any) ghttp.ErrorResponse {
	result := map[string]string{
		"org_id":   h.GetID("org_id", params),
		"album_id": h.GetID("album_id", params),
		"song_id":  h.GetID("song_id", params),
	}
	resp.JsonResponse(result)
	return nil
}

// ColonSyntaxHandler tests :param syntax
type ColonSyntaxHandler struct {
	ghttp.DefaultHTTPHandlerTask
}

func (h *ColonSyntaxHandler) Get(ctx channel.HandlerContext, req *ghttp.Request, resp *ghttp.Response, params map[string]any) ghttp.ErrorResponse {
	result := map[string]string{
		"user_id": h.GetID("user_id", params),
		"post_id": h.GetID("post_id", params),
	}
	resp.JsonResponse(result)
	return nil
}

// DefaultSyntaxHandler tests default node name syntax
type DefaultSyntaxHandler struct {
	ghttp.DefaultHTTPHandlerTask
}

func (h *DefaultSyntaxHandler) Get(ctx channel.HandlerContext, req *ghttp.Request, resp *ghttp.Response, params map[string]any) ghttp.ErrorResponse {
	result := map[string]string{
		"categories": h.GetID("categories", params),
		"products":   h.GetID("products", params),
		"reviews":    h.GetID("reviews", params),
	}
	resp.JsonResponse(result)
	return nil
}

// MixedSyntaxHandler tests mixed syntax in single route
type MixedSyntaxHandler struct {
	ghttp.DefaultHTTPHandlerTask
}

func (h *MixedSyntaxHandler) Get(ctx channel.HandlerContext, req *ghttp.Request, resp *ghttp.Response, params map[string]any) ghttp.ErrorResponse {
	result := map[string]string{
		"org_id":  h.GetID("org_id", params),
		"team_id": h.GetID("team_id", params),
		"members": h.GetID("members", params),
	}
	resp.JsonResponse(result)
	return nil
}

// DeepNestedHandler tests deeply nested routes with brace syntax
type DeepNestedHandler struct {
	ghttp.DefaultHTTPHandlerTask
}

func (h *DeepNestedHandler) Get(ctx channel.HandlerContext, req *ghttp.Request, resp *ghttp.Response, params map[string]any) ghttp.ErrorResponse {
	result := map[string]string{
		"org_id":     h.GetID("org_id", params),
		"project_id": h.GetID("project_id", params),
		"task_id":    h.GetID("task_id", params),
		"comment_id": h.GetID("comment_id", params),
	}
	resp.JsonResponse(result)
	return nil
}

// DebugParamsHandler returns raw params for debugging
type DebugParamsHandler struct {
	ghttp.DefaultHTTPHandlerTask
}

func (h *DebugParamsHandler) Get(ctx channel.HandlerContext, req *ghttp.Request, resp *ghttp.Response, params map[string]any) ghttp.ErrorResponse {
	filtered := map[string]any{}
	for k, v := range params {
		if k == "[gone-http]context_pack" || k == "[gone-http]dispatcher" || k == "[gone-http]node" {
			continue
		}
		filtered[k] = v
	}
	resp.JsonResponse(filtered)
	return nil
}

// VerifyGetIDHandler returns both raw params and GetID results for verification
type VerifyGetIDHandler struct {
	ghttp.DefaultHTTPHandlerTask
}

func (h *VerifyGetIDHandler) Get(ctx channel.HandlerContext, req *ghttp.Request, resp *ghttp.Response, params map[string]any) ghttp.ErrorResponse {
	// Collect raw params (filtered)
	rawParams := map[string]any{}
	for k, v := range params {
		if k == "[gone-http]context_pack" || k == "[gone-http]dispatcher" || k == "[gone-http]node" {
			continue
		}
		rawParams[k] = v
	}

	// Use GetID to retrieve values
	getIDResults := map[string]string{
		"org_id":    h.GetID("org_id", params),
		"album_id":  h.GetID("album_id", params),
		"song_id":   h.GetID("song_id", params),
	}

	// Return both for verification
	result := map[string]any{
		"raw_params":     rawParams,
		"getid_results":  getIDResults,
	}
	resp.JsonResponse(result)
	return nil
}

// =============================================================================
// Route Setup for GetID Tests
// =============================================================================

func NewGetIDTestRoute() ghttp.Route {
	route := ghttp.NewSimpleRoute()

	// Brace syntax: {param}
	route.SetEndpoint("/api/v1/organizations/{org_id}/albums/{album_id}/songs/{song_id}", &BraceSyntaxHandler{})

	// Colon syntax: :param
	route.SetEndpoint("/api/v1/users/:user_id/posts/:post_id", &ColonSyntaxHandler{})

	// Default syntax: no custom ID
	route.SetEndpoint("/api/v1/categories", &DefaultSyntaxHandler{})
	route.SetEndpoint("/api/v1/categories/products", &DefaultSyntaxHandler{})
	route.SetEndpoint("/api/v1/categories/products/reviews", &DefaultSyntaxHandler{})

	// Mixed syntax: {param} + :param + default
	route.SetEndpoint("/api/v1/orgs/{org_id}/teams/:team_id/members", &MixedSyntaxHandler{})

	// Deep nested with brace syntax
	route.SetEndpoint("/mgmt/v1/organizations/{org_id}/projects/{project_id}/tasks/{task_id}/comments/{comment_id}", &DeepNestedHandler{})

	// Debug endpoint to see raw params
	route.SetEndpoint("/debug/{debug_id}", &DebugParamsHandler{})
	route.SetEndpoint("/debug2/:debug_id", &DebugParamsHandler{})
	route.SetEndpoint("/debug3", &DebugParamsHandler{})

	// Verification endpoint for GetID with brace syntax
	route.SetEndpoint("/verify/orgs/{org_id}/albums/{album_id}/songs/{song_id}", &VerifyGetIDHandler{})

	return route
}

// =============================================================================
// Integration Tests
// =============================================================================

func startGetIDTestServer(t *testing.T, port int) channel.Channel {
	kklogger.SetLogLevel("WARN")
	bootstrap := channel.NewServerBootstrap()
	bootstrap.ChannelType(&ghttp.ServerChannel{})
	bootstrap.Handler(channel.NewInitializer(func(ch channel.Channel) {
		ch.Pipeline().AddLast("INDICATE_HANDLER_INBOUND", &channel.IndicateHandlerInbound{})
		ch.Pipeline().AddLast("INDICATE_HANDLER_OUTBOUND", &channel.IndicateHandlerOutbound{})
	}))

	bootstrap.ChildHandler(channel.NewInitializer(func(ch channel.Channel) {
		ch.Pipeline().AddLast("INDICATE_HANDLER_INBOUND", &channel.IndicateHandlerInbound{})
		ch.Pipeline().AddLast("NET_STATUS_INBOUND", &channel.NetStatusInbound{})
		ch.Pipeline().AddLast("DISPATCHER", ghttp.NewDispatchHandler(NewGetIDTestRoute()))
		ch.Pipeline().AddLast("NET_STATUS_OUTBOUND", &channel.NetStatusOutbound{})
		ch.Pipeline().AddLast("INDICATE_HANDLER_OUTBOUND", &channel.IndicateHandlerOutbound{})
	}))

	ch := bootstrap.Bind(&net.TCPAddr{IP: nil, Port: port}).Sync().Channel()
	time.Sleep(50 * time.Millisecond)
	return ch
}

func getJSON(t *testing.T, url string) map[string]any {
	resp, err := http2.Get(url)
	if err != nil {
		t.Fatalf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Read body failed: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("JSON unmarshal failed: %v, body: %s", err, string(body))
	}
	return result
}

// TestGetID_BraceSyntax_Integration tests brace syntax {param} with real HTTP server
func TestGetID_BraceSyntax_Integration(t *testing.T) {
	ch := startGetIDTestServer(t, 19001)
	defer ch.Close()

	t.Run("BraceSyntax_AllParams", func(t *testing.T) {
		result := getJSON(t, "http://localhost:19001/api/v1/organizations/org-123/albums/album-456/songs/song-789")

		assert.Equal(t, "org-123", result["org_id"])
		assert.Equal(t, "album-456", result["album_id"])
		assert.Equal(t, "song-789", result["song_id"])
	})

	t.Run("BraceSyntax_UUIDValues", func(t *testing.T) {
		result := getJSON(t, "http://localhost:19001/api/v1/organizations/550e8400-e29b-41d4-a716-446655440000/albums/6ba7b810-9dad-11d1-80b4-00c04fd430c8/songs/f47ac10b-58cc-4372-a567-0e02b2c3d479")

		assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000", result["org_id"])
		assert.Equal(t, "6ba7b810-9dad-11d1-80b4-00c04fd430c8", result["album_id"])
		assert.Equal(t, "f47ac10b-58cc-4372-a567-0e02b2c3d479", result["song_id"])
	})

	t.Run("BraceSyntax_NumericValues", func(t *testing.T) {
		result := getJSON(t, "http://localhost:19001/api/v1/organizations/12345/albums/67890/songs/11111")

		assert.Equal(t, "12345", result["org_id"])
		assert.Equal(t, "67890", result["album_id"])
		assert.Equal(t, "11111", result["song_id"])
	})

	time.Sleep(100 * time.Millisecond)
}

// TestGetID_ColonSyntax_Integration tests colon syntax :param with real HTTP server
func TestGetID_ColonSyntax_Integration(t *testing.T) {
	ch := startGetIDTestServer(t, 19002)
	defer ch.Close()

	t.Run("ColonSyntax_AllParams", func(t *testing.T) {
		result := getJSON(t, "http://localhost:19002/api/v1/users/user-abc/posts/post-xyz")

		assert.Equal(t, "user-abc", result["user_id"])
		assert.Equal(t, "post-xyz", result["post_id"])
	})

	t.Run("ColonSyntax_SpecialChars", func(t *testing.T) {
		result := getJSON(t, "http://localhost:19002/api/v1/users/user_with_underscore/posts/post-with-dash")

		assert.Equal(t, "user_with_underscore", result["user_id"])
		assert.Equal(t, "post-with-dash", result["post_id"])
	})

	time.Sleep(100 * time.Millisecond)
}

// TestGetID_DefaultSyntax_Integration tests default node name syntax with real HTTP server
func TestGetID_DefaultSyntax_Integration(t *testing.T) {
	ch := startGetIDTestServer(t, 19003)
	defer ch.Close()

	t.Run("DefaultSyntax_ThreeLevels", func(t *testing.T) {
		result := getJSON(t, "http://localhost:19003/api/v1/categories/cat-001/products/prod-002/reviews/rev-003")

		assert.Equal(t, "cat-001", result["categories"])
		assert.Equal(t, "prod-002", result["products"])
		assert.Equal(t, "rev-003", result["reviews"])
	})

	t.Run("DefaultSyntax_TwoLevels", func(t *testing.T) {
		result := getJSON(t, "http://localhost:19003/api/v1/categories/cat-100/products/prod-200")

		assert.Equal(t, "cat-100", result["categories"])
		assert.Equal(t, "prod-200", result["products"])
	})

	t.Run("DefaultSyntax_OneLevel", func(t *testing.T) {
		result := getJSON(t, "http://localhost:19003/api/v1/categories/cat-only")

		assert.Equal(t, "cat-only", result["categories"])
	})

	time.Sleep(100 * time.Millisecond)
}

// TestGetID_MixedSyntax_Integration tests mixed syntax with real HTTP server
func TestGetID_MixedSyntax_Integration(t *testing.T) {
	ch := startGetIDTestServer(t, 19004)
	defer ch.Close()

	t.Run("MixedSyntax_BraceColonDefault", func(t *testing.T) {
		result := getJSON(t, "http://localhost:19004/api/v1/orgs/org-mixed/teams/team-mixed/members/member-mixed")

		assert.Equal(t, "org-mixed", result["org_id"])
		assert.Equal(t, "team-mixed", result["team_id"])
		assert.Equal(t, "member-mixed", result["members"])
	})

	time.Sleep(100 * time.Millisecond)
}

// TestGetID_DeepNested_Integration tests deeply nested routes with real HTTP server
func TestGetID_DeepNested_Integration(t *testing.T) {
	ch := startGetIDTestServer(t, 19005)
	defer ch.Close()

	t.Run("DeepNested_FourLevels", func(t *testing.T) {
		result := getJSON(t, "http://localhost:19005/mgmt/v1/organizations/org-deep/projects/proj-deep/tasks/task-deep/comments/comment-deep")

		assert.Equal(t, "org-deep", result["org_id"])
		assert.Equal(t, "proj-deep", result["project_id"])
		assert.Equal(t, "task-deep", result["task_id"])
		assert.Equal(t, "comment-deep", result["comment_id"])
	})

	t.Run("DeepNested_ComplexValues", func(t *testing.T) {
		result := getJSON(t, "http://localhost:19005/mgmt/v1/organizations/ORG_123_ABC/projects/proj-456-def/tasks/task_789_ghi/comments/COMMENT-000")

		assert.Equal(t, "ORG_123_ABC", result["org_id"])
		assert.Equal(t, "proj-456-def", result["project_id"])
		assert.Equal(t, "task_789_ghi", result["task_id"])
		assert.Equal(t, "COMMENT-000", result["comment_id"])
	})

	time.Sleep(100 * time.Millisecond)
}

// TestGetID_ParamsFormat_Integration verifies the actual params format stored
func TestGetID_ParamsFormat_Integration(t *testing.T) {
	ch := startGetIDTestServer(t, 19006)
	defer ch.Close()

	t.Run("BraceSyntax_ParamsFormat", func(t *testing.T) {
		result := getJSON(t, "http://localhost:19006/debug/test-value")

		// Brace syntax should store as [gone-http]p:param
		assert.Equal(t, "test-value", result["[gone-http]p:debug_id"])
	})

	t.Run("ColonSyntax_ParamsFormat", func(t *testing.T) {
		result := getJSON(t, "http://localhost:19006/debug2/test-value")

		// Colon syntax should store as [gone-http]param
		assert.Equal(t, "test-value", result["[gone-http]debug_id"])
	})

	t.Run("DefaultSyntax_ParamsFormat", func(t *testing.T) {
		result := getJSON(t, "http://localhost:19006/debug3/test-value")

		// Default syntax should store as [gone-http]node_id
		assert.Equal(t, "test-value", result["[gone-http]debug3_id"])
	})

	time.Sleep(100 * time.Millisecond)
}

// TestGetID_EdgeCases_Integration tests edge cases with real HTTP server
func TestGetID_EdgeCases_Integration(t *testing.T) {
	ch := startGetIDTestServer(t, 19007)
	defer ch.Close()

	t.Run("LongIDValue", func(t *testing.T) {
		longID := ""
		for i := 0; i < 100; i++ {
			longID += "abcdefghij"
		}
		result := getJSON(t, fmt.Sprintf("http://localhost:19007/debug/%s", longID))
		assert.Equal(t, longID, result["[gone-http]p:debug_id"])
	})

	t.Run("SpecialCharactersInID", func(t *testing.T) {
		result := getJSON(t, "http://localhost:19007/debug/test_value-123.456")
		assert.Equal(t, "test_value-123.456", result["[gone-http]p:debug_id"])
	})

	time.Sleep(100 * time.Millisecond)
}

// TestGetID_ConcurrentRequests_Integration tests concurrent access
func TestGetID_ConcurrentRequests_Integration(t *testing.T) {
	ch := startGetIDTestServer(t, 19008)
	defer ch.Close()

	t.Run("ConcurrentBraceSyntax", func(t *testing.T) {
		done := make(chan bool, 20)

		for i := 0; i < 20; i++ {
			go func(idx int) {
				url := fmt.Sprintf("http://localhost:19008/api/v1/organizations/org-%d/albums/album-%d/songs/song-%d", idx, idx*2, idx*3)
				result := getJSON(t, url)

				assert.Equal(t, fmt.Sprintf("org-%d", idx), result["org_id"])
				assert.Equal(t, fmt.Sprintf("album-%d", idx*2), result["album_id"])
				assert.Equal(t, fmt.Sprintf("song-%d", idx*3), result["song_id"])

				done <- true
			}(i)
		}

		for i := 0; i < 20; i++ {
			<-done
		}
	})

	time.Sleep(100 * time.Millisecond)
}

// TestGetID_BackwardCompatibility_Integration ensures old code still works
func TestGetID_BackwardCompatibility_Integration(t *testing.T) {
	ch := startGetIDTestServer(t, 19009)
	defer ch.Close()

	t.Run("ColonSyntax_LegacyBehavior", func(t *testing.T) {
		result := getJSON(t, "http://localhost:19009/api/v1/users/legacy-user/posts/legacy-post")

		// Old colon syntax should still work
		assert.Equal(t, "legacy-user", result["user_id"])
		assert.Equal(t, "legacy-post", result["post_id"])
	})

	t.Run("DefaultSyntax_LegacyBehavior", func(t *testing.T) {
		result := getJSON(t, "http://localhost:19009/api/v1/categories/legacy-cat/products/legacy-prod/reviews/legacy-rev")

		// Default syntax with node name should still work
		assert.Equal(t, "legacy-cat", result["categories"])
		assert.Equal(t, "legacy-prod", result["products"])
		assert.Equal(t, "legacy-rev", result["reviews"])
	})

	time.Sleep(100 * time.Millisecond)
}

// TestGetID_VerifyPrefixKeyExists_Integration verifies that brace syntax stores params with p: prefix
// AND that GetID correctly retrieves values from those p: prefixed keys
func TestGetID_VerifyPrefixKeyExists_Integration(t *testing.T) {
	ch := startGetIDTestServer(t, 19010)
	defer ch.Close()

	t.Run("BraceSyntax_PrefixKeyExists_And_GetID_Works", func(t *testing.T) {
		result := getJSON(t, "http://localhost:19010/verify/orgs/org-123/albums/album-456/songs/song-789")

		// Verify raw_params contains p: prefixed keys
		rawParams, ok := result["raw_params"].(map[string]any)
		if !ok {
			t.Fatal("Expected raw_params to be a map")
		}

		// CRITICAL: Verify p: prefix keys exist in raw params
		assert.Equal(t, "org-123", rawParams["[gone-http]p:org_id"], "p:org_id key must exist in raw params")
		assert.Equal(t, "album-456", rawParams["[gone-http]p:album_id"], "p:album_id key must exist in raw params")
		assert.Equal(t, "song-789", rawParams["[gone-http]p:song_id"], "p:song_id key must exist in raw params")

		// Verify GetID correctly retrieves values from p: prefixed keys
		getIDResults, ok := result["getid_results"].(map[string]any)
		if !ok {
			t.Fatal("Expected getid_results to be a map")
		}

		assert.Equal(t, "org-123", getIDResults["org_id"], "GetID must retrieve org_id from p: prefixed key")
		assert.Equal(t, "album-456", getIDResults["album_id"], "GetID must retrieve album_id from p: prefixed key")
		assert.Equal(t, "song-789", getIDResults["song_id"], "GetID must retrieve song_id from p: prefixed key")
	})

	t.Run("BraceSyntax_UUIDValues_PrefixVerification", func(t *testing.T) {
		result := getJSON(t, "http://localhost:19010/verify/orgs/550e8400-e29b-41d4-a716-446655440000/albums/6ba7b810-9dad-11d1-80b4-00c04fd430c8/songs/f47ac10b-58cc-4372-a567-0e02b2c3d479")

		rawParams := result["raw_params"].(map[string]any)
		getIDResults := result["getid_results"].(map[string]any)

		// Verify p: prefix keys exist
		assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000", rawParams["[gone-http]p:org_id"])
		assert.Equal(t, "6ba7b810-9dad-11d1-80b4-00c04fd430c8", rawParams["[gone-http]p:album_id"])
		assert.Equal(t, "f47ac10b-58cc-4372-a567-0e02b2c3d479", rawParams["[gone-http]p:song_id"])

		// Verify GetID retrieves correctly
		assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000", getIDResults["org_id"])
		assert.Equal(t, "6ba7b810-9dad-11d1-80b4-00c04fd430c8", getIDResults["album_id"])
		assert.Equal(t, "f47ac10b-58cc-4372-a567-0e02b2c3d479", getIDResults["song_id"])
	})

	time.Sleep(100 * time.Millisecond)
}
