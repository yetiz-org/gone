package erresponse

import (
	"sync"
)

var Collection = &Collect{}

type Collect struct {
	once           sync.Once
	ErrorResponses map[ErrorResponse]ErrorResponse
}

func (c *Collect) Register(err ErrorResponse) ErrorResponse {
	c.once.Do(func() {
		c.ErrorResponses = map[ErrorResponse]ErrorResponse{}
	})

	c.ErrorResponses[err] = err
	return err
}
