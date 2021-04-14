package channel

import "github.com/kklab-com/gone/concurrent"

type Future interface {
	concurrent.Future
	Sync() Future
	Channel() Channel
}

type DefaultFuture struct {
	concurrent.Future
	channel Channel
}

func NewChannelFuture(channel Channel, f func(f concurrent.Future) interface{}) Future {
	future := DefaultFuture{}
	future.channel = channel
	future.Future = concurrent.NewFuture(f, nil)
	return &future
}

func (d *DefaultFuture) Sync() Future {
	d.Future.Get()
	return d
}

func (d *DefaultFuture) Channel() Channel {
	return d.channel
}
