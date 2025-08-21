package memory

import (
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

	// Manually set clean time to past to force cleanup
	provider.lastClean = time.Now().Add(-15 * time.Second)

	// Trigger cleanup (through Save method)
	provider.Save(validSession)

	// Wait for cleanup to complete (cleanup is asynchronous)
	time.Sleep(200 * time.Millisecond)

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
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			session := provider.NewSession(expire)
			sessionIds[index] = session.Id()

			// Perform multiple operations
			for j := 0; j < numOperations; j++ {
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
	for i := 0; i < numGoroutines; i++ {
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
