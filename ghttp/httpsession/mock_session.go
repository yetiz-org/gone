package httpsession

import (
	"time"

	"github.com/stretchr/testify/mock"
)

// MockSessionProvider is a mock implementation of SessionProvider interface
// It provides complete testify/mock integration for testing session provider behaviors
type MockSessionProvider struct {
	mock.Mock
}

// NewMockSessionProvider creates a new MockSessionProvider instance
func NewMockSessionProvider() *MockSessionProvider {
	return &MockSessionProvider{}
}

// Type returns the session provider type
func (m *MockSessionProvider) Type() SessionType {
	args := m.Called()
	return SessionType(args.String(0))
}

// NewSession creates a new session with expiration time
func (m *MockSessionProvider) NewSession(expire time.Time) Session {
	args := m.Called(expire)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(Session)
}

// Sessions returns all sessions (readonly)
func (m *MockSessionProvider) Sessions() map[string]Session {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(map[string]Session)
}

// Session returns a specific session by key
func (m *MockSessionProvider) Session(key string) Session {
	args := m.Called(key)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(Session)
}

// Save saves a session
func (m *MockSessionProvider) Save(session Session) error {
	args := m.Called(session)
	return args.Error(0)
}

// Delete removes a session by key
func (m *MockSessionProvider) Delete(key string) {
	m.Called(key)
}

// SetSaveError sets up the mock to return an error on Save calls (for testing error scenarios)
func (m *MockSessionProvider) SetSaveError(err error) {
	m.On("Save", mock.Anything).Return(err)
}

// MockSession is a mock implementation of Session interface
// It provides complete testify/mock integration for testing session behaviors
type MockSession struct {
	mock.Mock
}

// NewMockSession creates a new MockSession instance
func NewMockSession() *MockSession {
	return &MockSession{}
}

// Id returns the session ID
func (m *MockSession) Id() string {
	args := m.Called()
	return args.String(0)
}

// GetString gets a string value by key
func (m *MockSession) GetString(key string) string {
	args := m.Called(key)
	return args.String(0)
}

// PutString stores a string value by key
func (m *MockSession) PutString(key string, value string) Session {
	args := m.Called(key, value)
	if args.Get(0) == nil {
		return m
	}
	return args.Get(0).(Session)
}

// GetInt64 gets an int64 value by key
func (m *MockSession) GetInt64(key string) int64 {
	args := m.Called(key)
	return args.Get(0).(int64)
}

// PutInt64 stores an int64 value by key
func (m *MockSession) PutInt64(key string, value int64) Session {
	args := m.Called(key, value)
	if args.Get(0) == nil {
		return m
	}
	return args.Get(0).(Session)
}

// GetStruct gets a struct value by key and unmarshals into obj
func (m *MockSession) GetStruct(key string, obj any) {
	m.Called(key, obj)
}

// PutStruct stores a struct value by key (marshals to JSON)
func (m *MockSession) PutStruct(key string, value any) Session {
	args := m.Called(key, value)
	if args.Get(0) == nil {
		return m
	}
	return args.Get(0).(Session)
}

// Clear removes all data from the session
func (m *MockSession) Clear() Session {
	args := m.Called()
	if args.Get(0) == nil {
		return m
	}
	return args.Get(0).(Session)
}

// Delete removes a specific key from the session
func (m *MockSession) Delete(key string) {
	m.Called(key)
}

// Created returns the session creation time
func (m *MockSession) Created() time.Time {
	args := m.Called()
	return args.Get(0).(time.Time)
}

// Updated returns the session last update time
func (m *MockSession) Updated() time.Time {
	args := m.Called()
	return args.Get(0).(time.Time)
}

// Expire returns the session expiration time
func (m *MockSession) Expire() time.Time {
	args := m.Called()
	return args.Get(0).(time.Time)
}

// Save persists the session
func (m *MockSession) Save() error {
	args := m.Called()
	return args.Error(0)
}

// Reload reloads the session from storage
func (m *MockSession) Reload() Session {
	args := m.Called()
	if args.Get(0) == nil {
		return m
	}
	return args.Get(0).(Session)
}

// Data returns all session data
func (m *MockSession) Data() map[string]string {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(map[string]string)
}

// SetExpire sets the session expiration time
func (m *MockSession) SetExpire(expire time.Time) Session {
	args := m.Called(expire)
	if args.Get(0) == nil {
		return m
	}
	return args.Get(0).(Session)
}

// IsExpire checks if the session has expired
func (m *MockSession) IsExpire() bool {
	args := m.Called()
	return args.Bool(0)
}

// Ensure interface compliance
var _ SessionProvider = (*MockSessionProvider)(nil)
var _ Session = (*MockSession)(nil)
