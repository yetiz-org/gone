package httpsession

import (
	"time"
)

type SessionProvider interface {
	NewSession(expire time.Time) Session
	// Sessions Readonly
	Sessions() map[string]Session
	Session(key string) Session
	Save(session Session) error
	Delete(key string)
}
