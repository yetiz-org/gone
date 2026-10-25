package memory

import (
	"cmp"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/yetiz-org/gone/ghttp/httpsession"
)

const SessionTypeMemory httpsession.SessionType = "MEMORY"

// DefaultMaxSessions is the MaxSessions of a provider made by NewSessionProvider.
const DefaultMaxSessions = 100000

type SessionProvider struct {
	// MaxSessions caps the stored sessions, so requests that keep creating sessions cannot grow memory
	// without bound. Going over the cap drops expired sessions and then the least recently saved ones
	// until a tenth of the cap is free. Nonpositive values disable the cap.
	MaxSessions int
	sessions    sync.Map // id -> *storedSession
	count       atomic.Int64
	cleanMu     sync.Mutex
	lastClean   time.Time
	evictMu     sync.Mutex
}

func NewSessionProvider() (provider *SessionProvider) {
	return &SessionProvider{MaxSessions: DefaultMaxSessions, lastClean: time.Now()}
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
		sessions[key.(string)] = value.(*storedSession).session
		return true
	})

	return sessions
}

func (s *SessionProvider) Session(key string) (session httpsession.Session) {
	for {
		value, found := s.sessions.Load(key)
		if !found {
			return nil
		}

		session = value.(*storedSession).session
		if !session.IsExpire() {
			return session
		}

		if s.remove(key, value.(*storedSession), true) {
			return nil
		}
	}
}

func (s *SessionProvider) Save(session httpsession.Session) (rtErr error) {
	if session.IsExpire() {
		return nil
	}

	id := session.Id()
	store := func() {
		if _, loaded := s.sessions.Swap(id, &storedSession{session: session}); !loaded {
			s.count.Add(1)
		}
	}
	if guarded, ok := session.(interface{ StoreIfActive(func()) bool }); ok {
		if !guarded.StoreIfActive(store) {
			return nil
		}
	} else {
		store()
	}

	s.cleanSessions()
	if s.MaxSessions > 0 && s.count.Load() > int64(s.MaxSessions) {
		s.evict(id)
	}

	return nil
}

// evict drops expired and then least recently saved sessions, other than keep, until a tenth of
// MaxSessions is free, so one scan and sort serves many new sessions. A concurrent caller skips it and
// the next save over the cap evicts again.
func (s *SessionProvider) evict(keep string) {
	if !s.evictMu.TryLock() {
		return
	}

	defer s.evictMu.Unlock()
	type candidate struct {
		key, value any
		expired    bool
		updated    int64
	}

	var candidates []candidate
	s.sessions.Range(func(key, value any) bool {
		if session := value.(*storedSession).session; key != keep {
			candidates = append(candidates, candidate{key: key, value: value, expired: session.IsExpire(), updated: session.Updated().Unix()})
		}

		return true
	})

	slices.SortFunc(candidates, func(a, b candidate) int {
		if a.expired != b.expired {
			if a.expired {
				return -1
			}

			return 1
		}

		return cmp.Compare(a.updated, b.updated)
	})

	target := int64(s.MaxSessions - s.MaxSessions/10)
	for _, c := range candidates {
		if s.count.Load() <= target {
			return
		}

		s.remove(c.key.(string), c.value.(*storedSession), c.expired)
	}
}

// Delete removes the session and marks a stored DefaultSession deleted, so a request still holding it
// cannot store it again, for example after RenewSession replaced it.
func (s *SessionProvider) Delete(key string) {
	if value, ok := s.sessions.Load(key); ok {
		if session, ok := value.(*storedSession).session.(interface{ MarkDeleted() }); ok {
			session.MarkDeleted()
		}

		if !s.remove(key, value.(*storedSession), false) {
			// A Save of the same object may have replaced the entry before MarkDeleted acquired its lock.
			// Expiry lookup removes that revoked generation and preserves a live replacement object.
			s.Session(key)
		}
	}
}

// cleanSessions sweeps expired entries at most once every 10 seconds. The
// sweep runs synchronously on the caller's goroutine so Save() does not
// return until the post-sweep state is visible.
func (s *SessionProvider) cleanSessions() {
	s.cleanMu.Lock()
	if time.Since(s.lastClean) <= 10*time.Second {
		s.cleanMu.Unlock()
		return
	}
	s.lastClean = time.Now()
	s.cleanMu.Unlock()

	s.sessions.Range(func(key, value any) bool {
		s.remove(key.(string), value.(*storedSession), true)
		return true
	})
}

// remove couples DefaultSession deletion and revocation without waiting for Save's outer lock:
// cleanup can run inside another session's Save. A stale scan must not revoke a newer saved entry.
func (s *SessionProvider) remove(key string, value *storedSession, expiredOnly bool) (removed bool) {
	remove := func() bool {
		if s.sessions.CompareAndDelete(key, value) {
			s.count.Add(-1)
			return true
		}

		return false
	}
	if guarded, ok := value.session.(interface{ RemoveIfCurrent(bool, func() bool) bool }); ok {
		return guarded.RemoveIfCurrent(expiredOnly, remove)
	}

	if expiredOnly {
		if !value.session.IsExpire() {
			return false
		}
	}

	if deleted, ok := value.session.(interface{ MarkDeleted() }); ok {
		deleted.MarkDeleted()
	}

	return remove()
}

// storedSession wraps each saved session, so the sync.Map compares provider-owned pointers even when a
// Session implementation is not comparable, and each Save stores a new entry that an earlier eviction
// scan no longer matches.
type storedSession struct {
	session httpsession.Session
}
