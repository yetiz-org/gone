package memory

import (
	"encoding/json"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/yetiz-org/gone/ghttp/httpsession"
)

func TestNewSessionProvider(t *testing.T) {
	provider := NewSessionProvider()

	if provider == nil {
		t.Fatal("NewSessionProvider() should not return nil")
	}

	// Check initial state
	sessions := provider.Sessions()
	if len(sessions) != 0 {
		t.Errorf("Newly created SessionProvider should have no sessions, but got %d", len(sessions))
	}
}

func TestSessionProvider_NewSession(t *testing.T) {
	provider := NewSessionProvider()
	expire := time.Now().Add(1 * time.Hour)

	session := provider.NewSession(expire)

	if session == nil {
		t.Fatal("NewSession() should not return nil")
	}

	if session.Id() == "" {
		t.Error("Session ID should not be empty")
	}

	// Compare Unix timestamps to avoid precision issues
	if session.Expire().Unix() != expire.Unix() {
		t.Errorf("Expected expire time %v, but got %v", expire.Unix(), session.Expire().Unix())
	}

	// Verify session has been saved
	if provider.Session(session.Id()) == nil {
		t.Error("Newly created session should be findable from provider")
	}
}

func TestSessionProvider_Session(t *testing.T) {
	provider := NewSessionProvider()
	expire := time.Now().Add(1 * time.Hour)

	// Test getting non-existent session
	session := provider.Session("nonexistent")
	if session != nil {
		t.Error("Non-existent session should return nil")
	}

	// Create a session and test retrieval
	newSession := provider.NewSession(expire)
	retrievedSession := provider.Session(newSession.Id())

	if retrievedSession == nil {
		t.Fatal("Should be able to retrieve existing session")
	}

	if retrievedSession.Id() != newSession.Id() {
		t.Errorf("Expected session ID %s, but got %s", newSession.Id(), retrievedSession.Id())
	}
}

func TestSessionProvider_Sessions(t *testing.T) {
	provider := NewSessionProvider()
	expire := time.Now().Add(1 * time.Hour)

	// Test empty state
	sessions := provider.Sessions()
	if len(sessions) != 0 {
		t.Errorf("Initial state should have no sessions, but got %d", len(sessions))
	}

	// Create multiple sessions
	session1 := provider.NewSession(expire)
	session2 := provider.NewSession(expire.Add(1 * time.Hour))

	sessions = provider.Sessions()
	if len(sessions) != 2 {
		t.Errorf("Expected 2 sessions, but got %d", len(sessions))
	}

	// Verify sessions are in the map
	if _, exists := sessions[session1.Id()]; !exists {
		t.Error("session1 should be in sessions map")
	}

	if _, exists := sessions[session2.Id()]; !exists {
		t.Error("session2 should be in sessions map")
	}
}

func TestSessionProvider_Save(t *testing.T) {
	provider := NewSessionProvider()
	expire := time.Now().Add(1 * time.Hour)

	session := provider.NewSession(expire)

	// Modify session data
	session.PutString("key1", "value1")

	// Save
	err := provider.Save(session)
	if err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	// Verify data has been saved
	retrievedSession := provider.Session(session.Id())
	if retrievedSession == nil {
		t.Fatal("Should be able to retrieve session after saving")
	}

	if retrievedSession.GetString("key1") != "value1" {
		t.Errorf("Expected 'value1', but got '%s'", retrievedSession.GetString("key1"))
	}
}

func TestSessionProvider_Delete(t *testing.T) {
	provider := NewSessionProvider()
	expire := time.Now().Add(1 * time.Hour)

	session := provider.NewSession(expire)
	sessionId := session.Id()

	// Verify session exists
	if provider.Session(sessionId) == nil {
		t.Fatal("Session should exist")
	}

	// Delete session
	provider.Delete(sessionId)

	// Verify session has been deleted
	if provider.Session(sessionId) != nil {
		t.Error("Session should have been deleted")
	}

	// Verify it's also removed from Sessions()
	sessions := provider.Sessions()
	if _, exists := sessions[sessionId]; exists {
		t.Error("Deleted session should not appear in Sessions() result")
	}
}

func TestSessionProvider_CleanSessions(t *testing.T) {
	provider := NewSessionProvider()

	// Create an expired session
	expiredTime := time.Now().Add(-1 * time.Hour)
	expiredSession := provider.NewSession(expiredTime)

	// Create a valid session
	validTime := time.Now().Add(1 * time.Hour)
	validSession := provider.NewSession(validTime)

	// Verify both sessions exist
	sessions := provider.Sessions()
	if len(sessions) != 2 {
		t.Fatalf("Should have 2 sessions, but got %d", len(sessions))
	}

	// Backdate lastClean so the next Save() triggers a sweep. The sweep runs
	// synchronously inside Save().
	provider.cleanMu.Lock()
	provider.lastClean = time.Now().Add(-15 * time.Second)
	provider.cleanMu.Unlock()

	provider.Save(validSession)

	// Verify expired session was cleaned
	if provider.Session(expiredSession.Id()) != nil {
		t.Error("Expired session should have been cleaned")
	}

	// Verify valid session still exists
	if provider.Session(validSession.Id()) == nil {
		t.Error("Valid session should not have been cleaned")
	}
}

func TestSessionProvider_ConcurrentAccess(t *testing.T) {
	provider := NewSessionProvider()
	expire := time.Now().Add(1 * time.Hour)

	const numGoroutines = 100
	const numOperations = 10

	var wg sync.WaitGroup
	sessionIds := make([]string, numGoroutines)

	// Concurrently create sessions
	for i := range numGoroutines {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			session := provider.NewSession(expire)
			sessionIds[index] = session.Id()

			// Perform multiple operations
			for range numOperations {
				session.PutString("key", "value")
				provider.Save(session)
				provider.Session(session.Id())
			}
		}(i)
	}

	wg.Wait()

	// Verify all sessions exist
	sessions := provider.Sessions()
	if len(sessions) != numGoroutines {
		t.Errorf("Expected %d sessions, but got %d", numGoroutines, len(sessions))
	}

	// Concurrently delete sessions
	for i := range numGoroutines {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			provider.Delete(sessionIds[index])
		}(i)
	}

	wg.Wait()

	// Verify all sessions have been deleted
	sessions = provider.Sessions()
	if len(sessions) != 0 {
		t.Errorf("All sessions should have been deleted, but %d remain", len(sessions))
	}
}

func TestSessionProvider_CleanFrequency(t *testing.T) {
	provider := NewSessionProvider()

	// Set initial clean time to past
	provider.lastClean = time.Now().Add(-15 * time.Second)

	session := provider.NewSession(time.Now().Add(1 * time.Hour))

	// First Save should trigger cleanup
	provider.Save(session)

	// Record cleanup time
	firstCleanTime := provider.lastClean

	// Immediately Save again, should not cleanup again due to < 10 second interval
	provider.Save(session)

	if !provider.lastClean.Equal(firstCleanTime) {
		t.Error("Should not cleanup again within 10 seconds")
	}

	// Wait for 10 seconds then trigger again (in actual tests you might need to adjust this time)
	// Here we directly modify lastClean to simulate time passing
	provider.lastClean = time.Now().Add(-15 * time.Second)
	provider.Save(session)

	if provider.lastClean.Equal(firstCleanTime) {
		t.Error("Should cleanup again after 10 seconds")
	}
}

// Test that SessionProvider correctly implements httpsession.SessionProvider interface
func TestSessionProvider_ImplementsInterface(t *testing.T) {
	var _ httpsession.SessionProvider = (*SessionProvider)(nil)
}

func TestSessionProvider_MaxSessions(t *testing.T) {
	provider := NewSessionProvider()
	provider.MaxSessions = 3
	expire := time.Now().Add(time.Hour)
	var last httpsession.Session
	for range 5 {
		last = provider.NewSession(expire)
	}

	if got := len(provider.Sessions()); got != 3 {
		t.Fatalf("stored sessions = %d, want the cap 3", got)
	}

	if provider.Session(last.Id()) == nil {
		t.Fatal("the session just saved must stay stored")
	}

	for id, session := range provider.Sessions() {
		if err := session.Save(); err != nil || provider.Session(id) == nil {
			t.Fatalf("saving a stored session must not evict it: %v", err)
		}
	}

	if got := len(provider.Sessions()); got != 3 {
		t.Fatalf("stored sessions after re-saving = %d, want 3", got)
	}
}

// TestSessionProvider_SharedSessionConcurrentRequests mirrors concurrent requests carrying one session
// cookie: they all receive the same stored session. Run with -race.
func TestSessionProvider_SharedSessionConcurrentRequests(t *testing.T) {
	provider := NewSessionProvider()
	id := provider.NewSession(time.Now().Add(time.Hour)).Id()
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Go(func() {
			session := provider.Session(id)
			for j := range 100 {
				session.PutString("key", "value")
				session.PutInt64("n", int64(i*j))
				_ = session.GetString("key")
				session.Delete("missing")
				for range session.(*httpsession.DefaultSession).Snapshot() {
				}

				_ = session.Save()
			}
		})
	}

	wg.Wait()
	if got := provider.Session(id).GetString("key"); got != "value" {
		t.Fatalf("GetString = %q, want value", got)
	}

	snapshot := provider.Session(id).(*httpsession.DefaultSession).Snapshot()
	snapshot["key"] = "changed"
	if got := provider.Session(id).GetString("key"); got != "value" {
		t.Fatalf("changing a Snapshot must not change the session, GetString = %q", got)
	}
}

// TestSessionProvider_ReloadWhileWriting reloads a copy of a stored session, as a provider that decodes
// sessions hands out, while another request writes the stored one. Run with -race.
func TestSessionProvider_ReloadWhileWriting(t *testing.T) {
	provider := NewSessionProvider()
	stored := provider.NewSession(time.Now().Add(time.Hour))
	_ = stored.Save()
	encoded, err := json.Marshal(stored)
	if err != nil {
		t.Fatal(err)
	}

	session := httpsession.NewDefaultSession(provider)
	if err := json.Unmarshal(encoded, session); err != nil {
		t.Fatal(err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := range 50000 {
			stored.PutInt64(strconv.Itoa(i%4096), int64(i))
		}
	}()

	for {
		select {
		case <-done:
			return
		default:
			session.Reload()
		}
	}
}

func TestSessionProvider_EvictsExpiredThenLeastRecentlySaved(t *testing.T) {
	provider := NewSessionProvider()
	provider.MaxSessions = 10
	expire := time.Now().Add(time.Hour)
	var sessions []httpsession.Session
	for range 10 {
		sessions = append(sessions, provider.NewSession(expire))
	}

	expired := sessions[9]
	expired.SetExpire(time.Now().Add(-time.Second))
	time.Sleep(1100 * time.Millisecond)
	active := sessions[0]
	if err := active.Save(); err != nil {
		t.Fatal(err)
	}

	provider.NewSession(expire)

	if got := len(provider.Sessions()); got != 9 {
		t.Fatalf("stored sessions = %d, want a tenth of the cap freed", got)
	}

	if provider.Session(expired.Id()) != nil {
		t.Fatal("an expired session must be evicted first")
	}

	if provider.Session(active.Id()) == nil {
		t.Fatal("the most recently saved session must survive eviction")
	}
}

// TestSessionProvider_DeletedSessionIsNotStoredAgain mirrors a request that still holds a session while
// another request renews or deletes it.
func TestSessionProvider_DeletedSessionIsNotStoredAgain(t *testing.T) {
	provider := NewSessionProvider()
	id := provider.NewSession(time.Now().Add(time.Hour)).Id()
	inFlight := provider.Session(id)

	provider.Delete(id)
	inFlight.PutString("user", "u")
	if err := inFlight.Save(); err != nil {
		t.Fatal(err)
	}

	if provider.Session(id) != nil {
		t.Fatal("a deleted session must not be stored again by a request still holding it")
	}

	wrapped := wrappedSession{httpsession.NewDefaultSession(provider)}
	_ = provider.Save(wrapped)
	provider.Delete(wrapped.Id())
	_ = wrapped.Save()
	if provider.Session(wrapped.Id()) != nil {
		t.Fatal("a deleted session that embeds DefaultSession must not be stored again")
	}
}

type wrappedSession struct {
	*httpsession.DefaultSession
}

// TestSessionProvider_DeleteRacesSave runs Delete against a concurrent Save of the same session many
// times; the deleted session must never remain stored. Run with -race.
func TestSessionProvider_DeleteRacesSave(t *testing.T) {
	provider := NewSessionProvider()
	for range 20000 {
		session := provider.NewSession(time.Now().Add(time.Hour))
		var wg sync.WaitGroup
		wg.Go(func() { _ = session.Save() })
		wg.Go(func() { provider.Delete(session.Id()) })
		wg.Wait()
		if provider.Session(session.Id()) != nil {
			t.Fatal("a session deleted while a Save was in progress must not remain stored")
		}
	}
}

func TestSessionProvider_ConcurrentCreationStaysCapped(t *testing.T) {
	provider := NewSessionProvider()
	provider.MaxSessions = 100
	var wg sync.WaitGroup
	for range 16 {
		wg.Go(func() {
			for range 200 {
				provider.NewSession(time.Now().Add(time.Hour))
			}
		})
	}

	wg.Wait()
	provider.NewSession(time.Now().Add(time.Hour))
	if got := len(provider.Sessions()); got > provider.MaxSessions {
		t.Fatalf("stored sessions = %d, want at most %d", got, provider.MaxSessions)
	}
}

// uncomparableSession is a valid Session whose value cannot be compared with ==.
type uncomparableSession struct {
	*httpsession.DefaultSession
	tags []string
}

func TestSessionProvider_EvictsUncomparableSessions(t *testing.T) {
	provider := NewSessionProvider()
	provider.MaxSessions = 1
	for range 3 {
		session := uncomparableSession{DefaultSession: httpsession.NewDefaultSession(provider), tags: []string{"custom"}}
		session.SetExpire(time.Now().Add(time.Hour))
		if err := provider.Save(session); err != nil {
			t.Fatal(err)
		}
	}

	if got := len(provider.Sessions()); got != 1 {
		t.Fatalf("stored sessions = %d, want the cap 1", got)
	}
}
