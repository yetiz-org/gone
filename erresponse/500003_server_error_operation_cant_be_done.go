package erresponse

import (
	"github.com/yetiz-org/gone/erresponse/constant"
	"github.com/yetiz-org/gone/http/httpstatus"
	kkerror "github.com/yetiz-org/goth-kkerror"
)

var ServerErrorOperationCantBeDone = Collection.Register(&DefaultErrorResponse{
	StatusCode:  httpstatus.InternalServerError,
	Name:        constant.ErrorServerError,
	Description: "operation can't be done",
	DefaultKKError: kkerror.DefaultKKError{
		ErrorLevel:    kkerror.Critical,
		ErrorCategory: kkerror.Process,
		ErrorCode:     "500003",
	},
})
