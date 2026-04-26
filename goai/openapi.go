package goai

// This file models every Object defined by the OpenAPI Specification 3.0.3.
//
// The mapping is intentionally one-to-one with the spec so that emitted YAML
// is round-trippable and any spec validator (Spectral, Redocly, openapi-cli)
// will accept goai's output without warnings.
//
// Reference: https://spec.openapis.org/oas/v3.0.3
//
// Extensions: every object that allows Specification Extensions per the
// spec carries an `Extensions map[string]any` field tagged with
// `yaml:",inline"`. Keys in that map MUST start with "x-" — goai does not
// enforce the prefix at marshal time but downstream validators will. Use
// these maps to attach project-specific metadata (x-internal, x-rate-limit,
// vendor extensions, etc.) without forking goai's types.

// Document is the root of an OpenAPI 3.0.3 specification.
type Document struct {
	OpenAPI      string                 `yaml:"openapi"`
	Info         Info                   `yaml:"info"`
	Servers      []Server               `yaml:"servers,omitempty"`
	Tags         []Tag                  `yaml:"tags,omitempty"`
	Paths        map[string]*PathItem   `yaml:"paths"`
	Components   *Components            `yaml:"components,omitempty"`
	Security     []map[string][]string  `yaml:"security,omitempty"`
	ExternalDocs *ExternalDocumentation `yaml:"externalDocs,omitempty"`
	Extensions   map[string]any         `yaml:",inline,omitempty"`
}

// Info carries metadata about the API.
type Info struct {
	Title          string         `yaml:"title"`
	Description    string         `yaml:"description,omitempty"`
	TermsOfService string         `yaml:"termsOfService,omitempty"`
	Contact        *Contact       `yaml:"contact,omitempty"`
	License        *License       `yaml:"license,omitempty"`
	Version        string         `yaml:"version"`
	Extensions     map[string]any `yaml:",inline,omitempty"`
}

// Contact is the optional API contact block.
type Contact struct {
	Name       string         `yaml:"name,omitempty"`
	URL        string         `yaml:"url,omitempty"`
	Email      string         `yaml:"email,omitempty"`
	Extensions map[string]any `yaml:",inline,omitempty"`
}

// License is the optional API license block. Required field: Name.
type License struct {
	Name       string         `yaml:"name"`
	URL        string         `yaml:"url,omitempty"`
	Extensions map[string]any `yaml:",inline,omitempty"`
}

// Server represents one entry in the top-level servers array, or in
// PathItem.Servers / Operation.Servers when overriding the global value.
type Server struct {
	URL         string                     `yaml:"url"`
	Description string                     `yaml:"description,omitempty"`
	Variables   map[string]*ServerVariable `yaml:"variables,omitempty"`
	Extensions  map[string]any             `yaml:",inline,omitempty"`
}

// ServerVariable describes one URL template variable for a Server URL.
// Default is required by the spec; Enum constrains the allowed values.
type ServerVariable struct {
	Enum        []string       `yaml:"enum,omitempty"`
	Default     string         `yaml:"default"`
	Description string         `yaml:"description,omitempty"`
	Extensions  map[string]any `yaml:",inline,omitempty"`
}

// Tag is a top-level tag definition.
type Tag struct {
	Name         string                 `yaml:"name"`
	Description  string                 `yaml:"description,omitempty"`
	ExternalDocs *ExternalDocumentation `yaml:"externalDocs,omitempty"`
	Extensions   map[string]any         `yaml:",inline,omitempty"`
}

// ExternalDocumentation references additional external documentation.
type ExternalDocumentation struct {
	Description string         `yaml:"description,omitempty"`
	URL         string         `yaml:"url"`
	Extensions  map[string]any `yaml:",inline,omitempty"`
}

// PathItem holds the operations defined for a single path.
type PathItem struct {
	Ref         string         `yaml:"$ref,omitempty"`
	Summary     string         `yaml:"summary,omitempty"`
	Description string         `yaml:"description,omitempty"`
	Get         *Operation     `yaml:"get,omitempty"`
	Put         *Operation     `yaml:"put,omitempty"`
	Post        *Operation     `yaml:"post,omitempty"`
	Delete      *Operation     `yaml:"delete,omitempty"`
	Options     *Operation     `yaml:"options,omitempty"`
	Head        *Operation     `yaml:"head,omitempty"`
	Patch       *Operation     `yaml:"patch,omitempty"`
	Trace       *Operation     `yaml:"trace,omitempty"`
	Servers     []Server       `yaml:"servers,omitempty"`
	Parameters  []*Parameter   `yaml:"parameters,omitempty"`
	Extensions  map[string]any `yaml:",inline,omitempty"`
}

// SetOperation assigns op under the slot for the given upper-case method.
// Unknown methods are silently ignored.
func (p *PathItem) SetOperation(method string, op *Operation) {
	switch method {
	case "GET":
		p.Get = op
	case "PUT":
		p.Put = op
	case "POST":
		p.Post = op
	case "DELETE":
		p.Delete = op
	case "OPTIONS":
		p.Options = op
	case "HEAD":
		p.Head = op
	case "PATCH":
		p.Patch = op
	case "TRACE":
		p.Trace = op
	}
}

// Operation describes a single HTTP operation.
type Operation struct {
	Tags         []string               `yaml:"tags,omitempty"`
	Summary      string                 `yaml:"summary,omitempty"`
	Description  string                 `yaml:"description,omitempty"`
	ExternalDocs *ExternalDocumentation `yaml:"externalDocs,omitempty"`
	OperationID  string                 `yaml:"operationId,omitempty"`
	Parameters   []*Parameter           `yaml:"parameters,omitempty"`
	RequestBody  *RequestBody           `yaml:"requestBody,omitempty"`
	Responses    map[string]*Response   `yaml:"responses,omitempty"`
	Callbacks    map[string]Callback    `yaml:"callbacks,omitempty"`
	Deprecated   bool                   `yaml:"deprecated,omitempty"`
	// Security is a pointer so the encoder can distinguish three states:
	//   nil          → inherit document-level Security (default).
	//   &[]          → explicit "no security required"; overrides document
	//                  default with an empty array per OpenAPI 3.0.3.
	//   &[{...}, ..] → operation-level requirement set; overrides document
	//                  default with the supplied alternatives.
	Security   *[]map[string][]string `yaml:"security,omitempty"`
	Servers    []Server               `yaml:"servers,omitempty"`
	Extensions map[string]any         `yaml:",inline,omitempty"`
}

// Parameter is a path/query/header/cookie parameter, or a top-level entry
// inside Components.Parameters. Either Schema or Content (but not both)
// describes the value's shape; for simple parameters use Schema, for
// complex media-typed ones use Content.
type Parameter struct {
	Name            string                `yaml:"name"`
	In              string                `yaml:"in"`
	Description     string                `yaml:"description,omitempty"`
	Required        bool                  `yaml:"required,omitempty"`
	Deprecated      bool                  `yaml:"deprecated,omitempty"`
	AllowEmptyValue bool                  `yaml:"allowEmptyValue,omitempty"`
	Style           string                `yaml:"style,omitempty"`
	Explode         *bool                 `yaml:"explode,omitempty"`
	AllowReserved   bool                  `yaml:"allowReserved,omitempty"`
	Schema          *Schema               `yaml:"schema,omitempty"`
	Example         any                   `yaml:"example,omitempty"`
	Examples        map[string]*Example   `yaml:"examples,omitempty"`
	Content         map[string]*MediaType `yaml:"content,omitempty"`
	Extensions      map[string]any        `yaml:",inline,omitempty"`
}

// RequestBody describes an operation request body.
type RequestBody struct {
	Description string                `yaml:"description,omitempty"`
	Content     map[string]*MediaType `yaml:"content,omitempty"`
	Required    bool                  `yaml:"required,omitempty"`
	Extensions  map[string]any        `yaml:",inline,omitempty"`
}

// MediaType describes one entry under content (per media-type key).
type MediaType struct {
	Schema     *Schema              `yaml:"schema,omitempty"`
	Example    any                  `yaml:"example,omitempty"`
	Examples   map[string]*Example  `yaml:"examples,omitempty"`
	Encoding   map[string]*Encoding `yaml:"encoding,omitempty"`
	Extensions map[string]any       `yaml:",inline,omitempty"`
}

// Encoding describes a single multipart/form-encoded property.
type Encoding struct {
	ContentType   string             `yaml:"contentType,omitempty"`
	Headers       map[string]*Header `yaml:"headers,omitempty"`
	Style         string             `yaml:"style,omitempty"`
	Explode       *bool              `yaml:"explode,omitempty"`
	AllowReserved bool               `yaml:"allowReserved,omitempty"`
	Extensions    map[string]any     `yaml:",inline,omitempty"`
}

// Response describes a single HTTP status code response.
type Response struct {
	Description string                `yaml:"description"`
	Headers     map[string]*Header    `yaml:"headers,omitempty"`
	Content     map[string]*MediaType `yaml:"content,omitempty"`
	Links       map[string]*Link      `yaml:"links,omitempty"`
	Extensions  map[string]any        `yaml:",inline,omitempty"`
}

// Callback is a map of expressions (e.g. "{$request.body#/callbackUrl}")
// to PathItem values. Per the OpenAPI 3.0.3 spec, the keys are "Runtime
// Expression" strings; x-* extension keys may appear at the same level
// alongside expression keys. yaml.v3 cannot carry two inline maps on one
// struct, so Callback is modeled as a plain map[string]*PathItem and any
// extension keys live alongside the expressions in this same map.
type Callback map[string]*PathItem

// Example is the OpenAPI Example Object. Use Value for inline JSON-y
// examples, or ExternalValue when the example lives in a separate file.
type Example struct {
	Summary       string         `yaml:"summary,omitempty"`
	Description   string         `yaml:"description,omitempty"`
	Value         any            `yaml:"value,omitempty"`
	ExternalValue string         `yaml:"externalValue,omitempty"`
	Extensions    map[string]any `yaml:",inline,omitempty"`
}

// Link Object — describes a possible design-time link from a response to
// another operation. OperationRef and OperationID are mutually exclusive.
type Link struct {
	OperationRef string         `yaml:"operationRef,omitempty"`
	OperationID  string         `yaml:"operationId,omitempty"`
	Parameters   map[string]any `yaml:"parameters,omitempty"`
	RequestBody  any            `yaml:"requestBody,omitempty"`
	Description  string         `yaml:"description,omitempty"`
	Server       *Server        `yaml:"server,omitempty"`
	Extensions   map[string]any `yaml:",inline,omitempty"`
}

// Header is the response header object. Per OpenAPI 3.0.3 it is a Parameter
// without `name` and `in`.
type Header struct {
	Description     string                `yaml:"description,omitempty"`
	Required        bool                  `yaml:"required,omitempty"`
	Deprecated      bool                  `yaml:"deprecated,omitempty"`
	AllowEmptyValue bool                  `yaml:"allowEmptyValue,omitempty"`
	Style           string                `yaml:"style,omitempty"`
	Explode         *bool                 `yaml:"explode,omitempty"`
	AllowReserved   bool                  `yaml:"allowReserved,omitempty"`
	Schema          *Schema               `yaml:"schema,omitempty"`
	Example         any                   `yaml:"example,omitempty"`
	Examples        map[string]*Example   `yaml:"examples,omitempty"`
	Content         map[string]*MediaType `yaml:"content,omitempty"`
	Extensions      map[string]any        `yaml:",inline,omitempty"`
}

// Schema is the OpenAPI 3.0.3 Schema Object — a strict subset of JSON
// Schema Draft 4 with OAS-specific keywords (nullable, discriminator,
// readOnly/writeOnly, xml, externalDocs, example).
type Schema struct {
	Ref string `yaml:"$ref,omitempty"`

	Title       string `yaml:"title,omitempty"`
	Type        string `yaml:"type,omitempty"`
	Format      string `yaml:"format,omitempty"`
	Description string `yaml:"description,omitempty"`

	// Numeric validation
	MultipleOf       *float64 `yaml:"multipleOf,omitempty"`
	Maximum          *float64 `yaml:"maximum,omitempty"`
	ExclusiveMaximum bool     `yaml:"exclusiveMaximum,omitempty"`
	Minimum          *float64 `yaml:"minimum,omitempty"`
	ExclusiveMinimum bool     `yaml:"exclusiveMinimum,omitempty"`

	// String validation
	MaxLength *uint64 `yaml:"maxLength,omitempty"`
	MinLength *uint64 `yaml:"minLength,omitempty"`
	Pattern   string  `yaml:"pattern,omitempty"`

	// Array validation
	MaxItems    *uint64 `yaml:"maxItems,omitempty"`
	MinItems    *uint64 `yaml:"minItems,omitempty"`
	UniqueItems bool    `yaml:"uniqueItems,omitempty"`

	// Object validation
	MaxProperties *uint64 `yaml:"maxProperties,omitempty"`
	MinProperties *uint64 `yaml:"minProperties,omitempty"`

	// Enumeration / fixed
	Enum    []any `yaml:"enum,omitempty"`
	Default any   `yaml:"default,omitempty"`

	// Composition
	OneOf []*Schema `yaml:"oneOf,omitempty"`
	AllOf []*Schema `yaml:"allOf,omitempty"`
	AnyOf []*Schema `yaml:"anyOf,omitempty"`
	Not   *Schema   `yaml:"not,omitempty"`

	// Object
	Properties           map[string]*Schema `yaml:"properties,omitempty"`
	Required             []string           `yaml:"required,omitempty"`
	AdditionalProperties any                `yaml:"additionalProperties,omitempty"`

	// Array
	Items *Schema `yaml:"items,omitempty"`

	// OAS-specific
	Nullable      bool                   `yaml:"nullable,omitempty"`
	Discriminator *Discriminator         `yaml:"discriminator,omitempty"`
	ReadOnly      bool                   `yaml:"readOnly,omitempty"`
	WriteOnly     bool                   `yaml:"writeOnly,omitempty"`
	XML           *XML                   `yaml:"xml,omitempty"`
	ExternalDocs  *ExternalDocumentation `yaml:"externalDocs,omitempty"`
	Example       any                    `yaml:"example,omitempty"`
	Deprecated    bool                   `yaml:"deprecated,omitempty"`

	Extensions map[string]any `yaml:",inline,omitempty"`
}

// Discriminator supports oneOf/anyOf polymorphism by naming the property
// whose value selects the concrete sub-schema.
type Discriminator struct {
	PropertyName string            `yaml:"propertyName"`
	Mapping      map[string]string `yaml:"mapping,omitempty"`
}

// XML adds XML serialization metadata to a Schema.
type XML struct {
	Name       string         `yaml:"name,omitempty"`
	Namespace  string         `yaml:"namespace,omitempty"`
	Prefix     string         `yaml:"prefix,omitempty"`
	Attribute  bool           `yaml:"attribute,omitempty"`
	Wrapped    bool           `yaml:"wrapped,omitempty"`
	Extensions map[string]any `yaml:",inline,omitempty"`
}

// Components is the reusable definitions block.
type Components struct {
	Schemas         map[string]*Schema         `yaml:"schemas,omitempty"`
	Responses       map[string]*Response       `yaml:"responses,omitempty"`
	Parameters      map[string]*Parameter      `yaml:"parameters,omitempty"`
	Examples        map[string]*Example        `yaml:"examples,omitempty"`
	RequestBodies   map[string]*RequestBody    `yaml:"requestBodies,omitempty"`
	Headers         map[string]*Header         `yaml:"headers,omitempty"`
	SecuritySchemes map[string]*SecurityScheme `yaml:"securitySchemes,omitempty"`
	Links           map[string]*Link           `yaml:"links,omitempty"`
	Callbacks       map[string]Callback        `yaml:"callbacks,omitempty"`
	Extensions      map[string]any             `yaml:",inline,omitempty"`
}

// NewComponents returns a Components value with all submaps preallocated.
func NewComponents() *Components {
	return &Components{
		Schemas:         map[string]*Schema{},
		Responses:       map[string]*Response{},
		Parameters:      map[string]*Parameter{},
		Examples:        map[string]*Example{},
		RequestBodies:   map[string]*RequestBody{},
		Headers:         map[string]*Header{},
		SecuritySchemes: map[string]*SecurityScheme{},
		Links:           map[string]*Link{},
		Callbacks:       map[string]Callback{},
	}
}

// SecurityScheme is the security scheme object. Supported types per the
// spec: apiKey, http, oauth2, openIdConnect.
type SecurityScheme struct {
	Type             string         `yaml:"type"`
	Description      string         `yaml:"description,omitempty"`
	Name             string         `yaml:"name,omitempty"`
	In               string         `yaml:"in,omitempty"`
	Scheme           string         `yaml:"scheme,omitempty"`
	BearerFormat     string         `yaml:"bearerFormat,omitempty"`
	Flows            *OAuthFlows    `yaml:"flows,omitempty"`
	OpenIDConnectURL string         `yaml:"openIdConnectUrl,omitempty"`
	Extensions       map[string]any `yaml:",inline,omitempty"`
}

// OAuthFlows aggregates all flow types under a SecurityScheme.
type OAuthFlows struct {
	Implicit          *OAuthFlow     `yaml:"implicit,omitempty"`
	Password          *OAuthFlow     `yaml:"password,omitempty"`
	ClientCredentials *OAuthFlow     `yaml:"clientCredentials,omitempty"`
	AuthorizationCode *OAuthFlow     `yaml:"authorizationCode,omitempty"`
	Extensions        map[string]any `yaml:",inline,omitempty"`
}

// OAuthFlow is one specific OAuth2 flow.
type OAuthFlow struct {
	AuthorizationURL string            `yaml:"authorizationUrl,omitempty"`
	TokenURL         string            `yaml:"tokenUrl,omitempty"`
	RefreshURL       string            `yaml:"refreshUrl,omitempty"`
	Scopes           map[string]string `yaml:"scopes,omitempty"`
	Extensions       map[string]any    `yaml:",inline,omitempty"`
}
