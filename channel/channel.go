package channel

import (
	"net"
	"sync"

	"github.com/google/uuid"
	"github.com/kklab-com/gone/concurrent"
	"github.com/kklab-com/goth-base62"
	"github.com/pkg/errors"
)

type Channel interface {
	ID() string
	Init() Channel
	Pipeline() Pipeline
	CloseFuture() Future
	Bind(localAddr net.Addr) Channel
	Close() Channel
	Connect(remoteAddr net.Addr) Channel
	Disconnect() Channel
	Read() Channel
	FireRead(obj interface{}) Channel
	FireReadCompleted() Channel
	Write(obj interface{}) Channel
	IsActive() bool
	SetParam(key ParamKey, value interface{})
	Param(key ParamKey) interface{}
	Params() *Params
	unsafe() *Unsafe
}

var ErrNotActive = errors.Errorf("channel not active")
var IDEncoder = base62.ShiftEncoding

type DefaultChannel struct {
	id              string
	Name            string
	ChannelPipeline Pipeline
	atomicLock      sync.Mutex
	params          Params
	Unsafe          Unsafe
}

type Unsafe struct {
	WriteFunc      func(obj interface{}) error
	BindFunc       func(localAddr net.Addr) error
	CloseFunc      func() error
	ConnectFunc    func(remoteAddr net.Addr) error
	DisconnectFunc func() error
	CloseLock      sync.Mutex
	DisconnectLock sync.Mutex
}

var UnsafeDefaultWriteFunc = func(obj interface{}) error { return nil }
var UnsafeDefaultBindFunc = func(localAddr net.Addr) error { return nil }
var UnsafeDefaultConnectFunc = func(remoteAddr net.Addr) error { return nil }

func (c *DefaultChannel) SetParam(key ParamKey, value interface{}) {
	c.params.Store(key, value)
}

func (c *DefaultChannel) Param(key ParamKey) interface{} {
	if v, f := c.params.Load(key); f {
		return v
	}

	return nil
}

func (c *DefaultChannel) Params() *Params {
	return &c.params
}

func (c *DefaultChannel) ID() string {
	if c.id == "" {
		c.atomicLock.Lock()
		defer c.atomicLock.Unlock()
		if c.id == "" {
			u := uuid.New()
			c.id = IDEncoder.EncodeToString(u[:])
		}
	}

	return c.id
}

func EmptyDefaultChannel() *DefaultChannel {
	u := uuid.New()
	var channel = DefaultChannel{
		id: IDEncoder.EncodeToString(u[:]),
		Unsafe: Unsafe{
			WriteFunc:   UnsafeDefaultWriteFunc,
			BindFunc:    UnsafeDefaultBindFunc,
			ConnectFunc: UnsafeDefaultConnectFunc,
		},
	}

	return &channel
}

func NewDefaultChannel() *DefaultChannel {
	channel := EmptyDefaultChannel()
	channel.Init()
	return channel
}

func (c *DefaultChannel) Init() Channel {
	c.ChannelPipeline = NewDefaultPipeline(c)
	return c
}

func (c *DefaultChannel) Pipeline() Pipeline {
	if c.ChannelPipeline == nil {
		c.ChannelPipeline = NewDefaultPipeline(c)
	}

	return c.ChannelPipeline
}

func (c *DefaultChannel) CloseFuture() Future {
	return NewChannelFuture(c, func(f concurrent.Future) interface{} {
		c.Unsafe.CloseLock.Lock()
		c.Unsafe.CloseLock.Unlock()
		return nil
	})
}

func (c *DefaultChannel) DisconnectFuture() Future {
	return NewChannelFuture(c, func(f concurrent.Future) interface{} {
		c.Unsafe.DisconnectLock.Lock()
		c.Unsafe.DisconnectLock.Unlock()
		return nil
	})
}

func (c *DefaultChannel) Bind(localAddr net.Addr) Channel {
	c.Pipeline().Bind(localAddr)
	return c
}

func (c *DefaultChannel) Close() Channel {
	c.Pipeline().Close()
	return c
}

func (c *DefaultChannel) Connect(remoteAddr net.Addr) Channel {
	c.Pipeline().Connect(remoteAddr)
	return c
}

func (c *DefaultChannel) Disconnect() Channel {
	c.Pipeline().Disconnect()
	return c
}

func (c *DefaultChannel) PreStart() Channel {
	panic("implement me")
}

func (c *DefaultChannel) Read() Channel {
	return c
}

func (c *DefaultChannel) FireRead(obj interface{}) Channel {
	c.Pipeline().fireRead(obj)
	return c
}

func (c *DefaultChannel) FireReadCompleted() Channel {
	c.Pipeline().fireReadCompleted()
	return c
}

func (c *DefaultChannel) Write(obj interface{}) Channel {
	c.Pipeline().Write(obj)
	return c
}

func (c *DefaultChannel) IsActive() bool {
	panic("implement me")
}

func (c *DefaultChannel) unsafe() *Unsafe {
	return &c.Unsafe
}
