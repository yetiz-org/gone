package erresponse

import (
	"github.com/yetiz-org/gone/erresponse/constant"
	"github.com/yetiz-org/gone/ghttp/httpstatus"
	kkerror "github.com/yetiz-org/goth-kkerror"
)

var SlowDownTooFast = Collection.Register(&DefaultErrorResponse{
	StatusCode:  httpstatus.TooManyRequests,
	Name:        constant.ErrorSlowDown,
	Description: "too fast, rate limit exceeded",
	DefaultKKError: kkerror.DefaultKKError{
		ErrorLevel:    kkerror.Normal,
		ErrorCategory: kkerror.Client,
		ErrorCode:     "429001",
	},
})
