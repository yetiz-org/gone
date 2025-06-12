package httpsession

import (
	"time"
)

type SessionProvider interface {
	NewSession(expire *time.Time) Session
	/* Readonly */
	Sessions() map[string]Session
	Session(key string) Session
}
