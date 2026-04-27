package goai

import (
	"reflect"
	"strings"
	"time"
)

// SchemaNameProvider lets a struct type choose its OpenAPI component schema
// name when goai builds schemas from reflection.
type SchemaNameProvider interface {
	GOAISchemaName() string
}

// schemaBuilder converts Go reflect types into OpenAPI schemas while
// deduplicating named struct types into the shared Components.Schemas map.
type schemaBuilder struct {
	components *Components
	visiting   map[reflect.Type]string // type → component name; tracks recursion
	typeOfName map[string]reflect.Type
	// pkgOfName tracks "<canonical component key> → <full pkg path>" so we
	// can detect collisions between same-typename structs that live in
	// different packages but share the same last package segment (e.g.
	// foo/admin.User vs bar/admin.User both want "admin.User"). On
	// collision we mangle the second occurrence using the full pkg path
	// to keep $ref pointers honest.
	pkgOfName map[string]string
}

// newSchemaBuilder returns a builder writing into the supplied components.
func newSchemaBuilder(components *Components) *schemaBuilder {
	if components == nil {
		components = NewComponents()
	}

	return &schemaBuilder{
		components: components,
		visiting:   map[reflect.Type]string{},
		typeOfName: map[string]reflect.Type{},
		pkgOfName:  map[string]string{},
	}
}

// build converts t into a Schema. Named struct types become $ref entries
// pointing into Components.Schemas; primitives, slices, maps and unnamed
// struct types are inlined.
func (b *schemaBuilder) build(t reflect.Type) *Schema {
	if t == nil {
		return nil
	}

	// Pointer is treated as nullable wrapper around the element type.
	if t.Kind() == reflect.Pointer {
		inner := b.build(t.Elem())
		if inner == nil {
			return nil
		}

		if inner.Ref != "" {
			// Cannot mark a $ref nullable directly in OpenAPI 3.0; wrap it.
			return &Schema{
				Nullable: true,
				AllOf:    []*Schema{inner},
			}
		}

		inner.Nullable = true

		return inner
	}

	// time.Time → string/date-time without recursing into struct fields.
	if t == reflect.TypeOf(time.Time{}) {
		return &Schema{Type: "string", Format: "date-time"}
	}

	switch t.Kind() {
	case reflect.Bool:
		return &Schema{Type: "boolean"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32:
		return &Schema{Type: "integer", Format: "int32"}
	case reflect.Int64:
		return &Schema{Type: "integer", Format: "int64"}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32:
		return &Schema{Type: "integer", Format: "int32"}
	case reflect.Uint64:
		return &Schema{Type: "integer", Format: "int64"}
	case reflect.Float32:
		return &Schema{Type: "number", Format: "float"}
	case reflect.Float64:
		return &Schema{Type: "number", Format: "double"}
	case reflect.String:
		return &Schema{Type: "string"}
	case reflect.Slice, reflect.Array:
		// []byte → base64-encoded string.
		if t.Elem().Kind() == reflect.Uint8 {
			return &Schema{Type: "string", Format: "byte"}
		}

		return &Schema{Type: "array", Items: b.build(t.Elem())}
	case reflect.Map:
		// OpenAPI maps are modelled as object with additionalProperties.
		valueSchema := b.build(t.Elem())

		return &Schema{Type: "object", AdditionalProperties: valueSchema}
	case reflect.Struct:
		return b.buildStruct(t)
	case reflect.Interface:
		// Empty interface → free-form object.
		return &Schema{}
	default:
		return &Schema{}
	}
}

// buildStruct registers (if not already) and returns either a $ref to a
// named component or an inline object schema for anonymous structs.
func (b *schemaBuilder) buildStruct(t reflect.Type) *Schema {
	name := b.componentName(t)

	if name != "" {
		if existing, ok := b.components.Schemas[name]; ok && existing != nil {
			return &Schema{Ref: "#/components/schemas/" + name}
		}

		// Recursion guard: insert a placeholder before descending.
		if _, recursing := b.visiting[t]; recursing {
			return &Schema{Ref: "#/components/schemas/" + name}
		}

		b.visiting[t] = name
		schema := b.structSchema(t)
		b.components.Schemas[name] = schema
		delete(b.visiting, t)

		return &Schema{Ref: "#/components/schemas/" + name}
	}

	return b.structSchema(t)
}

// structSchema builds the actual object Schema for a struct type, walking
// exported fields and respecting `json` and `goai` struct tags.
func (b *schemaBuilder) structSchema(t reflect.Type) *Schema {
	props := map[string]*Schema{}
	var required []string

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}

		// Embedded struct: flatten its fields up.
		if field.Anonymous && field.Type.Kind() == reflect.Struct {
			inner := b.structSchema(field.Type)
			for k, v := range inner.Properties {
				props[k] = v
			}
			required = append(required, inner.Required...)

			continue
		}

		jsonTag := field.Tag.Get("json")
		if jsonTag == "-" {
			continue
		}

		name, omitempty := splitJSONTag(jsonTag, field.Name)
		fieldSchema := b.build(field.Type)
		if fieldSchema == nil {
			continue
		}

		if desc := field.Tag.Get("goai"); desc != "" {
			applyGoaiTag(fieldSchema, desc)
		}

		props[name] = fieldSchema
		if !omitempty && field.Type.Kind() != reflect.Pointer {
			required = append(required, name)
		}
	}

	return &Schema{Type: "object", Properties: props, Required: required}
}

// componentName returns the canonical Components.Schemas key for a struct
// type, or empty for anonymous types. The default format is
// "<pkgPath last segment>.<TypeName>" because it produces compact, readable
// keys for the common case where each leaf package owns a distinct set of
// types. When a same-named type from a different package would collide
// with an already-recorded short key, the colliding type's full pkg path
// (with "/" replaced by ".") is used instead, so $ref pointers always
// resolve to the intended Go type.
//
// componentName is a method on schemaBuilder because collision tracking
// must be scoped to a single Build pass — using a package-level map would
// leak state across independent Build calls.
func (b *schemaBuilder) componentName(t reflect.Type) string {
	if t.Name() == "" {
		return ""
	}

	pkg := t.PkgPath()
	short := schemaComponentBaseName(t)
	if pkg == "" {
		return b.uniqueComponentName(short, t)
	}

	return b.uniqueComponentName(short, t)
}

func (b *schemaBuilder) uniqueComponentName(name string, t reflect.Type) string {
	if name == "" {
		return ""
	}

	pkg := t.PkgPath()
	if existing, seen := b.typeOfName[name]; seen {
		if existing == t {
			return name
		}

		full := fullSchemaComponentName(t)
		b.typeOfName[full] = t
		b.pkgOfName[full] = pkg

		return full
	}

	if existing, seen := b.pkgOfName[name]; seen && existing != pkg {
		full := fullSchemaComponentName(t)
		b.typeOfName[full] = t
		b.pkgOfName[full] = pkg

		return full
	}

	if _, exists := b.components.Schemas[name]; exists {
		existingPkg := b.pkgOfName[name]
		if existingPkg == "" || existingPkg != pkg {
			full := fullSchemaComponentName(t)
			b.typeOfName[full] = t
			b.pkgOfName[full] = pkg

			return full
		}
	}

	b.typeOfName[name] = t
	b.pkgOfName[name] = pkg

	return name
}

func schemaComponentBaseName(t reflect.Type) string {
	if name := schemaNameFromProvider(t); name != "" {
		return name
	}

	if t.PkgPath() == "" {
		return t.Name()
	}

	return shortPkgName(t.PkgPath()) + "." + t.Name()
}

func schemaNameFromProvider(t reflect.Type) string {
	if t == nil || t.Name() == "" {
		return ""
	}

	providerType := reflect.TypeOf((*SchemaNameProvider)(nil)).Elem()
	pointerType := reflect.PointerTo(t)
	if !pointerType.Implements(providerType) {
		return ""
	}

	provider := reflect.New(t).Interface().(SchemaNameProvider)

	return _SanitizeComponentName(strings.TrimSpace(provider.GOAISchemaName()))
}

func fullSchemaComponentName(t reflect.Type) string {
	if t.PkgPath() == "" {
		return t.Name()
	}

	return strings.ReplaceAll(t.PkgPath(), "/", ".") + "." + t.Name()
}

// shortPkgName returns the last "/"-separated segment of pkg, which is the
// idiomatic Go package import name for non-vanity paths.
func shortPkgName(pkg string) string {
	if idx := strings.LastIndex(pkg, "/"); idx >= 0 {
		return pkg[idx+1:]
	}

	return pkg
}

// splitJSONTag returns the JSON name and omitempty flag from a struct tag.
func splitJSONTag(tag, fallback string) (string, bool) {
	if tag == "" {
		return fallback, false
	}

	parts := strings.Split(tag, ",")
	name := parts[0]
	if name == "" {
		name = fallback
	}

	for _, p := range parts[1:] {
		if p == "omitempty" {
			return name, true
		}
	}

	return name, false
}

// applyGoaiTag parses `goai:"..."` annotations, e.g.:
//
//	goai:"description=The album title;example=Greatest Hits;minLength=1;maxLength=200"
//
// Tokens are separated by ';'; key=value pairs by '='. Unknown keys are
// silently ignored to keep the tag forward compatible.
//
// Supported keys (mirror the OpenAPI 3.0.3 Schema Object):
//
//   - title             — schema title.
//   - description, desc — schema description.
//   - example           — example value (string).
//   - default           — default value (string).
//   - format            — format hint (date-time, uuid, email, ...).
//   - enum              — comma-separated allowed values.
//   - pattern           — regex constraint for strings.
//   - minLength,
//     maxLength         — string length bounds.
//   - minimum, maximum  — numeric bounds (parsed as float64).
//   - exclusiveMinimum,
//     exclusiveMaximum  — boolean flags toggling inclusive→exclusive bounds.
//   - multipleOf        — numeric multiple constraint.
//   - minItems, maxItems, uniqueItems
//     — array constraints.
//   - minProperties,
//     maxProperties     — object constraints.
//   - readOnly,
//     writeOnly         — request/response visibility flags.
//   - deprecated        — marks the property deprecated.
//   - nullable          — marks the property nullable (OpenAPI 3.0 only).
func applyGoaiTag(s *Schema, tag string) {
	for _, segment := range strings.Split(tag, ";") {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			continue
		}

		kv := strings.SplitN(segment, "=", 2)
		key := strings.TrimSpace(kv[0])
		var value string
		if len(kv) == 2 {
			value = strings.TrimSpace(kv[1])
		}

		switch key {
		case "title":
			s.Title = value
		case "description", "desc":
			s.Description = value
		case "example":
			s.Example = value
		case "default":
			s.Default = value
		case "format":
			s.Format = value
		case "enum":
			for _, v := range strings.Split(value, ",") {
				s.Enum = append(s.Enum, strings.TrimSpace(v))
			}
		case "nullable":
			s.Nullable = boolFlag(value)
		case "deprecated":
			s.Deprecated = boolFlag(value)
		case "readOnly":
			s.ReadOnly = boolFlag(value)
		case "writeOnly":
			s.WriteOnly = boolFlag(value)
		case "uniqueItems":
			s.UniqueItems = boolFlag(value)
		case "exclusiveMinimum":
			s.ExclusiveMinimum = boolFlag(value)
		case "exclusiveMaximum":
			s.ExclusiveMaximum = boolFlag(value)
		case "pattern":
			s.Pattern = value
		case "minLength":
			s.MinLength = parseUint(value)
		case "maxLength":
			s.MaxLength = parseUint(value)
		case "minItems":
			s.MinItems = parseUint(value)
		case "maxItems":
			s.MaxItems = parseUint(value)
		case "minProperties":
			s.MinProperties = parseUint(value)
		case "maxProperties":
			s.MaxProperties = parseUint(value)
		case "minimum":
			s.Minimum = parseFloat(value)
		case "maximum":
			s.Maximum = parseFloat(value)
		case "multipleOf":
			s.MultipleOf = parseFloat(value)
		}
	}
}

// boolFlag returns true for the empty string and the literal "true";
// anything else (including "false") returns false. The empty case lets
// callers write `goai:"deprecated"` as a shorthand for `deprecated=true`.
func boolFlag(value string) bool {
	return value == "" || value == "true"
}

// parseUint returns a *uint64 for non-empty digit strings, nil otherwise.
// Using a pointer type lets the caller distinguish "constraint absent" from
// "constraint == 0" — both of which are meaningful in OpenAPI.
func parseUint(value string) *uint64 {
	if value == "" {
		return nil
	}

	var n uint64
	for i := 0; i < len(value); i++ {
		c := value[i]
		if c < '0' || c > '9' {
			return nil
		}

		n = n*10 + uint64(c-'0')
	}

	v := n

	return &v
}

// parseFloat returns a *float64 for numeric strings, nil otherwise. Accepts
// optional leading '-', a single decimal point, and ASCII digits.
func parseFloat(value string) *float64 {
	if value == "" {
		return nil
	}

	negative := false
	i := 0
	if value[0] == '-' {
		negative = true
		i = 1
	} else if value[0] == '+' {
		i = 1
	}

	if i >= len(value) {
		return nil
	}

	var intPart float64
	var fracPart float64
	var fracDiv float64 = 1
	sawDigit := false
	sawDot := false
	for ; i < len(value); i++ {
		c := value[i]
		switch {
		case c >= '0' && c <= '9':
			if sawDot {
				fracPart = fracPart*10 + float64(c-'0')
				fracDiv *= 10
			} else {
				intPart = intPart*10 + float64(c-'0')
			}

			sawDigit = true
		case c == '.' && !sawDot:
			sawDot = true
		default:
			return nil
		}
	}

	if !sawDigit {
		return nil
	}

	v := intPart + fracPart/fracDiv
	if negative {
		v = -v
	}

	return &v
}
