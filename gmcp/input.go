package gmcp

import (
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
)

// ID is an identifier a tool forwards in a URL path or query; its schema enforces Options.IDPattern.
type ID string

// Date is a calendar date in YYYY-MM-DD form.
type Date string

// _TagKeys are the keys an mcp tag accepts. path, query, and body place the field in the REST request; the others
// follow the goai tag schema keys, and keys prefixed with "items." apply to array elements.
var _TagKeys = []string{
	"path", "query", "body", "description", "enum", "minLength", "minItems", "maxItems", "uniqueItems",
	"items.description", "items.enum", "items.minLength",
}

// _Tag is a parsed mcp struct tag. Like a goai tag it is a ";"-separated key=value list, so values cannot contain ";".
type _Tag map[string]string

// _Field places one input field in the REST request. _Kind is path, query, or body; a path _Name is an ancestor node,
// or empty for the endpoint's own ID, and a query _Name is the query parameter.
type _Field struct {
	_Index []int
	_Kind  string
	_Name  string
}

// _Input is the input schema and field placement Bind derives from an input type.
type _Input struct {
	_Schema *jsonschema.Schema
	_Fields []_Field
}

// _NewTag parses the mcp tag of field and panics on an unknown key or a valued uniqueItems.
func _NewTag(owner reflect.Type, field reflect.StructField) (tag _Tag) {
	tag = _Tag{}
	raw, found := field.Tag.Lookup("mcp")
	if !found {
		return tag
	}

	for segment := range strings.SplitSeq(raw, ";") {
		key, value, _ := strings.Cut(strings.TrimSpace(segment), "=")
		if key == "" {
			continue
		}

		if !slices.Contains(_TagKeys, key) || key == "uniqueItems" && value != "" {
			panic(fmt.Sprintf("gmcp: %s.%s has invalid mcp tag key %q", owner, field.Name, key))
		}

		tag[key] = value
	}

	return tag
}

// _NewInput derives the input schema and field placement of t. jsonschema-go infers the structure, the mcp tags add
// schema keys, a field is optional when its json tag has omitempty or omitzero or it is a pointer, and null is removed
// from optional types. Every top-level field must be placed in the path, query, or body with a forwardable type.
func (s *Server) _NewInput(t reflect.Type) (input *_Input) {
	schema, err := jsonschema.ForType(t, &jsonschema.ForOptions{TypeSchemas: s._TypeSchemas})
	if err != nil {
		panic(fmt.Sprintf("gmcp: input %s: %v", t, err))
	}

	_Describe(t, schema, false)
	input = &_Input{_Schema: schema}
	for _, field := range reflect.VisibleFields(t) {
		name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
		if field.Anonymous || !field.IsExported() || name == "-" {
			continue
		}

		tag := _NewTag(t, field)
		placed := _Field{_Index: field.Index}
		switch {
		case tag._Has("body") && field.Type.Kind() == reflect.Struct && !slices.ContainsFunc(input._Fields, func(existing _Field) bool { return existing._Kind == "body" }):
			placed._Kind = "body"

		case tag._Has("path") && field.Type.Kind() == reflect.String && !_Optional(field):
			placed._Kind, placed._Name = "path", tag["path"]

		case tag._Has("query") && tag["query"] != "" && _QueryType(field.Type):
			placed._Kind, placed._Name = "query", tag["query"]

		default:
			panic(fmt.Sprintf("gmcp: %s.%s needs exactly one valid path, query, or body placement", t, field.Name))
		}

		input._Fields = append(input._Fields, placed)
	}

	return input
}

// _Describe applies the mcp tag schema keys of t to its object schema, recomputes required, and removes null. nested
// marks fields inside a body, which cannot be placed in the request themselves.
func _Describe(t reflect.Type, schema *jsonschema.Schema, nested bool) {
	schema.Required = nil
	for _, field := range reflect.VisibleFields(t) {
		name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
		property := schema.Properties[name]
		if field.Anonymous || !field.IsExported() || property == nil {
			continue
		}

		tag := _NewTag(t, field)
		if nested && (tag._Has("path") || tag._Has("query") || tag._Has("body")) {
			panic(fmt.Sprintf("gmcp: %s.%s is inside a body and cannot be placed in the request", t, field.Name))
		}

		tag._Apply(t, field, property)
		if !_Optional(field) {
			schema.Required = append(schema.Required, name)
		}

		if field.Type.Kind() == reflect.Struct {
			_Describe(field.Type, property, true)
		}
	}
}

// _Optional reports whether field is optional: its json tag has omitempty or omitzero, or it is a pointer.
func _Optional(field reflect.StructField) (optional bool) {
	_, options, _ := strings.Cut(field.Tag.Get("json"), ",")
	return field.Type.Kind() == reflect.Pointer || slices.ContainsFunc(strings.Split(options, ","), func(option string) bool {
		return option == "omitempty" || option == "omitzero"
	})
}

// _QueryType reports whether t forwards as one query value: a string or integer, a pointer to one, or a string slice
// joined with commas.
func _QueryType(t reflect.Type) (ok bool) {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	switch t.Kind() {
	case reflect.String, reflect.Int, reflect.Int64:
		return true

	case reflect.Slice:
		return t.Elem().Kind() == reflect.String

	default:
		return false
	}
}

// _Has reports whether the tag declares key.
func (t _Tag) _Has(key string) (found bool) {
	_, found = t[key]
	return found
}

// _Apply sets the schema keys on property, or on its items for "items." keys. It panics on an unparsable number or an
// "items." key on a field that is not an array.
func (t _Tag) _Apply(owner reflect.Type, field reflect.StructField, property *jsonschema.Schema) {
	_Strict(property)
	if property.Items != nil {
		_Strict(property.Items)
	}

	for key, value := range t {
		target := property
		if name, found := strings.CutPrefix(key, "items."); found {
			if property.Items == nil {
				panic(fmt.Sprintf("gmcp: %s.%s uses %q but is not an array", owner, field.Name, key))
			}

			key, target = name, property.Items
		}

		switch key {
		case "description":
			target.Description = value

		case "enum":
			for item := range strings.SplitSeq(value, ",") {
				target.Enum = append(target.Enum, item)
			}

		case "minLength":
			target.MinLength = new(_TagInt(owner, field, value))

		case "minItems":
			target.MinItems = new(_TagInt(owner, field, value))

		case "maxItems":
			target.MaxItems = new(_TagInt(owner, field, value))

		case "uniqueItems":
			target.UniqueItems = true
		}
	}
}

// _TagInt parses a numeric schema key and panics unless it is a non-negative integer.
func _TagInt(owner reflect.Type, field reflect.StructField, value string) (number int) {
	number, err := strconv.Atoi(value)
	if err != nil || number < 0 {
		panic(fmt.Sprintf("gmcp: %s.%s has invalid mcp tag number %q", owner, field.Name, value))
	}

	return number
}

// _Strict turns the ["null", X] type jsonschema-go infers for pointers and slices back into X, so omission rather
// than null expresses an absent value.
func _Strict(schema *jsonschema.Schema) {
	if len(schema.Types) == 2 && schema.Types[0] == "null" {
		schema.Type, schema.Types = schema.Types[1], nil
	}
}

// _Check panics when the placement does not fit the bound endpoint: an ancestor path ID must name an ancestor
// segment, an MCPIndex tool cannot carry the endpoint's own ID, and a GET tool cannot carry a body.
func (i *_Input) _Check(name string, path string, method string, index bool) {
	segments := strings.Split(strings.TrimPrefix(path, "/"), "/")
	for _, field := range i._Fields {
		switch {
		case field._Kind == "path" && field._Name != "" && !slices.Contains(segments[:len(segments)-1], field._Name):
			panic(fmt.Sprintf("gmcp: tool %q path ID %q is not an ancestor node of %s", name, field._Name, path))

		case field._Kind == "path" && field._Name == "" && index:
			panic(fmt.Sprintf("gmcp: tool %q is declared by MCPIndex and cannot carry the ID of %s", name, path))

		case field._Kind == "body" && method == http.MethodGet:
			panic(fmt.Sprintf("gmcp: tool %q forwards GET and cannot carry a body", name))
		}
	}
}

// _Binding places a decoded input in the REST request. Nil pointers, zero values, and empty slices are not sent as
// query values; ancestor IDs are always sent for _Target to validate. An empty endpoint ID is invalid because the
// request would fall back to Index. An input implementing BindingAdjuster then adds its fixed values.
func (i *_Input) _Binding(value any) (binding Binding, ok bool) {
	input := reflect.ValueOf(value)
	binding = Binding{IDs: map[string]string{}, Query: url.Values{}}
	for _, field := range i._Fields {
		fieldValue := input.FieldByIndex(field._Index)
		switch field._Kind {
		case "body":
			binding.Body = fieldValue.Interface()

		case "path":
			if field._Name == "" {
				if binding.ID = fieldValue.String(); binding.ID == "" {
					return binding, false
				}
			} else {
				binding.IDs[field._Name] = fieldValue.String()
			}

		case "query":
			if text, ok := _QueryText(fieldValue); ok {
				binding.Query.Set(field._Name, text)
			}
		}
	}

	if adjuster, found := value.(BindingAdjuster); found {
		binding = adjuster.AdjustBinding(binding)
	}

	return binding, true
}

// _QueryText formats a query field: pointers are dereferenced and slices are joined with commas. Nil pointers, zero
// values, and empty slices report false.
func _QueryText(value reflect.Value) (text string, ok bool) {
	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return "", false
		}

		value = value.Elem()
	} else if value.IsZero() {
		return "", false
	}

	switch value.Kind() {
	case reflect.Slice:
		items := make([]string, 0, value.Len())
		for index := range value.Len() {
			items = append(items, value.Index(index).String())
		}

		return strings.Join(items, ","), len(items) > 0

	case reflect.Int, reflect.Int64:
		return strconv.FormatInt(value.Int(), 10), true

	default:
		return value.String(), true
	}
}
