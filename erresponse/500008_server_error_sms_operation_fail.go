package erresponse

import (
	"fmt"
	kkerror "github.com/yetiz-org/goth-kkerror"

	"github.com/yetiz-org/gone/erresponse/constant"
	"github.com/yetiz-org/gone/http/httpstatus"
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

func ServerErrorSMSOperationWithMessage(format string, params ...interface{}) ErrorResponse {
	return &DefaultErrorResponse{
		StatusCode:  httpstatus.InternalServerError,
		Name:        constant.ErrorServerError,
		Description: "sms operation fail",
		DefaultKKError: kkerror.DefaultKKError{
			ErrorLevel:    kkerror.Critical,
			ErrorCategory: kkerror.Internal,
			ErrorCode:     "500008",
			ErrorMessage:  fmt.Sprintf(format, params...),
		},
	}
}
