package erresponse

import (
	"github.com/yetiz-org/gone/erresponse/constant"
	"github.com/yetiz-org/gone/ghttp/httpstatus"
	kkerror "github.com/yetiz-org/goth-kkerror"
)

var InvalidRequestWrongTokenFormat = Collection.Register(&DefaultErrorResponse{
	StatusCode:  httpstatus.BadRequest,
	Name:        constant.ErrorInvalidRequest,
	Description: "wrong token format",
	DefaultKKError: kkerror.DefaultKKError{
		ErrorLevel:    kkerror.Normal,
		ErrorCategory: kkerror.Client,
		ErrorCode:     "400005",
	},
})
