package ghttp

import (
	"strings"
	"sync"
	"time"

	"github.com/yetiz-org/gone/ghttp/httpheadername"
	"github.com/yetiz-org/gone/ghttp/httpsession"
	"github.com/yetiz-org/gone/ghttp/httpsession/memory"
	kklogger "github.com/yetiz-org/goth-kklogger"

	"github.com/yetiz-org/goth-util/hash"
)

var defaultSessionProvider httpsession.SessionProvider = nil
var mutex = sync.Mutex{}
var sessionProviderOnce = sync.Once{}

var DefaultSessionType = memory.SessionTypeMemory
var SessionKey = "DEFAULT"
var SessionDomain = ""

// SessionDomainFromRequestHost restores legacy host-derived Domain cookies. Deprecated: set
// SessionDomain explicitly when cross-subdomain sharing is required; the default is host-only.
var SessionDomainFromRequestHost = false

var SessionExpireTime = 86400

// SessionHttpOnly hides the session cookie from scripts.
var SessionHttpOnly = true

// SessionSecure always marks the session cookie Secure. Without it, the cookie is still Secure on
// requests received over TLS, or forwarded with X-Forwarded-Proto: https by a peer TrustForwardedFor accepts.
var SessionSecure = false

var sessionProviders = make(map[httpsession.SessionType]httpsession.SessionProvider)

func RegisterSessionProvider(provider httpsession.SessionProvider) {
	if provider == nil {
		return
	}

	mutex.Lock()
	defer mutex.Unlock()
	sessionProviders[provider.Type()] = provider
}

func SessionProvider() httpsession.SessionProvider {
	sessionProviderOnce.Do(func() {
		provider := memory.NewSessionProvider()
		mutex.Lock()
		defer mutex.Unlock()
		if provider != nil {
			sessionProviders[provider.Type()] = provider
		}
		defaultSessionProvider = sessionProviders[DefaultSessionType]
		if defaultSessionProvider == nil {
			kklogger.WarnJ("ghttp:SessionProvider.init#init!default_provider", "default session provider not found, use memory session provider")
			defaultSessionProvider = sessionProviders[memory.SessionTypeMemory]
		}
	})

	return defaultSessionProvider
}

func GetSession(req *Request) httpsession.Session {
	if session := _StoredSession(req); session != nil {
		return session
	}

	return _NewSession(req)
}

// _StoredSession checks server-side expiry even when the cookie carries a later renewal timestamp.
func _StoredSession(req *Request) (session httpsession.Session) {
	sessionID, err := req.Cookie(SessionKey)
	if err != nil {
		return nil
	}

	data := hash.DataOfTimeHash(sessionID.Value)
	timestamp := hash.TimestampOfTimeHash(sessionID.Value)
	if data == nil || timestamp == 0 || time.Now().Unix() >= timestamp {
		return nil
	}

	session = SessionProvider().Session(string(data))
	if session != nil && session.IsExpire() {
		return nil
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
		for cookie := range strings.SplitSeq(hc, ";") {
			if key, _, _ := strings.Cut(strings.TrimSpace(cookie), "="); key == SessionKey {
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
