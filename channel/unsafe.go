package channel

import (
	"fmt"
	"net"
	"sync/atomic"
	"time"

	concurrent "github.com/yetiz-org/goth-concurrent"
	kklogger "github.com/yetiz-org/goth-kklogger"
)

const DefaultAcceptTimeout = 5000

var ErrLocalAddrIsEmpty = fmt.Errorf("local addr is empty")
var ErrRemoteAddrIsEmpty = fmt.Errorf("remote addr is empty")
var ErrChannelNotActive = fmt.Errorf("channel not active")
var ErrChannelClosed = fmt.Errorf("channel closed")
var ErrAcceptTimeout = fmt.Errorf("accept timeout")
var ErrUnsupportedOperation = fmt.Errorf("unsupported operation")
var ErrOperationInProgress = fmt.Errorf("operation in progress")

type Unsafe interface {
	Read()
	Write(obj any, future Future)
	Bind(localAddr net.Addr, future Future)
	Close(future Future)
	Connect(localAddr net.Addr, remoteAddr net.Addr, future Future)
	Disconnect(future Future)
}

type DefaultUnsafe struct {
	channel Channel
	readS,
	writeS,
	bindS,
	closeS,
	connectS,
	disconnectS int32
	writeBuffer    concurrent.Queue
	writeBufferLen atomic.Int64 // Push/Pop mirror — safe cross-goroutine length check
}

func NewUnsafe(channel Channel) Unsafe {
	return &DefaultUnsafe{channel: channel}
}

func (u *DefaultUnsafe) Read() {
	if uf, ok := u.channel.(UnsafeRead); ok && u.markState(&u.readS) && u.channel.IsActive() {
		go func(u *DefaultUnsafe, uf UnsafeRead) {
			defer u.resetState(&u.readS)
			lastObjRead := false
			for {
				// Check if channel is still active before processing
				if !u.channel.IsActive() {
					break
				}

				if obj, err := uf.UnsafeRead(); err != nil {
					if err == ErrSkip {
						if u.channel.IsActive() && lastObjRead {
							lastObjRead = false
							u.channel.FireReadCompleted()
						}
					} else {
						// Call inactiveChannel synchronously - cleanup is now synchronous but returns
						// a completed future to avoid complex chaining deadlocks
						u.channel.inactiveChannel()
						break
					}
				} else {
					if obj != nil {
						u.channel.FireRead(obj)
						lastObjRead = true
					}
				}

				if !uf.UnsafeIsAutoRead() {
					break
				}
			}
		}(u, uf)
	}
}

func (u *DefaultUnsafe) Write(obj any, future Future) {
	if future == nil {
		future = u.channel.Pipeline().NewFuture()
	}

	if obj != nil && u.channel.IsActive() {
		future.(concurrent.Settable).Set(obj)
		u.writeBuffer.Push(future)
		u.writeBufferLen.Add(1)
	} else {
		if obj == nil {
			u.futureSuccess(future)
		} else if !u.channel.IsActive() {
			u.futureFail(future, ErrChannelNotActive)
			return
		}
	}

	if uf, ok := u.channel.(UnsafeWrite); ok && u.markState(&u.writeS) {
		go func(u *DefaultUnsafe, uf UnsafeWrite) {
			for u.channel.IsActive() {
				var future Future
				if v := u.writeBuffer.Pop(); v != nil {
					future = v.(Future)
					u.writeBufferLen.Add(-1)
				}

				if future == nil {
					// pending close
					break
				}

				if err := uf.UnsafeWrite(future.GetNow()); err != nil {
					u.channel.inactiveChannel()
					u.futureFail(future, err)
				} else {
					u.futureSuccess(future)
				}

				continue
			}

			if !u.channel.IsActive() {
				for v := u.writeBuffer.Pop(); v != nil; v = u.writeBuffer.Pop() {
					u.writeBufferLen.Add(-1)
					future := v.(Future)
					if u.channel.CloseFuture().IsDone() {
						u.futureFail(future, ErrChannelClosed)
					} else {
						u.futureFail(future, ErrChannelNotActive)
					}
				}
			}

			u.resetState(&u.writeS)
			if u.writeBufferLen.Load() > 0 {
				u.Write(nil, nil)
			}
		}(u, uf)
	}
}

func (u *DefaultUnsafe) Bind(localAddr net.Addr, future Future) {
	future = u.ensureFuture(future)

	if localAddr == nil {
		kklogger.WarnJ("channel:DefaultUnsafe.Bind#bind!nil_addr", "localAddr is nil")
		u.futureFail(future, ErrLocalAddrIsEmpty)
		return
	}

	_, hasUnsafeBind := u.channel.(UnsafeBind)
	if !hasUnsafeBind {
		u.futureFail(future, ErrUnsupportedOperation)
		return
	}

	if !u.acquireOperation(&u.bindS, future) {
		return
	}

	go func(u *DefaultUnsafe, localAddr net.Addr, future Future) {
		defer u.resetState(&u.bindS)
		if err := u.channel.(UnsafeBind).UnsafeBind(localAddr); err != nil {
			kklogger.ErrorJ("channel:DefaultUnsafe.Bind#bind!bind_error", fmt.Sprintf("channel_id: %s, error: %s", u.channel.ID(), err.Error()))
			u.channel.inactiveChannel()
			future.(*DefaultFuture).channel = nil
			u.futureFail(future, err)
		} else {
			u.channel.activeChannel()
			if channel, ok := u.channel.(UnsafeAccept); ok {
				go func() {
					for u.channel.IsActive() {
						if child, future := channel.UnsafeAccept(); child == nil {
							if u.channel.IsActive() {
								kklogger.WarnJ("channel:DefaultUnsafe.UnsafeAccept#accept!nil_child", "nil child")
							}

							u.futureCancel(future)
						} else {
							go func(u *DefaultUnsafe, child Channel, future Future) {
								child.Pipeline().fireRegistered()
								child.activeChannel()
								u.futureSuccess(future)
							}(u, child, future)

							go func(u *DefaultUnsafe, child Channel, future Future) {
								<-time.After(time.Duration(GetParamIntDefault(child, ParamAcceptTimeout, DefaultAcceptTimeout)) * time.Millisecond)
								if u.futureFail(future, ErrAcceptTimeout) {
									kklogger.ErrorJ("channel:DefaultUnsafe.UnsafeAccept#accept!accept_error", future.Error().Error())
									child.inactiveChannel()
								}
							}(u, child, future)
						}
					}
				}()
			}

			u.futureSuccess(future)
		}
	}(u, localAddr, future)
}

func (u *DefaultUnsafe) Close(future Future) {
	future = u.ensureFuture(future)

	channel, ok := u.channel.(UnsafeClose)
	if !ok {
		u.futureFail(future, ErrUnsupportedOperation)
		return
	}

	if u.channel.CloseFuture().IsDone() {
		u.futureSuccess(future)
		return
	}

	if !u.markState(&u.closeS) {
		go func() {
			u.channel.CloseFuture().Await()
			u.futureSuccess(future)
		}()
		return
	}

	go func(u *DefaultUnsafe, future Future) {
		defer u.resetState(&u.closeS)
		func() concurrent.Future { _, f := u.channel.inactiveChannel(); return f }().Await()
		err := channel.UnsafeClose()
		if err != nil {
			kklogger.WarnJ("channel:DefaultUnsafe.Close#close!close_error", fmt.Sprintf("channel_id: %s, error: %s", u.channel.ID(), err.Error()))
		}

		u.futureSuccess(u.channel.CloseFuture())
		u.futureSuccess(future)
	}(u, future)
}

func (u *DefaultUnsafe) Connect(localAddr net.Addr, remoteAddr net.Addr, future Future) {
	future = u.ensureFuture(future)

	if remoteAddr == nil {
		u.futureFail(future, ErrRemoteAddrIsEmpty)
		return
	}

	channel, ok := u.channel.(UnsafeConnect)
	if !ok {
		u.futureFail(future, ErrUnsupportedOperation)
		return
	}

	if !u.acquireOperation(&u.connectS, future) {
		return
	}

	go func(u *DefaultUnsafe, localAddr net.Addr, remoteAddr net.Addr, future Future) {
		defer u.resetState(&u.connectS)
		if err := channel.UnsafeConnect(localAddr, remoteAddr); err != nil {
			kklogger.WarnJ("channel:DefaultUnsafe.Connect#connect!connect_error", fmt.Sprintf("channel_id: %s, error: %s", u.channel.ID(), err.Error()))
			u.channel.inactiveChannel()
			future.(*DefaultFuture).channel = nil
			u.futureFail(future, err)
		} else {
			u.channel.activeChannel()
			u.futureSuccess(future)
		}
	}(u, localAddr, remoteAddr, future)
}

func (u *DefaultUnsafe) Disconnect(future Future) {
	future = u.ensureFuture(future)

	channel, ok := u.channel.(UnsafeDisconnect)
	if !ok {
		u.futureFail(future, ErrUnsupportedOperation)
		return
	}

	if !u.acquireOperation(&u.disconnectS, future) {
		return
	}

	go func(u *DefaultUnsafe, future Future) {
		defer u.resetState(&u.disconnectS)
		u.channel.inactiveChannel()
		err := channel.UnsafeDisconnect()
		if err != nil {
			kklogger.WarnJ("channel:DefaultUnsafe.Disconnect#disconnect!disconnect_error", fmt.Sprintf("channel_id: %s, error: %s", u.channel.ID(), err.Error()))
		}

		u.futureSuccess(future)
	}(u, future)
}

func (u *DefaultUnsafe) markState(state *int32) bool {
	return atomic.CompareAndSwapInt32(state, 0, 1)
}

func (u *DefaultUnsafe) resetState(state *int32) {
	atomic.StoreInt32(state, 0)
}

func (u *DefaultUnsafe) ensureFuture(future Future) Future {
	if future != nil {
		return future
	}

	return u.channel.Pipeline().NewFuture()
}

func (u *DefaultUnsafe) acquireOperation(state *int32, future Future) bool {
	if u.channel.CloseFuture().IsDone() {
		u.futureFail(future, ErrChannelClosed)
		return false
	}

	if !u.markState(state) {
		u.futureFail(future, ErrOperationInProgress)
		return false
	}

	return true
}

func (u *DefaultUnsafe) futureSuccess(future Future) bool {
	return future.Completable().Complete(u.channel)
}

func (u *DefaultUnsafe) futureFail(future Future, err error) bool {
	return future.Completable().Fail(err)
}

func (u *DefaultUnsafe) futureCancel(future Future) bool {
	return future.Completable().Cancel()
}
