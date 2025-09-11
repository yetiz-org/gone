package erresponse

import (
	"fmt"

	"github.com/yetiz-org/gone/ghttp/httpstatus"
	kkerror "github.com/yetiz-org/goth-kkerror"

	"github.com/yetiz-org/gone/erresponse/constant"
)

var ServerErrorCacheOperationFail = Collection.Register(&DefaultErrorResponse{
	StatusCode:  httpstatus.InternalServerError,
	Name:        constant.ErrorServerError,
	Description: "cache operation fail",
	DefaultKKError: kkerror.DefaultKKError{
		ErrorLevel:    kkerror.Critical,
		ErrorCategory: kkerror.Cache,
		ErrorCode:     "500006",
	},
})

func ServerErrorCacheOperationWithMessage(message string) ErrorResponse {
	return &DefaultErrorResponse{
		StatusCode:  httpstatus.InternalServerError,
		Name:        constant.ErrorServerError,
		Description: "cache operation fail",
		DefaultKKError: kkerror.DefaultKKError{
			ErrorLevel:    kkerror.Critical,
			ErrorCategory: kkerror.Cache,
			ErrorCode:     "500006",
			ErrorMessage:  message,
		},
	}
}

// ServerErrorCacheOperationWithFormat provides backward compatibility for dynamic format strings
func ServerErrorCacheOperationWithFormat(format string, args ...interface{}) ErrorResponse {
	return ServerErrorCacheOperationWithMessage(fmt.Sprintf(format, args...))
}
