package erresponse

import (
	"encoding/json"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestClonePreservesMapShape pins the nil-preserving contract Clone relies on.
// A nil map must stay nil so it keeps costing no allocation and keeps being
// omitted by json omitempty; an empty non-nil map must stay empty non-nil.
func TestClonePreservesMapShape(t *testing.T) {
	cases := map[string]*DefaultErrorResponse{
		"both_nil":           {StatusCode: 404, Name: "not_found"},
		"both_empty_non_nil": {StatusCode: 400, Name: "bad", Data: map[string]any{}, I18nParams: map[string]string{}},
		"data_only":          {StatusCode: 500, Name: "internal", Data: map[string]any{"trace": "abc"}},
		"i18n_only":          {StatusCode: 401, Name: "unauthorized", I18nParams: map[string]string{"scope": "read"}},
	}

	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			got := src.Clone().(*DefaultErrorResponse)

			assert.Equal(t, src.Data == nil, got.Data == nil, "Data nil-ness must be preserved")
			assert.Equal(t, src.I18nParams == nil, got.I18nParams == nil, "I18nParams nil-ness must be preserved")
			assert.Equal(t, src.Data, got.Data)
			assert.Equal(t, src.I18nParams, got.I18nParams)

			srcJSON, err := json.Marshal(src)
			require.NoError(t, err)
			gotJSON, err := json.Marshal(got)
			require.NoError(t, err)
			assert.JSONEq(t, string(srcJSON), string(gotJSON), "clone must serialize identically")
		})
	}
}

// TestCloneIsolatesFromRegisteredSingleton reproduces the production path:
// registered responses are package-level singletons and ghttp writes per-request
// cid/tid into ErrorData() right after cloning. Sharing the map would let one
// request observe another's identifiers.
func TestCloneIsolatesFromRegisteredSingleton(t *testing.T) {
	singleton := &DefaultErrorResponse{
		StatusCode: 400,
		Name:       "invalid_request",
		Data:       map[string]any{"seed": "shared"},
		I18nParams: map[string]string{"field": "name"},
	}

	a := singleton.Clone()
	b := singleton.Clone()

	a.ErrorData()["cid"] = "channel-A"
	a.(*DefaultErrorResponse).I18nParams["field"] = "mutated"

	_, leakedToB := b.ErrorData()["cid"]
	assert.False(t, leakedToB, "clone B must not observe clone A's cid")
	_, leakedToSingleton := singleton.Data["cid"]
	assert.False(t, leakedToSingleton, "the singleton must not observe a clone's cid")
	assert.Equal(t, "name", singleton.I18nParams["field"], "the singleton's I18nParams must be untouched")
	assert.Equal(t, "shared", b.ErrorData()["seed"], "clone B must still carry the seeded data")
}

// TestCloneLazyInitStaysOnClone covers Clone interacting with the lazy map init
// inside ErrorData(), which every caller triggers immediately after cloning.
func TestCloneLazyInitStaysOnClone(t *testing.T) {
	singleton := &DefaultErrorResponse{StatusCode: 404, Name: "not_found"}
	require.Nil(t, singleton.Data)

	c := singleton.Clone()
	c.ErrorData()["cid"] = "channel"

	assert.Nil(t, singleton.Data, "lazy init on a clone must not populate the singleton")
	assert.Equal(t, "channel", c.ErrorData()["cid"])
}

// TestConcurrentCloneAndMutate models concurrent requests each producing an
// error response from the same singleton. Meaningful under -race.
func TestConcurrentCloneAndMutate(t *testing.T) {
	singleton := &DefaultErrorResponse{
		StatusCode: 400,
		Name:       "invalid_request",
		Data:       map[string]any{"seed": "shared"},
		I18nParams: map[string]string{"field": "name"},
	}

	var wg sync.WaitGroup
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			c := singleton.Clone()
			c.ErrorData()["cid"] = id
			c.ErrorData()["tid"] = id
			_ = c.Error()
		}(i)
	}
	wg.Wait()

	assert.Len(t, singleton.Data, 1, "the singleton must still hold only its seeded key")
}
