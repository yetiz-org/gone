package goai

import (
	"encoding"
	"encoding/json"
	"reflect"
	"sort"
	"strconv"
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
		if schema := b.jsonValueWrapperSchema(t); schema != nil {
			return schema
		}

		if schema := b.textMarshalerStructSchema(t); schema != nil {
			return schema
		}

		return b.buildStruct(t)
	case reflect.Interface:
		// Empty interface → free-form object.
		return &Schema{}
	default:
		return &Schema{}
	}
}

func (b *schemaBuilder) jsonValueWrapperSchema(t reflect.Type) *Schema {
	if !typeImplementsJSONUnmarshaler(t) {
		return nil
	}

	setField, ok := t.FieldByName("Set")
	if !ok || setField.Type.Kind() != reflect.Bool {
		return nil
	}

	valueField, ok := t.FieldByName("Value")
	if !ok {
		return nil
	}

	schema := b.build(valueField.Type)
	if schema == nil {
		return nil
	}

	if schema.Ref != "" {
		return &Schema{
			Nullable: true,
			AllOf:    []*Schema{schema},
		}
	}

	schema.Nullable = true

	return schema
}

func (b *schemaBuilder) textMarshalerStructSchema(t reflect.Type) *Schema {
	if t == reflect.TypeOf(time.Time{}) || !typeImplementsTextMarshaler(t) {
		return nil
	}

	name := b.componentName(t)
	if name == "" {
		return &Schema{Type: "string"}
	}

	if _, exists := b.components.Schemas[name]; !exists {
		b.components.Schemas[name] = &Schema{Type: "string"}
	}

	return &Schema{Ref: "#/components/schemas/" + name}
}

func typeImplementsTextMarshaler(t reflect.Type) bool {
	textMarshaler := reflect.TypeOf((*encoding.TextMarshaler)(nil)).Elem()

	return t.Implements(textMarshaler) || reflect.PointerTo(t).Implements(textMarshaler)
}

func typeImplementsJSONUnmarshaler(t reflect.Type) bool {
	jsonUnmarshaler := reflect.TypeOf((*json.Unmarshaler)(nil)).Elem()

	return t.Implements(jsonUnmarshaler) || reflect.PointerTo(t).Implements(jsonUnmarshaler)
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
	var embeddedSchemas []*Schema

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}

		// `goai:"-"` excludes a field from the generated schema while leaving
		// its json/runtime behavior untouched, mirroring the encoding/json
		// convention. Checked before the embedded-flatten and build() paths so
		// the field's type is never registered as a component when no other
		// field references it.
		if field.Tag.Get("goai") == "-" {
			continue
		}

		// Embedded struct: flatten its fields up.
		if field.Anonymous && field.Type.Kind() == reflect.Struct {
			inner := b.build(field.Type)
			if inner != nil && inner.Ref == "" && inner.Type != "object" {
				embeddedSchemas = append(embeddedSchemas, inner)

				continue
			}

			inner = b.structSchema(field.Type)
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

	if len(props) == 0 && len(required) == 0 && len(embeddedSchemas) == 1 {
		return embeddedSchemas[0]
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
//   - example           — example value parsed according to the schema type.
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
			if value != "" {
				prepareSchemaRefForSiblings(s)
			}
			s.Title = value
		case "description", "desc":
			if value != "" {
				prepareSchemaRefForSiblings(s)
			}
			s.Description = value
		case "example":
			if value != "" {
				prepareSchemaRefForSiblings(s)
			}
			s.Example = parseSchemaExample(s, value)
		case "default":
			if value != "" {
				prepareSchemaRefForSiblings(s)
			}
			s.Default = value
		case "format":
			if value != "" {
				prepareSchemaRefForSiblings(s)
			}
			s.Format = value
		case "enum":
			if value != "" {
				prepareSchemaRefForSiblings(s)
			}
			for _, v := range strings.Split(value, ",") {
				s.Enum = append(s.Enum, strings.TrimSpace(v))
			}
		case "nullable":
			if parsed := boolFlag(value); parsed {
				prepareSchemaRefForSiblings(s)
				s.Nullable = parsed
			} else {
				s.Nullable = parsed
			}
		case "deprecated":
			if parsed := boolFlag(value); parsed {
				prepareSchemaRefForSiblings(s)
				s.Deprecated = parsed
			} else {
				s.Deprecated = parsed
			}
		case "readOnly":
			if parsed := boolFlag(value); parsed {
				prepareSchemaRefForSiblings(s)
				s.ReadOnly = parsed
			} else {
				s.ReadOnly = parsed
			}
		case "writeOnly":
			if parsed := boolFlag(value); parsed {
				prepareSchemaRefForSiblings(s)
				s.WriteOnly = parsed
			} else {
				s.WriteOnly = parsed
			}
		case "uniqueItems":
			if parsed := boolFlag(value); parsed {
				prepareSchemaRefForSiblings(s)
				s.UniqueItems = parsed
			} else {
				s.UniqueItems = parsed
			}
		case "exclusiveMinimum":
			if parsed := boolFlag(value); parsed {
				prepareSchemaRefForSiblings(s)
				s.ExclusiveMinimum = parsed
			} else {
				s.ExclusiveMinimum = parsed
			}
		case "exclusiveMaximum":
			if parsed := boolFlag(value); parsed {
				prepareSchemaRefForSiblings(s)
				s.ExclusiveMaximum = parsed
			} else {
				s.ExclusiveMaximum = parsed
			}
		case "pattern":
			if value != "" {
				prepareSchemaRefForSiblings(s)
			}
			s.Pattern = value
		case "minLength":
			if parsed := parseUint(value); parsed != nil {
				prepareSchemaRefForSiblings(s)
				s.MinLength = parsed
			} else {
				s.MinLength = parsed
			}
		case "maxLength":
			if parsed := parseUint(value); parsed != nil {
				prepareSchemaRefForSiblings(s)
				s.MaxLength = parsed
			} else {
				s.MaxLength = parsed
			}
		case "minItems":
			if parsed := parseUint(value); parsed != nil {
				prepareSchemaRefForSiblings(s)
				s.MinItems = parsed
			} else {
				s.MinItems = parsed
			}
		case "maxItems":
			if parsed := parseUint(value); parsed != nil {
				prepareSchemaRefForSiblings(s)
				s.MaxItems = parsed
			} else {
				s.MaxItems = parsed
			}
		case "minProperties":
			if parsed := parseUint(value); parsed != nil {
				prepareSchemaRefForSiblings(s)
				s.MinProperties = parsed
			} else {
				s.MinProperties = parsed
			}
		case "maxProperties":
			if parsed := parseUint(value); parsed != nil {
				prepareSchemaRefForSiblings(s)
				s.MaxProperties = parsed
			} else {
				s.MaxProperties = parsed
			}
		case "minimum":
			if parsed := parseFloat(value); parsed != nil {
				prepareSchemaRefForSiblings(s)
				s.Minimum = parsed
			} else {
				s.Minimum = parsed
			}
		case "maximum":
			if parsed := parseFloat(value); parsed != nil {
				prepareSchemaRefForSiblings(s)
				s.Maximum = parsed
			} else {
				s.Maximum = parsed
			}
		case "multipleOf":
			if parsed := parseFloat(value); parsed != nil {
				prepareSchemaRefForSiblings(s)
				s.MultipleOf = parsed
			} else {
				s.MultipleOf = parsed
			}
		}
	}
}

func prepareSchemaRefForSiblings(s *Schema) {
	if s == nil || s.Ref == "" {
		return
	}

	ref := s.Ref
	s.Ref = ""
	s.AllOf = append([]*Schema{{Ref: ref}}, s.AllOf...)
}

func normalizeSchemaRefSiblings(s *Schema) {
	if s == nil {
		return
	}

	if s.Ref != "" && schemaHasRefSiblings(s) {
		prepareSchemaRefForSiblings(s)
	}

	normalizeSchemaRefSiblings(s.Items)
	normalizeSchemaRefSiblings(s.Not)
	for _, item := range s.OneOf {
		normalizeSchemaRefSiblings(item)
	}
	for _, item := range s.AllOf {
		normalizeSchemaRefSiblings(item)
	}
	for _, item := range s.AnyOf {
		normalizeSchemaRefSiblings(item)
	}
	for _, prop := range s.Properties {
		normalizeSchemaRefSiblings(prop)
	}
	if additional, ok := s.AdditionalProperties.(*Schema); ok {
		normalizeSchemaRefSiblings(additional)
	}
}

func normalizeDocumentSchemas(doc *Document) {
	if doc == nil {
		return
	}

	for _, item := range doc.Paths {
		normalizePathItemSchemas(item, doc.Components)
	}

	if doc.Components == nil {
		return
	}

	for _, schema := range doc.Components.Schemas {
		normalizeSchemaForDocument(schema, doc.Components)
	}
	for _, response := range doc.Components.Responses {
		normalizeResponseSchemas(response, doc.Components)
	}
	for _, parameter := range doc.Components.Parameters {
		normalizeParameterSchemas(parameter, doc.Components)
	}
	for _, requestBody := range doc.Components.RequestBodies {
		normalizeRequestBodySchemas(requestBody, doc.Components)
	}
	for _, header := range doc.Components.Headers {
		normalizeHeaderSchemas(header, doc.Components)
	}
}

func normalizePathItemSchemas(item *PathItem, components *Components) {
	if item == nil {
		return
	}

	for _, parameter := range item.Parameters {
		normalizeParameterSchemas(parameter, components)
	}
	for _, op := range []*Operation{item.Get, item.Put, item.Post, item.Delete, item.Options, item.Head, item.Patch, item.Trace} {
		normalizeOperationSchemas(op, components)
	}
}

func normalizeOperationSchemas(op *Operation, components *Components) {
	if op == nil {
		return
	}

	normalizeOperationSecurity(op)
	for _, parameter := range op.Parameters {
		normalizeParameterSchemas(parameter, components)
	}
	normalizeRequestBodySchemas(op.RequestBody, components)
	for _, response := range op.Responses {
		normalizeResponseSchemas(response, components)
	}
	for _, callback := range op.Callbacks {
		for _, item := range callback {
			normalizePathItemSchemas(item, components)
		}
	}
}

func normalizeOperationSecurity(op *Operation) {
	if op == nil || op.Security == nil {
		return
	}

	security := dedupeSecurityRequirements(*op.Security)
	op.Security = &security
}

func dedupeSecurityRefs(refs []SecurityRef) []SecurityRef {
	if len(refs) < 2 {
		return refs
	}

	out := make([]SecurityRef, 0, len(refs))
	seen := map[string]struct{}{}
	for _, ref := range refs {
		key := securityRequirementKey(ref.Scheme, ref.Scopes)
		if _, ok := seen[key]; ok {
			continue
		}

		seen[key] = struct{}{}
		out = append(out, ref)
	}

	return out
}

func dedupeSecurityRequirements(requirements []map[string][]string) []map[string][]string {
	if len(requirements) < 2 {
		return requirements
	}

	out := make([]map[string][]string, 0, len(requirements))
	seen := map[string]struct{}{}
	for _, requirement := range requirements {
		key := securityRequirementMapKey(requirement)
		if _, ok := seen[key]; ok {
			continue
		}

		seen[key] = struct{}{}
		out = append(out, requirement)
	}

	return out
}

func securityRequirementMapKey(requirement map[string][]string) string {
	if len(requirement) == 0 {
		return ""
	}

	parts := make([]string, 0, len(requirement))
	for scheme, scopes := range requirement {
		parts = append(parts, securityRequirementKey(scheme, scopes))
	}
	sort.Strings(parts)

	return strings.Join(parts, "|")
}

func securityRequirementKey(scheme string, scopes []string) string {
	sortedScopes := append([]string(nil), scopes...)
	sort.Strings(sortedScopes)

	return scheme + "\x00" + strings.Join(sortedScopes, "\x00")
}

func normalizeRequestBodySchemas(requestBody *RequestBody, components *Components) {
	if requestBody == nil {
		return
	}

	normalizeMediaTypeSchemas(requestBody.Content, components)
}

func normalizeResponseSchemas(response *Response, components *Components) {
	if response == nil {
		return
	}

	normalizeMediaTypeSchemas(response.Content, components)
	for _, header := range response.Headers {
		normalizeHeaderSchemas(header, components)
	}
}

func normalizeParameterSchemas(parameter *Parameter, components *Components) {
	if parameter == nil {
		return
	}

	normalizeSchemaForDocument(parameter.Schema, components)
	normalizeMediaTypeSchemas(parameter.Content, components)
}

func normalizeHeaderSchemas(header *Header, components *Components) {
	if header == nil {
		return
	}

	normalizeSchemaForDocument(header.Schema, components)
	normalizeMediaTypeSchemas(header.Content, components)
}

func normalizeMediaTypeSchemas(content map[string]*MediaType, components *Components) {
	for _, mediaType := range content {
		if mediaType == nil {
			continue
		}

		normalizeSchemaForDocument(mediaType.Schema, components)
	}
}

func normalizeSchemaForDocument(s *Schema, components *Components) {
	if s == nil {
		return
	}

	normalizeSchemaRefSiblings(s)
	applyNullableRefType(s, components)
	normalizeSchemaForDocument(s.Items, components)
	normalizeSchemaForDocument(s.Not, components)
	for _, item := range s.OneOf {
		normalizeSchemaForDocument(item, components)
	}
	for _, item := range s.AllOf {
		normalizeSchemaForDocument(item, components)
	}
	for _, item := range s.AnyOf {
		normalizeSchemaForDocument(item, components)
	}
	for _, prop := range s.Properties {
		normalizeSchemaForDocument(prop, components)
	}
	if additional, ok := s.AdditionalProperties.(*Schema); ok {
		normalizeSchemaForDocument(additional, components)
	}
}

func applyNullableRefType(s *Schema, components *Components) {
	if s == nil || !s.Nullable || s.Type != "" || len(s.AllOf) != 1 || s.AllOf[0] == nil || s.AllOf[0].Ref == "" {
		return
	}

	if typ := schemaTypeForRef(s.AllOf[0].Ref, components, map[string]bool{}); typ != "" {
		s.Type = typ
	}
}

func schemaTypeForRef(ref string, components *Components, seen map[string]bool) string {
	const prefix = "#/components/schemas/"
	if components == nil || !strings.HasPrefix(ref, prefix) {
		return ""
	}

	name := strings.TrimPrefix(ref, prefix)
	if seen[name] {
		return ""
	}
	seen[name] = true

	schema := components.Schemas[name]
	if schema == nil {
		return ""
	}

	if schema.Type != "" {
		return schema.Type
	}
	if schema.Ref != "" {
		return schemaTypeForRef(schema.Ref, components, seen)
	}
	if len(schema.Properties) > 0 || len(schema.Required) > 0 || schema.AdditionalProperties != nil {
		return "object"
	}
	if schema.Items != nil {
		return "array"
	}
	if len(schema.AllOf) == 1 && schema.AllOf[0] != nil && schema.AllOf[0].Ref != "" {
		return schemaTypeForRef(schema.AllOf[0].Ref, components, seen)
	}

	return ""
}

func cloneSchema(schema *Schema) *Schema {
	if schema == nil {
		return nil
	}

	clone := *schema
	clone.Enum = append([]any(nil), schema.Enum...)
	clone.OneOf = cloneSchemaSlice(schema.OneOf)
	clone.AllOf = cloneSchemaSlice(schema.AllOf)
	clone.AnyOf = cloneSchemaSlice(schema.AnyOf)
	clone.Not = cloneSchema(schema.Not)
	clone.Properties = cloneSchemaMap(schema.Properties)
	clone.Required = append([]string(nil), schema.Required...)
	clone.Items = cloneSchema(schema.Items)
	if additional, ok := schema.AdditionalProperties.(*Schema); ok {
		clone.AdditionalProperties = cloneSchema(additional)
	}
	if schema.Extensions != nil {
		clone.Extensions = map[string]any{}
		for key, value := range schema.Extensions {
			clone.Extensions[key] = value
		}
	}

	return &clone
}

func cloneSchemaSlice(schemas []*Schema) []*Schema {
	if len(schemas) == 0 {
		return nil
	}

	out := make([]*Schema, len(schemas))
	for i, schema := range schemas {
		out[i] = cloneSchema(schema)
	}

	return out
}

func cloneSchemaMap(schemas map[string]*Schema) map[string]*Schema {
	if len(schemas) == 0 {
		return nil
	}

	out := make(map[string]*Schema, len(schemas))
	for name, schema := range schemas {
		out[name] = cloneSchema(schema)
	}

	return out
}

func schemaHasRefSiblings(s *Schema) bool {
	return s.Title != "" ||
		s.Type != "" ||
		s.Format != "" ||
		s.Description != "" ||
		s.MultipleOf != nil ||
		s.Maximum != nil ||
		s.ExclusiveMaximum ||
		s.Minimum != nil ||
		s.ExclusiveMinimum ||
		s.MaxLength != nil ||
		s.MinLength != nil ||
		s.Pattern != "" ||
		s.MaxItems != nil ||
		s.MinItems != nil ||
		s.UniqueItems ||
		s.MaxProperties != nil ||
		s.MinProperties != nil ||
		len(s.Enum) > 0 ||
		s.Default != nil ||
		len(s.OneOf) > 0 ||
		len(s.AllOf) > 0 ||
		len(s.AnyOf) > 0 ||
		s.Not != nil ||
		len(s.Properties) > 0 ||
		len(s.Required) > 0 ||
		s.AdditionalProperties != nil ||
		s.Items != nil ||
		s.Nullable ||
		s.Discriminator != nil ||
		s.ReadOnly ||
		s.WriteOnly ||
		s.XML != nil ||
		s.ExternalDocs != nil ||
		s.Example != nil ||
		s.Deprecated ||
		len(s.Extensions) > 0
}

func parseSchemaExample(s *Schema, value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	if s == nil {
		return value
	}

	switch s.Type {
	case "boolean":
		if v, ok := parseExampleBool(value); ok {
			return v
		}
	case "integer":
		if v, err := strconv.ParseInt(value, 10, 64); err == nil {
			return v
		}

		if v, err := strconv.ParseUint(value, 10, 64); err == nil {
			return v
		}
	case "number":
		if v, err := strconv.ParseFloat(value, 64); err == nil {
			return v
		}
	case "object", "array":
		var v any
		if err := json.Unmarshal([]byte(value), &v); err == nil {
			return v
		}
	case "string":
		return value
	default:
		var v any
		if err := json.Unmarshal([]byte(value), &v); err == nil {
			return v
		}
	}

	return value
}

func parseExampleBool(value string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true":
		return true, true
	case "false":
		return false, true
	default:
		return false, false
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
