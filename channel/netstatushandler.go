package channel

import (
	"fmt"
	kklogger "github.com/yetiz-org/goth-kklogger"
	"net"
)

type NetStatusInbound struct {
	DefaultHandler
	LogLevel kklogger.Level
}

func (h *NetStatusInbound) _AddrString(ctx HandlerContext) string {
	lAddr := func() string {
		if addr := ctx.Channel().LocalAddr(); addr != nil {
			return addr.String()
		}

		return ""
	}()

	rAddr := func() string {
		if nc, ok := ctx.Channel().(NetChannel); ok {
			if addr := nc.RemoteAddr(); addr != nil {
				return addr.String()
			}
		}

		return ""
	}()

	return fmt.Sprintf("LocalAddr: %s, RemoteAddr: %s", lAddr, rAddr)
}

func (h *NetStatusInbound) Active(ctx HandlerContext) {
	h._Init()
	kklogger.LogJ(h.LogLevel, "channel:NetStatusHandler.Active#status!active", h._AddrString(ctx))
	ctx.FireActive()
}

func (h *NetStatusInbound) Inactive(ctx HandlerContext) {
	h._Init()
	kklogger.LogJ(h.LogLevel, "channel:NetStatusHandler.Inactive#status!inactive", h._AddrString(ctx))
	ctx.FireInactive()
}

func (h *NetStatusInbound) _Init() {
	if h.LogLevel == 0 {
		h.LogLevel = kklogger.TraceLevel
	}
}

type NetStatusOutbound struct {
	DefaultHandler
	LogLevel kklogger.Level
}

func (h *NetStatusOutbound) _AddrString(ctx HandlerContext) string {
	lAddr := func() string {
		if addr := ctx.Channel().LocalAddr(); addr != nil {
			return addr.String()
		}

		return ""
	}()

	rAddr := func() string {
		if nc, ok := ctx.Channel().(NetChannel); ok {
			if addr := nc.RemoteAddr(); addr != nil {
				return addr.String()
			}
		}

		return ""
	}()

	return fmt.Sprintf("LocalAddr: %s, RemoteAddr: %s", lAddr, rAddr)
}

func (h *NetStatusOutbound) Bind(ctx HandlerContext, localAddr net.Addr, future Future) {
	h._Init()
	kklogger.LogJ(h.LogLevel, "channel:NetStatusHandler.Bind#status!bind", h._AddrString(ctx))
	ctx.Bind(localAddr, future)
}

func (h *NetStatusOutbound) Close(ctx HandlerContext, future Future) {
	h._Init()
	kklogger.LogJ(h.LogLevel, "channel:NetStatusHandler.Close#status!close", h._AddrString(ctx))
	ctx.Close(future)
}

func (h *NetStatusOutbound) Connect(ctx HandlerContext, localAddr net.Addr, remoteAddr net.Addr, future Future) {
	lAddr := func() string {
		if localAddr != nil {
			return localAddr.String()
		}

		return ""
	}()

	rAddr := func() string {
		if remoteAddr != nil {
			return remoteAddr.String()
		}

		return ""
	}()

	h._Init()
	kklogger.LogJ(h.LogLevel, "channel:NetStatusHandler.Connect#status!connect", fmt.Sprintf("LocalAddr: %s, RemoteAddr: %s", lAddr, rAddr))
	ctx.Connect(localAddr, remoteAddr, future)
}

func (h *NetStatusOutbound) Disconnect(ctx HandlerContext, future Future) {
	h._Init()
	kklogger.LogJ(h.LogLevel, "channel:NetStatusHandler.Disconnect#status!disconnect", h._AddrString(ctx))
	ctx.Disconnect(future)
}

func (h *NetStatusOutbound) _Init() {
	if h.LogLevel == 0 {
		h.LogLevel = kklogger.TraceLevel
	}
}
