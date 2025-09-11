package erresponse

import (
	"fmt"

	"github.com/yetiz-org/gone/ghttp/httpstatus"
	kkerror "github.com/yetiz-org/goth-kkerror"

	"github.com/yetiz-org/gone/erresponse/constant"
)

var ServerErrorSMSOperationFail = Collection.Register(&DefaultErrorResponse{
	StatusCode:  httpstatus.InternalServerError,
	Name:        constant.ErrorServerError,
	Description: "sms operation fail",
	DefaultKKError: kkerror.DefaultKKError{
		ErrorLevel:    kkerror.Critical,
		ErrorCategory: kkerror.Internal,
		ErrorCode:     "500008",
	},
})

func ServerErrorSMSOperationWithMessage(message string) ErrorResponse {
	return &DefaultErrorResponse{
		StatusCode:  httpstatus.InternalServerError,
		Name:        constant.ErrorServerError,
		Description: "sms operation fail",
		DefaultKKError: kkerror.DefaultKKError{
			ErrorLevel:    kkerror.Critical,
			ErrorCategory: kkerror.Internal,
			ErrorCode:     "500008",
			ErrorMessage:  message,
		},
	}
}

// ServerErrorSMSOperationWithFormat provides backward compatibility for dynamic format strings
func ServerErrorSMSOperationWithFormat(format string, args ...interface{}) ErrorResponse {
	return ServerErrorSMSOperationWithMessage(fmt.Sprintf(format, args...))
}
