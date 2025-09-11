package erresponse

import (
	"fmt"

	"github.com/yetiz-org/gone/ghttp/httpstatus"
	kkerror "github.com/yetiz-org/goth-kkerror"

	"github.com/yetiz-org/gone/erresponse/constant"
)

var _NotFoundWithMessage = Collection.Register(&DefaultErrorResponse{
	StatusCode:  httpstatus.BadRequest,
	Name:        constant.ErrorNotFound,
	Description: "",
	DefaultKKError: kkerror.DefaultKKError{
		ErrorLevel:    kkerror.Normal,
		ErrorCategory: kkerror.Client,
		ErrorCode:     "404101",
	},
})

func NotFoundWithMessage(message string) ErrorResponse {
	return &DefaultErrorResponse{
		StatusCode:  httpstatus.BadRequest,
		Name:        constant.ErrorNotFound,
		Description: "",
		DefaultKKError: kkerror.DefaultKKError{
			ErrorLevel:    kkerror.Normal,
			ErrorCategory: kkerror.Client,
			ErrorCode:     "404101",
			ErrorMessage:  message,
		},
	}
}

// NotFoundWithFormat provides backward compatibility for dynamic format strings
func NotFoundWithFormat(format string, args ...interface{}) ErrorResponse {
	return NotFoundWithMessage(fmt.Sprintf(format, args...))
}
