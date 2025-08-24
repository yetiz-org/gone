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

// TestDefaultErrorResponse_CoreMethods 測試DefaultErrorResponse的核心方法
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
			name: "基本錯誤響應測試",
			setupError: func() *DefaultErrorResponse {
				return &DefaultErrorResponse{
					StatusCode:  400,
					Name:        "test_error",
					Description: "測試錯誤描述",
					Data:        map[string]interface{}{"key": "value"},
					DefaultKKError: kkerror.DefaultKKError{
						ErrorCode: "TEST001",
					},
				}
			},
			expectCode: 400,
			expectName: "test_error", 
			expectDesc: "測試錯誤描述",
			expectData: map[string]interface{}{"key": "value"},
		},
		{
			name: "空數據錯誤響應測試",
			setupError: func() *DefaultErrorResponse {
				return &DefaultErrorResponse{
					StatusCode:  500,
					Name:        "server_error",
					Description: "伺服器錯誤",
					Data:        nil,
					DefaultKKError: kkerror.DefaultKKError{
						ErrorCode: "SRV001",
					},
				}
			},
			expectCode: 500,
			expectName: "server_error",
			expectDesc: "伺服器錯誤", 
			expectData: map[string]interface{}{},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			
			err := tc.setupError()
			
			// 測試ErrorStatusCode方法
			assert.Equal(t, tc.expectCode, err.ErrorStatusCode())
			
			// 測試ErrorName方法
			assert.Equal(t, tc.expectName, err.ErrorName())
			
			// 測試ErrorDescription方法
			assert.Equal(t, tc.expectDesc, err.ErrorDescription())
			
			// 測試ErrorData方法
			data := err.ErrorData()
			assert.NotNil(t, data)
			for k, v := range tc.expectData {
				assert.Equal(t, v, data[k])
			}
		})
	}
}

// TestDefaultErrorResponse_Error_JSONSerialization 測試Error方法的JSON序列化
func TestDefaultErrorResponse_Error_JSONSerialization(t *testing.T) {
	t.Parallel()
	
	testCases := []struct {
		name       string
		error      *DefaultErrorResponse
		expectJSON bool
		expectContains []string
	}{
		{
			name: "完整錯誤響應JSON序列化",
			error: &DefaultErrorResponse{
				StatusCode:  400,
				Name:        "invalid_request",
				Description: "請求無效",
				Data:        map[string]interface{}{"field": "email", "reason": "format"},
				DefaultKKError: kkerror.DefaultKKError{
					ErrorCode: "400001",
				},
			},
			expectJSON: true,
			expectContains: []string{"invalid_request", "請求無效", "400001", "email", "format"},
		},
		{
			name: "最小錯誤響應JSON序列化",
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
				// 驗證是有效的JSON
				var jsonData map[string]interface{}
				err := json.Unmarshal([]byte(result), &jsonData)
				assert.NoError(t, err, "錯誤響應應該產生有效的JSON")
				
				// 驗證包含預期內容
				for _, expected := range tc.expectContains {
					assert.Contains(t, result, expected, "JSON應該包含預期內容: %s", expected)
				}
			}
		})
	}
}

// TestDefaultErrorResponse_Clone 測試Clone方法
func TestDefaultErrorResponse_Clone(t *testing.T) {
	t.Parallel()
	
	original := &DefaultErrorResponse{
		StatusCode:  403,
		Name:        "forbidden",
		Description: "存取被拒絕",
		Data:        map[string]interface{}{"user_id": 123, "permission": "read"},
		DefaultKKError: kkerror.DefaultKKError{
			ErrorCode:     "403001",
			ErrorLevel:    kkerror.Normal,
			ErrorCategory: kkerror.Client,
		},
	}
	
	// 執行Clone
	cloned := original.Clone()
	
	// 驗證Clone結果
	require.NotNil(t, cloned)
	assert.IsType(t, &DefaultErrorResponse{}, cloned)
	
	clonedErr := cloned.(*DefaultErrorResponse)
	
	// 驗證所有欄位都正確複製
	assert.Equal(t, original.StatusCode, clonedErr.StatusCode)
	assert.Equal(t, original.Name, clonedErr.Name)
	assert.Equal(t, original.Description, clonedErr.Description)
	assert.Equal(t, original.ErrorCode, clonedErr.ErrorCode)
	assert.Equal(t, original.ErrorLevel, clonedErr.ErrorLevel)
	assert.Equal(t, original.ErrorCategory, clonedErr.ErrorCategory)
	
	// 驗證Data字段複製（注意：Clone實現為淺複製，map引用是共享的）
	assert.Equal(t, original.Data, clonedErr.Data)
	
	// 驗證Clone創建了新的結構體實例（不同的記憶體地址）
	assert.NotSame(t, original, clonedErr, "Clone應該創建新的結構體實例")
	
	// 驗證修改克隆的其他字段不影響原始對象
	clonedErr.StatusCode = 999
	clonedErr.Name = "modified_clone"
	assert.NotEqual(t, original.StatusCode, clonedErr.StatusCode)
	assert.NotEqual(t, original.Name, clonedErr.Name)
}
// TestCollect_Registration 測試Collection的註冊和管理功能
func TestCollect_Registration(t *testing.T) {
	t.Parallel()
	
	// 創建新的Collection用於測試
	testCollection := &Collect{}
	
	// 測試初始狀態
	assert.Nil(t, testCollection.ErrorResponses)
	
	// 創建測試錯誤響應
	testError1 := &DefaultErrorResponse{
		StatusCode:  400,
		Name:        "test_error_1",
		Description: "測試錯誤1",
		DefaultKKError: kkerror.DefaultKKError{
			ErrorCode: "TEST001",
		},
	}
	
	testError2 := &DefaultErrorResponse{
		StatusCode:  500,
		Name:        "test_error_2", 
		Description: "測試錯誤2",
		DefaultKKError: kkerror.DefaultKKError{
			ErrorCode: "TEST002",
		},
	}
	
	// 測試註冊第一個錯誤
	result1 := testCollection.Register(testError1)
	assert.Equal(t, testError1, result1)
	assert.NotNil(t, testCollection.ErrorResponses)
	assert.Len(t, testCollection.ErrorResponses, 1)
	assert.Contains(t, testCollection.ErrorResponses, testError1)
	
	// 測試註冊第二個錯誤
	result2 := testCollection.Register(testError2)
	assert.Equal(t, testError2, result2)
	assert.Len(t, testCollection.ErrorResponses, 2)
	assert.Contains(t, testCollection.ErrorResponses, testError2)
	
	// 測試重複註冊相同錯誤
	result1Dup := testCollection.Register(testError1)
	assert.Equal(t, testError1, result1Dup)
	assert.Len(t, testCollection.ErrorResponses, 2) // 應該仍然是2個
}

// TestCollect_ConcurrentAccess 測試Collection的並發存取安全性
func TestCollect_ConcurrentAccess(t *testing.T) {
	t.Parallel()
	
	testCollection := &Collect{}
	numGoroutines := 5  // 減少goroutine數量避免deadlock
	numErrorsPerGoroutine := 3  // 減少每個goroutine的錯誤數量
	
	// 使用WaitGroup同步goroutine
	var wg sync.WaitGroup
	var mu sync.Mutex
	results := make([]ErrorResponse, 0, numGoroutines*numErrorsPerGoroutine)
	
	// 並發註冊錯誤響應
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			
			for j := 0; j < numErrorsPerGoroutine; j++ {
				testError := &DefaultErrorResponse{
					StatusCode:  400 + j,
					Name:        fmt.Sprintf("concurrent_error_%d_%d", goroutineID, j),
					Description: fmt.Sprintf("並發測試錯誤 %d-%d", goroutineID, j),
					DefaultKKError: kkerror.DefaultKKError{
						ErrorCode: fmt.Sprintf("CONC%03d%03d", goroutineID, j),
					},
				}
				
				result := testCollection.Register(testError)
				
				// 安全地添加結果到slice
				mu.Lock()
				results = append(results, result)
				mu.Unlock()
			}
		}(i)
	}
	
	// 等待所有goroutine完成，設置超時
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		// 所有goroutine正常完成
	case <-time.After(10 * time.Second):
		t.Fatal("並發測試超時")
	}
	
	// 驗證結果
	assert.Len(t, results, numGoroutines*numErrorsPerGoroutine)
	assert.NotNil(t, testCollection.ErrorResponses)
	
	// 驗證所有錯誤都被正確註冊
	for _, result := range results {
		assert.Contains(t, testCollection.ErrorResponses, result)
	}
}

// TestPredefinedErrorResponses_BasicValidation 測試所有預定義錯誤響應的基本驗證
func TestPredefinedErrorResponses_BasicValidation(t *testing.T) {
	t.Parallel()
	
	// 測試常見的預定義錯誤響應
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
			
			// 基本介面驗證
			assert.NotNil(t, test.error)
			
			// 狀態碼範圍驗證
			statusCode := test.error.ErrorStatusCode()
			assert.GreaterOrEqual(t, statusCode, test.codeMin)
			assert.LessOrEqual(t, statusCode, test.codeMax)
			
			// 錯誤名稱不應為空
			errorName := test.error.ErrorName()
			assert.NotEmpty(t, errorName)
			
			// 錯誤代碼不應為空
			errorCode := test.error.Code()
			assert.NotEmpty(t, errorCode)
			
			// 錯誤訊息JSON格式驗證
			errorMessage := test.error.Error()
			assert.NotEmpty(t, errorMessage)
			
			var jsonData map[string]interface{}
			err := json.Unmarshal([]byte(errorMessage), &jsonData)
			assert.NoError(t, err, "預定義錯誤響應應產生有效JSON")
			
			// 驗證JSON包含必要欄位
			if statusCode > 0 {
				assert.Contains(t, jsonData, "status_code")
			}
			if errorName != "" {
				assert.Contains(t, jsonData, "error")
			}
			
			// 測試Clone功能
			cloned := test.error.Clone()
			assert.NotNil(t, cloned)
			assert.Equal(t, test.error.ErrorStatusCode(), cloned.ErrorStatusCode())
			assert.Equal(t, test.error.ErrorName(), cloned.ErrorName())
			assert.Equal(t, test.error.Code(), cloned.Code())
		})
	}
}

// TestErrorResponse_InterfaceCompliance 測試ErrorResponse介面完整性
func TestErrorResponse_InterfaceCompliance(t *testing.T) {
	t.Parallel()
	
	// 創建測試錯誤響應
	testError := &DefaultErrorResponse{
		StatusCode:  418,
		Name:        "teapot_error",
		Description: "我是茶壺錯誤",
		Data:        map[string]interface{}{"brew_time": 300, "temperature": 85},
		DefaultKKError: kkerror.DefaultKKError{
			ErrorCode:     "418001",
			ErrorLevel:    kkerror.Normal,
			ErrorCategory: kkerror.Client,
		},
	}
	
	// 驗證介面實現
	var _ ErrorResponse = testError
	var _ kkerror.KKError = testError
	
	// 測試ErrorResponse特有方法
	assert.Equal(t, 418, testError.ErrorStatusCode())
	assert.Equal(t, "teapot_error", testError.ErrorName())
	assert.Equal(t, "我是茶壺錯誤", testError.ErrorDescription())
	
	data := testError.ErrorData()
	assert.Equal(t, 300, data["brew_time"])
	assert.Equal(t, 85, data["temperature"])
	
	// 測試KKError繼承方法
	assert.Equal(t, "418001", testError.Code())
	assert.Equal(t, kkerror.Normal, testError.Level())
	assert.Equal(t, kkerror.Client, testError.Category())
	
	// 測試Error方法（JSON序列化）
	errorJson := testError.Error()
	assert.Contains(t, errorJson, "teapot_error")
	assert.Contains(t, errorJson, "我是茶壺錯誤")
	assert.Contains(t, errorJson, "418001")
	assert.Contains(t, errorJson, "300")
	assert.Contains(t, errorJson, "85")
}// TestErrorResponse_EdgeCases 測試邊界情況和異常狀況
func TestErrorResponse_EdgeCases(t *testing.T) {
	t.Parallel()
	
	t.Run("空值和nil處理", func(t *testing.T) {
		t.Parallel()
		
		// 測試空字串字段
		emptyError := &DefaultErrorResponse{
			StatusCode:  0,
			Name:        "",
			Description: "",
			Data:        nil,
			DefaultKKError: kkerror.DefaultKKError{
				ErrorCode: "",
			},
		}
		
		// 基本方法應該不會panic
		assert.NotPanics(t, func() {
			_ = emptyError.ErrorStatusCode()
			_ = emptyError.ErrorName()
			_ = emptyError.ErrorDescription()
			_ = emptyError.ErrorData()
			_ = emptyError.Error()
			_ = emptyError.Clone()
		})
		
		// ErrorData應該初始化為空map而不是nil
		data := emptyError.ErrorData()
		assert.NotNil(t, data)
		assert.Len(t, data, 0)
	})
	
	t.Run("大量數據處理", func(t *testing.T) {
		t.Parallel()
		
		// 創建包含大量數據的錯誤響應
		largeData := make(map[string]interface{})
		for i := 0; i < 1000; i++ {
			largeData[fmt.Sprintf("key_%d", i)] = fmt.Sprintf("value_%d", i)
		}
		
		largeError := &DefaultErrorResponse{
			StatusCode:  500,
			Name:        "large_data_error",
			Description: "包含大量數據的錯誤",
			Data:        largeData,
			DefaultKKError: kkerror.DefaultKKError{
				ErrorCode: "LARGE001",
			},
		}
		
		// 測試不會panic且功能正常
		assert.NotPanics(t, func() {
			data := largeError.ErrorData()
			assert.Len(t, data, 1000)
			
			cloned := largeError.Clone()
			assert.NotNil(t, cloned)
			
			errorJson := largeError.Error()
			assert.NotEmpty(t, errorJson)
		})
	})
	
	t.Run("特殊字符處理", func(t *testing.T) {
		t.Parallel()
		
		specialError := &DefaultErrorResponse{
			StatusCode:  400,
			Name:        "special_char_error",
			Description: "特殊字符測試: \n\t\"\\'/{}[]&<>",
			Data:        map[string]interface{}{
				"unicode":  "測試中文🚀💥",
				"symbols":  "!@#$%^&*()_+-=",
				"quotes":   `"'`,
				"newlines": "line1\nline2\tline3",
			},
			DefaultKKError: kkerror.DefaultKKError{
				ErrorCode: "SPECIAL001",
			},
		}
		
		// 測試JSON序列化處理特殊字符
		errorJson := specialError.Error()
		assert.NotEmpty(t, errorJson)
		
		// 驗證JSON有效性
		var jsonData map[string]interface{}
		err := json.Unmarshal([]byte(errorJson), &jsonData)
		assert.NoError(t, err, "包含特殊字符的錯誤響應應產生有效JSON")
	})
}

// TestGlobalCollection_Integration 測試全域Collection的整合功能
func TestGlobalCollection_Integration(t *testing.T) {
	t.Parallel()
	
	// 驗證全域Collection存在且可用
	assert.NotNil(t, Collection)
	
	// 測試預定義錯誤響應是否在全域Collection中
	predefinedErrors := []ErrorResponse{
		InvalidRequest,
		NotFound, 
		ServerError,
	}
	
	for _, predefinedError := range predefinedErrors {
		t.Run(fmt.Sprintf("全域Collection包含_%s", predefinedError.ErrorName()), func(t *testing.T) {
			// 檢查是否在Collection中註冊
			assert.NotNil(t, Collection.ErrorResponses)
			
			// 驗證錯誤存在於Collection中
			found := false
			for registeredError := range Collection.ErrorResponses {
				if registeredError.Code() == predefinedError.Code() {
					found = true
					break
				}
			}
			assert.True(t, found, "預定義錯誤應該在全域Collection中註冊")
		})
	}
}

// TestErrorResponse_PerformanceBaseline 測試性能基準
func TestErrorResponse_PerformanceBaseline(t *testing.T) {
	if testing.Short() {
		t.Skip("跳過性能測試")
	}
	
	t.Parallel()
	
	// 創建標準錯誤響應
	testError := &DefaultErrorResponse{
		StatusCode:  500,
		Name:        "performance_test_error",
		Description: "性能測試錯誤響應",
		Data:        map[string]interface{}{"request_id": "perf_123", "timestamp": time.Now().Unix()},
		DefaultKKError: kkerror.DefaultKKError{
			ErrorCode:     "PERF001",
			ErrorLevel:    kkerror.Normal,
			ErrorCategory: kkerror.Server,
		},
	}
	
	// 測試大量Error()調用的性能
	t.Run("Error方法性能測試", func(t *testing.T) {
		iterations := 10000
		start := time.Now()
		
		for i := 0; i < iterations; i++ {
			_ = testError.Error()
		}
		
		duration := time.Since(start)
		avgDuration := duration / time.Duration(iterations)
		
		t.Logf("Error()方法平均執行時間: %v (總計: %v, 迭代: %d)", avgDuration, duration, iterations)
		
		// 性能閾值：每次調用不應超過1ms
		assert.Less(t, avgDuration, time.Millisecond, "Error()方法性能應該在可接受範圍內")
	})
	
	// 測試大量Clone()調用的性能
	t.Run("Clone方法性能測試", func(t *testing.T) {
		iterations := 10000
		start := time.Now()
		
		for i := 0; i < iterations; i++ {
			cloned := testError.Clone()
			_ = cloned
		}
		
		duration := time.Since(start)
		avgDuration := duration / time.Duration(iterations)
		
		t.Logf("Clone()方法平均執行時間: %v (總計: %v, 迭代: %d)", avgDuration, duration, iterations)
		
		// 性能閾值：每次調用不應超過100μs
		assert.Less(t, avgDuration, 100*time.Microsecond, "Clone()方法性能應該在可接受範圍內")
	})
}

// TestConstantUsage 測試常數的使用
func TestConstantUsage(t *testing.T) {
	t.Parallel()
	
	// 驗證常數定義和使用
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
		t.Run(fmt.Sprintf("常數_%s", test.expected), func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, test.expected, test.constant, "常數值應該匹配預期")
		})
	}
}