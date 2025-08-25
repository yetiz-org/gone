package httpsession

import (
	"encoding/json"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
)

// Use the unified MockSessionProvider from mock_session.go

func TestNewDefaultSession(t *testing.T) {
	provider := NewMockSessionProvider()
	session := NewDefaultSession(provider)

	if session == nil {
		t.Fatal("NewDefaultSession() should not return nil")
	}

	if session.Id() == "" {
		t.Error("Session ID should not be empty")
	}

	if session.Created().IsZero() {
		t.Error("Created time should not be zero")
	}

	if session.Updated().IsZero() {
		t.Error("Updated time should not be zero")
	}

	if session.Expire().Unix() != math.MaxInt64 {
		t.Errorf("Default expire time should be MaxInt64, but got %d", session.Expire().Unix())
	}

	if session.Data() == nil {
		t.Error("Data map should not be nil")
	}

	if len(session.Data()) != 0 {
		t.Error("New session should have empty data map")
	}
}

func TestDefaultSession_Id(t *testing.T) {
	provider := NewMockSessionProvider()
	session1 := NewDefaultSession(provider)
	session2 := NewDefaultSession(provider)

	if session1.Id() == session2.Id() {
		t.Error("Different sessions should have different IDs")
	}

	if len(session1.Id()) == 0 {
		t.Error("Session ID should not be empty")
	}
}

func TestDefaultSession_PutString_GetString(t *testing.T) {
	provider := NewMockSessionProvider()
	session := NewDefaultSession(provider)

	// Test putting and getting string values
	result := session.PutString("key1", "value1")
	if result != session {
		t.Error("PutString should return the same session instance")
	}

	value := session.GetString("key1")
	if value != "value1" {
		t.Errorf("Expected 'value1', but got '%s'", value)
	}

	// Test getting non-existent key
	value = session.GetString("nonexistent")
	if value != "" {
		t.Errorf("Non-existent key should return empty string, but got '%s'", value)
	}

	// Test overwriting existing key
	session.PutString("key1", "new_value")
	value = session.GetString("key1")
	if value != "new_value" {
		t.Errorf("Expected 'new_value', but got '%s'", value)
	}
}

func TestDefaultSession_PutInt64_GetInt64(t *testing.T) {
	provider := NewMockSessionProvider()
	session := NewDefaultSession(provider)

	// Test putting and getting int64 values
	result := session.PutInt64("age", 25)
	if result != session {
		t.Error("PutInt64 should return the same session instance")
	}

	value := session.GetInt64("age")
	if value != 25 {
		t.Errorf("Expected 25, but got %d", value)
	}

	// Test getting non-existent key
	value = session.GetInt64("nonexistent")
	if value != 0 {
		t.Errorf("Non-existent key should return 0, but got %d", value)
	}

	// Test negative numbers
	session.PutInt64("negative", -100)
	value = session.GetInt64("negative")
	if value != -100 {
		t.Errorf("Expected -100, but got %d", value)
	}

	// Test large numbers
	largeNum := int64(9223372036854775807) // max int64
	session.PutInt64("large", largeNum)
	value = session.GetInt64("large")
	if value != largeNum {
		t.Errorf("Expected %d, but got %d", largeNum, value)
	}

	// Test invalid string data (simulate corrupted data)
	session.PutString("invalid_int", "not_a_number")
	value = session.GetInt64("invalid_int")
	if value != 0 {
		t.Errorf("Invalid int string should return 0, but got %d", value)
	}
}

func TestDefaultSession_PutStruct_GetStruct(t *testing.T) {
	provider := NewMockSessionProvider()
	session := NewDefaultSession(provider)

	// Test with a simple struct
	type Person struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	original := Person{Name: "John", Age: 30}
	result := session.PutStruct("person", original)
	if result != session {
		t.Error("PutStruct should return the same session instance")
	}

	var retrieved Person
	session.GetStruct("person", &retrieved)

	if retrieved.Name != original.Name {
		t.Errorf("Expected name '%s', but got '%s'", original.Name, retrieved.Name)
	}

	if retrieved.Age != original.Age {
		t.Errorf("Expected age %d, but got %d", original.Age, retrieved.Age)
	}

	// Test with nil value
	var nilPerson Person
	session.GetStruct("nonexistent", &nilPerson)
	if nilPerson.Name != "" || nilPerson.Age != 0 {
		t.Error("Getting non-existent struct should not modify the target object")
	}

	// Test with complex nested struct
	type Address struct {
		Street string `json:"street"`
		City   string `json:"city"`
	}

	type User struct {
		Name    string   `json:"name"`
		Address Address  `json:"address"`
		Tags    []string `json:"tags"`
	}

	originalUser := User{
		Name: "Alice",
		Address: Address{
			Street: "123 Main St",
			City:   "New York",
		},
		Tags: []string{"admin", "user"},
	}

	session.PutStruct("user", originalUser)
	var retrievedUser User
	session.GetStruct("user", &retrievedUser)

	if retrievedUser.Name != originalUser.Name {
		t.Errorf("Expected name '%s', but got '%s'", originalUser.Name, retrievedUser.Name)
	}

	if retrievedUser.Address.City != originalUser.Address.City {
		t.Errorf("Expected city '%s', but got '%s'", originalUser.Address.City, retrievedUser.Address.City)
	}

	if len(retrievedUser.Tags) != len(originalUser.Tags) {
		t.Errorf("Expected %d tags, but got %d", len(originalUser.Tags), len(retrievedUser.Tags))
	}
}

func TestDefaultSession_Clear(t *testing.T) {
	provider := NewMockSessionProvider()
	session := NewDefaultSession(provider)

	// Add some data
	session.PutString("key1", "value1")
	session.PutInt64("key2", 42)
	session.PutStruct("key3", map[string]string{"test": "data"})

	// Verify data exists
	if len(session.Data()) != 3 {
		t.Errorf("Expected 3 data items, but got %d", len(session.Data()))
	}

	// Clear the session
	result := session.Clear()
	if result != session {
		t.Error("Clear should return the same session instance")
	}

	// Verify data is cleared
	if len(session.Data()) != 0 {
		t.Errorf("After clear, session should have no data, but got %d items", len(session.Data()))
	}

	// Verify specific keys are gone
	if session.GetString("key1") != "" {
		t.Error("After clear, string value should be empty")
	}

	if session.GetInt64("key2") != 0 {
		t.Error("After clear, int64 value should be 0")
	}
}

func TestDefaultSession_Delete(t *testing.T) {
	provider := NewMockSessionProvider()
	session := NewDefaultSession(provider)

	// Add some data
	session.PutString("key1", "value1")
	session.PutString("key2", "value2")
	session.PutString("key3", "value3")

	// Verify data exists
	if len(session.Data()) != 3 {
		t.Errorf("Expected 3 data items, but got %d", len(session.Data()))
	}

	// Delete one key
	session.Delete("key2")

	// Verify key is deleted
	if session.GetString("key2") != "" {
		t.Error("Deleted key should return empty string")
	}

	// Verify other keys still exist
	if session.GetString("key1") != "value1" {
		t.Error("Other keys should remain unchanged")
	}

	if session.GetString("key3") != "value3" {
		t.Error("Other keys should remain unchanged")
	}

	if len(session.Data()) != 2 {
		t.Errorf("After deleting one key, should have 2 items, but got %d", len(session.Data()))
	}

	// Delete non-existent key (should not cause error)
	session.Delete("nonexistent")
	if len(session.Data()) != 2 {
		t.Error("Deleting non-existent key should not affect data count")
	}
}

func TestDefaultSession_SetExpire_Expire_IsExpire(t *testing.T) {
	provider := NewMockSessionProvider()
	session := NewDefaultSession(provider)

	// Test default expiration (should be MaxInt64)
	if session.IsExpire() {
		t.Error("New session should not be expired by default")
	}

	// Set expiration to future
	futureTime := time.Now().Add(1 * time.Hour)
	result := session.SetExpire(futureTime)
	if result != session {
		t.Error("SetExpire should return the same session instance")
	}

	if session.Expire().Unix() != futureTime.Unix() {
		t.Errorf("Expected expire time %d, but got %d", futureTime.Unix(), session.Expire().Unix())
	}

	if session.IsExpire() {
		t.Error("Session with future expiration should not be expired")
	}

	// Set expiration to past
	pastTime := time.Now().Add(-1 * time.Hour)
	session.SetExpire(pastTime)

	if !session.IsExpire() {
		t.Error("Session with past expiration should be expired")
	}

	// Test edge case: exactly now
	nowTime := time.Now()
	session.SetExpire(nowTime)
	// Due to execution time, this might be expired immediately
	// We'll just verify the time was set correctly
	if session.Expire().Unix() != nowTime.Unix() {
		t.Errorf("Expected expire time %d, but got %d", nowTime.Unix(), session.Expire().Unix())
	}
}

func TestDefaultSession_Created_Updated(t *testing.T) {
	provider := NewMockSessionProvider()

	beforeCreation := time.Now().Add(-1 * time.Second) // Give some buffer
	session := NewDefaultSession(provider)
	afterCreation := time.Now().Add(1 * time.Second) // Give some buffer

	// Test created time
	created := session.Created()
	if created.Before(beforeCreation) || created.After(afterCreation) {
		t.Errorf("Created time %v should be between %v and %v", created, beforeCreation, afterCreation)
	}

	// Test updated time (should be same as created initially)
	updated := session.Updated()
	if updated.Unix() != created.Unix() {
		t.Error("Updated time should be same as created time initially")
	}

	// Note: Updated time is set during creation and doesn't change in the current implementation
	// If the implementation changes to update this timestamp on data changes,
	// additional tests should be added
}

func TestDefaultSession_Data(t *testing.T) {
	provider := NewMockSessionProvider()
	session := NewDefaultSession(provider)

	// Test empty data
	data := session.Data()
	if data == nil {
		t.Error("Data() should not return nil")
	}

	if len(data) != 0 {
		t.Error("New session should have empty data")
	}

	// Add some data
	session.PutString("key1", "value1")
	session.PutInt64("key2", 42)

	data = session.Data()
	if len(data) != 2 {
		t.Errorf("Expected 2 data items, but got %d", len(data))
	}

	if data["key1"] != "value1" {
		t.Errorf("Expected 'value1', but got '%s'", data["key1"])
	}

	if data["key2"] != "42" {
		t.Errorf("Expected '42', but got '%s'", data["key2"])
	}

	// Note: The Data() method returns the actual internal map, not a copy
	// So modifications to the returned map will affect the original session data
	originalLen := len(session.Data())
	data["key3"] = "value3"
	if len(session.Data()) != originalLen+1 {
		t.Error("Data() returns the actual internal map, so modifications should affect the session")
	}
}

func TestDefaultSession_Save(t *testing.T) {
	provider := NewMockSessionProvider()
	session := NewDefaultSession(provider)

	// Set up mock expectation for successful save
	provider.On("Save", session).Return(nil).Once()

	// Test successful save
	err := session.Save()
	if err != nil {
		t.Errorf("Save() should not return error, but got: %v", err)
	}

	// Test save with provider error - create a new provider to avoid mock conflicts
	provider2 := NewMockSessionProvider()
	session2 := NewDefaultSession(provider2)
	expectedErr := &MockError{"save failed"}
	provider2.SetSaveError(expectedErr)

	err = session2.Save()
	if err != expectedErr {
		t.Errorf("Expected save error, but got: %v", err)
	}
}

func TestDefaultSession_Reload(t *testing.T) {
	provider := NewMockSessionProvider()
	session := NewDefaultSession(provider)

	// Set up mock expectation for Save call
	provider.On("Save", session).Return(nil).Once()

	// Set up mock expectation for Session call (used by Reload)
	provider.On("Session", mock.AnythingOfType("string")).Return(session).Maybe()

	// Add some data and save
	session.PutString("key1", "value1")
	session.Save()

	// Modify local data
	session.PutString("key2", "value2")

	// Reload should restore from provider
	result := session.Reload()
	if result != session {
		t.Error("Reload should return the same session instance")
	}

	// After reload, both saved and unsaved data should exist because
	// the Reload method copies data from the provider, but the provider
	// has the session with both keys since local changes affect the same instance
	if session.GetString("key1") != "value1" {
		t.Error("Reloaded session should have saved data")
	}

	// The key2 will still exist because Data() returns the actual map reference
	// and the provider has the same session instance
	if len(session.Data()) == 0 {
		t.Error("Session should have some data after reload")
	}
}

func TestDefaultSession_MarshalJSON(t *testing.T) {
	provider := NewMockSessionProvider()
	session := NewDefaultSession(provider)

	// Set some test data
	session.PutString("name", "test")
	session.PutInt64("age", 25)
	session.SetExpire(time.Unix(1640995200, 0)) // Fixed timestamp for testing

	data, err := session.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON() returned error: %v", err)
	}

	// Parse the JSON to verify structure
	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	if err != nil {
		t.Fatalf("Failed to parse marshaled JSON: %v", err)
	}

	// Verify required fields exist
	requiredFields := []string{"id", "created", "updated", "expired", "data"}
	for _, field := range requiredFields {
		if _, exists := result[field]; !exists {
			t.Errorf("JSON should contain field '%s'", field)
		}
	}

	// Verify data content
	dataMap, ok := result["data"].(map[string]interface{})
	if !ok {
		t.Error("JSON data field should be a map")
	}

	if dataMap["name"] != "test" {
		t.Errorf("Expected name 'test', but got '%v'", dataMap["name"])
	}

	if dataMap["age"] != "25" {
		t.Errorf("Expected age '25', but got '%v'", dataMap["age"])
	}
}

func TestDefaultSession_UnmarshalJSON(t *testing.T) {
	provider := NewMockSessionProvider()
	session := NewDefaultSession(provider)

	// Create test JSON data
	testData := map[string]interface{}{
		"id":      "test-id",
		"created": int64(1640995200),
		"updated": int64(1640995300),
		"expired": int64(1640999999),
		"data": map[string]string{
			"name": "test",
			"age":  "30",
		},
	}

	jsonData, err := json.Marshal(testData)
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}

	// Unmarshal into session
	err = session.UnmarshalJSON(jsonData)
	if err != nil {
		t.Fatalf("UnmarshalJSON() returned error: %v", err)
	}

	// Verify data was correctly unmarshaled
	if session.Id() != "test-id" {
		t.Errorf("Expected ID 'test-id', but got '%s'", session.Id())
	}

	if session.Created().Unix() != 1640995200 {
		t.Errorf("Expected created time 1640995200, but got %d", session.Created().Unix())
	}

	if session.Updated().Unix() != 1640995300 {
		t.Errorf("Expected updated time 1640995300, but got %d", session.Updated().Unix())
	}

	if session.Expire().Unix() != 1640999999 {
		t.Errorf("Expected expire time 1640999999, but got %d", session.Expire().Unix())
	}

	if session.GetString("name") != "test" {
		t.Errorf("Expected name 'test', but got '%s'", session.GetString("name"))
	}

	if session.GetInt64("age") != 30 {
		t.Errorf("Expected age 30, but got %d", session.GetInt64("age"))
	}

	// Test invalid JSON
	err = session.UnmarshalJSON([]byte("invalid json"))
	if err == nil {
		t.Error("UnmarshalJSON() should return error for invalid JSON")
	}
}

// Test that DefaultSession correctly implements Session interface
func TestDefaultSession_ImplementsInterface(t *testing.T) {
	var _ Session = (*DefaultSession)(nil)
}

// MockError for testing error conditions
type MockError struct {
	message string
}

func (e *MockError) Error() string {
	return e.message
}
