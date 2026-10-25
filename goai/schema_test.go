package goai

import (
	"encoding/json"
	"reflect"
	"strings"
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

func TestSchemaBuilderSkipsGoaiDashField(t *testing.T) {
	type GoaiSkipInner struct {
		Time  time.Time `json:"time"`
		Valid bool      `json:"valid"`
	}
	type GoaiSkipEmbedded struct {
		Secret string `json:"secret"`
	}
	type GoaiSkipOuter struct {
		ID               int           `json:"id"`
		Skipped          GoaiSkipInner `json:"skipped" goai:"-"`
		GoaiSkipEmbedded `goai:"-"`
	}

	components := NewComponents()
	builder := newSchemaBuilder(components)

	ref := builder.build(reflect.TypeOf(GoaiSkipOuter{}))
	require.NotNil(t, ref)
	require.Equal(t, "#/components/schemas/goai.GoaiSkipOuter", ref.Ref)

	schema := components.Schemas["goai.GoaiSkipOuter"]
	require.NotNil(t, schema)
	assert.Contains(t, schema.Properties, "id")
	assert.NotContains(t, schema.Properties, "skipped")
	assert.NotContains(t, schema.Required, "skipped")

	// A `goai:"-"` embedded field is skipped before flattening, so its
	// promoted fields never appear.
	assert.NotContains(t, schema.Properties, "secret")

	// Skipping happens before build(), so the field's type is never
	// registered as a component when nothing else references it.
	assert.NotContains(t, components.Schemas, "goai.GoaiSkipInner")
	assert.NotContains(t, components.Schemas, "goai.GoaiSkipEmbedded")
}

func TestSchemaBuildersSkipJSONDashEmbeddedField(t *testing.T) {
	type Secret struct {
		Token string `json:"token" goai:"description=Private token"`
	}
	type Visible struct {
		Public string `json:"public"`
	}
	type Payload struct {
		Secret `json:"-"`
		Visible
		ID string `json:"id"`
	}

	for _, kind := range []string{"reflection", "AST"} {
		t.Run(kind, func(t *testing.T) {
			var schemas map[string]*Schema
			var ref *Schema
			if kind == "reflection" {
				components := NewComponents()
				ref = newSchemaBuilder(components).build(reflect.TypeFor[Payload]())
				schemas = components.Schemas
			} else {
				builder := _ASTSchemaBuilderFromSource(t, "example.test/p", "package p\n"+
					"type Secret struct { Token string `json:\"token\" goai:\"description=Private token\"` }\n"+
					"type Visible struct { Public string `json:\"public\"` }\n"+
					"type Payload struct { Secret `json:\"-\"`; Visible; ID string `json:\"id\"` }\n")
				var ok bool
				ref, ok = builder._SchemaForTypeName("Payload")
				require.True(t, ok)
				schemas = builder._Components
			}

			require.NotNil(t, ref)
			payload := schemas[strings.TrimPrefix(ref.Ref, "#/components/schemas/")]
			require.NotNil(t, payload)
			assert.ElementsMatch(t, []string{"id", "public"}, payload.Required)
			require.Len(t, payload.Properties, 2)
			assert.Contains(t, payload.Properties, "id")
			assert.Contains(t, payload.Properties, "public")
			for name := range schemas {
				assert.NotEqual(t, "Secret", name[strings.LastIndex(name, ".")+1:])
			}
		})
	}
}

func TestSchemaBuildersUnsignedConstraintBounds(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value string
		want  *uint64
	}{
		{name: "absent", value: "", want: nil},
		{name: "zero", value: "0", want: new(uint64(0))},
		{name: "maximum", value: "18446744073709551615", want: new(^uint64(0))},
		{name: "overflow", value: "18446744073709551616", want: nil},
		{name: "long overflow", value: "184467440737095516160000000000000000000000", want: nil},
		{name: "syntax", value: "1x", want: nil},
		{name: "sign", value: "+1", want: nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lengthTag := `json:"text" goai:"minLength=` + tc.value + `;maxLength=` + tc.value + `"`
			itemsTag := `json:"items" goai:"minItems=` + tc.value + `;maxItems=` + tc.value + `"`
			propertiesTag := `json:"object" goai:"minProperties=` + tc.value + `;maxProperties=` + tc.value + `"`
			for _, kind := range []string{"reflection", "AST"} {
				t.Run(kind, func(t *testing.T) {
					var payload *Schema
					if kind == "reflection" {
						typ := reflect.StructOf([]reflect.StructField{
							{Name: "Text", Type: reflect.TypeFor[string](), Tag: reflect.StructTag(lengthTag)},
							{Name: "Items", Type: reflect.TypeFor[[]string](), Tag: reflect.StructTag(itemsTag)},
							{Name: "Object", Type: reflect.TypeFor[map[string]string](), Tag: reflect.StructTag(propertiesTag)},
						})
						payload = newSchemaBuilder(NewComponents()).build(typ)
					} else {
						builder := _ASTSchemaBuilderFromSource(t, "example.test/p", "package p\ntype Payload struct { Text string `"+lengthTag+"`; Items []string `"+itemsTag+"`; Object map[string]string `"+propertiesTag+"` }")
						ref, ok := builder._SchemaForTypeName("Payload")
						require.True(t, ok)
						payload = builder._Components[strings.TrimPrefix(ref.Ref, "#/components/schemas/")]
					}

					require.NotNil(t, payload)
					text := payload.Properties["text"]
					items := payload.Properties["items"]
					object := payload.Properties["object"]
					require.NotNil(t, text)
					require.NotNil(t, items)
					require.NotNil(t, object)
					for _, got := range []*uint64{text.MinLength, text.MaxLength, items.MinItems, items.MaxItems, object.MinProperties, object.MaxProperties} {
						assert.Equal(t, tc.want, got)
					}
				})
			}
		})
	}
}
