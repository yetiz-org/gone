package channel

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestGetParamIntDefault tests GetParamIntDefault function with various types
func TestGetParamIntDefault(t *testing.T) {
	deadline := time.Now().Add(10 * time.Second)
	defaultValue := 42
	
	testCases := []struct {
		name     string
		value    interface{}
		expected int
	}{
		{"nil value", nil, defaultValue},
		{"int8 value", int8(10), 10},
		{"uint8 value", uint8(20), 20},
		{"int16 value", int16(30), 30},
		{"uint16 value", uint16(40), 40},
		{"int32 value", int32(50), 50},
		{"int value", 90, 90},
		{"uint32 unsupported", uint32(60), defaultValue},
		{"int64 unsupported", int64(70), defaultValue},
		{"uint64 unsupported", uint64(80), defaultValue},
		{"uint unsupported", uint(100), defaultValue},
		{"non-numeric value", "not-a-number", defaultValue},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockChannel := NewMockChannel()
			testKey := ParamKey("test-int-key")
			
			mockChannel.On("Param", testKey).Return(tc.value)
			result := GetParamIntDefault(mockChannel, testKey, defaultValue)
			assert.Equal(t, tc.expected, result)
			
			mockChannel.AssertExpectations(t)
		})
	}
	
	if time.Now().After(deadline) {
		t.Fatal("Test exceeded timeout")
	}
}

// TestGetParamInt64Default tests GetParamInt64Default function with various types
func TestGetParamInt64Default(t *testing.T) {
	deadline := time.Now().Add(10 * time.Second)
	defaultValue := int64(1234567890)
	
	testCases := []struct {
		name     string
		value    interface{}
		expected int64
	}{
		{"nil value", nil, defaultValue},
		{"int8 value", int8(10), int64(10)},
		{"int64 value", int64(9876543210), int64(9876543210)},
		{"uint32 value", uint32(2000000000), int64(2000000000)},
		{"non-numeric value", "not-a-number", defaultValue},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockChannel := NewMockChannel()
			testKey := ParamKey("test-int64-key")
			
			mockChannel.On("Param", testKey).Return(tc.value)
			result := GetParamInt64Default(mockChannel, testKey, defaultValue)
			assert.Equal(t, tc.expected, result)
			
			mockChannel.AssertExpectations(t)
		})
	}
	
	if time.Now().After(deadline) {
		t.Fatal("Test exceeded timeout")
	}
}

// TestGetParamStringDefault tests GetParamStringDefault function with various types
func TestGetParamStringDefault(t *testing.T) {
	deadline := time.Now().Add(5 * time.Second)
	defaultValue := "default-string"
	
	testCases := []struct {
		name     string
		value    interface{}
		expected string
	}{
		{"nil value", nil, defaultValue},
		{"string value", "test-value", "test-value"},
		{"non-string value", 12345, defaultValue},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockChannel := NewMockChannel()
			testKey := ParamKey("test-string-key")
			
			mockChannel.On("Param", testKey).Return(tc.value)
			result := GetParamStringDefault(mockChannel, testKey, defaultValue)
			assert.Equal(t, tc.expected, result)
			
			mockChannel.AssertExpectations(t)
		})
	}
	
	if time.Now().After(deadline) {
		t.Fatal("Test exceeded timeout")
	}
}

// TestGetParamBoolDefault tests GetParamBoolDefault function with various types
func TestGetParamBoolDefault(t *testing.T) {
	deadline := time.Now().Add(5 * time.Second)
	defaultValue := true
	
	testCases := []struct {
		name     string
		value    interface{}
		expected bool
	}{
		{"nil value", nil, defaultValue},
		{"bool value false", false, false},
		{"bool value true", true, true},
		{"non-bool value", "not-a-bool", defaultValue},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockChannel := NewMockChannel()
			testKey := ParamKey("test-bool-key")
			
			mockChannel.On("Param", testKey).Return(tc.value)
			result := GetParamBoolDefault(mockChannel, testKey, defaultValue)
			assert.Equal(t, tc.expected, result)
			
			mockChannel.AssertExpectations(t)
		})
	}
	
	if time.Now().After(deadline) {
		t.Fatal("Test exceeded timeout")
	}
}

// TestParamSyncMap tests the sync.Map functionality for parameter storage
func TestParamSyncMap(t *testing.T) {
	deadline := time.Now().Add(5 * time.Second)
	
	// Create a new Params struct
	params := &Params{}
	testKey := ParamKey("test-key")
	testValue := "test-value"
	
	// Test Store and Load
	params.Store(testKey, testValue)
	value, ok := params.Load(testKey)
	assert.True(t, ok)
	assert.Equal(t, testValue, value)
	
	// Test LoadOrStore with existing key
	actualValue, loaded := params.LoadOrStore(testKey, "different-value")
	assert.True(t, loaded)
	assert.Equal(t, testValue, actualValue)
	
	// Test LoadOrStore with new key  
	newValue := "new-value"
	newKey := ParamKey("new-key")
	actualValue, loaded = params.LoadOrStore(newKey, newValue)
	assert.False(t, loaded)
	assert.Equal(t, newValue, actualValue)
	
	// Test Range function
	rangeCount := 0
	params.Range(func(key ParamKey, value any) bool {
		rangeCount++
		assert.True(t, key == testKey || key == newKey)
		return true
	})
	assert.Equal(t, 2, rangeCount)
	
	// Test Delete
	params.Delete(testKey)
	_, ok = params.Load(testKey)
	assert.False(t, ok)
	
	if time.Now().After(deadline) {
		t.Fatal("Test exceeded timeout")
	}
}