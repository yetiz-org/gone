package outer

import "github.com/yetiz-org/gone/goai/internal/testfixtures/generic/page"

type Envelope[T any] struct {
	Page page.Page[T] `json:"page"`
}
