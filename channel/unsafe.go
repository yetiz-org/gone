package channel

import (
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	buf "github.com/yetiz-org/goth-bytebuf"
	concurrent "github.com/yetiz-org/goth-concurrent"
	kklogger "github.com/yetiz-org/goth-kklogger"
)

const DefaultAcceptTimeout = 5000

// Include the queue entry and Future bookkeeping, not just payload bytes.
const queuedWriteOverhead = 96

// An idle channel keeps a small write ring for reuse but returns one grown by a burst.
const maxIdleWriteEntries = 1024

var ErrLocalAddrIsEmpty = fmt.Errorf("local addr is empty")
var ErrRemoteAddrIsEmpty = fmt.Errorf("remote addr is empty")
var ErrChannelNotActive = fmt.Errorf("channel not active")
var ErrChannelClosed = fmt.Errorf("channel closed")
var ErrAcceptTimeout = fmt.Errorf("accept timeout")
var ErrUnsupportedOperation = fmt.Errorf("unsupported operation")
var ErrOperationInProgress = fmt.Errorf("operation in progress")
var ErrWriteQueueFull = fmt.Errorf("write queue limit exceeded")

type Unsafe interface {
	Read()
	Write(obj any, future Future)
	Bind(localAddr net.Addr, future Future)
	Close(future Future)
	Connect(localAddr net.Addr, remoteAddr net.Addr, future Future)
	Disconnect(future Future)
}

type queuedWrite struct {
	future Future
	size   int64
}

type DefaultUnsafe struct {
	channel Channel
	readS,
	bindS,
	closeS,
	connectS,
	disconnectS int32
	writeBuffer   []queuedWrite
	writeHead     int
	writeCount    int
	writeMu       sync.Mutex
	writeBytes    int64
	maxWriteBytes int64
	writing       bool
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
	future = u.ensureFuture(future)
	if obj == nil {
		u.futureSuccess(future)
		return
	}
	writer, ok := u.channel.(UnsafeWrite)
	if !ok {
		u.futureFail(future, ErrUnsupportedOperation)
		return
	}
	size := u.writeSize(obj)
	u.writeMu.Lock()
	if u.maxWriteBytes == 0 {
		u.maxWriteBytes = GetParamInt64Default(u.channel, ParamMaxQueuedWriteBytes, DefaultMaxQueuedWriteBytes)
		if u.maxWriteBytes <= 0 {
			u.maxWriteBytes = DefaultMaxQueuedWriteBytes
		}
	}
	if !u.channel.IsActive() {
		u.writeMu.Unlock()
		u.futureFail(future, ErrChannelNotActive)
		return
	}
	if size < 0 || u.maxWriteBytes-u.writeBytes < queuedWriteOverhead || size > u.maxWriteBytes-u.writeBytes-queuedWriteOverhead {
		u.writeMu.Unlock()
		u.futureFail(future, ErrWriteQueueFull)
		u.channel.inactiveChannel()
		return
	}
	future.(concurrent.Settable).Set(obj)
	size += queuedWriteOverhead
	if u.writeCount == len(u.writeBuffer) {
		next := make([]queuedWrite, max(8, 2*len(u.writeBuffer)))
		copied := copy(next, u.writeBuffer[u.writeHead:])
		copy(next[copied:], u.writeBuffer[:u.writeHead])
		u.writeBuffer = next
		u.writeHead = 0
	}
	tail := (u.writeHead + u.writeCount) % len(u.writeBuffer)
	u.writeBuffer[tail] = queuedWrite{future: future, size: size}
	u.writeCount++
	u.writeBytes += size
	start := !u.writing
	u.writing = true
	u.writeMu.Unlock()
	if start {
		go u.drainWrites(writer)
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

								// A server that began closing has already closed the children it had active, so close
								// one it accepted meanwhile; otherwise Close waits for it.
								if server, ok := u.channel.(interface{ closing() bool }); ok && server.closing() {
									child.inactiveChannel()
								}
							}(u, child, future)

							go func(u *DefaultUnsafe, child Channel, future Future) {
								timer := time.NewTimer(time.Duration(GetParamIntDefault(child, ParamAcceptTimeout, DefaultAcceptTimeout)) * time.Millisecond)
								defer timer.Stop()
								select {
								case <-future.Done():
									return
								case <-timer.C:
								}
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
		if drain, ok := u.channel.(UnsafeDrain); ok {
			if err := drain.UnsafeDrain(); err != nil {
				kklogger.WarnJ("channel:DefaultUnsafe.Close#close!drain_error", fmt.Sprintf("channel_id: %s, error: %s", u.channel.ID(), err.Error()))
			}
		}

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

func (u *DefaultUnsafe) writeSize(obj any) (size int64) {
	switch value := obj.(type) {
	case buf.ByteBuf:
		return int64(value.ReadableBytes())
	case []byte:
		return int64(len(value))
	case string:
		return int64(len(value))
	case interface{ WriteSize() int }:
		return int64(value.WriteSize())
	default:
		return 0
	}
}

func (u *DefaultUnsafe) drainWrites(writer UnsafeWrite) {
	for {
		u.writeMu.Lock()
		if u.writeCount == 0 {
			u.writeHead = 0
			if len(u.writeBuffer) > maxIdleWriteEntries {
				u.writeBuffer = nil
			}
			u.writing = false
			u.writeMu.Unlock()
			return
		}
		item := u.writeBuffer[u.writeHead]
		u.writeBuffer[u.writeHead] = queuedWrite{}
		u.writeHead = (u.writeHead + 1) % len(u.writeBuffer)
		u.writeCount--
		u.writeMu.Unlock()
		var writeErr error
		if !u.channel.IsActive() {
			if u.channel.CloseFuture().IsDone() {
				writeErr = ErrChannelClosed
			} else {
				writeErr = ErrChannelNotActive
			}
		} else {
			writeErr = writer.UnsafeWrite(item.future.GetNow())
			if writeErr != nil {
				u.channel.inactiveChannel()
			}
		}
		u.writeMu.Lock()
		u.writeBytes -= item.size
		u.writeMu.Unlock()
		if writeErr != nil {
			u.futureFail(item.future, writeErr)
		} else {
			u.futureSuccess(item.future)
		}
	}
}
