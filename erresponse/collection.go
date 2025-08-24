package erresponse

import (
	"sync"
)

var Collection = &Collect{}

type Collect struct {
	once           sync.Once
	mu             sync.RWMutex
	ErrorResponses map[ErrorResponse]ErrorResponse
}

func (c *Collect) Register(err ErrorResponse) ErrorResponse {
	c.once.Do(func() {
		c.ErrorResponses = map[ErrorResponse]ErrorResponse{}
	})

	c.mu.Lock()
	c.ErrorResponses[err] = err
	c.mu.Unlock()
	return err
}
