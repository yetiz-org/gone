package erresponse

import (
	"github.com/yetiz-org/gone/erresponse/constant"
	"github.com/yetiz-org/gone/http/httpstatus"
	kkerror "github.com/yetiz-org/goth-kkerror"
)

var NotFoundUserNotFound = Collection.Register(&DefaultErrorResponse{
	StatusCode:  httpstatus.NotFound,
	Name:        constant.ErrorNotFound,
	Description: "user not found",
	DefaultKKError: kkerror.DefaultKKError{
		ErrorLevel:    kkerror.Normal,
		ErrorCategory: kkerror.Client,
		ErrorCode:     "404003",
	},
})
