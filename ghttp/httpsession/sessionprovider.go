package httpsession

import (
	"time"
)

type SessionType string

type SessionProvider interface {
	Type() SessionType
	NewSession(expire time.Time) Session
	// Sessions Readonly
	Sessions() map[string]Session
	Session(key string) Session
	Save(session Session) error
	Delete(key string)
}
