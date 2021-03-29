package channel

type ServerChannel interface {
	Channel
	SetChildHandler(handler Handler) ServerChannel
	ChildHandler() Handler
}

type DefaultServerChannel struct {
	DefaultChannel
	childHandler Handler
}

func (d *DefaultServerChannel) Init() Channel {
	d.ChannelPipeline = NewDefaultPipeline(d)
	d.Unsafe.CloseFunc = func() error {
		d.Unsafe.CloseLock.Unlock()
		return nil
	}

	d.Unsafe.CloseLock.Lock()
	return d
}

func (d *DefaultServerChannel) SetChildHandler(handler Handler) ServerChannel {
	d.childHandler = handler
	return d
}

func (d *DefaultServerChannel) ChildHandler() Handler {
	return d.childHandler
}
