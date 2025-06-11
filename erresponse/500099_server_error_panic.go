package erresponse

import (
	"fmt"
	kkerror "github.com/yetiz-org/goth-kkerror"

	"github.com/yetiz-org/gone/erresponse/constant"
	"github.com/yetiz-org/gone/http/httpstatus"
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

func ServerErrorPanicWithMessage(format string, params ...interface{}) ErrorResponse {
	return &DefaultErrorResponse{
		StatusCode:  httpstatus.InternalServerError,
		Name:        constant.ErrorServerError,
		Description: "panic",
		DefaultKKError: kkerror.DefaultKKError{
			ErrorLevel:    kkerror.Urgent,
			ErrorCategory: kkerror.Server,
			ErrorCode:     "500099",
			ErrorMessage:  fmt.Sprintf(format, params...),
		},
	}
}
