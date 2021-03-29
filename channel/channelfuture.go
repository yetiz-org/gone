package channel

import "github.com/kklab-com/gone/concurrent"

type Future interface {
	Sync() Future
	Channel() Channel
}

type DefaultFuture struct {
	channel Channel
	future  concurrent.Future
}

func NewChannelFuture(channel Channel, f func() interface{}) Future {
	future := DefaultFuture{}
	future.channel = channel
	future.future = concurrent.NewFuture(f)
	return &future
}

func (d *DefaultFuture) Sync() Future {
	d.future.Await()
	return d
}

func (d *DefaultFuture) Channel() Channel {
	return d.channel
}
