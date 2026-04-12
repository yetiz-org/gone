package erresponse

import (
	"cmp"
	"fmt"
	"slices"
	"testing"
)

func Test_Collect(t *testing.T) {
	var ers []ErrorResponse
	for er := range Collection.ErrorResponses {
		ers = append(ers, er)
	}

	slices.SortFunc(ers, func(a, b ErrorResponse) int {
		return cmp.Compare(a.(*DefaultErrorResponse).ErrorCode, b.(*DefaultErrorResponse).ErrorCode)
	})

	println("|error code|sample|")
	println("|---|---|")
	for _, er := range ers {
		println(fmt.Sprintf("|%s|%s|", er.Code(), er.Error()))
	}
}
