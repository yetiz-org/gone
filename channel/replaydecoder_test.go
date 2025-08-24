package channel

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	buf "github.com/yetiz-org/goth-bytebuf"
	"github.com/yetiz-org/goth-util/structs"
)

// TestReplayDecoder_NewReplayDecoder tests ReplayDecoder construction
func TestReplayDecoder_NewReplayDecoder(t *testing.T) {
	deadline := time.Now().Add(5 * time.Second)

	// Test decoder function
	decodeFunc := func(ctx HandlerContext, in buf.ByteBuf, out structs.Queue) {
		// Simple test decoder that reads int32 values
		for in.ReadableBytes() >= 4 {
			val := in.ReadInt32()
			out.Push(val)
		}
	}

	// Create ReplayDecoder with initial state
	initialState := ReplayState(1)
	decoder := NewReplayDecoder(initialState, decodeFunc)

	// Verify construction
	assert.NotNil(t, decoder)
	assert.Equal(t, initialState, decoder.State())
	assert.NotNil(t, decoder.Decode) // Decode function should be set

	if time.Now().After(deadline) {
		t.Fatal("Test exceeded timeout")
	}
}

// TestReplayDecoder_Skip tests Skip method (should panic)
func TestReplayDecoder_Skip(t *testing.T) {
	deadline := time.Now().Add(5 * time.Second)

	decoder := NewReplayDecoder(ReplayState(0), nil)

	// Skip should panic with buf.ErrInsufficientSize
	assert.PanicsWithError(t, buf.ErrInsufficientSize.Error(), func() {
		decoder.Skip()
	})

	if time.Now().After(deadline) {
		t.Fatal("Test exceeded timeout")
	}
}

// TestReplayDecoder_State tests State getter
func TestReplayDecoder_State(t *testing.T) {
	deadline := time.Now().Add(5 * time.Second)

	testStates := []ReplayState{0, 1, 5, 100, -1}

	for _, state := range testStates {
		decoder := NewReplayDecoder(state, nil)
		assert.Equal(t, state, decoder.State())
	}

	if time.Now().After(deadline) {
		t.Fatal("Test exceeded timeout")
	}
}

// TestReplayDecoder_Checkpoint tests Checkpoint state management
func TestReplayDecoder_Checkpoint(t *testing.T) {
	deadline := time.Now().Add(10 * time.Second)

	decoder := NewReplayDecoder(ReplayState(0), nil)

	// Test state change
	newState := ReplayState(42)
	decoder.Checkpoint(newState)
	assert.Equal(t, newState, decoder.State())

	// Test another state change
	anotherState := ReplayState(-5)
	decoder.Checkpoint(anotherState)
	assert.Equal(t, anotherState, decoder.State())

	// Test with buffer initialized (through Added)
	mockContext := NewMockHandlerContext()
	decoder.Added(mockContext)

	// Checkpoint should work with initialized buffer
	finalState := ReplayState(100)
	decoder.Checkpoint(finalState)
	assert.Equal(t, finalState, decoder.State())

	if time.Now().After(deadline) {
		t.Fatal("Test exceeded timeout")
	}
}

// TestReplayDecoder_Added tests Added method (buffer initialization)
func TestReplayDecoder_Added(t *testing.T) {
	deadline := time.Now().Add(5 * time.Second)

	decoder := NewReplayDecoder(ReplayState(0), nil)
	mockContext := NewMockHandlerContext()

	// Added should initialize the buffer without error
	decoder.Added(mockContext)

	// Buffer should be initialized (not nil)
	assert.NotNil(t, decoder.in)

	if time.Now().After(deadline) {
		t.Fatal("Test exceeded timeout")
	}
}

// TestReplayDecoder_Read_WithDecoder tests Read method with valid decoder
func TestReplayDecoder_Read_WithDecoder(t *testing.T) {
	deadline := time.Now().Add(10 * time.Second)

	// Track decoded values
	var decodedValues []interface{}

	// Create decoder function that processes int32 values
	decodeFunc := func(ctx HandlerContext, in buf.ByteBuf, out structs.Queue) {
		for in.ReadableBytes() >= 4 {
			val := in.ReadInt32()
			out.Push(val)
		}
	}

	decoder := NewReplayDecoder(ReplayState(0), decodeFunc)

	// Create mock context
	mockContext := NewMockHandlerContext()

	// Setup mocks - only FireRead is actually called by ReplayDecoder.Read
	mockContext.On("FireRead", mock.Anything).Run(func(args mock.Arguments) {
		decodedValues = append(decodedValues, args.Get(0))
	}).Return(mockContext)

	// Initialize buffer
	decoder.Added(mockContext)

	// Create input data (two int32 values: 1234, 5678)
	inputBuf := buf.NewByteBuf(make([]byte, 0, 8))
	inputBuf.WriteInt32(1234)
	inputBuf.WriteInt32(5678)

	// Process data
	decoder.Read(mockContext, inputBuf)

	// Verify decoded values
	assert.Equal(t, 2, len(decodedValues))
	assert.Equal(t, int32(1234), decodedValues[0])
	assert.Equal(t, int32(5678), decodedValues[1])

	// Verify mocks were called
	mockContext.AssertExpectations(t)

	if time.Now().After(deadline) {
		t.Fatal("Test exceeded timeout")
	}
}

// TestReplayDecoder_Read_WithoutDecoder tests Read method with nil decoder
func TestReplayDecoder_Read_WithoutDecoder(t *testing.T) {
	deadline := time.Now().Add(5 * time.Second)

	// Create decoder without decode function (nil)
	decoder := NewReplayDecoder(ReplayState(0), nil)

	mockContext := NewMockHandlerContext()
	mockChannel := NewMockChannel()

	// Setup mocks
	mockContext.On("Channel").Return(mockChannel)

	// Initialize buffer
	decoder.Added(mockContext)

	// Create input data
	inputBuf := buf.NewByteBuf(make([]byte, 0, 4))
	inputBuf.WriteInt32(1234)

	// Process data (should log warning, not crash)
	decoder.Read(mockContext, inputBuf)

	// No FireRead should be called since decoder is nil
	mockContext.AssertNotCalled(t, "FireRead")

	if time.Now().After(deadline) {
		t.Fatal("Test exceeded timeout")
	}
}

// TestReplayDecoder_Read_InsufficientDataHandling tests exception handling in Read method
func TestReplayDecoder_Read_InsufficientDataHandling(t *testing.T) {
	deadline := time.Now().Add(10 * time.Second)

	// Create decoder function that expects complete int32 values (will trigger Skip/ErrInsufficientSize)
	decodeFunc := func(ctx HandlerContext, in buf.ByteBuf, out structs.Queue) {
		if in.ReadableBytes() < 4 {
			// This should trigger the panic/exception handling
			panic(buf.ErrInsufficientSize)
		}
		val := in.ReadInt32()
		out.Push(val)
	}

	decoder := NewReplayDecoder(ReplayState(0), decodeFunc)

	// Create mock context and channel
	mockContext := NewMockHandlerContext()
	mockChannel := NewMockChannel()

	// Setup mocks
	mockContext.On("Channel").Return(mockChannel)

	// Initialize buffer
	decoder.Added(mockContext)

	// Create input data with insufficient bytes (only 2 bytes for int32)
	inputBuf := buf.NewByteBuf(make([]byte, 0, 2))
	inputBuf.WriteByte(0x12)
	inputBuf.WriteByte(0x34)

	// Process data (should handle ErrInsufficientSize gracefully)
	decoder.Read(mockContext, inputBuf)

	// No FireRead should be called due to insufficient data
	mockContext.AssertNotCalled(t, "FireRead")

	if time.Now().After(deadline) {
		t.Fatal("Test exceeded timeout")
	}
}
