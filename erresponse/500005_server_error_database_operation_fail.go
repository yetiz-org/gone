package erresponse

import (
	"fmt"

	"github.com/yetiz-org/gone/ghttp/httpstatus"
	kkerror "github.com/yetiz-org/goth-kkerror"

	"github.com/yetiz-org/gone/erresponse/constant"
)

var ServerErrorDatabaseOperationFail = Collection.Register(&DefaultErrorResponse{
	StatusCode:  httpstatus.InternalServerError,
	Name:        constant.ErrorServerError,
	Description: "database operation fail",
	DefaultKKError: kkerror.DefaultKKError{
		ErrorLevel:    kkerror.Critical,
		ErrorCategory: kkerror.Database,
		ErrorCode:     "500005",
	},
})

func ServerErrorDatabaseOperationWithMessage(message string) ErrorResponse {
	return &DefaultErrorResponse{
		StatusCode:  httpstatus.InternalServerError,
		Name:        constant.ErrorServerError,
		Description: "database operation fail",
		DefaultKKError: kkerror.DefaultKKError{
			ErrorLevel:    kkerror.Critical,
			ErrorCategory: kkerror.Database,
			ErrorCode:     "500005",
			ErrorMessage:  message,
		},
	}
}

// ServerErrorDatabaseOperationWithFormat provides backward compatibility for dynamic format strings
func ServerErrorDatabaseOperationWithFormat(format string, args ...interface{}) ErrorResponse {
	return ServerErrorDatabaseOperationWithMessage(fmt.Sprintf(format, args...))
}
