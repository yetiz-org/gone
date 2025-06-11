package erresponse

import (
	"github.com/yetiz-org/gone/erresponse/constant"
	"github.com/yetiz-org/gone/http/httpstatus"
	kkerror "github.com/yetiz-org/goth-kkerror"
)

var SRPUnsupported = Collection.Register(&DefaultErrorResponse{
	StatusCode:  httpstatus.Forbidden,
	Name:        constant.ErrorSRPUnsupported,
	Description: "",
	DefaultKKError: kkerror.DefaultKKError{
		ErrorLevel:    kkerror.Normal,
		ErrorCategory: kkerror.Client,
		ErrorCode:     "403201",
	},
})
