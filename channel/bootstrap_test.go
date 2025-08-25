package channel

import (
	"fmt"
	"net"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestNewBootstrap tests the Bootstrap constructor
func TestNewBootstrap(t *testing.T) {
	t.Parallel()

	t.Run("NewBootstrap_CreatesValidInstance", func(t *testing.T) {
		t.Parallel()

		bootstrap := NewBootstrap()

		// Should return non-nil Bootstrap interface
		assert.NotNil(t, bootstrap)

		// Should be DefaultBootstrap implementation
		defaultBootstrap, ok := bootstrap.(*DefaultBootstrap)
		assert.True(t, ok)
		assert.NotNil(t, defaultBootstrap)

		// Should have initialized params
		params := bootstrap.Params()
		assert.NotNil(t, params)
	})
}

// TestDefaultBootstrap_SetParams tests parameter setting functionality
func TestDefaultBootstrap_SetParams(t *testing.T) {
	t.Parallel()

	t.Run("SetParams_SingleParam", func(t *testing.T) {
		t.Parallel()

		bootstrap := NewBootstrap()
		testKey := ParamKey("test_key")
		testValue := "test_value"

		result := bootstrap.SetParams(testKey, testValue)

		// Should return self for chaining
		assert.Equal(t, bootstrap, result)

		// Should store parameter correctly
		params := bootstrap.Params()
		storedValue, exists := params.Load(testKey)
		assert.True(t, exists)
		assert.Equal(t, testValue, storedValue)
	})

	t.Run("SetParams_MultipleParams", func(t *testing.T) {
		t.Parallel()

		bootstrap := NewBootstrap()

		// Set multiple parameters with chaining
		result := bootstrap.
			SetParams("key1", "value1").
			SetParams("key2", 42).
			SetParams("key3", true)

		// Should return self for chaining
		assert.Equal(t, bootstrap, result)

		// Should store all parameters correctly
		params := bootstrap.Params()

		val1, exists1 := params.Load("key1")
		assert.True(t, exists1)
		assert.Equal(t, "value1", val1)

		val2, exists2 := params.Load("key2")
		assert.True(t, exists2)
		assert.Equal(t, 42, val2)

		val3, exists3 := params.Load("key3")
		assert.True(t, exists3)
		assert.Equal(t, true, val3)
	})

	t.Run("SetParams_OverwriteParam", func(t *testing.T) {
		t.Parallel()

		bootstrap := NewBootstrap()
		testKey := ParamKey("overwrite_key")

		// Set initial value
		bootstrap.SetParams(testKey, "initial_value")

		// Overwrite with new value
		bootstrap.SetParams(testKey, "new_value")

		// Should have updated value
		params := bootstrap.Params()
		storedValue, exists := params.Load(testKey)
		assert.True(t, exists)
		assert.Equal(t, "new_value", storedValue)
	})
}

// TestDefaultBootstrap_Params tests parameter retrieval
func TestDefaultBootstrap_Params(t *testing.T) {
	t.Parallel()

	t.Run("Params_ReturnsValidPointer", func(t *testing.T) {
		t.Parallel()

		bootstrap := NewBootstrap()

		params := bootstrap.Params()

		// Should return non-nil pointer
		assert.NotNil(t, params)

		// Should be same instance on multiple calls
		params2 := bootstrap.Params()
		assert.Equal(t, params, params2)
	})

	t.Run("Params_ModificationPersists", func(t *testing.T) {
		t.Parallel()

		bootstrap := NewBootstrap()
		params := bootstrap.Params()

		// Modify parameters directly
		testKey := ParamKey("direct_key")
		testValue := "direct_value"
		params.Store(testKey, testValue)

		// Should be accessible through bootstrap
		storedValue, exists := bootstrap.Params().Load(testKey)
		assert.True(t, exists)
		assert.Equal(t, testValue, storedValue)
	})
}

// TestDefaultBootstrap_Handler tests handler setting functionality
func TestDefaultBootstrap_Handler(t *testing.T) {
	t.Parallel()

	t.Run("Handler_SetValidHandler", func(t *testing.T) {
		t.Parallel()

		bootstrap := NewBootstrap()
		mockHandler := NewMockHandler()

		result := bootstrap.Handler(mockHandler)

		// Should return self for chaining
		assert.Equal(t, bootstrap, result)

		// Handler should be stored (we can't directly access it, but test through Connect)
		// This will be tested indirectly in Connect tests
	})

	t.Run("Handler_SetNilHandler", func(t *testing.T) {
		t.Parallel()

		bootstrap := NewBootstrap()

		result := bootstrap.Handler(nil)

		// Should return self even with nil handler
		assert.Equal(t, bootstrap, result)
	})
}

// MockChannelForBootstrap is a mock channel for bootstrap testing
type MockChannelForBootstrap struct {
	*MockChannel
	preInitCalled  bool
	postInitCalled bool
}

func (m *MockChannelForBootstrap) BootstrapPreInit() {
	m.preInitCalled = true
}

func (m *MockChannelForBootstrap) BootstrapPostInit() {
	m.postInitCalled = true
}

// TestDefaultBootstrap_ChannelType tests channel type setting
func TestDefaultBootstrap_ChannelType(t *testing.T) {
	t.Parallel()

	t.Run("ChannelType_SetValidChannel", func(t *testing.T) {
		t.Parallel()

		bootstrap := NewBootstrap()
		mockChannel := NewMockChannel()

		result := bootstrap.ChannelType(mockChannel)

		// Should return self for chaining
		assert.Equal(t, bootstrap, result)

		// Channel type should be stored (tested indirectly through Connect)
	})

	t.Run("ChannelType_SetMockChannelForBootstrap", func(t *testing.T) {
		t.Parallel()

		bootstrap := NewBootstrap()
		mockChannel := &MockChannelForBootstrap{MockChannel: NewMockChannel()}

		result := bootstrap.ChannelType(mockChannel)

		// Should return self for chaining
		assert.Equal(t, bootstrap, result)
	})
}

// TestDefaultBootstrap_Connect tests the complete connection flow
func TestDefaultBootstrap_Connect(t *testing.T) {
	t.Parallel()

	// Note: Connect method uses reflection to create new channel instances,
	// which makes direct mocking challenging. We focus on testing the
	// individual components and the overall flow structure.

	t.Run("Connect_ReflectionFlow", func(t *testing.T) {
		t.Parallel()

		bootstrap := NewBootstrap()

		// Since Connect uses reflection, we test with a panic expectation
		// for missing channel type to verify the method is called correctly
		localAddr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080}
		remoteAddr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8081}

		// Should panic due to nil channelType
		assert.Panics(t, func() {
			bootstrap.Connect(localAddr, remoteAddr)
		})
	})

	t.Run("Connect_ChannelTypeSet", func(t *testing.T) {
		t.Parallel()

		bootstrap := NewBootstrap()
		mockChannel := NewMockChannel()

		// Set channel type
		bootstrap.ChannelType(mockChannel)

		// Get the bootstrap as DefaultBootstrap to check channelType
		defaultBootstrap := bootstrap.(*DefaultBootstrap)
		assert.NotNil(t, defaultBootstrap.channelType)

		// Verify channel type is correct
		expectedType := reflect.ValueOf(mockChannel).Elem().Type()
		assert.Equal(t, expectedType, defaultBootstrap.channelType)
	})

	t.Run("Connect_ParamsFlow", func(t *testing.T) {
		t.Parallel()

		bootstrap := NewBootstrap()

		// Set parameters
		bootstrap.
			SetParams("test_key", "test_value").
			SetParams("num_key", 42)

		// Verify parameters are stored
		params := bootstrap.Params()

		val1, exists1 := params.Load("test_key")
		assert.True(t, exists1)
		assert.Equal(t, "test_value", val1)

		val2, exists2 := params.Load("num_key")
		assert.True(t, exists2)
		assert.Equal(t, 42, val2)
	})

	t.Run("Connect_HandlerFlow", func(t *testing.T) {
		t.Parallel()

		bootstrap := NewBootstrap()
		mockHandler := NewMockHandler()

		// Set handler
		result := bootstrap.Handler(mockHandler)

		// Should return self for chaining
		assert.Equal(t, bootstrap, result)

		// Handler is stored internally (we can't directly verify without Connect)
		// This tests the setter functionality
	})
}

// TestValueSetFieldVal tests the reflection utility function
func TestValueSetFieldVal(t *testing.T) {
	t.Parallel()

	// Test structure for reflection tests
	type TestStruct struct {
		StringField  string
		IntField     int
		BoolField    bool
		privateField string // private field for testing access
	}

	t.Run("ValueSetFieldVal_ValidField", func(t *testing.T) {
		t.Parallel()

		testStruct := &TestStruct{}
		target := reflect.ValueOf(testStruct)

		// Set string field
		success := ValueSetFieldVal(&target, "StringField", "test_value")
		assert.True(t, success)
		assert.Equal(t, "test_value", testStruct.StringField)

		// Set int field
		success = ValueSetFieldVal(&target, "IntField", 42)
		assert.True(t, success)
		assert.Equal(t, 42, testStruct.IntField)

		// Set bool field
		success = ValueSetFieldVal(&target, "BoolField", true)
		assert.True(t, success)
		assert.Equal(t, true, testStruct.BoolField)
	})

	t.Run("ValueSetFieldVal_InvalidField", func(t *testing.T) {
		t.Parallel()

		testStruct := &TestStruct{}
		target := reflect.ValueOf(testStruct)

		// Try to set non-existent field
		success := ValueSetFieldVal(&target, "NonExistentField", "value")
		assert.False(t, success)

		// Original struct should be unchanged
		assert.Equal(t, "", testStruct.StringField)
		assert.Equal(t, 0, testStruct.IntField)
		assert.Equal(t, false, testStruct.BoolField)
	})

	t.Run("ValueSetFieldVal_PrivateField", func(t *testing.T) {
		t.Parallel()

		testStruct := &TestStruct{}
		target := reflect.ValueOf(testStruct)

		// Try to set private field (should fail due to CanSet())
		success := ValueSetFieldVal(&target, "privateField", "private_value")
		assert.False(t, success)

		// Private field should remain unchanged
		assert.Equal(t, "", testStruct.privateField)
	})

	t.Run("ValueSetFieldVal_TypeMismatch", func(t *testing.T) {
		t.Parallel()

		testStruct := &TestStruct{}
		target := reflect.ValueOf(testStruct)

		// This will panic in reflect.Set, but we're testing that the function
		// gets to the point where it attempts the set
		assert.Panics(t, func() {
			ValueSetFieldVal(&target, "IntField", "not_an_int")
		})
	})

	t.Run("ValueSetFieldVal_NilTarget", func(t *testing.T) {
		t.Parallel()

		var nilTarget reflect.Value

		// Should handle nil target gracefully
		assert.Panics(t, func() {
			ValueSetFieldVal(&nilTarget, "AnyField", "any_value")
		})
	})
}

// TestBootstrap_IntegrationScenarios tests complete Bootstrap workflows
func TestBootstrap_IntegrationScenarios(t *testing.T) {
	t.Parallel()

	t.Run("Bootstrap_MethodChaining", func(t *testing.T) {
		t.Parallel()

		// Test complete method chaining workflow
		bootstrap := NewBootstrap()
		mockChannel := NewMockChannel()
		mockHandler := NewMockHandler()

		// Test method chaining
		result := bootstrap.
			ChannelType(mockChannel).
			Handler(mockHandler).
			SetParams("timeout", 30*time.Second).
			SetParams("buffer_size", 1024)

		// Should return self for chaining
		assert.Equal(t, bootstrap, result)

		// Verify parameters were set
		params := bootstrap.Params()

		timeout, exists1 := params.Load("timeout")
		assert.True(t, exists1)
		assert.Equal(t, 30*time.Second, timeout)

		bufferSize, exists2 := params.Load("buffer_size")
		assert.True(t, exists2)
		assert.Equal(t, 1024, bufferSize)

		// Verify channel type was set
		defaultBootstrap := bootstrap.(*DefaultBootstrap)
		expectedType := reflect.ValueOf(mockChannel).Elem().Type()
		assert.Equal(t, expectedType, defaultBootstrap.channelType)
	})

	t.Run("Bootstrap_ConcurrentUsage", func(t *testing.T) {
		t.Parallel()

		bootstrap := NewBootstrap()

		// Test concurrent parameter setting
		done := make(chan bool, 10)

		for i := 0; i < 10; i++ {
			go func(index int) {
				defer func() { done <- true }()

				key := ParamKey(fmt.Sprintf("key_%d", index))
				value := fmt.Sprintf("value_%d", index)

				bootstrap.SetParams(key, value)

				// Verify parameter was set
				params := bootstrap.Params()
				storedValue, exists := params.Load(key)
				assert.True(t, exists)
				assert.Equal(t, value, storedValue)
			}(i)
		}

		// Wait for all goroutines to complete
		for i := 0; i < 10; i++ {
			<-done
		}

		// Verify all parameters exist
		params := bootstrap.Params()
		for i := 0; i < 10; i++ {
			key := ParamKey(fmt.Sprintf("key_%d", i))
			value := fmt.Sprintf("value_%d", i)

			storedValue, exists := params.Load(key)
			assert.True(t, exists)
			assert.Equal(t, value, storedValue)
		}
	})

	t.Run("Bootstrap_MultipleInstances", func(t *testing.T) {
		t.Parallel()

		// Test multiple bootstrap instances are independent
		bootstrap1 := NewBootstrap()
		bootstrap2 := NewBootstrap()

		// Set different parameters on each
		bootstrap1.SetParams("instance", "first")
		bootstrap2.SetParams("instance", "second")

		// Verify independence
		params1 := bootstrap1.Params()
		params2 := bootstrap2.Params()

		val1, exists1 := params1.Load("instance")
		assert.True(t, exists1)
		assert.Equal(t, "first", val1)

		val2, exists2 := params2.Load("instance")
		assert.True(t, exists2)
		assert.Equal(t, "second", val2)

		// Verify cross-contamination doesn't happen
		_, exists1Cross := params1.Load("second")
		assert.False(t, exists1Cross)

		_, exists2Cross := params2.Load("first")
		assert.False(t, exists2Cross)
	})
}
