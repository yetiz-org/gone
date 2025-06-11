package erresponse

import (
	"github.com/yetiz-org/gone/erresponse/constant"
	"github.com/yetiz-org/gone/http/httpstatus"
	kkerror "github.com/yetiz-org/goth-kkerror"
)

var ServerError = Collection.Register(&DefaultErrorResponse{
	StatusCode: httpstatus.InternalServerError,
	Name:       constant.ErrorServerError,
	DefaultKKError: kkerror.DefaultKKError{
		ErrorLevel:    kkerror.Urgent,
		ErrorCategory: kkerror.Server,
		ErrorCode:     "500001",
	},
})
