package concurrent

import (
	"math"
	"sync"
	"sync/atomic"
	"time"
)

type Future interface {
	Get() interface{}
	GetWithTimeout(duration time.Duration) interface{}
	Await() Future
	IsDone() bool
}

type DefaultFuture struct {
	lock   sync.Mutex
	state  int32
	sign   chan int
	result interface{}
}

func (d *DefaultFuture) IsDone() bool {
	return atomic.LoadInt32(&d.state) == 1
}

func (d *DefaultFuture) Get() interface{} {
	return d.GetWithTimeout(math.MaxInt64)
}

func (d *DefaultFuture) GetWithTimeout(duration time.Duration) interface{} {
	if atomic.LoadInt32(&d.state) == 1 {
		return d.result
	}

	for {
		select {
		case <-d.sign:
			return d.result
		case <-time.After(duration):
			if atomic.LoadInt32(&d.state) == 1 {
				return d.result
			} else {
				return nil
			}
		}
	}
}

func (d *DefaultFuture) Await() Future {
	d.Get()
	return d
}

func NewFuture(f func() interface{}) Future {
	future := new(DefaultFuture)
	future.sign = make(chan int, 1)
	future.do(f)
	return future
}

func (d *DefaultFuture) do(f func() interface{}) Future {
	d.lock.Lock()
	go func() {
		defer d.lock.Unlock()
		d.result = f()
		atomic.StoreInt32(&d.state, 1)
		d.sign <- 1
	}()

	return d
}
