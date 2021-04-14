package channel

type ClientChannel interface {
	Channel
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
