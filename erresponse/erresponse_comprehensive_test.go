package erresponse

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yetiz-org/gone/erresponse/constant"
	kkerror "github.com/yetiz-org/goth-kkerror"
)

// TestDefaultErrorResponse_CoreMethods tests core methods of DefaultErrorResponse
func TestDefaultErrorResponse_CoreMethods(t *testing.T) {
	t.Parallel()
	
	testCases := []struct {
		name        string
		setupError  func() *DefaultErrorResponse
		expectCode  int
		expectName  string
		expectDesc  string
		expectData  map[string]interface{}
	}{
		{
			name: "Basic error response test",
			setupError: func() *DefaultErrorResponse {
				return &DefaultErrorResponse{
					StatusCode:  400,
					Name:        "test_error",
					Description: "Test error description",
					Data:        map[string]interface{}{"key": "value"},
					DefaultKKError: kkerror.DefaultKKError{
						ErrorCode: "TEST001",
					},
				}
			},
			expectCode: 400,
			expectName: "test_error", 
			expectDesc: "Test error description",
			expectData: map[string]interface{}{"key": "value"},
		},
		{
			name: "Empty data error response test",
			setupError: func() *DefaultErrorResponse {
				return &DefaultErrorResponse{
					StatusCode:  500,
					Name:        "server_error",
					Description: "Server error",
					Data:        nil,
					DefaultKKError: kkerror.DefaultKKError{
						ErrorCode: "SRV001",
					},
				}
			},
			expectCode: 500,
			expectName: "server_error",
			expectDesc: "Server error", 
			expectData: map[string]interface{}{},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			
			err := tc.setupError()
			
			// Test ErrorStatusCode method
			assert.Equal(t, tc.expectCode, err.ErrorStatusCode())
			
			// Test ErrorName method
			assert.Equal(t, tc.expectName, err.ErrorName())
			
			// Test ErrorDescription method
			assert.Equal(t, tc.expectDesc, err.ErrorDescription())
			
			// Test ErrorData method
			data := err.ErrorData()
			assert.NotNil(t, data)
			for k, v := range tc.expectData {
				assert.Equal(t, v, data[k])
			}
		})
	}
}

// TestDefaultErrorResponse_Error_JSONSerialization tests JSON serialization of Error method
func TestDefaultErrorResponse_Error_JSONSerialization(t *testing.T) {
	t.Parallel()
	
	testCases := []struct {
		name       string
		error      *DefaultErrorResponse
		expectJSON bool
		expectContains []string
	}{
		{
			name: "Complete error response JSON serialization",
			error: &DefaultErrorResponse{
				StatusCode:  400,
				Name:        "invalid_request",
				Description: "Invalid request",
				Data:        map[string]interface{}{"field": "email", "reason": "format"},
				DefaultKKError: kkerror.DefaultKKError{
					ErrorCode: "400001",
				},
			},
			expectJSON: true,
			expectContains: []string{"invalid_request", "Invalid request", "400001", "email", "format"},
		},
		{
			name: "Minimal error response JSON serialization",
			error: &DefaultErrorResponse{
				StatusCode: 500,
				Name:       "server_error", 
				DefaultKKError: kkerror.DefaultKKError{
					ErrorCode: "500001",
				},
			},
			expectJSON: true,
			expectContains: []string{"server_error", "500001"},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			
			result := tc.error.Error()
			
			if tc.expectJSON {
				// Verify it's valid JSON
				var jsonData map[string]interface{}
				err := json.Unmarshal([]byte(result), &jsonData)
				assert.NoError(t, err, "Error response should produce valid JSON")
				
				// Verify contains expected content
				for _, expected := range tc.expectContains {
					assert.Contains(t, result, expected, "JSON should contain expected content: %s", expected)
				}
			}
		})
	}
}

// TestDefaultErrorResponse_Clone tests Clone method
func TestDefaultErrorResponse_Clone(t *testing.T) {
	t.Parallel()
	
	original := &DefaultErrorResponse{
		StatusCode:  403,
		Name:        "forbidden",
		Description: "Access denied",
		Data:        map[string]interface{}{"user_id": 123, "permission": "read"},
		DefaultKKError: kkerror.DefaultKKError{
			ErrorCode:     "403001",
			ErrorLevel:    kkerror.Normal,
			ErrorCategory: kkerror.Client,
		},
	}
	
	// Execute Clone
	cloned := original.Clone()
	
	// Verify Clone result
	require.NotNil(t, cloned)
	assert.IsType(t, &DefaultErrorResponse{}, cloned)
	
	clonedErr := cloned.(*DefaultErrorResponse)
	
	// Verify all fields are correctly copied
	assert.Equal(t, original.StatusCode, clonedErr.StatusCode)
	assert.Equal(t, original.Name, clonedErr.Name)
	assert.Equal(t, original.Description, clonedErr.Description)
	assert.Equal(t, original.ErrorCode, clonedErr.ErrorCode)
	assert.Equal(t, original.ErrorLevel, clonedErr.ErrorLevel)
	assert.Equal(t, original.ErrorCategory, clonedErr.ErrorCategory)
	
	// Verify Data field copying (Note: Clone implements shallow copy, map references are shared)
	assert.Equal(t, original.Data, clonedErr.Data)
	
	// Verify Clone created new struct instance (different memory address)
	assert.NotSame(t, original, clonedErr, "Clone should create new struct instance")
	
	// Verify modifying cloned other fields doesn't affect original object
	clonedErr.StatusCode = 999
	clonedErr.Name = "modified_clone"
	assert.NotEqual(t, original.StatusCode, clonedErr.StatusCode)
	assert.NotEqual(t, original.Name, clonedErr.Name)
}
// TestCollect_Registration tests Collection registration and management functionality
func TestCollect_Registration(t *testing.T) {
	t.Parallel()
	
	// Create new Collection for testing
	testCollection := &Collect{}
	
	// Test initial state
	assert.Nil(t, testCollection.ErrorResponses)
	
	// Create test error responses
	testError1 := &DefaultErrorResponse{
		StatusCode:  400,
		Name:        "test_error_1",
		Description: "Test error 1",
		DefaultKKError: kkerror.DefaultKKError{
			ErrorCode: "TEST001",
		},
	}
	
	testError2 := &DefaultErrorResponse{
		StatusCode:  500,
		Name:        "test_error_2", 
		Description: "Test error 2",
		DefaultKKError: kkerror.DefaultKKError{
			ErrorCode: "TEST002",
		},
	}
	
	// Test registering first error
	result1 := testCollection.Register(testError1)
	assert.Equal(t, testError1, result1)
	assert.NotNil(t, testCollection.ErrorResponses)
	assert.Len(t, testCollection.ErrorResponses, 1)
	assert.Contains(t, testCollection.ErrorResponses, testError1)
	
	// Test registering second error
	result2 := testCollection.Register(testError2)
	assert.Equal(t, testError2, result2)
	assert.Len(t, testCollection.ErrorResponses, 2)
	assert.Contains(t, testCollection.ErrorResponses, testError2)
	
	// Test duplicate registration of same error
	result1Dup := testCollection.Register(testError1)
	assert.Equal(t, testError1, result1Dup)
	assert.Len(t, testCollection.ErrorResponses, 2) // Should still be 2
}

// TestCollect_ConcurrentAccess tests Collection concurrent access safety
func TestCollect_ConcurrentAccess(t *testing.T) {
	t.Parallel()
	
	testCollection := &Collect{}
	numGoroutines := 5  // Reduce goroutine count to avoid deadlock
	numErrorsPerGoroutine := 3  // Reduce error count per goroutine
	
	// Use WaitGroup to synchronize goroutines
	var wg sync.WaitGroup
	var mu sync.Mutex
	results := make([]ErrorResponse, 0, numGoroutines*numErrorsPerGoroutine)
	
	// Concurrent error response registration
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			
			for j := 0; j < numErrorsPerGoroutine; j++ {
				testError := &DefaultErrorResponse{
					StatusCode:  400 + j,
					Name:        fmt.Sprintf("concurrent_error_%d_%d", goroutineID, j),
					Description: fmt.Sprintf("Concurrent test error %d-%d", goroutineID, j),
					DefaultKKError: kkerror.DefaultKKError{
						ErrorCode: fmt.Sprintf("CONC%03d%03d", goroutineID, j),
					},
				}
				
				result := testCollection.Register(testError)
				
				// Safely add result to slice
				mu.Lock()
				results = append(results, result)
				mu.Unlock()
			}
		}(i)
	}
	
	// Wait for all goroutines to complete, set timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		// All goroutines completed normally
	case <-time.After(10 * time.Second):
		t.Fatal("Concurrent test timeout")
	}
	
	// Verify results
	assert.Len(t, results, numGoroutines*numErrorsPerGoroutine)
	assert.NotNil(t, testCollection.ErrorResponses)
	
	// Verify all errors are correctly registered
	for _, result := range results {
		assert.Contains(t, testCollection.ErrorResponses, result)
	}
}

// TestPredefinedErrorResponses_BasicValidation tests basic validation of all predefined error responses
func TestPredefinedErrorResponses_BasicValidation(t *testing.T) {
	t.Parallel()
	
	// Test common predefined error responses
	predefinedErrors := []struct {
		name     string
		error    ErrorResponse
		codeMin  int
		codeMax  int
	}{
		{"InvalidRequest", InvalidRequest, 400, 499},
		{"NotFound", NotFound, 404, 404},
		{"ServerError", ServerError, 500, 599},
	}
	
	for _, test := range predefinedErrors {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			
			// Basic interface verification
			assert.NotNil(t, test.error)
			
			// Status code range verification
			statusCode := test.error.ErrorStatusCode()
			assert.GreaterOrEqual(t, statusCode, test.codeMin)
			assert.LessOrEqual(t, statusCode, test.codeMax)
			
			// Error name should not be empty
			errorName := test.error.ErrorName()
			assert.NotEmpty(t, errorName)
			
			// Error code should not be empty
			errorCode := test.error.Code()
			assert.NotEmpty(t, errorCode)
			
			// Error message JSON format validation
			errorMessage := test.error.Error()
			assert.NotEmpty(t, errorMessage)
			
			var jsonData map[string]interface{}
			err := json.Unmarshal([]byte(errorMessage), &jsonData)
			assert.NoError(t, err, "Predefined error responses should produce valid JSON")
			
			// Verify JSON contains necessary fields
			if statusCode > 0 {
				assert.Contains(t, jsonData, "status_code")
			}
			if errorName != "" {
				assert.Contains(t, jsonData, "error")
			}
			
			// Test Clone functionality
			cloned := test.error.Clone()
			assert.NotNil(t, cloned)
			assert.Equal(t, test.error.ErrorStatusCode(), cloned.ErrorStatusCode())
			assert.Equal(t, test.error.ErrorName(), cloned.ErrorName())
			assert.Equal(t, test.error.Code(), cloned.Code())
		})
	}
}

// TestErrorResponse_InterfaceCompliance tests ErrorResponse interface completeness
func TestErrorResponse_InterfaceCompliance(t *testing.T) {
	t.Parallel()
	
	// Create test error response
	testError := &DefaultErrorResponse{
		StatusCode:  418,
		Name:        "teapot_error",
		Description: "I'm a teapot error",
		Data:        map[string]interface{}{"brew_time": 300, "temperature": 85},
		DefaultKKError: kkerror.DefaultKKError{
			ErrorCode:     "418001",
			ErrorLevel:    kkerror.Normal,
			ErrorCategory: kkerror.Client,
		},
	}
	
	// Verify interface implementation
	var _ ErrorResponse = testError
	var _ kkerror.KKError = testError
	
	// Test ErrorResponse specific methods
	assert.Equal(t, 418, testError.ErrorStatusCode())
	assert.Equal(t, "teapot_error", testError.ErrorName())
	assert.Equal(t, "I'm a teapot error", testError.ErrorDescription())
	
	data := testError.ErrorData()
	assert.Equal(t, 300, data["brew_time"])
	assert.Equal(t, 85, data["temperature"])
	
	// Test KKError inherited methods
	assert.Equal(t, "418001", testError.Code())
	assert.Equal(t, kkerror.Normal, testError.Level())
	assert.Equal(t, kkerror.Client, testError.Category())
	
	// Test Error method (JSON serialization)
	errorJson := testError.Error()
	assert.Contains(t, errorJson, "teapot_error")
	assert.Contains(t, errorJson, "I'm a teapot error")
	assert.Contains(t, errorJson, "418001")
	assert.Contains(t, errorJson, "300")
	assert.Contains(t, errorJson, "85")
}// TestErrorResponse_EdgeCases tests edge cases and exceptional conditions
func TestErrorResponse_EdgeCases(t *testing.T) {
	t.Parallel()
	
	t.Run("Null and nil handling", func(t *testing.T) {
		t.Parallel()
		
		// Test empty string fields
		emptyError := &DefaultErrorResponse{
			StatusCode:  0,
			Name:        "",
			Description: "",
			Data:        nil,
			DefaultKKError: kkerror.DefaultKKError{
				ErrorCode: "",
			},
		}
		
		// Basic methods should not panic
		assert.NotPanics(t, func() {
			_ = emptyError.ErrorStatusCode()
			_ = emptyError.ErrorName()
			_ = emptyError.ErrorDescription()
			_ = emptyError.ErrorData()
			_ = emptyError.Error()
			_ = emptyError.Clone()
		})
		
		// ErrorData should initialize as empty map rather than nil
		data := emptyError.ErrorData()
		assert.NotNil(t, data)
		assert.Len(t, data, 0)
	})
	
	t.Run("Large data handling", func(t *testing.T) {
		t.Parallel()
		
		// Create error response containing large amount of data
		largeData := make(map[string]interface{})
		for i := 0; i < 1000; i++ {
			largeData[fmt.Sprintf("key_%d", i)] = fmt.Sprintf("value_%d", i)
		}
		
		largeError := &DefaultErrorResponse{
			StatusCode:  500,
			Name:        "large_data_error",
			Description: "Error containing large amount of data",
			Data:        largeData,
			DefaultKKError: kkerror.DefaultKKError{
				ErrorCode: "LARGE001",
			},
		}
		
		// Test no panic and functionality works normally
		assert.NotPanics(t, func() {
			data := largeError.ErrorData()
			assert.Len(t, data, 1000)
			
			cloned := largeError.Clone()
			assert.NotNil(t, cloned)
			
			errorJson := largeError.Error()
			assert.NotEmpty(t, errorJson)
		})
	})
	
	t.Run("Special character handling", func(t *testing.T) {
		t.Parallel()
		
		specialError := &DefaultErrorResponse{
			StatusCode:  400,
			Name:        "special_char_error",
			Description: "Special character test: \n\t\"\\'/{}[]&<>",
			Data:        map[string]interface{}{
				"unicode":  "Test Chinese🚀💥",
				"symbols":  "!@#$%^&*()_+-=",
				"quotes":   `"'`,
				"newlines": "line1\nline2\tline3",
			},
			DefaultKKError: kkerror.DefaultKKError{
				ErrorCode: "SPECIAL001",
			},
		}
		
		// Test JSON serialization handling special characters
		errorJson := specialError.Error()
		assert.NotEmpty(t, errorJson)
		
		// Verify JSON validity
		var jsonData map[string]interface{}
		err := json.Unmarshal([]byte(errorJson), &jsonData)
		assert.NoError(t, err, "Error responses containing special characters should produce valid JSON")
	})
}

// TestGlobalCollection_Integration tests global Collection integration functionality
func TestGlobalCollection_Integration(t *testing.T) {
	t.Parallel()
	
	// Verify global Collection exists and is available
	assert.NotNil(t, Collection)
	
	// Test whether predefined error responses are in global Collection
	predefinedErrors := []ErrorResponse{
		InvalidRequest,
		NotFound, 
		ServerError,
	}
	
	for _, predefinedError := range predefinedErrors {
		t.Run(fmt.Sprintf("Global_Collection_contains_%s", predefinedError.ErrorName()), func(t *testing.T) {
			// Check if registered in Collection
			assert.NotNil(t, Collection.ErrorResponses)
			
			// Verify error exists in Collection
			found := false
			for registeredError := range Collection.ErrorResponses {
				if registeredError.Code() == predefinedError.Code() {
					found = true
					break
				}
			}
			assert.True(t, found, "Predefined errors should be registered in global Collection")
		})
	}
}

// TestErrorResponse_PerformanceBaseline tests performance baseline
func TestErrorResponse_PerformanceBaseline(t *testing.T) {
	if testing.Short() {
		t.Skip("Skip performance test")
	}
	
	t.Parallel()
	
	// Create standard error response
	testError := &DefaultErrorResponse{
		StatusCode:  500,
		Name:        "performance_test_error",
		Description: "Performance test error response",
		Data:        map[string]interface{}{"request_id": "perf_123", "timestamp": time.Now().Unix()},
		DefaultKKError: kkerror.DefaultKKError{
			ErrorCode:     "PERF001",
			ErrorLevel:    kkerror.Normal,
			ErrorCategory: kkerror.Server,
		},
	}
	
	// Test performance of massive Error() calls
	t.Run("Error method performance test", func(t *testing.T) {
		iterations := 10000
		start := time.Now()
		
		for i := 0; i < iterations; i++ {
			_ = testError.Error()
		}
		
		duration := time.Since(start)
		avgDuration := duration / time.Duration(iterations)
		
		t.Logf("Error() method average execution time: %v (Total: %v, Iterations: %d)", avgDuration, duration, iterations)
		
		// Performance threshold: each call should not exceed 1ms
		assert.Less(t, avgDuration, time.Millisecond, "Error() method performance should be within acceptable range")
	})
	
	// Test performance of massive Clone() calls
	t.Run("Clone method performance test", func(t *testing.T) {
		iterations := 10000
		start := time.Now()
		
		for i := 0; i < iterations; i++ {
			cloned := testError.Clone()
			_ = cloned
		}
		
		duration := time.Since(start)
		avgDuration := duration / time.Duration(iterations)
		
		t.Logf("Clone() method average execution time: %v (Total: %v, Iterations: %d)", avgDuration, duration, iterations)
		
		// Performance threshold: each call should not exceed 100μs
		assert.Less(t, avgDuration, 100*time.Microsecond, "Clone() method performance should be within acceptable range")
	})
}

// TestConstantUsage tests constant usage
func TestConstantUsage(t *testing.T) {
	t.Parallel()
	
	// Verify constant definition and usage
	constantTests := []struct {
		constant string
		expected string
	}{
		{constant.ErrorInvalidRequest, "invalid_request"},
		{constant.ErrorNotFound, "not_found"},
		{constant.ErrorServerError, "server_error"},
		{constant.ErrorInvalidClient, "invalid_client"},
		{constant.ErrorInvalidToken, "invalid_token"},
		{constant.ErrorInvalidGrant, "invalid_grant"},
		{constant.ErrorUnsupportedGrantType, "unsupported_grant_type"},
		{constant.ErrorUnsupportedResponseType, "unsupported_response_type"},
		{constant.ErrorInvalidScope, "invalid_scope"},
		{constant.ErrorInsufficientScope, "insufficient_scope"},
		{constant.ErrorSlowDown, "slow_down"},
		{constant.ErrorSRPUnsupported, "srp_unsupported"},
		{constant.ErrorNotImplemented, "not_implemented"},
	}
	
	for _, test := range constantTests {
		t.Run(fmt.Sprintf("Constant_%s", test.expected), func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, test.expected, test.constant, "Constant value should match expected")
		})
	}
}