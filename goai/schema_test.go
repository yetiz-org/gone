package goai

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
