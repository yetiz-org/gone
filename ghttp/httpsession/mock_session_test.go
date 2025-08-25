package httpsession

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestMockSessionProvider_InterfaceCompliance(t *testing.T) {
	// Test that MockSessionProvider implements SessionProvider interface
	var mockProvider interface{} = NewMockSessionProvider()
	assert.Implements(t, (*SessionProvider)(nil), mockProvider, "MockSessionProvider should implement SessionProvider interface")
}

func TestMockSession_InterfaceCompliance(t *testing.T) {
	// Test that MockSession implements Session interface
	var mockSession interface{} = NewMockSession()
	assert.Implements(t, (*Session)(nil), mockSession, "MockSession should implement Session interface")
}

func TestMockSessionProvider_BasicMethods(t *testing.T) {
	mockProvider := NewMockSessionProvider()
	expectedType := SessionType("memory")
	expireTime := time.Now().Add(time.Hour)
	mockSession := NewMockSession()

	// Test Type method
	mockProvider.On("Type").Return(string(expectedType)).Once()
	result := mockProvider.Type()
	assert.Equal(t, expectedType, result, "Type should return expected session type")

	// Test NewSession method
	mockProvider.On("NewSession", expireTime).Return(mockSession).Once()
	result2 := mockProvider.NewSession(expireTime)
	assert.Equal(t, mockSession, result2, "NewSession should return expected session")

	// Test NewSession with nil return
	mockProvider.On("NewSession", mock.AnythingOfType("time.Time")).Return(nil).Once()
	result2 = mockProvider.NewSession(time.Now())
	assert.Nil(t, result2, "NewSession should return nil when configured to do so")

	// Verify all expectations
	mockProvider.AssertExpectations(t)
}

func TestMockSessionProvider_SessionManagement(t *testing.T) {
	mockProvider := NewMockSessionProvider()
	mockSession1 := NewMockSession()
	mockSession2 := NewMockSession()
	sessionKey := "session_123"

	expectedSessions := map[string]Session{
		"session_1": mockSession1,
		"session_2": mockSession2,
	}

	// Test Sessions method
	mockProvider.On("Sessions").Return(expectedSessions).Once()
	result := mockProvider.Sessions()
	assert.Equal(t, expectedSessions, result, "Sessions should return expected sessions map")

	// Test Sessions with nil return
	mockProvider.On("Sessions").Return(nil).Once()
	result = mockProvider.Sessions()
	assert.Nil(t, result, "Sessions should return nil when no sessions")

	// Test Session method
	mockProvider.On("Session", sessionKey).Return(mockSession1).Once()
	result2 := mockProvider.Session(sessionKey)
	assert.Equal(t, mockSession1, result2, "Session should return expected session")

	// Test Session with nil return (not found)
	mockProvider.On("Session", "not_found").Return(nil).Once()
	result2 = mockProvider.Session("not_found")
	assert.Nil(t, result2, "Session should return nil for not found key")

	// Verify all expectations
	mockProvider.AssertExpectations(t)
}

func TestMockSessionProvider_SaveAndDelete(t *testing.T) {
	mockProvider := NewMockSessionProvider()
	mockSession := NewMockSession()
	sessionKey := "session_to_delete"

	// Test Save method - success case
	mockProvider.On("Save", mockSession).Return(nil).Once()
	err := mockProvider.Save(mockSession)
	assert.NoError(t, err, "Save should return no error on success")

	// Test Save method - error case
	expectedError := assert.AnError
	mockProvider.On("Save", mockSession).Return(expectedError).Once()
	err = mockProvider.Save(mockSession)
	assert.Equal(t, expectedError, err, "Save should return expected error")

	// Test Delete method
	mockProvider.On("Delete", sessionKey).Once()
	mockProvider.Delete(sessionKey)

	// Verify all expectations
	mockProvider.AssertExpectations(t)
}

func TestMockSession_BasicMethods(t *testing.T) {
	mockSession := NewMockSession()
	sessionId := "session_abc123"

	// Test Id method
	mockSession.On("Id").Return(sessionId).Once()
	result := mockSession.Id()
	assert.Equal(t, sessionId, result, "Id should return expected session ID")

	// Test GetString and PutString methods
	key := "username"
	value := "john_doe"
	mockSession.On("GetString", key).Return(value).Once()
	result = mockSession.GetString(key)
	assert.Equal(t, value, result, "GetString should return expected value")

	mockSession.On("PutString", key, value).Return(mockSession).Once()
	result2 := mockSession.PutString(key, value)
	assert.Equal(t, mockSession, result2, "PutString should return session for chaining")

	// Test GetString with empty result
	mockSession.On("GetString", "not_found").Return("").Once()
	result = mockSession.GetString("not_found")
	assert.Empty(t, result, "GetString should return empty string for not found key")

	// Verify all expectations
	mockSession.AssertExpectations(t)
}

func TestMockSession_IntegerMethods(t *testing.T) {
	mockSession := NewMockSession()
	key := "counter"
	value := int64(42)

	// Test GetInt64 and PutInt64 methods
	mockSession.On("GetInt64", key).Return(value).Once()
	result := mockSession.GetInt64(key)
	assert.Equal(t, value, result, "GetInt64 should return expected value")

	mockSession.On("PutInt64", key, value).Return(mockSession).Once()
	result2 := mockSession.PutInt64(key, value)
	assert.Equal(t, mockSession, result2, "PutInt64 should return session for chaining")

	// Test GetInt64 with zero result
	mockSession.On("GetInt64", "not_found").Return(int64(0)).Once()
	result = mockSession.GetInt64("not_found")
	assert.Equal(t, int64(0), result, "GetInt64 should return zero for not found key")

	// Verify all expectations
	mockSession.AssertExpectations(t)
}

func TestMockSession_StructMethods(t *testing.T) {
	mockSession := NewMockSession()
	key := "user_data"

	type TestStruct struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	testData := TestStruct{Name: "Alice", Age: 30}

	// Test GetStruct method (void method, just verify it's called)
	var capturedObj TestStruct
	mockSession.On("GetStruct", key, mock.AnythingOfType("*httpsession.TestStruct")).Once()
	mockSession.GetStruct(key, &capturedObj)

	// Test PutStruct method
	mockSession.On("PutStruct", key, testData).Return(mockSession).Once()
	result := mockSession.PutStruct(key, testData)
	assert.Equal(t, mockSession, result, "PutStruct should return session for chaining")

	// Verify all expectations
	mockSession.AssertExpectations(t)
}

func TestMockSession_ManagementMethods(t *testing.T) {
	mockSession := NewMockSession()
	key := "temp_data"

	// Test Clear method
	mockSession.On("Clear").Return(mockSession).Once()
	result := mockSession.Clear()
	assert.Equal(t, mockSession, result, "Clear should return session for chaining")

	// Test Delete method
	mockSession.On("Delete", key).Once()
	mockSession.Delete(key)

	// Test Save method - success
	mockSession.On("Save").Return(nil).Once()
	err := mockSession.Save()
	assert.NoError(t, err, "Save should return no error on success")

	// Test Save method - error
	expectedError := assert.AnError
	mockSession.On("Save").Return(expectedError).Once()
	err = mockSession.Save()
	assert.Equal(t, expectedError, err, "Save should return expected error")

	// Test Reload method
	mockSession.On("Reload").Return(mockSession).Once()
	result = mockSession.Reload()
	assert.Equal(t, mockSession, result, "Reload should return session")

	// Verify all expectations
	mockSession.AssertExpectations(t)
}

func TestMockSession_TimeAndExpiration(t *testing.T) {
	mockSession := NewMockSession()
	createdTime := time.Now().Add(-time.Hour)
	updatedTime := time.Now().Add(-time.Minute)
	expireTime := time.Now().Add(time.Hour)

	// Test timestamp methods
	mockSession.On("Created").Return(createdTime).Once()
	result := mockSession.Created()
	assert.Equal(t, createdTime, result, "Created should return expected creation time")

	mockSession.On("Updated").Return(updatedTime).Once()
	result = mockSession.Updated()
	assert.Equal(t, updatedTime, result, "Updated should return expected update time")

	mockSession.On("Expire").Return(expireTime).Once()
	result = mockSession.Expire()
	assert.Equal(t, expireTime, result, "Expire should return expected expiration time")

	// Test SetExpire method
	newExpireTime := time.Now().Add(2 * time.Hour)
	mockSession.On("SetExpire", newExpireTime).Return(mockSession).Once()
	result2 := mockSession.SetExpire(newExpireTime)
	assert.Equal(t, mockSession, result2, "SetExpire should return session for chaining")

	// Test IsExpire method
	mockSession.On("IsExpire").Return(false).Once()
	isExpired := mockSession.IsExpire()
	assert.False(t, isExpired, "IsExpire should return false for non-expired session")

	mockSession.On("IsExpire").Return(true).Once()
	isExpired = mockSession.IsExpire()
	assert.True(t, isExpired, "IsExpire should return true for expired session")

	// Verify all expectations
	mockSession.AssertExpectations(t)
}

func TestMockSession_DataMethod(t *testing.T) {
	mockSession := NewMockSession()
	expectedData := map[string]string{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}

	// Test Data method
	mockSession.On("Data").Return(expectedData).Once()
	result := mockSession.Data()
	assert.Equal(t, expectedData, result, "Data should return expected data map")

	// Test Data method with nil return
	mockSession.On("Data").Return(nil).Once()
	result = mockSession.Data()
	assert.Nil(t, result, "Data should return nil when no data")

	// Verify all expectations
	mockSession.AssertExpectations(t)
}

func TestMockSession_ChainedOperations(t *testing.T) {
	mockSession := NewMockSession()

	// Test method chaining scenario
	mockSession.On("PutString", "name", "Alice").Return(mockSession).Once()
	mockSession.On("PutInt64", "age", int64(25)).Return(mockSession).Once()
	mockSession.On("SetExpire", mock.AnythingOfType("time.Time")).Return(mockSession).Once()
	mockSession.On("Save").Return(nil).Once()

	// Execute chained operations
	result := mockSession.PutString("name", "Alice")
	assert.Equal(t, mockSession, result)

	result = result.PutInt64("age", 25)
	assert.Equal(t, mockSession, result)

	result = result.SetExpire(time.Now().Add(time.Hour))
	assert.Equal(t, mockSession, result)

	err := result.Save()
	assert.NoError(t, err)

	// Verify all expectations
	mockSession.AssertExpectations(t)
}
