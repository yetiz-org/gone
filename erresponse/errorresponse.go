package erresponse

import (
	"encoding/json"
	"maps"

	kkerror "github.com/yetiz-org/goth-kkerror"
)

type ErrorResponse interface {
	kkerror.KKError
	ErrorStatusCode() int
	ErrorName() string
	ErrorDescription() string
	ErrorData() map[string]any
	Clone() ErrorResponse
}

// DefaultErrorResponse is the standard JSON error envelope returned by ghttp handlers.
//
// @goai.schemaName erresponse.DefaultErrorResponse
type DefaultErrorResponse struct {
	kkerror.DefaultKKError
	StatusCode  int            `json:"status_code,omitempty" goai:"description=HTTP status code;example=401"`
	Name        string         `json:"error,omitempty" goai:"description=Error code;example=invalid_token"`
	Description string         `json:"error_description,omitempty" goai:"description=Error description;example=insufficient authentication"`
	Title       string         `json:"title,omitempty" goai:"description=Localized error title for end-user display;example=Invalid Request"`
	Detail      string         `json:"detail,omitempty" goai:"description=Localized error detail for end-user display;example=name can't be empty"`
	Data        map[string]any `json:"data,omitempty" goai:"description=Additional error data"`
	// I18nParams carries named parameters for localization template
	// interpolation by consuming services (e.g. {name} placeholders).
	// It is never serialized into the response body.
	I18nParams map[string]string `json:"-" goai:"-"`
}

func (d *DefaultErrorResponse) GOAISchemaName() string {
	return "erresponse.DefaultErrorResponse"
}

func (d *DefaultErrorResponse) Error() string {
	if jsonByte, err := json.Marshal(d); err == nil {
		return string(jsonByte)
	}

	return ""
}

func (d *DefaultErrorResponse) ErrorStatusCode() int {
	return d.StatusCode
}

func (d *DefaultErrorResponse) ErrorName() string {
	return d.Name
}

func (d *DefaultErrorResponse) ErrorDescription() string {
	return d.Description
}

func (d *DefaultErrorResponse) ErrorData() map[string]any {
	if d.Data == nil {
		d.Data = map[string]any{}
	}

	return d.Data
}

// Clone returns a copy that shares no mutable state with the receiver.
//
// The map copies are required, not defensive: registered responses are
// package-level singletons and callers write per-request keys into ErrorData()
// immediately after cloning (see ghttp Response.ResponseError). Sharing the maps
// would let concurrent requests write the same map.
//
// maps.Clone preserves nil, so a nil map stays nil and costs no allocation.
func (d *DefaultErrorResponse) Clone() ErrorResponse {
	r := new(*d)
	r.Data = maps.Clone(d.Data)
	r.I18nParams = maps.Clone(d.I18nParams)
	return r
}
