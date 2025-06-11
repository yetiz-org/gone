package erresponse

import (
	"github.com/yetiz-org/gone/erresponse/constant"
	"github.com/yetiz-org/gone/http/httpstatus"
	kkerror "github.com/yetiz-org/goth-kkerror"
)

var InvalidRequestInvalidRequestUri = Collection.Register(&DefaultErrorResponse{
	StatusCode:  httpstatus.BadRequest,
	Name:        constant.ErrorInvalidRequest,
	Description: "invalid request_uri",
	DefaultKKError: kkerror.DefaultKKError{
		ErrorLevel:    kkerror.Normal,
		ErrorCategory: kkerror.Client,
		ErrorCode:     "400011",
	},
})
