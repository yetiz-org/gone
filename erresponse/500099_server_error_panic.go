package erresponse

import (
	"fmt"

	"github.com/yetiz-org/gone/ghttp/httpstatus"
	kkerror "github.com/yetiz-org/goth-kkerror"

	"github.com/yetiz-org/gone/erresponse/constant"
)

var ServerErrorPanic = Collection.Register(&DefaultErrorResponse{
	StatusCode:  httpstatus.InternalServerError,
	Name:        constant.ErrorServerError,
	Description: "panic",
	DefaultKKError: kkerror.DefaultKKError{
		ErrorLevel:    kkerror.Urgent,
		ErrorCategory: kkerror.Server,
		ErrorCode:     "500099",
	},
})

func ServerErrorPanicWithMessage(message string) ErrorResponse {
	return &DefaultErrorResponse{
		StatusCode:  httpstatus.InternalServerError,
		Name:        constant.ErrorServerError,
		Description: "panic",
		DefaultKKError: kkerror.DefaultKKError{
			ErrorLevel:    kkerror.Urgent,
			ErrorCategory: kkerror.Server,
			ErrorCode:     "500099",
			ErrorMessage:  message,
		},
	}
}

// ServerErrorPanicWithFormat provides backward compatibility for dynamic format strings
func ServerErrorPanicWithFormat(format string, args ...interface{}) ErrorResponse {
	return ServerErrorPanicWithMessage(fmt.Sprintf(format, args...))
}
