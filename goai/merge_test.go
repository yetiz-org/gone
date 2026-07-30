package goai

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestMerge3WayAppliesOverridesAsFinalOverlay(t *testing.T) {
	generated := []byte(`
openapi: 3.0.3
info:
  title: Generated
  version: 1.0.0
paths:
  /ping:
    get:
      summary: Generated summary
      responses:
        "200":
          description: ok
`)
	existing := []byte(`
openapi: 3.0.3
info:
  title: Existing
  version: 1.0.0
paths:
  /ping:
    get:
      summary: Existing summary
      responses:
        "200":
          description: ok
`)
	overrides := []byte(`
info:
  title: Override
paths:
  /ping:
    get:
      summary: Override summary
`)

	got, err := Merge3Way(generated, existing, overrides)
	require.NoError(t, err)

	var doc map[string]any
	require.NoError(t, yaml.Unmarshal(got, &doc))
	info := doc["info"].(map[string]any)
	require.Equal(t, "Override", info["title"])
	require.Equal(t, "1.0.0", info["version"])
	operation := doc["paths"].(map[string]any)["/ping"].(map[string]any)["get"].(map[string]any)
	require.Equal(t, "Override summary", operation["summary"])
	require.Contains(t, operation, "responses")
}

func TestMerge3WayInvalidOverridesReturnError(t *testing.T) {
	_, err := Merge3Way(
		[]byte("openapi: 3.0.3\ninfo:\n  title: Generated\n"),
		nil,
		[]byte("info: ["),
	)

	require.Error(t, err)
}
