package erresponse

import (
	"encoding/json"

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
	Data        map[string]any `json:"data,omitempty" goai:"description=Additional error data"`
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

func (d *DefaultErrorResponse) Clone() ErrorResponse {
	r := *d
	return &r
}
