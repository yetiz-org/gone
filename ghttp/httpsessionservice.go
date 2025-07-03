package ghttp

import (
	"github.com/yetiz-org/gone/ghttp/httpheadername"
	"github.com/yetiz-org/gone/ghttp/httpsession"
	"github.com/yetiz-org/gone/ghttp/httpsession/memory"
	kklogger "github.com/yetiz-org/goth-kklogger"
	"strings"
	"sync"
	"time"

	"github.com/yetiz-org/goth-util/hash"
)

var defaultSessionProvider httpsession.SessionProvider = nil
var mutex = sync.Mutex{}

var DefaultSessionType = memory.SessionTypeMemory
var SessionKey = "DEFAULT"
var SessionDomain = ""
var SessionExpireTime = 86400
var SessionHttpOnly = false
var SessionSecure = false

var sessionProviders = make(map[httpsession.SessionType]httpsession.SessionProvider)

func RegisterSessionProvider(provider httpsession.SessionProvider) {
	sessionProviders[provider.Type()] = provider
}

func SessionProvider() httpsession.SessionProvider {
	if defaultSessionProvider == nil {
		RegisterSessionProvider(memory.NewSessionProvider())
		mutex.Lock()
		defer mutex.Unlock()
		defaultSessionProvider = sessionProviders[DefaultSessionType]
		if defaultSessionProvider == nil {
			kklogger.WarnJ("ghttp:SessionProvider.init#init!default_provider", "default session provider not found, use memory session provider")
			defaultSessionProvider = sessionProviders[memory.SessionTypeMemory]
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
			session = SessionProvider().Session(string(data))
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
	session := SessionProvider().NewSession(expireTime)
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
