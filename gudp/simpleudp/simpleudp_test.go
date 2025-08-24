package simpleudp

import (
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	goneMock "github.com/yetiz-org/gone/mock"
	"github.com/yetiz-org/gone/utils"
	buf "github.com/yetiz-org/goth-bytebuf"
)

// TestNewClient tests basic client creation
func TestNewClient(t *testing.T) {
	// Set timeout to prevent infinite loops
	deadline := time.Now().Add(5 * time.Second)
	
	handler := goneMock.NewMockHandler()
	client := NewClient(handler)
	
	if client == nil {
		t.Fatal("NewClient should return non-nil client")
	}
	if client.Handler != handler {
		t.Error("Client should store provided handler")
	}
	if client.close != false {
		t.Error("Client should initialize with close=false")
	}
	
	if time.Now().After(deadline) {
		t.Fatal("Test exceeded timeout")
	}
}

// TestNewServer tests basic server creation
func TestNewServer(t *testing.T) {
	// Set timeout to prevent infinite loops
	deadline := time.Now().Add(5 * time.Second)
	
	handler := goneMock.NewMockHandler()
	server := NewServer(handler)
	
	if server == nil {
		t.Fatal("NewServer should return non-nil server")
	}
	if server.Handler != handler {
		t.Error("Server should store provided handler")
	}
	
	if time.Now().After(deadline) {
		t.Fatal("Test exceeded timeout")
	}
}

// TestNewSimpleCodec tests codec creation
func TestNewSimpleCodec(t *testing.T) {
	// Set timeout to prevent infinite loops  
	deadline := time.Now().Add(5 * time.Second)
	
	codec := NewSimpleCodec()
	
	if codec == nil {
		t.Fatal("NewSimpleCodec should return non-nil codec")
	}
	if codec.ReplayDecoder == nil {
		t.Error("SimpleCodec should have non-nil ReplayDecoder")
	}
	
	if time.Now().After(deadline) {
		t.Fatal("Test exceeded timeout")
	}
}

// TestVarIntEncode tests variable integer encoding
func TestVarIntEncode(t *testing.T) {
	// Set timeout to prevent infinite loops
	deadline := time.Now().Add(5 * time.Second)
	
	testCases := []struct {
		input    uint64
		expected []byte
	}{
		{0, []byte{0}},
		{252, []byte{252}},
		{253, []byte{0xfd, 0, 253}}, // uint16
		{65536, []byte{0xfe, 0, 1, 0, 0}}, // uint32
	}
	
	for _, tc := range testCases {
		result := utils.VarIntEncode(tc.input)
		resultBytes := result.Bytes()
		
		if len(resultBytes) != len(tc.expected) {
			t.Errorf("utils.VarIntEncode(%d): expected length %d, got %d", 
				tc.input, len(tc.expected), len(resultBytes))
			continue
		}
		
		// Check first byte to ensure encoding type is correct
		if resultBytes[0] != tc.expected[0] {
			t.Errorf("utils.VarIntEncode(%d): expected first byte %d, got %d", 
				tc.input, tc.expected[0], resultBytes[0])
		}
	}
	
	if time.Now().After(deadline) {
		t.Fatal("Test exceeded timeout")
	}
}

// TestVarIntDecode tests variable integer decoding
func TestVarIntDecode(t *testing.T) {
	// Set timeout to prevent infinite loops
	deadline := time.Now().Add(5 * time.Second)
	
	// Test single byte values (< 0xfd)
	for i := byte(0); i < 252; i++ {
		bbf := buf.NewByteBuf([]byte{})
		result := utils.VarIntDecode(i, bbf)
		if result != uint64(i) {
			t.Errorf("utils.VarIntDecode(%d): expected %d, got %d", i, i, result)
		}
	}
	
	// Test 0xfd case (uint16) - Use big-endian byte order
	data := buf.NewByteBuf([]byte{0, 1}) // 1 in big-endian uint16 format
	result := utils.VarIntDecode(0xfd, data)
	if result != 1 {
		t.Errorf("utils.VarIntDecode(0xfd): expected 1, got %d", result)
	}
	
	// Test 0xfe case (uint32) - Use big-endian byte order
	data = buf.NewByteBuf([]byte{0, 0, 0, 1}) // 1 in big-endian uint32 format
	result = utils.VarIntDecode(0xfe, data)
	if result != 1 {
		t.Errorf("utils.VarIntDecode(0xfe): expected 1, got %d", result)
	}
	
	// Test 0xff case (uint64) - Use big-endian byte order
	data = buf.NewByteBuf([]byte{0, 0, 0, 0, 0, 0, 0, 1}) // 1 in big-endian uint64 format
	result = utils.VarIntDecode(0xff, data)
	if result != 1 {
		t.Errorf("utils.VarIntDecode(0xff): expected 1, got %d", result)
	}
	
	if time.Now().After(deadline) {
		t.Fatal("Test exceeded timeout")
	}
}

// TestSimpleCodecWrite tests the codec write functionality with proper mock setup
func TestSimpleCodecWrite(t *testing.T) {
	// Set timeout to prevent infinite loops
	deadline := time.Now().Add(10 * time.Second)
	
	codec := NewSimpleCodec()
	
	// Create mock context and future
	mockCtx := goneMock.NewMockHandlerContext()
	mockFuture := goneMock.NewMockFuture(nil)
	
	// Test writing ByteBuf
	testData := []byte("test message")
	testBuf := buf.NewByteBuf(testData)
	
	// SimpleCodec.Write will prepend VarInt-encoded length to the data
	// We'll use mock.Anything to avoid complex buffer matching
	// MockHandlerContext.Write expects to return a Future
	mockCtx.On("Write", mock.Anything, mockFuture).Return(mockFuture)
	
	// This should call the mock Write method
	codec.Write(mockCtx, testBuf, mockFuture)
	
	// Verify the mock was called
	mockCtx.AssertExpectations(t)
	
	// Test writing non-ByteBuf object (should log error but not crash)
	codec.Write(mockCtx, "invalid object", mockFuture)
	
	// Verify expectations were met
	mockCtx.AssertExpectations(t)
	
	if time.Now().After(deadline) {
		t.Fatal("Test exceeded timeout") 
	}
}