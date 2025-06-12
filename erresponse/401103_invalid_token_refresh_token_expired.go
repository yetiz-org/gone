package erresponse

import (
	"github.com/yetiz-org/gone/erresponse/constant"
	"github.com/yetiz-org/gone/ghttp/httpstatus"
	kkerror "github.com/yetiz-org/goth-kkerror"
)

var InvalidGrantRefreshTokenExpired = Collection.Register(&DefaultErrorResponse{
	StatusCode:  httpstatus.Unauthorized,
	Name:        constant.ErrorInvalidToken,
	Description: "refresh_token is expired",
	DefaultKKError: kkerror.DefaultKKError{
		ErrorLevel:    kkerror.Normal,
		ErrorCategory: kkerror.Client,
		ErrorCode:     "401103",
	},
})
