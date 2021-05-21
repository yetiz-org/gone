package channel

import (
	"github.com/kklab-com/gone/concurrent"
)

type Future interface {
	concurrent.Future
	Sync() Future
	Channel() Channel
	_channel() Channel
}

type DefaultFuture struct {
	concurrent.Future
	channel Channel
}

func NewFuture(channel Channel) Future {
	future := &DefaultFuture{}
	future.channel = channel
	future.Future = concurrent.NewFuture(nil)
	return future
}

func (d *DefaultFuture) Get() interface{} {
	return d.Sync()._channel()
}

func (d *DefaultFuture) IsDone() bool {
	return d.Future.IsDone()
}

func (d *DefaultFuture) IsSuccess() bool {
	return d.Future.IsSuccess()
}

func (d *DefaultFuture) IsCancelled() bool {
	return d.Future.IsCancelled()
}

func (d *DefaultFuture) Error() error {
	return d.Future.Error()
}

func (d *DefaultFuture) AddListener(listener concurrent.FutureListener) concurrent.Future {
	return d.Future.AddListener(listener)
}

func (d *DefaultFuture) AddListeners(listener ...concurrent.FutureListener) concurrent.Future {
	return d.Future.AddListeners(listener...)
}

func (d *DefaultFuture) Success() {
	d.Future.(concurrent.ManualFuture).Success()
}

func (d *DefaultFuture) Cancel() {
	d.Future.(concurrent.ManualFuture).Cancel()
}

func (d *DefaultFuture) Sync() Future {
	d.Future.Await()
	return d
}

func (d *DefaultFuture) Channel() Channel {
	if !d.IsDone() {
		return nil
	} else {
		if d.IsSuccess() {
			return d._channel()
		}
	}

	return nil
}

func (d *DefaultFuture) _channel() Channel {
	return d.channel
}
