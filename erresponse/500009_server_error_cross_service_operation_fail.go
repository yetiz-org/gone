package erresponse

import (
	"fmt"

	"github.com/yetiz-org/gone/ghttp/httpstatus"
	kkerror "github.com/yetiz-org/goth-kkerror"

	"github.com/yetiz-org/gone/erresponse/constant"
)

var ServerErrorCrossServiceOperationFail = Collection.Register(&DefaultErrorResponse{
	StatusCode:  httpstatus.InternalServerError,
	Name:        constant.ErrorServerError,
	Description: "cross service operation fail",
	DefaultKKError: kkerror.DefaultKKError{
		ErrorLevel:    kkerror.Critical,
		ErrorCategory: kkerror.Internal,
		ErrorCode:     "500009",
	},
})

func ServerErrorCrossServiceOperationWithMessage(message string) ErrorResponse {
	return &DefaultErrorResponse{
		StatusCode:  httpstatus.InternalServerError,
		Name:        constant.ErrorServerError,
		Description: "cross service operation fail",
		DefaultKKError: kkerror.DefaultKKError{
			ErrorLevel:    kkerror.Critical,
			ErrorCategory: kkerror.Internal,
			ErrorCode:     "500009",
			ErrorMessage:  message,
		},
	}
}

// ServerErrorCrossServiceOperationWithFormat provides backward compatibility for dynamic format strings
func ServerErrorCrossServiceOperationWithFormat(format string, args ...interface{}) ErrorResponse {
	return ServerErrorCrossServiceOperationWithMessage(fmt.Sprintf(format, args...))
}
