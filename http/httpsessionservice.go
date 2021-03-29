package http

import (
	"strings"
	"sync"
	"time"

	"github.com/kklab-com/gone-httpheadername"
	"github.com/kklab-com/gone/http/httpsession"
	"github.com/kklab-com/gone/http/httpsession/memory"
	"github.com/kklab-com/gone/http/httpsession/redis"
	"github.com/kklab-com/goth-kkutil/hash"
)

type SessionType string

const SessionTypeMemory SessionType = "MEMORY"
const SessionTypeRedis SessionType = "REDIS"

var defaultSessionProvider httpsession.SessionProvider = nil
var once = sync.Once{}

var DefaultSessionType = SessionTypeMemory
var SessionKey = "KKBOX"
var SessionDomain = ""
var SessionExpireTime = 86400
var SessionHttpOnly = false
var SessionSecure = false

func DefaultProvider() httpsession.SessionProvider {
	once.Do(func() {
		switch DefaultSessionType {
		case SessionTypeMemory:
			defaultSessionProvider = memory.SessionProvider()
		case SessionTypeRedis:
			defaultSessionProvider = redis.SessionProvider()
		default:
			defaultSessionProvider = memory.SessionProvider()
		}
	})

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
			session = DefaultProvider().Session(string(data))
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
	session := DefaultProvider().NewSession(&expireTime)
	if session == nil {
		return nil
	}

	if hc := req.Header.Get(httpheadername.Cookie); hc != "" {
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

		req.Header.Set(httpheadername.Cookie, rehc)
	}

	return session
}
