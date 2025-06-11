package erresponse

import (
	"github.com/yetiz-org/gone/erresponse/constant"
	"github.com/yetiz-org/gone/http/httpstatus"
	kkerror "github.com/yetiz-org/goth-kkerror"
)

var InvalidClientExpiresAtIsTooLong = Collection.Register(&DefaultErrorResponse{
	StatusCode:  httpstatus.Unauthorized,
	Name:        constant.ErrorInvalidClient,
	Description: "client secret exp is too long",
	DefaultKKError: kkerror.DefaultKKError{
		ErrorLevel:    kkerror.Normal,
		ErrorCategory: kkerror.Client,
		ErrorCode:     "401002",
	},
})
