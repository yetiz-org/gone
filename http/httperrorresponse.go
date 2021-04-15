package http

import (
	"github.com/kklab-com/goth-erresponse"
	kkpanic "github.com/kklab-com/goth-panic"
)

type ErrorResponse interface {
	erresponse.ErrorResponse
}

type ErrorResponseImpl struct {
	erresponse.ErrorResponse
	Caught *kkpanic.CaughtImpl `json:"caught,omitempty"`
}

func (e *ErrorResponseImpl) String() string {
	return e.Caught.String()
}
