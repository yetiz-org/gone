package erresponse

import (
	"fmt"
	"github.com/yetiz-org/gone/erresponse/constant"
	"github.com/yetiz-org/gone/ghttp/httpstatus"
	kkerror "github.com/yetiz-org/goth-kkerror"
)

func InvalidTokenWithMessage(format string, params ...interface{}) ErrorResponse {
	return &DefaultErrorResponse{
		StatusCode:  httpstatus.Unauthorized,
		Name:        constant.ErrorInvalidToken,
		Description: "",
		DefaultKKError: kkerror.DefaultKKError{
			ErrorLevel:    kkerror.Normal,
			ErrorCategory: kkerror.Client,
			ErrorCode:     "401402",
			ErrorMessage:  fmt.Sprintf(format, params...),
		},
	}
}
