package erresponse

import (
	"github.com/yetiz-org/gone/erresponse/constant"
	kkerror "github.com/yetiz-org/goth-kkerror"
)

var NotImplemented = Collection.Register(&DefaultErrorResponse{
	StatusCode:  0,
	Name:        constant.ErrorNotImplemented,
	Description: "",
	DefaultKKError: kkerror.DefaultKKError{
		ErrorLevel:    kkerror.Normal,
		ErrorCategory: kkerror.Server,
		ErrorCode:     "000000",
		ErrorMessage:  "not_implemented",
	},
})
