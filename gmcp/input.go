package gmcp

import (
	"cmp"
	"encoding/json/v2"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
)

// ID is an identifier a tool forwards in a URL path or query; its schema enforces Options.IDPattern.
type ID string

// Date is a calendar date in YYYY-MM-DD form.
type Date string

// _InputKeys and _OutputKeys are the keys a gmcp tag accepts on input and output types. path, query, and body place
// an input field in the REST request; the others follow the goai tag schema keys, and element keys also apply to
// array elements with the "items." prefix. Output keys only describe: the SDK validates every result against the
// output schema, so a validation key would turn a valid REST response into a tool error. default is never accepted
// because the SDK writes schema defaults into arguments and results.
var (
	_InputKeys = _TagKeys([]string{"path", "query", "body", "deprecated", "minItems", "maxItems", "uniqueItems"},
		[]string{"description", "example", "format", "enum", "minLength", "maxLength", "pattern", "minimum", "maximum"})
	_OutputKeys = _TagKeys([]string{"deprecated"}, []string{"description", "example"})
)

// _FlagKeys are the keys written without a value.
var _FlagKeys = []string{"deprecated", "uniqueItems"}

// _TagKeys lists the field keys and the element keys, plus each element key with the "items." prefix.
func _TagKeys(field []string, element []string) (keys []string) {
	keys = slices.Concat(field, element)
	for _, key := range element {
		keys = append(keys, "items."+key)
	}

	return keys
}

// _Tag is a parsed gmcp struct tag. Like a goai tag it is a ";"-separated key=value list, so values cannot contain ";".
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

// _NewTag parses the gmcp tag of field and panics on a key outside keys or a flag key with a value.
func _NewTag(owner reflect.Type, field reflect.StructField, keys []string) (tag _Tag) {
	tag = _Tag{}
	raw, found := field.Tag.Lookup("gmcp")
	if !found {
		return tag
	}

	for segment := range strings.SplitSeq(raw, ";") {
		key, value, _ := strings.Cut(strings.TrimSpace(segment), "=")
		if key == "" {
			continue
		}

		if !slices.Contains(keys, key) || slices.Contains(_FlagKeys, key) && value != "" {
			panic(fmt.Sprintf("gmcp: %s.%s has invalid gmcp tag key %q", owner, field.Name, key))
		}

		tag[key] = value
	}

	return tag
}

// _NewInput derives the input schema and field placement of t. jsonschema-go infers the structure and the gmcp tags
// add schema keys (see _Annotate). Every top-level field must be placed in the path, query, or body with a
// forwardable type.
func (s *Server) _NewInput(t reflect.Type) (input *_Input) {
	schema, err := jsonschema.ForType(t, &jsonschema.ForOptions{TypeSchemas: s._TypeSchemas})
	if err != nil {
		panic(fmt.Sprintf("gmcp: input %s: %v", t, err))
	}

	_Annotate(t, schema, false, true)
	input = &_Input{_Schema: schema}
	for _, field := range reflect.VisibleFields(t) {
		name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
		if field.Anonymous || !field.IsExported() || name == "-" {
			continue
		}

		tag := _NewTag(t, field, _InputKeys)
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

// _NewOutput derives the output schema of t the way the SDK infers it and adds the gmcp tag descriptions (see
// _Annotate). ID and Date stay plain strings, so a REST response is never checked against Options.IDPattern. Like
// the SDK, it derives no schema for any and follows one pointer.
func _NewOutput(t reflect.Type) (schema any) {
	if t == reflect.TypeFor[any]() {
		return nil
	}

	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	output, err := jsonschema.ForType(t, &jsonschema.ForOptions{})
	if err != nil {
		panic(fmt.Sprintf("gmcp: output %s: %v", t, err))
	}

	_Annotate(t, output, true, false)
	return output
}

// _Annotate applies the gmcp tags of the struct fields reachable from t, through pointers, slices, arrays, and map
// values, to schema, the schema jsonschema-go inferred for t. Input annotation also removes null and makes a field
// required unless it is optional; output annotation keeps the inferred null and required list so every REST response
// still validates. Only top-level input fields may carry a placement.
func _Annotate(t reflect.Type, schema *jsonschema.Schema, output bool, top bool) {
	if !output {
		_Strict(schema)
	}

	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	switch t.Kind() {
	case reflect.Slice, reflect.Array:
		if schema.Items != nil {
			_Annotate(t.Elem(), schema.Items, output, false)
		}

	case reflect.Map:
		if schema.AdditionalProperties != nil {
			_Annotate(t.Elem(), schema.AdditionalProperties, output, false)
		}

	case reflect.Struct:
		_AnnotateFields(t, schema, output, top)
	}
}

// _AnnotateFields annotates the properties of struct type t. A struct inferred without properties, such as a type
// schema override, is left unchanged.
func _AnnotateFields(t reflect.Type, schema *jsonschema.Schema, output bool, top bool) {
	if schema.Properties == nil {
		return
	}

	keys := _InputKeys
	if output {
		keys = _OutputKeys
	} else {
		schema.Required = nil
	}

	for _, field := range reflect.VisibleFields(t) {
		name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
		name = cmp.Or(name, field.Name)
		property := schema.Properties[name]
		if field.Anonymous || !field.IsExported() || property == nil {
			continue
		}

		tag := _NewTag(t, field, keys)
		if !top && (tag._Has("path") || tag._Has("query") || tag._Has("body")) {
			panic(fmt.Sprintf("gmcp: %s.%s is inside a body and cannot be placed in the request", t, field.Name))
		}

		tag._Apply(t, field, property)
		if !output && !_Optional(field) {
			schema.Required = append(schema.Required, name)
		}

		_Annotate(field.Type, property, output, false)
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

// _Apply sets the schema keys on property, or on its items for "items." keys. It panics on an unparsable number,
// example, or pattern, or an "items." key on a field that is not an array.
func (t _Tag) _Apply(owner reflect.Type, field reflect.StructField, property *jsonschema.Schema) {
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

		case "example":
			target.Examples = []any{_TagExample(owner, field, target, value)}

		case "deprecated":
			target.Deprecated = true

		case "format":
			target.Format = value

		case "pattern":
			if _, err := regexp.Compile(value); err != nil {
				panic(fmt.Sprintf("gmcp: %s.%s has invalid gmcp tag pattern %q", owner, field.Name, value))
			}

			target.Pattern = value

		case "minimum":
			target.Minimum = new(_TagNumber(owner, field, value))

		case "maximum":
			target.Maximum = new(_TagNumber(owner, field, value))

		case "maxLength":
			target.MaxLength = new(_TagInt(owner, field, value))

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
		panic(fmt.Sprintf("gmcp: %s.%s has invalid gmcp tag number %q", owner, field.Name, value))
	}

	return number
}

// _TagNumber parses a numeric bound and panics unless it is a finite JSON number.
func _TagNumber(owner reflect.Type, field reflect.StructField, value string) (number float64) {
	if err := json.Unmarshal([]byte(value), &number); err != nil {
		panic(fmt.Sprintf("gmcp: %s.%s has invalid gmcp tag number %q", owner, field.Name, value))
	}

	return number
}

// _TagExample parses an example by the schema type of target: a string example stays as written, and any other
// example is JSON of that type. It panics when the value does not parse.
func _TagExample(owner reflect.Type, field reflect.StructField, target *jsonschema.Schema, value string) (example any) {
	var err error
	switch _SchemaType(target) {
	case "string":
		return value

	case "integer":
		example, err = _DecodeExample[int64](value)

	case "number":
		example, err = _DecodeExample[float64](value)

	case "boolean":
		example, err = _DecodeExample[bool](value)

	default:
		example, err = _DecodeExample[any](value)
	}

	if err != nil {
		panic(fmt.Sprintf("gmcp: %s.%s has invalid gmcp tag example %q", owner, field.Name, value))
	}

	return example
}

// _DecodeExample decodes value as JSON of type T.
func _DecodeExample[T any](value string) (example any, err error) {
	var decoded T
	err = json.Unmarshal([]byte(value), &decoded)
	return decoded, err
}

// _SchemaType returns the type of schema other than null, or "" when it has none.
func _SchemaType(schema *jsonschema.Schema) (schemaType string) {
	if schema.Type != "" {
		return schema.Type
	}

	for _, candidate := range schema.Types {
		if candidate != "null" {
			return candidate
		}
	}

	return ""
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
