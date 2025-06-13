package ghttp

import (
	"github.com/yetiz-org/gone/ghttp/httpheadername"
	"github.com/yetiz-org/gone/ghttp/httpsession"
	"github.com/yetiz-org/gone/ghttp/httpsession/memory"
	kklogger "github.com/yetiz-org/goth-kklogger"
	"strings"
	"sync"
	"time"

	"github.com/yetiz-org/goth-kkutil/hash"
)

type SessionType string

const SessionTypeMemory SessionType = "MEMORY"

var defaultSessionProvider httpsession.SessionProvider = nil
var mutex = sync.Mutex{}

var DefaultSessionType = SessionTypeMemory
var SessionKey = "DEFAULT"
var SessionDomain = ""
var SessionExpireTime = 86400
var SessionHttpOnly = false
var SessionSecure = false

var sessionProviders = make(map[SessionType]httpsession.SessionProvider)

func RegisterSessionProvider(name SessionType, provider httpsession.SessionProvider) {
	sessionProviders[name] = provider
}

func DefaultSessionProvider() httpsession.SessionProvider {
	if defaultSessionProvider == nil {
		RegisterSessionProvider(SessionTypeMemory, memory.SessionProvider())
		mutex.Lock()
		defer mutex.Unlock()
		defaultSessionProvider = sessionProviders[DefaultSessionType]
		if defaultSessionProvider == nil {
			kklogger.WarnJ("ghttp:DefaultSessionProvider", "default session provider not found, use memory session provider")
			defaultSessionProvider = memory.SessionProvider()
		}
	}

	return defaultSessionProvider
}

func GetSession(req *Request) httpsession.Session {
	var session httpsession.Session
	if sessionID, e := req.Cookie(SessionKey); e == nil {
		data := hash.DataOfTimeHash(sessionID.Value)
		timestamp := hash.TimestampOfTimeHash(sessionID.Value)
		if data == nil || timestamp == 0 || time.Now().Unix() >= timestamp {
			session = _NewSession(req)
		} else {
			session = DefaultSessionProvider().Session(string(data))
		}
		if session == nil {
			session = _NewSession(req)
		}
	} else {
		session = _NewSession(req)
	}

	return session
}

func _NewSession(req *Request) httpsession.Session {
	expireTime := time.Now().Add(time.Second * time.Duration(SessionExpireTime))
	session := DefaultSessionProvider().NewSession(&expireTime)
	if session == nil {
		return nil
	}

	if hc := req.Header().Get(httpheadername.Cookie); hc != "" {
		var rehc string
		for _, cookie := range strings.Split(hc, ";") {
			if strings.Split(strings.TrimSpace(cookie), "=")[0] == SessionKey {
				continue
			}

			if rehc == "" {
				rehc = cookie
			} else {
				rehc = rehc + "; " + cookie
			}
		}

		req.Header().Set(httpheadername.Cookie, rehc)
	}

	return session
}
