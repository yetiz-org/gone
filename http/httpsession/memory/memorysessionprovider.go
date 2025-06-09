package memory

import (
	"github.com/yetiz-org/gone/http/httpsession"
	"sync"
	"time"
)

var (
	provider *MemSessionProvider
	once     sync.Once
)

type MemSessionProvider struct {
	sessions sync.Map
}

func (s *MemSessionProvider) NewSession(expire *time.Time) httpsession.Session {
	session := _NewMemorySession(expire)
	s.sessions.Store(session.ID(), session)
	session.timer =
		time.AfterFunc(session.Expire().Sub(time.Now()),
			func() {
				s.cleanSessions(session)
			})

	return session
}

func (s *MemSessionProvider) Sessions() map[string]httpsession.Session {
	sessions := map[string]httpsession.Session{}
	s.sessions.Range(func(key, value any) bool {
		sessions[key.(string)] = value.(httpsession.Session)
		return true
	})

	return sessions
}

func (s *MemSessionProvider) Session(key string) httpsession.Session {
	if sess, f := s.sessions.Load(key); f {
		return sess.(httpsession.Session)
	}

	return nil
}

func SessionProvider() httpsession.SessionProvider {
	once.Do(func() {
		provider = &MemSessionProvider{}
	})

	return provider
}

func (s *MemSessionProvider) cleanSessions(session *MemorySession) {
	if session.IsExpire() {
		s.sessions.Delete(session.ID())
	} else {
		session.timer = time.AfterFunc(session.Expire().Sub(time.Now()),
			func() {
				s.cleanSessions(session)
			})
	}
}
