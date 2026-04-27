package goai

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yetiz-org/gone/erresponse"
)

func TestApplyGoaiTagParsesExampleBySchemaType(t *testing.T) {
	tests := []struct {
		name     string
		schema   *Schema
		tag      string
		expected any
	}{
		{
			name:     "string example remains string",
			schema:   &Schema{Type: "string"},
			tag:      "example=100",
			expected: "100",
		},
		{
			name:     "integer example becomes number",
			schema:   &Schema{Type: "integer", Format: "int64"},
			tag:      "example=1704240000",
			expected: int64(1704240000),
		},
		{
			name:     "number example becomes float",
			schema:   &Schema{Type: "number", Format: "double"},
			tag:      "example=12.34",
			expected: 12.34,
		},
		{
			name:     "boolean example becomes bool",
			schema:   &Schema{Type: "boolean"},
			tag:      "example=true",
			expected: true,
		},
		{
			name: "object example becomes object",
			schema: &Schema{
				Type: "object",
				Properties: map[string]*Schema{
					"name": {Type: "string"},
				},
			},
			tag: "example={\"name\":\"Alice\"}",
			expected: map[string]any{
				"name": "Alice",
			},
		},
		{
			name: "array example becomes array",
			schema: &Schema{
				Type:  "array",
				Items: &Schema{Type: "string"},
			},
			tag:      "example=[\"TWN\",\"USA\"]",
			expected: []any{"TWN", "USA"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			applyGoaiTag(tt.schema, tt.tag)

			assert.Equal(t, tt.expected, tt.schema.Example)
		})
	}
}

func TestSchemaBuilderUsesDefaultErrorResponseSchemaMetadata(t *testing.T) {
	components := NewComponents()
	builder := newSchemaBuilder(components)

	schema := builder.build(reflect.TypeOf(erresponse.DefaultErrorResponse{}))

	require.NotNil(t, schema)
	assert.Equal(t, "#/components/schemas/erresponse.DefaultErrorResponse", schema.Ref)

	errorSchema := components.Schemas["erresponse.DefaultErrorResponse"]
	require.NotNil(t, errorSchema)

	statusCode := errorSchema.Properties["status_code"]
	require.NotNil(t, statusCode)
	assert.Equal(t, "HTTP status code", statusCode.Description)
	assert.Equal(t, int64(401), statusCode.Example)

	errorName := errorSchema.Properties["error"]
	require.NotNil(t, errorName)
	assert.Equal(t, "Error code", errorName.Description)
	assert.Equal(t, "invalid_token", errorName.Example)

	errorDescription := errorSchema.Properties["error_description"]
	require.NotNil(t, errorDescription)
	assert.Equal(t, "Error description", errorDescription.Description)
	assert.Equal(t, "insufficient authentication", errorDescription.Example)

	data := errorSchema.Properties["data"]
	require.NotNil(t, data)
	assert.Equal(t, "Additional error data", data.Description)
}
