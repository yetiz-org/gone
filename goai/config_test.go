package goai

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigDocstringExtractionWiresExtractor(t *testing.T) {
	cfg := &Config{}

	assert.Nil(t, cfg.ToRunOptions().OperationDocExtractor)
	assert.Nil(t, cfg.ToBuildOptions().OperationDocExtractor)

	cfg.EnableDocstringExtraction = true

	assert.NotNil(t, cfg.ToRunOptions().OperationDocExtractor)
	assert.NotNil(t, cfg.ToBuildOptions().OperationDocExtractor)
}

func TestConfigDocstringBuildTagsCombineWithGOFLAGS(t *testing.T) {
	t.Setenv("GOFLAGS", "-tags=from_env")

	cfg := &Config{
		EnableDocstringExtraction: true,
		DocstringBuildTags:        []string{"from_config"},
	}

	assert.Equal(t, []string{"from_env", "from_config"}, _MergeBuildTags(_BuildTagsFromGOFLAGS(), cfg.DocstringBuildTags))
	assert.NotNil(t, cfg.ToRunOptions().OperationDocExtractor)
	assert.NotNil(t, cfg.ToBuildOptions().OperationDocExtractor)
}

func TestConfigSuppressEmptySchemasWiresBuildOption(t *testing.T) {
	cfg := &Config{}

	assert.False(t, cfg.ToBuildOptions().SuppressEmptySchemas)
	assert.False(t, cfg.ToRunOptions().SuppressEmptySchemas)

	cfg.SuppressEmptySchemas = true

	assert.True(t, cfg.ToBuildOptions().SuppressEmptySchemas)
	assert.True(t, cfg.ToRunOptions().SuppressEmptySchemas)
}

func TestConfigConcurrencyDefaultsAndValidation(t *testing.T) {
	assert.Equal(t, 1, DefaultConfig().Concurrency)
	cfg, err := LoadConfig(t.TempDir())
	require.NoError(t, err)
	assert.Equal(t, 1, cfg.Concurrency)
	dir := t.TempDir()
	_WriteGoaiYAML(t, dir, "title: API\n")
	cfg, err = LoadConfig(dir)
	require.NoError(t, err)
	assert.Equal(t, 1, cfg.Concurrency)
	_WriteGoaiYAML(t, dir, "concurrency: 0\n")
	cfg, err = LoadConfig(dir)
	require.NoError(t, err)
	assert.Equal(t, 1, cfg.Concurrency)
	_WriteGoaiYAML(t, dir, "concurrency: 4\n")
	cfg, err = LoadConfig(dir)
	require.NoError(t, err)
	assert.Equal(t, 4, cfg.Concurrency)
	_WriteGoaiYAML(t, dir, "concurrency: -1\n")
	_, err = LoadConfig(dir)
	assert.Error(t, err)
}

func _WriteGoaiYAML(t *testing.T, dir string, contents string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "goai.yaml"), []byte(contents), 0o644))
}
