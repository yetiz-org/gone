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
	future := DefaultFuture{}
	future.channel = channel
	future.Future = concurrent.NewFuture(nil)
	return &future
}

func (d *DefaultFuture) Sync() Future {
	d.Future.Get()
	return d
}

func (d *DefaultFuture) Get() interface{} {
	return d.Sync()._channel()
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
