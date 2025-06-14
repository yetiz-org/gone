package erresponse

import (
	"fmt"
	"github.com/yetiz-org/gone/ghttp/httpstatus"
	kkerror "github.com/yetiz-org/goth-kkerror"

	"github.com/yetiz-org/gone/erresponse/constant"
)

var ServerErrorEmailOperationFail = Collection.Register(&DefaultErrorResponse{
	StatusCode:  httpstatus.InternalServerError,
	Name:        constant.ErrorServerError,
	Description: "email operation fail",
	DefaultKKError: kkerror.DefaultKKError{
		ErrorLevel:    kkerror.Urgent,
		ErrorCategory: kkerror.Internal,
		ErrorCode:     "500007",
	},
})

func ServerErrorEmailOperationWithMessage(format string, params ...interface{}) ErrorResponse {
	return &DefaultErrorResponse{
		StatusCode:  httpstatus.InternalServerError,
		Name:        constant.ErrorServerError,
		Description: "email operation fail",
		DefaultKKError: kkerror.DefaultKKError{
			ErrorLevel:    kkerror.Urgent,
			ErrorCategory: kkerror.Internal,
			ErrorCode:     "500007",
			ErrorMessage:  fmt.Sprintf(format, params...),
		},
	}
}
