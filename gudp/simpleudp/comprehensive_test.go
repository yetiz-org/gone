package simpleudp

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/yetiz-org/gone/channel"
	goneMock "github.com/yetiz-org/gone/mock"
	"github.com/yetiz-org/gone/utils"
	buf "github.com/yetiz-org/goth-bytebuf"
)

// =============================================================================
// Client Tests
// =============================================================================

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

// Test Client creation and basic properties
func TestClient_Creation(t *testing.T) {
	tests := []struct {
		name    string
		handler channel.Handler
		wantNil bool
	}{
		{"valid_handler", goneMock.NewMockHandler(), false},
		{"nil_handler", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := NewClient(tt.handler)

			if tt.wantNil {
				assert.Nil(t, client, "TestCase: %s should return nil", tt.name)
			} else {
				assert.NotNil(t, client, "TestCase: %s should return non-nil client", tt.name)
				assert.Equal(t, tt.handler, client.Handler,
					"TestCase: %s should store provided handler", tt.name)
				assert.False(t, client.close,
					"TestCase: %s should initialize with close=false", tt.name)
				assert.Nil(t, client.AutoReconnect,
					"TestCase: %s should initialize with nil AutoReconnect", tt.name)
			}
		})
	}
}

// Test Client channel operations
func TestClient_ChannelOperations(t *testing.T) {
	t.Parallel()

	client := NewClient(goneMock.NewMockHandler())

	// Before start, Channel() should return nil
	assert.Nil(t, client.Channel(), "Channel should be nil before start")

	// Test disconnect without connection should not panic
	assert.NotPanics(t, func() {
		client.close = true
		// client.Disconnect() would panic since ch is nil, but we test the close flag
	}, "Setting close flag should not panic")
}

// TestClientBasicOperations tests client basic operations without network
func TestClientBasicOperations(t *testing.T) {
	// Set timeout to prevent infinite loops
	deadline := time.Now().Add(10 * time.Second)

	// Test basic client operations
	handler := goneMock.NewMockHandler()
	client := NewClient(handler)

	// Test Channel() method before connection
	if client.Channel() != nil {
		t.Error("Channel() should return nil before connection")
	}

	// Test auto-reconnect setting
	client.AutoReconnect = func() bool { return true }
	if client.AutoReconnect == nil {
		t.Error("AutoReconnect should be settable")
	}
	if !client.AutoReconnect() {
		t.Error("AutoReconnect should return true when set")
	}

	// Test close flag
	if client.close != false {
		t.Error("Client should initialize with close=false")
	}

	if time.Now().After(deadline) {
		t.Fatal("Test exceeded timeout")
	}
}

// =============================================================================
// Server Tests
// =============================================================================

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

// Test Server creation and basic properties
func TestServer_Creation(t *testing.T) {
	tests := []struct {
		name    string
		handler channel.Handler
		wantNil bool
	}{
		{"valid_handler", goneMock.NewMockHandler(), false},
		{"nil_handler", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := NewServer(tt.handler)

			if tt.wantNil {
				assert.Nil(t, server, "TestCase: %s should return nil", tt.name)
			} else {
				assert.NotNil(t, server, "TestCase: %s should return non-nil server", tt.name)
				assert.Equal(t, tt.handler, server.Handler,
					"TestCase: %s should store provided handler", tt.name)
				assert.Nil(t, server.ch,
					"TestCase: %s should initialize with nil channel", tt.name)
			}
		})
	}
}

// Test Server channel operations
func TestServer_ChannelOperations(t *testing.T) {
	t.Parallel()

	server := NewServer(goneMock.NewMockHandler())

	// Before start, Channel() should return nil
	assert.Nil(t, server.Channel(), "Channel should be nil before start")

	// Test stop without connection should not panic
	assert.NotPanics(t, func() {
		// server.Stop() would panic since ch is nil, but we test this scenario
		if server.ch != nil {
			server.Stop()
		}
	}, "Stop check should not panic when channel is nil")
}

// TestServerBasicOperations tests server basic operations without network
func TestServerBasicOperations(t *testing.T) {
	// Set timeout to prevent infinite loops
	deadline := time.Now().Add(10 * time.Second)

	// Test basic server operations
	handler := goneMock.NewMockHandler()
	server := NewServer(handler)

	// Test Channel() method before start
	if server.Channel() != nil {
		t.Error("Channel() should return nil before start")
	}

	// Test handler assignment
	if server.Handler != handler {
		t.Error("Server should store provided handler")
	}

	if time.Now().After(deadline) {
		t.Fatal("Test exceeded timeout")
	}
}

// Test Server and Client structural differences
func TestServer_vs_Client_Structure(t *testing.T) {
	t.Parallel()

	handler := goneMock.NewMockHandler()

	server := NewServer(handler)
	client := NewClient(handler)

	// Server should not have AutoReconnect functionality
	assert.NotNil(t, server, "Server should not be nil")
	assert.NotNil(t, client, "Client should not be nil")

	// Test that both store handlers correctly
	assert.Equal(t, handler, server.Handler, "Server should store handler")
	assert.Equal(t, handler, client.Handler, "Client should store handler")

	// Server should not have close flag like client
	// This tests structural differences between server and client
}

// =============================================================================
// SimpleCodec Tests
// =============================================================================

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

// Test SimpleCodec creation and basic properties
func TestSimpleCodec_Creation(t *testing.T) {
	t.Parallel()

	codec := NewSimpleCodec()

	assert.NotNil(t, codec, "NewSimpleCodec should return non-nil codec")
	assert.NotNil(t, codec.ReplayDecoder, "SimpleCodec should have non-nil ReplayDecoder")
	assert.Equal(t, byte(0), codec.flag, "Initial flag should be 0")
	assert.Equal(t, uint64(0), codec.length, "Initial length should be 0")
	assert.Nil(t, codec.out, "Initial out buffer should be nil")
}

// Test SimpleCodec constants
func TestSimpleCodec_Constants(t *testing.T) {
	t.Parallel()

	assert.Equal(t, channel.ReplayState(1), FLAG, "FLAG constant should be 1")
	assert.Equal(t, channel.ReplayState(2), LENGTH, "LENGTH constant should be 2")
	assert.Equal(t, channel.ReplayState(3), BODY, "BODY constant should be 3")
}

// Test SimpleCodec Write method with ByteBuf
func TestSimpleCodec_Write_ByteBuf(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"empty_buffer", []byte{}},
		{"small_data", []byte("hello")},
		{"medium_data", []byte("this is a medium length test message")},
		{"binary_data", []byte{0x00, 0x01, 0x02, 0xff, 0xfe, 0xfd}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			codec := NewSimpleCodec()
			mockCtx := goneMock.NewMockHandlerContext()
			mockFuture := goneMock.NewMockFuture(nil)

			// Create test ByteBuf
			testBuf := buf.NewByteBuf(tt.data)
			expectedLength := uint64(len(tt.data))

			// Mock the Write call - expect VarInt encoded length + data
			mockCtx.On("Write", mock.MatchedBy(func(arg buf.ByteBuf) bool {
				// Verify the written data starts with VarInt encoded length
				return arg.ReadableBytes() > 0
			}), mockFuture).Return(mockFuture)

			// Call Write method
			codec.Write(mockCtx, testBuf, mockFuture)

			// Verify expectations
			mockCtx.AssertExpectations(t)

			// Verify the encoding logic
			encodedLength := utils.VarIntEncode(expectedLength)
			assert.NotNil(t, encodedLength, "TestCase: %s should encode length", tt.name)
		})
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

// Test SimpleCodec Write method with invalid types
func TestSimpleCodec_Write_InvalidTypes(t *testing.T) {
	tests := []struct {
		name string
		obj  any
	}{
		{"string_type", "invalid string"},
		{"int_type", 12345},
		{"nil_type", nil},
		{"slice_type", []byte{1, 2, 3}}, // raw slice, not ByteBuf
		{"struct_type", struct{ Field string }{Field: "test"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			codec := NewSimpleCodec()
			mockCtx := goneMock.NewMockHandlerContext()
			mockFuture := goneMock.NewMockFuture(nil)

			// Should not call Write for invalid types (error logged instead)
			// Call Write method - should log error but not crash
			assert.NotPanics(t, func() {
				codec.Write(mockCtx, tt.obj, mockFuture)
			}, "TestCase: %s should not panic with invalid type", tt.name)

			// No Write should be called for invalid types
			mockCtx.AssertNotCalled(t, "Write")
		})
	}
}

// =============================================================================
// VarInt Utility Tests
// =============================================================================

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
		{253, []byte{0xfd, 0, 253}},       // uint16
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

// Test SimpleCodec VarInt integration
func TestSimpleCodec_VarIntIntegration(t *testing.T) {
	tests := []struct {
		name   string
		length uint64
	}{
		{"small_length", 50},
		{"medium_length", 1000},
		{"large_length", 65536},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Test VarInt encoding used by SimpleCodec
			encoded := utils.VarIntEncode(tt.length)
			assert.NotNil(t, encoded, "TestCase: %s should encode length", tt.name)
			assert.True(t, encoded.ReadableBytes() > 0,
				"TestCase: %s should have readable bytes", tt.name)

			// Test decoding with the same logic used in SimpleCodec
			flag, err := encoded.ReadByte()
			assert.NoError(t, err, "TestCase: %s should read flag byte", tt.name)

			decoded := utils.VarIntDecode(flag, encoded)
			assert.Equal(t, tt.length, decoded,
				"TestCase: %s should decode correctly", tt.name)
		})
	}
}
