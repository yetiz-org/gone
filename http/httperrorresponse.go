package http

import (
	"github.com/yetiz-org/gone/erresponse"
	kkpanic "github.com/yetiz-org/goth-panic"
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
