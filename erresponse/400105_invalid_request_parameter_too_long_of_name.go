package erresponse

import (
	"fmt"
	"github.com/yetiz-org/gone/ghttp/httpstatus"
	kkerror "github.com/yetiz-org/goth-kkerror"

	"github.com/yetiz-org/gone/erresponse/constant"
)

var _InvalidRequestParameterTooLongOfName = Collection.Register(&DefaultErrorResponse{
	StatusCode:  httpstatus.BadRequest,
	Name:        constant.ErrorInvalidRequest,
	Description: "parameter too long",
	DefaultKKError: kkerror.DefaultKKError{
		ErrorLevel:    kkerror.Normal,
		ErrorCategory: kkerror.Client,
		ErrorCode:     "400105",
		ErrorMessage:  fmt.Sprintf("%s is too long", "[name]"),
	},
})

func InvalidRequestParameterTooLongOfName(name string) ErrorResponse {
	return &DefaultErrorResponse{
		StatusCode:  httpstatus.BadRequest,
		Name:        constant.ErrorInvalidRequest,
		Description: "parameter too long",
		DefaultKKError: kkerror.DefaultKKError{
			ErrorLevel:    kkerror.Normal,
			ErrorCategory: kkerror.Client,
			ErrorCode:     "400105",
			ErrorMessage:  fmt.Sprintf("%s is too long", name),
		},
	}
}
