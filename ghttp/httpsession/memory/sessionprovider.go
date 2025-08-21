package memory

import (
	"sync"
	"time"

	"github.com/yetiz-org/gone/ghttp/httpsession"
)

const SessionTypeMemory httpsession.SessionType = "MEMORY"

type SessionProvider struct {
	sessions  sync.Map
	lastClean time.Time
}

func NewSessionProvider() *SessionProvider {
	return &SessionProvider{}
}

func (s *SessionProvider) Type() httpsession.SessionType {
	return SessionTypeMemory
}

func (s *SessionProvider) NewSession(expire time.Time) httpsession.Session {
	session := httpsession.NewDefaultSession(s)
	session.SetExpire(expire)
	if err := session.Save(); err != nil {
		return nil
	}

	return session
}

func (s *SessionProvider) Sessions() map[string]httpsession.Session {
	sessions := map[string]httpsession.Session{}
	s.sessions.Range(func(key, value any) bool {
		sessions[key.(string)] = value.(httpsession.Session)
		return true
	})

	return sessions
}

func (s *SessionProvider) Session(key string) httpsession.Session {
	if session, f := s.sessions.Load(key); f {
		return session.(httpsession.Session)
	}

	return nil
}

func (s *SessionProvider) Save(session httpsession.Session) error {
	s.cleanSessions()
	s.sessions.Store(session.Id(), session)
	return nil
}

func (s *SessionProvider) Delete(key string) {
	s.sessions.Delete(key)
}

func (s *SessionProvider) cleanSessions() {
	if time.Now().Sub(s.lastClean) > 10*time.Second {
		s.lastClean = time.Now()
		go func(s *SessionProvider) {
			s.sessions.Range(func(key, value any) bool {
				if value.(httpsession.Session).IsExpire() {
					s.sessions.Delete(key)
				}
				return true
			})
		}(s)
	}
}
