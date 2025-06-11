package erresponse

import (
	"fmt"
	"sort"
	"testing"
)

func Test_Collect(t *testing.T) {
	var ers []ErrorResponse
	for er := range Collection.ErrorResponses {
		ers = append(ers, er)
	}

	sort.Slice(ers, func(i, j int) bool {
		return ers[i].(*DefaultErrorResponse).ErrorCode < ers[j].(*DefaultErrorResponse).ErrorCode
	})

	println("|error code|sample|")
	println("|---|---|")
	for _, er := range ers {
		println(fmt.Sprintf("|%s|%s|", er.Code(), er.Error()))
	}
}
