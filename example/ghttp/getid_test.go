package example

import (
	"encoding/json"
	"fmt"
	"io"
	http2 "net/http"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		"org_id":   h.GetID("org_id", params),
		"album_id": h.GetID("album_id", params),
		"song_id":  h.GetID("song_id", params),
	}

	// Return both for verification
	result := map[string]any{
		"raw_params":    rawParams,
		"getid_results": getIDResults,
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

func init() {
	kklogger.SetLogLevel("WARN")
}

func startGetIDTestServer(t *testing.T) (baseURL string, client *http2.Client) {
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

	_, baseURL, client = startExampleHTTPServer(t, bootstrap)
	return baseURL, client
}

func getJSON(t *testing.T, client *http2.Client, url string) (result map[string]any, rtErr error) {
	req, err := http2.NewRequestWithContext(t.Context(), "GET", url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected HTTP status: %d", resp.StatusCode)
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// TestGetID_BraceSyntax_Integration tests brace syntax {param} with real HTTP server
func TestGetID_BraceSyntax_Integration(t *testing.T) {
	baseURL, client := startGetIDTestServer(t)

	t.Run("BraceSyntax_AllParams", func(t *testing.T) {
		result, err := getJSON(t, client, baseURL+"/api/v1/organizations/org-123/albums/album-456/songs/song-789")
		require.NoError(t, err)

		assert.Equal(t, "org-123", result["org_id"])
		assert.Equal(t, "album-456", result["album_id"])
		assert.Equal(t, "song-789", result["song_id"])
	})

	t.Run("BraceSyntax_UUIDValues", func(t *testing.T) {
		result, err := getJSON(t, client, baseURL+"/api/v1/organizations/550e8400-e29b-41d4-a716-446655440000/albums/6ba7b810-9dad-11d1-80b4-00c04fd430c8/songs/f47ac10b-58cc-4372-a567-0e02b2c3d479")
		require.NoError(t, err)

		assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000", result["org_id"])
		assert.Equal(t, "6ba7b810-9dad-11d1-80b4-00c04fd430c8", result["album_id"])
		assert.Equal(t, "f47ac10b-58cc-4372-a567-0e02b2c3d479", result["song_id"])
	})

	t.Run("BraceSyntax_NumericValues", func(t *testing.T) {
		result, err := getJSON(t, client, baseURL+"/api/v1/organizations/12345/albums/67890/songs/11111")
		require.NoError(t, err)

		assert.Equal(t, "12345", result["org_id"])
		assert.Equal(t, "67890", result["album_id"])
		assert.Equal(t, "11111", result["song_id"])
	})

}

// TestGetID_ColonSyntax_Integration tests colon syntax :param with real HTTP server
func TestGetID_ColonSyntax_Integration(t *testing.T) {
	baseURL, client := startGetIDTestServer(t)

	t.Run("ColonSyntax_AllParams", func(t *testing.T) {
		result, err := getJSON(t, client, baseURL+"/api/v1/users/user-abc/posts/post-xyz")
		require.NoError(t, err)

		assert.Equal(t, "user-abc", result["user_id"])
		assert.Equal(t, "post-xyz", result["post_id"])
	})

	t.Run("ColonSyntax_SpecialChars", func(t *testing.T) {
		result, err := getJSON(t, client, baseURL+"/api/v1/users/user_with_underscore/posts/post-with-dash")
		require.NoError(t, err)

		assert.Equal(t, "user_with_underscore", result["user_id"])
		assert.Equal(t, "post-with-dash", result["post_id"])
	})

}

// TestGetID_DefaultSyntax_Integration tests default node name syntax with real HTTP server
func TestGetID_DefaultSyntax_Integration(t *testing.T) {
	baseURL, client := startGetIDTestServer(t)

	t.Run("DefaultSyntax_ThreeLevels", func(t *testing.T) {
		result, err := getJSON(t, client, baseURL+"/api/v1/categories/cat-001/products/prod-002/reviews/rev-003")
		require.NoError(t, err)

		assert.Equal(t, "cat-001", result["categories"])
		assert.Equal(t, "prod-002", result["products"])
		assert.Equal(t, "rev-003", result["reviews"])
	})

	t.Run("DefaultSyntax_TwoLevels", func(t *testing.T) {
		result, err := getJSON(t, client, baseURL+"/api/v1/categories/cat-100/products/prod-200")
		require.NoError(t, err)

		assert.Equal(t, "cat-100", result["categories"])
		assert.Equal(t, "prod-200", result["products"])
	})

	t.Run("DefaultSyntax_OneLevel", func(t *testing.T) {
		result, err := getJSON(t, client, baseURL+"/api/v1/categories/cat-only")
		require.NoError(t, err)

		assert.Equal(t, "cat-only", result["categories"])
	})

}

// TestGetID_MixedSyntax_Integration tests mixed syntax with real HTTP server
func TestGetID_MixedSyntax_Integration(t *testing.T) {
	baseURL, client := startGetIDTestServer(t)

	t.Run("MixedSyntax_BraceColonDefault", func(t *testing.T) {
		result, err := getJSON(t, client, baseURL+"/api/v1/orgs/org-mixed/teams/team-mixed/members/member-mixed")
		require.NoError(t, err)

		assert.Equal(t, "org-mixed", result["org_id"])
		assert.Equal(t, "team-mixed", result["team_id"])
		assert.Equal(t, "member-mixed", result["members"])
	})

}

// TestGetID_DeepNested_Integration tests deeply nested routes with real HTTP server
func TestGetID_DeepNested_Integration(t *testing.T) {
	baseURL, client := startGetIDTestServer(t)

	t.Run("DeepNested_FourLevels", func(t *testing.T) {
		result, err := getJSON(t, client, baseURL+"/mgmt/v1/organizations/org-deep/projects/proj-deep/tasks/task-deep/comments/comment-deep")
		require.NoError(t, err)

		assert.Equal(t, "org-deep", result["org_id"])
		assert.Equal(t, "proj-deep", result["project_id"])
		assert.Equal(t, "task-deep", result["task_id"])
		assert.Equal(t, "comment-deep", result["comment_id"])
	})

	t.Run("DeepNested_ComplexValues", func(t *testing.T) {
		result, err := getJSON(t, client, baseURL+"/mgmt/v1/organizations/ORG_123_ABC/projects/proj-456-def/tasks/task_789_ghi/comments/COMMENT-000")
		require.NoError(t, err)

		assert.Equal(t, "ORG_123_ABC", result["org_id"])
		assert.Equal(t, "proj-456-def", result["project_id"])
		assert.Equal(t, "task_789_ghi", result["task_id"])
		assert.Equal(t, "COMMENT-000", result["comment_id"])
	})

}

// TestGetID_ParamsFormat_Integration verifies the actual params format stored
func TestGetID_ParamsFormat_Integration(t *testing.T) {
	baseURL, client := startGetIDTestServer(t)

	t.Run("BraceSyntax_ParamsFormat", func(t *testing.T) {
		result, err := getJSON(t, client, baseURL+"/debug/test-value")
		require.NoError(t, err)

		// Brace syntax should store as [gone-http]p:param
		assert.Equal(t, "test-value", result["[gone-http]p:debug_id"])
	})

	t.Run("ColonSyntax_ParamsFormat", func(t *testing.T) {
		result, err := getJSON(t, client, baseURL+"/debug2/test-value")
		require.NoError(t, err)

		// Colon syntax should store as [gone-http]param
		assert.Equal(t, "test-value", result["[gone-http]debug_id"])
	})

	t.Run("DefaultSyntax_ParamsFormat", func(t *testing.T) {
		result, err := getJSON(t, client, baseURL+"/debug3/test-value")
		require.NoError(t, err)

		// Default syntax should store as [gone-http]node_id
		assert.Equal(t, "test-value", result["[gone-http]debug3_id"])
	})

}

// TestGetID_EdgeCases_Integration tests edge cases with real HTTP server
func TestGetID_EdgeCases_Integration(t *testing.T) {
	baseURL, client := startGetIDTestServer(t)

	t.Run("LongIDValue", func(t *testing.T) {
		var longID strings.Builder
		for range 100 {
			longID.WriteString("abcdefghij")
		}
		result, err := getJSON(t, client, fmt.Sprintf(baseURL+"/debug/%s", longID.String()))
		require.NoError(t, err)
		assert.Equal(t, longID.String(), result["[gone-http]p:debug_id"])
	})

	t.Run("SpecialCharactersInID", func(t *testing.T) {
		result, err := getJSON(t, client, baseURL+"/debug/test_value-123.456")
		require.NoError(t, err)
		assert.Equal(t, "test_value-123.456", result["[gone-http]p:debug_id"])
	})

}

// TestGetID_ConcurrentRequests_Integration tests concurrent access
func TestGetID_ConcurrentRequests_Integration(t *testing.T) {
	baseURL, client := startGetIDTestServer(t)

	t.Run("ConcurrentBraceSyntax", func(t *testing.T) {
		type response struct {
			index  int
			result map[string]any
			err    error
		}
		responses := make(chan response, 20)
		var workers sync.WaitGroup
		for i := range 20 {
			workers.Go(func() {
				url := fmt.Sprintf(baseURL+"/api/v1/organizations/org-%d/albums/album-%d/songs/song-%d", i, i*2, i*3)
				result, err := getJSON(t, client, url)
				responses <- response{index: i, result: result, err: err}
			})
		}
		workers.Wait()
		close(responses)
		for response := range responses {
			if !assert.NoError(t, response.err) {
				continue
			}
			assert.Equal(t, fmt.Sprintf("org-%d", response.index), response.result["org_id"])
			assert.Equal(t, fmt.Sprintf("album-%d", response.index*2), response.result["album_id"])
			assert.Equal(t, fmt.Sprintf("song-%d", response.index*3), response.result["song_id"])
		}
	})

}

// TestGetID_ParameterFormats_Integration verifies supported parameter layouts.
func TestGetID_ParameterFormats_Integration(t *testing.T) {
	baseURL, client := startGetIDTestServer(t)

	t.Run("ColonSyntax", func(t *testing.T) {
		result, err := getJSON(t, client, baseURL+"/api/v1/users/colon-user/posts/colon-post")
		require.NoError(t, err)

		assert.Equal(t, "colon-user", result["user_id"])
		assert.Equal(t, "colon-post", result["post_id"])
	})

	t.Run("DefaultSyntax", func(t *testing.T) {
		result, err := getJSON(t, client, baseURL+"/api/v1/categories/default-cat/products/default-prod/reviews/default-rev")
		require.NoError(t, err)

		assert.Equal(t, "default-cat", result["categories"])
		assert.Equal(t, "default-prod", result["products"])
		assert.Equal(t, "default-rev", result["reviews"])
	})

}

// TestGetID_VerifyPrefixKeyExists_Integration verifies that brace syntax stores params with p: prefix
// AND that GetID correctly retrieves values from those p: prefixed keys
func TestGetID_VerifyPrefixKeyExists_Integration(t *testing.T) {
	baseURL, client := startGetIDTestServer(t)

	t.Run("BraceSyntax_PrefixKeyExists_And_GetID_Works", func(t *testing.T) {
		result, err := getJSON(t, client, baseURL+"/verify/orgs/org-123/albums/album-456/songs/song-789")
		require.NoError(t, err)

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
		result, err := getJSON(t, client, baseURL+"/verify/orgs/550e8400-e29b-41d4-a716-446655440000/albums/6ba7b810-9dad-11d1-80b4-00c04fd430c8/songs/f47ac10b-58cc-4372-a567-0e02b2c3d479")
		require.NoError(t, err)

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

}
