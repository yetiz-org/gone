package goai

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yetiz-org/gone/erresponse"
)

type _TextMarshalerSchemaID struct {
	value uint64
}

func (_TextMarshalerSchemaID) MarshalText() ([]byte, error) {
	return []byte("txt_123"), nil
}

type _OptionalSchemaValue[T any] struct {
	Set   bool
	Value *T
}

func (o *_OptionalSchemaValue[T]) UnmarshalJSON(data []byte) error {
	o.Set = true
	if string(data) == "null" {
		o.Value = nil
		return nil
	}

	var value T
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}

	o.Value = &value
	return nil
}

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

func TestApplyGoaiTagWrapsRefBeforeAddingSiblingFields(t *testing.T) {
	schema := &Schema{Ref: "#/components/schemas/Referenced"}

	applyGoaiTag(schema, "description=Referenced field;example=abc")

	assert.Empty(t, schema.Ref)
	require.Len(t, schema.AllOf, 1)
	assert.Equal(t, "#/components/schemas/Referenced", schema.AllOf[0].Ref)
	assert.Equal(t, "Referenced field", schema.Description)
	assert.Equal(t, "abc", schema.Example)
}

func TestApplyGoaiTagDoesNotWrapRefForNoOpOptions(t *testing.T) {
	schema := &Schema{Ref: "#/components/schemas/Referenced"}

	applyGoaiTag(schema, "nullable=false;deprecated=false;minLength=not-a-number")

	assert.Equal(t, "#/components/schemas/Referenced", schema.Ref)
	assert.Empty(t, schema.AllOf)
	assert.False(t, schema.Nullable)
	assert.False(t, schema.Deprecated)
	assert.Nil(t, schema.MinLength)
}

func TestSchemaBuilderUsesStringSchemaForEmbeddedTextMarshalerStruct(t *testing.T) {
	type TimeOfDay struct {
		time.Time
	}

	components := NewComponents()
	builder := newSchemaBuilder(components)

	ref := builder.build(reflect.TypeOf(TimeOfDay{}))

	require.NotNil(t, ref)
	assert.Equal(t, "#/components/schemas/goai.TimeOfDay", ref.Ref)
	schema := components.Schemas["goai.TimeOfDay"]
	require.NotNil(t, schema)
	assert.Equal(t, "string", schema.Type)
	assert.Empty(t, schema.Format)
}

func TestSchemaBuilderUsesStringSchemaForTextMarshalerStruct(t *testing.T) {
	components := NewComponents()
	builder := newSchemaBuilder(components)

	ref := builder.build(reflect.TypeOf(_TextMarshalerSchemaID{}))

	require.NotNil(t, ref)
	assert.Equal(t, "#/components/schemas/goai._TextMarshalerSchemaID", ref.Ref)
	schema := components.Schemas["goai._TextMarshalerSchemaID"]
	require.NotNil(t, schema)
	assert.Equal(t, "string", schema.Type)
}

func TestSchemaBuilderUsesValueSchemaForJSONValueWrapper(t *testing.T) {
	components := NewComponents()
	builder := newSchemaBuilder(components)

	schema := builder.build(reflect.TypeOf(_OptionalSchemaValue[string]{}))

	require.NotNil(t, schema)
	assert.Equal(t, "string", schema.Type)
	assert.True(t, schema.Nullable)
	assert.Empty(t, components.Schemas)
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
