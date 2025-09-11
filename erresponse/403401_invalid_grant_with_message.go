package erresponse

import (
	"fmt"

	"github.com/yetiz-org/gone/erresponse/constant"
	"github.com/yetiz-org/gone/ghttp/httpstatus"
	kkerror "github.com/yetiz-org/goth-kkerror"
)

func InvalidGrantWithMessage(message string) ErrorResponse {
	return &DefaultErrorResponse{
		StatusCode:  httpstatus.Forbidden,
		Name:        constant.ErrorInvalidGrant,
		Description: "",
		DefaultKKError: kkerror.DefaultKKError{
			ErrorLevel:    kkerror.Normal,
			ErrorCategory: kkerror.Client,
			ErrorCode:     "403401",
			ErrorMessage:  message,
		},
	}
}

// InvalidGrantWithFormat provides backward compatibility for dynamic format strings
func InvalidGrantWithFormat(format string, args ...interface{}) ErrorResponse {
	return InvalidGrantWithMessage(fmt.Sprintf(format, args...))
}
