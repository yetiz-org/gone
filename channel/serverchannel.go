package channel

type ServerChannel interface {
	Channel
	setChildHandler(handler Handler) ServerChannel
	setChildParams(key ParamKey, value interface{})
	ChildParams() *Params
}

type DefaultServerChannel struct {
	DefaultChannel
	childHandler Handler
	childParams  Params
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

func (d *DefaultServerChannel) setChildHandler(handler Handler) ServerChannel {
	d.childHandler = handler
	return d
}

func (d *DefaultServerChannel) setChildParams(key ParamKey, value interface{}) {
	d.childParams.Store(key, value)
}

func (d *DefaultServerChannel) ChildParams() *Params {
	return &d.childParams
}
