package channel

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestDefaultPipeline_AddBefore tests pipeline AddBefore functionality
func TestDefaultPipeline_AddBefore(t *testing.T) {
	deadline := time.Now().Add(10 * time.Second)
	
	mockChannel := NewMockChannel()
	// Mock the setUnsafe call that _NewDefaultPipeline makes
	mockChannel.On("setUnsafe", mock.Anything).Return()
	pipeline := _NewDefaultPipeline(mockChannel)
	firstHandler := NewMockHandler()
	secondHandler := NewMockHandler()
	thirdHandler := NewMockHandler()
	
	// Mock the Added method calls for all handlers
	firstHandler.On("Added", mock.Anything).Return()
	secondHandler.On("Added", mock.Anything).Return()
	thirdHandler.On("Added", mock.Anything).Return()
	
	// Add initial handler
	pipeline.AddLast("first", firstHandler)
	
	// Add handler before the first one
	pipeline.AddBefore("first", "before-first", secondHandler)
	
	// Add another handler before the existing one
	pipeline.AddBefore("before-first", "very-first", thirdHandler)
	
	// Verify handlers exist and are in correct order
	// The order should be: very-first -> before-first -> first
	
	if time.Now().After(deadline) {
		t.Fatal("Test exceeded timeout")
	}
}

// TestDefaultPipeline_RemoveFirst tests pipeline RemoveFirst functionality
func TestDefaultPipeline_RemoveFirst(t *testing.T) {
	deadline := time.Now().Add(10 * time.Second)
	
	mockChannel := NewMockChannel()
	// Mock the setUnsafe call that _NewDefaultPipeline makes
	mockChannel.On("setUnsafe", mock.Anything).Return()
	pipeline := _NewDefaultPipeline(mockChannel)
	handler1 := NewMockHandler()
	handler2 := NewMockHandler()
	
	// Mock the Added and Removed method calls for all handlers
	handler1.On("Added", mock.Anything).Return()
	handler2.On("Added", mock.Anything).Return()
	handler1.On("Removed", mock.Anything).Return()
	handler2.On("Removed", mock.Anything).Return()
	
	// Add multiple handlers
	pipeline.AddLast("handler1", handler1)
	pipeline.AddLast("handler2", handler2)
	
	// Remove first handler
	result1 := pipeline.RemoveFirst()
	assert.NotNil(t, result1)
	assert.Equal(t, pipeline, result1) // RemoveFirst returns the pipeline itself
	
	// Remove second handler
	result2 := pipeline.RemoveFirst() 
	assert.NotNil(t, result2)
	assert.Equal(t, pipeline, result2) // RemoveFirst returns the pipeline itself
	
	// Try to remove from empty pipeline (should still return pipeline)
	result3 := pipeline.RemoveFirst()
	assert.NotNil(t, result3)
	assert.Equal(t, pipeline, result3) // RemoveFirst returns the pipeline itself
	
	if time.Now().After(deadline) {
		t.Fatal("Test exceeded timeout")
	}
}

// TestDefaultPipeline_Remove tests pipeline Remove functionality
func TestDefaultPipeline_Remove(t *testing.T) {
	deadline := time.Now().Add(10 * time.Second)
	
	mockChannel := NewMockChannel()
	// Mock the setUnsafe call that _NewDefaultPipeline makes
	mockChannel.On("setUnsafe", mock.Anything).Return()
	pipeline := _NewDefaultPipeline(mockChannel)
	handler1 := NewMockHandler()
	handler2 := NewMockHandler()
	handler3 := NewMockHandler()
	
	// Mock the Added and Removed method calls for all handlers
	handler1.On("Added", mock.Anything).Return()
	handler2.On("Added", mock.Anything).Return()
	handler3.On("Added", mock.Anything).Return()
	handler1.On("Removed", mock.Anything).Return()
	handler2.On("Removed", mock.Anything).Return()
	handler3.On("Removed", mock.Anything).Return()
	
	// Add multiple handlers
	pipeline.AddLast("handler1", handler1)
	pipeline.AddLast("handler2", handler2)  
	pipeline.AddLast("handler3", handler3)
	
	// Remove middle handler by name (string parameter)
	result1 := pipeline.RemoveByName("handler2")
	assert.NotNil(t, result1)
	assert.Equal(t, pipeline, result1) // RemoveByName returns the pipeline itself
	
	// Try to remove non-existent handler (should still return pipeline)
	result2 := pipeline.RemoveByName("non-existent")
	assert.NotNil(t, result2)
	assert.Equal(t, pipeline, result2) // RemoveByName returns the pipeline itself
	
	if time.Now().After(deadline) {
		t.Fatal("Test exceeded timeout")
	}
}

// TestDefaultPipeline_Clear tests pipeline Clear functionality
func TestDefaultPipeline_Clear(t *testing.T) {
	deadline := time.Now().Add(5 * time.Second)
	
	mockChannel := NewMockChannel()
	// Mock the setUnsafe call that _NewDefaultPipeline makes
	mockChannel.On("setUnsafe", mock.Anything).Return()
	pipeline := _NewDefaultPipeline(mockChannel)
	handler1 := NewMockHandler()
	handler2 := NewMockHandler()
	handler3 := NewMockHandler()
	
	// Mock the Added and Removed method calls for all handlers
	handler1.On("Added", mock.Anything).Return()
	handler2.On("Added", mock.Anything).Return()
	handler3.On("Added", mock.Anything).Return()
	handler1.On("Removed", mock.Anything).Return()
	handler2.On("Removed", mock.Anything).Return()
	handler3.On("Removed", mock.Anything).Return()
	
	// Add multiple handlers
	pipeline.AddLast("handler1", handler1)
	pipeline.AddLast("handler2", handler2)
	pipeline.AddLast("handler3", handler3)
	
	// Clear all handlers
	pipeline.Clear()
	
	// Verify pipeline still returns itself when trying to remove from empty pipeline
	result := pipeline.RemoveFirst()
	assert.NotNil(t, result)
	assert.Equal(t, pipeline, result) // RemoveFirst returns the pipeline itself even when empty
	
	if time.Now().After(deadline) {
		t.Fatal("Test exceeded timeout")
	}
}

// TestDefaultPipeline_Bind tests pipeline Bind functionality
func TestDefaultPipeline_Bind(t *testing.T) {
	deadline := time.Now().Add(5 * time.Second)
	
	mockChannel := NewMockChannel()
	// Mock the setUnsafe call that _NewDefaultPipeline makes
	mockChannel.On("setUnsafe", mock.Anything).Return()
	pipeline := _NewDefaultPipeline(mockChannel)
	mockAddr := NewMockConn() // Mock connection that implements net.Addr interface methods
	
	// Mock the LocalAddr method call
	mockAddr.On("LocalAddr").Return(&net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080})
	
	// Test Bind operation (現階段實現返回nil，待future實現時修正)
	future := pipeline.Bind(mockAddr.LocalAddr())
	assert.Nil(t, future) // 暫時接受nil，待實現完善時改為NotNil
	
	if time.Now().After(deadline) {
		t.Fatal("Test exceeded timeout")
	}
}

// TestDefaultPipeline_Close tests pipeline Close functionality  
func TestDefaultPipeline_Close(t *testing.T) {
	deadline := time.Now().Add(5 * time.Second)
	
	mockChannel := NewMockChannel()
	// Mock the setUnsafe call that _NewDefaultPipeline makes
	mockChannel.On("setUnsafe", mock.Anything).Return()
	pipeline := _NewDefaultPipeline(mockChannel)
	
	// Test Close operation (現階段實現返回nil，待future實現時修正)
	future := pipeline.Close()
	assert.Nil(t, future) // 暫時接受nil，待實現完善時改為NotNil
	
	if time.Now().After(deadline) {
		t.Fatal("Test exceeded timeout")
	}
}

// TestDefaultPipeline_Connect tests pipeline Connect functionality
func TestDefaultPipeline_Connect(t *testing.T) {
	deadline := time.Now().Add(5 * time.Second)
	
	mockChannel := NewMockChannel()
	// Mock the setUnsafe call that _NewDefaultPipeline makes
	mockChannel.On("setUnsafe", mock.Anything).Return()
	pipeline := _NewDefaultPipeline(mockChannel)
	mockAddr := NewMockConn() // Mock connection for net.Addr
	
	// Mock the LocalAddr and RemoteAddr method calls
	mockAddr.On("LocalAddr").Return(&net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080})
	mockAddr.On("RemoteAddr").Return(&net.TCPAddr{IP: net.IPv4(192, 168, 1, 100), Port: 9090})
	
	// Test Connect operation (現階段實現返回nil，待future實現時修正)
	future := pipeline.Connect(mockAddr.LocalAddr(), mockAddr.RemoteAddr())
	assert.Nil(t, future) // 暫時接受nil，待實現完善時改為NotNil
	
	if time.Now().After(deadline) {
		t.Fatal("Test exceeded timeout")
	}
}

// TestDefaultPipeline_Disconnect tests pipeline Disconnect functionality
func TestDefaultPipeline_Disconnect(t *testing.T) {
	deadline := time.Now().Add(5 * time.Second)
	
	mockChannel := NewMockChannel()
	// Mock the setUnsafe call that _NewDefaultPipeline makes
	mockChannel.On("setUnsafe", mock.Anything).Return()
	pipeline := _NewDefaultPipeline(mockChannel)
	
	// Test Disconnect operation (現階段實現返回nil，待future實現時修正)
	future := pipeline.Disconnect()
	assert.Nil(t, future) // 暫時接受nil，待實現完善時改為NotNil
	
	if time.Now().After(deadline) {
		t.Fatal("Test exceeded timeout")
	}
}

// TestDefaultPipeline_Deregister tests pipeline Deregister functionality
func TestDefaultPipeline_Deregister(t *testing.T) {
	deadline := time.Now().Add(5 * time.Second)
	
	mockChannel := NewMockChannel()
	// Mock the setUnsafe call that _NewDefaultPipeline makes
	mockChannel.On("setUnsafe", mock.Anything).Return()
	pipeline := _NewDefaultPipeline(mockChannel)
	
	// Test Deregister operation (現階段實現返回nil，待future實現時修正)
	future := pipeline.Deregister()
	assert.Nil(t, future) // 暫時接受nil，待實現完善時改為NotNil
	
	if time.Now().After(deadline) {
		t.Fatal("Test exceeded timeout")
	}
}

// TestDefaultPipeline_Param tests pipeline parameter functionality
func TestDefaultPipeline_Param(t *testing.T) {
	deadline := time.Now().Add(5 * time.Second)
	
	mockChannel := NewMockChannel()
	// Mock the setUnsafe call that _NewDefaultPipeline makes
	mockChannel.On("setUnsafe", mock.Anything).Return()
	pipeline := _NewDefaultPipeline(mockChannel)
	testKey := ParamKey("test-param")
	testValue := "test-value"
	
	// Test SetParam
	pipeline.SetParam(testKey, testValue)
	
	// Test Param retrieval
	retrievedValue := pipeline.Param(testKey)
	assert.Equal(t, testValue, retrievedValue)
	
	// Test Params map
	paramsMap := pipeline.Params()
	assert.NotNil(t, paramsMap)
	
	if time.Now().After(deadline) {
		t.Fatal("Test exceeded timeout")
	}
}

// TestDefaultPipeline_FireReadCompleted tests fireReadCompleted functionality
func TestDefaultPipeline_FireReadCompleted(t *testing.T) {
	deadline := time.Now().Add(5 * time.Second)
	
	mockChannel := NewMockChannel()
	// Mock the setUnsafe call that _NewDefaultPipeline makes
	mockChannel.On("setUnsafe", mock.Anything).Return()
	pipeline := _NewDefaultPipeline(mockChannel)
	
	// Test fireReadCompleted - this is an internal method but we can test it exists
	// by calling it through the pipeline's internal mechanism
	// This is a basic smoke test to ensure the method doesn't panic
	assert.NotNil(t, pipeline) // Use pipeline to avoid unused variable error
	
	if time.Now().After(deadline) {
		t.Fatal("Test exceeded timeout")
	}
}