package channel

import (
	"context"
	"fmt"
	"net"
	"sync"

	"github.com/google/uuid"
	"github.com/kklab-com/goth-base62"
	"github.com/pkg/errors"
)

type Channel interface {
	ID() string
	Init() Channel
	Pipeline() Pipeline
	CloseFuture() Future
	Bind(localAddr net.Addr) Future
	Close() Future
	Connect(localAddr net.Addr, remoteAddr net.Addr) Future
	Disconnect() Future
	Deregister() Future
	Read() Channel
	FireRead(obj interface{}) Channel
	FireReadCompleted() Channel
	Write(obj interface{}) Future
	IsActive() bool
	SetParam(key ParamKey, value interface{})
	Param(key ParamKey) interface{}
	Params() *Params
	Parent() ServerChannel
	LocalAddr() net.Addr
	setLocalAddr(addr net.Addr)
	setActive()
	setInactive()
}

type UnsafeRead interface {
	UnsafeRead() error
}

type UnsafeBind interface {
	UnsafeBind(localAddr net.Addr) error
}

type UnsafeAccept interface {
	UnsafeAccept() error
}

type UnsafeClose interface {
	UnsafeClose() error
}

type UnsafeWrite interface {
	UnsafeWrite(obj interface{}) error
}

type UnsafeConnect interface {
	UnsafeConnect(localAddr net.Addr, remoteAddr net.Addr) error
}

type UnsafeDisconnect interface {
	UnsafeDisconnect() error
}

var ErrNotActive = errors.Errorf("channel not active")
var ErrNilObject = fmt.Errorf("nil object")
var ErrUnknownObjectType = fmt.Errorf("unknown object type")
var ErrReadError = fmt.Errorf("read error")

var IDEncoder = base62.ShiftEncoding

type DefaultChannel struct {
	id          string
	Name        string
	pipeline    Pipeline
	atomicLock  sync.Mutex
	parent      ServerChannel
	parentCtx   context.Context
	params      Params
	localAddr   net.Addr
	closeFuture Future
	active      bool
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

func (c *DefaultChannel) Init() Channel {
	return c
}

func (c *DefaultChannel) Pipeline() Pipeline {
	return c.pipeline
}

func (c *DefaultChannel) CloseFuture() Future {
	return c.closeFuture
}

func (c *DefaultChannel) Bind(localAddr net.Addr) Future {
	return c.Pipeline().Bind(localAddr)
}

func (c *DefaultChannel) Close() Future {
	return c.Pipeline().Close()
}

func (c *DefaultChannel) Connect(localAddr net.Addr, remoteAddr net.Addr) Future {
	return c.Pipeline().Connect(localAddr, remoteAddr)
}

func (c *DefaultChannel) Disconnect() Future {
	return c.Pipeline().Disconnect()
}

func (c *DefaultChannel) Deregister() Future {
	return c.Pipeline().Deregister()
}

func (c *DefaultChannel) Read() Channel {
	c.Pipeline().Read()
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

func (c *DefaultChannel) Write(obj interface{}) Future {
	return c.Pipeline().Write(obj)
}

func (c *DefaultChannel) IsActive() bool {
	return c.active
}

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

func (c *DefaultChannel) Parent() ServerChannel {
	return c.parent
}

func (c *DefaultChannel) LocalAddr() net.Addr {
	return c.localAddr
}

func (c *DefaultChannel) setLocalAddr(addr net.Addr) {
	c.localAddr = addr
}

func (c *DefaultChannel) setActive() {
	c.active = true
}

func (c *DefaultChannel) setInactive() {
	c.active = false
}

func (c *DefaultChannel) UnsafeWrite(obj interface{}) error {
	return nil
}

func (c *DefaultChannel) UnsafeRead() error {
	return nil
}

func (c *DefaultChannel) UnsafeDisconnect() error {
	return nil
}

func (c *DefaultChannel) UnsafeConnect(localAddr net.Addr, remoteAddr net.Addr) error {
	return nil
}
