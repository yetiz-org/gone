package channel

type ClientChannel interface {
	Channel
	Write(obj interface{}) ClientChannel
}

type DefaultClientChannel struct {
	DefaultChannel
}

func NewDefaultClientChannel() *DefaultClientChannel {
	var channel = &DefaultClientChannel{
		DefaultChannel: *NewDefaultChannel(),
	}

	channel.Unsafe.DisconnectFunc = func() error {
		channel.Unsafe.DisconnectLock.Unlock()
		return nil
	}

	channel.Unsafe.DisconnectLock.Lock()
	return channel
}

func (c *DefaultClientChannel) Init() Channel {
	c.ChannelPipeline = NewDefaultPipeline(c)
	return c
}

func (c *DefaultClientChannel) Write(obj interface{}) ClientChannel {
	c.Pipeline().Write(obj)
	return c
}
