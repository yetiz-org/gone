package erresponse

import (
	"fmt"

	"github.com/yetiz-org/gone/erresponse/constant"
	"github.com/yetiz-org/gone/ghttp/httpstatus"
	kkerror "github.com/yetiz-org/goth-kkerror"
)

func InvalidGrantWithMessage(format string, params ...any) ErrorResponse {
	return &DefaultErrorResponse{
		StatusCode:  httpstatus.Forbidden,
		Name:        constant.ErrorInvalidGrant,
		Description: "",
		DefaultKKError: kkerror.DefaultKKError{
			ErrorLevel:    kkerror.Normal,
			ErrorCategory: kkerror.Client,
			ErrorCode:     "403401",
			ErrorMessage:  fmt.Sprintf(format, params...),
		},
	}
}
