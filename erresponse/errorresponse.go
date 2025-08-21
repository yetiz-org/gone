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
	ErrorData() map[string]interface{}
	Clone() ErrorResponse
}

type DefaultErrorResponse struct {
	kkerror.DefaultKKError
	StatusCode  int                    `json:"status_code,omitempty"`
	Name        string                 `json:"error,omitempty"`
	Description string                 `json:"error_description,omitempty"`
	Data        map[string]interface{} `json:"data,omitempty"`
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

func (d *DefaultErrorResponse) ErrorData() map[string]interface{} {
	if d.Data == nil {
		d.Data = map[string]interface{}{}
	}

	return d.Data
}

func (d *DefaultErrorResponse) Clone() ErrorResponse {
	r := *d
	return &r
}
