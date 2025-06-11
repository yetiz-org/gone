package erresponse

import (
	"github.com/yetiz-org/gone/erresponse/constant"
	"github.com/yetiz-org/gone/http/httpstatus"
	kkerror "github.com/yetiz-org/goth-kkerror"
)

var NotFoundObjectNotExist = Collection.Register(&DefaultErrorResponse{
	StatusCode:  httpstatus.NotFound,
	Name:        constant.ErrorNotFound,
	Description: "target object is not exist",
	DefaultKKError: kkerror.DefaultKKError{
		ErrorLevel:    kkerror.Normal,
		ErrorCategory: kkerror.Client,
		ErrorCode:     "404002",
	},
})
