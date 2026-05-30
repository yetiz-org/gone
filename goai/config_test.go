package goai

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
