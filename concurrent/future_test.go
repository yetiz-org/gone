package concurrent

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type testListener struct {
	done   bool
	cancel bool
}

func (t *testListener) OperationCompleted(f Future) {
	t.done = true
	t.cancel = f.IsCancelled()
}

func TestDefaultFuture_Get(t *testing.T) {
	before := time.Now()
	fu := NewFuncFuture(func(f Future) interface{} {
		time.Sleep(time.Millisecond * 500)
		return 1
	}, nil)

	listener := &testListener{}
	fu.AddListener(listener)
	assert.False(t, listener.done)
	assert.False(t, fu.IsDone())
	assert.False(t, fu.IsCancelled())
	assert.False(t, fu.IsSuccess())
	for i := 0; i < 4; i++ {
		go assert.EqualValues(t, 1, fu.Get())
	}

	assert.EqualValues(t, 1, fu.Get())
	assert.True(t, listener.done)
	assert.Greater(t, time.Now().Sub(before).Nanoseconds(), (time.Millisecond * 500).Nanoseconds())
	assert.True(t, fu.IsDone())
	assert.True(t, fu.IsSuccess())
	assert.False(t, fu.IsCancelled())
	assert.NoError(t, fu.Error())
}

func TestDefaultFuture_GetCancel(t *testing.T) {
	before := time.Now()
	listener := &testListener{}
	fu := NewFuncFuture(func(f Future) interface{} {
		time.Sleep(time.Millisecond * 500)
		return 1
	}, nil)

	fu.AddListener(listener)
	assert.False(t, listener.done)
	assert.False(t, fu.IsDone())
	assert.False(t, fu.IsCancelled())
	assert.False(t, fu.IsSuccess())
	fu.(ManualFuture).Cancel()
	assert.Nil(t, fu.Get())
	assert.Less(t, time.Now().Sub(before).Nanoseconds(), (time.Millisecond * 500).Nanoseconds())
	assert.True(t, fu.IsDone())
	assert.False(t, fu.IsSuccess())
	assert.True(t, fu.IsCancelled())
	assert.True(t, listener.cancel)
	assert.Error(t, fu.Error())
}
