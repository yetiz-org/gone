package simpletcp

import (
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/yetiz-org/gone/utils"
	buf "github.com/yetiz-org/goth-bytebuf"
)

// Test VarInt encoding/decoding correctness
func TestVarInt_BasicCorrectness(t *testing.T) {
	testCases := []uint64{
		0, 1, 252, 253, 254, 255, 256,
		math.MaxUint16 - 1, math.MaxUint16, math.MaxUint16 + 1,
		math.MaxUint32 - 1, math.MaxUint32, math.MaxUint32 + 1,
		math.MaxUint64 - 1, math.MaxUint64,
	}

	for _, original := range testCases {
		t.Run(fmt.Sprintf("value_%d", original), func(t *testing.T) {
			// Encode
			encoded := utils.VarIntEncode(original)
			assert.NotNil(t, encoded, "Encoded result should not be nil")
			assert.Greater(t, len(encoded.Bytes()), 0, "Encoded length should be greater than 0")

			// Decode
			flag := encoded.MustReadByte()
			decoded := utils.VarIntDecode(flag, encoded)

			assert.Equal(t, original, decoded, "Decoded value should match original")
		})
	}
}

// Test concurrent VarInt encoding - THREAD SAFETY CRITICAL
func TestVarInt_ConcurrentEncoding(t *testing.T) {
	const numGoroutines = 200
	const encodingsPerGoroutine = 500

	var wg sync.WaitGroup
	var successfulEncodings int64

	// Test values covering all encoding ranges
	testValues := []uint64{
		0, 1, 100, 252,           // 1-byte range
		253, 1000, 65535,         // 2-byte range  
		65536, 1000000, math.MaxUint32, // 4-byte range
		math.MaxUint32 + 1, math.MaxUint64, // 8-byte range
	}

	// Concurrent encoding
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < encodingsPerGoroutine; j++ {
				// Select test value based on iteration
				testValue := testValues[(goroutineID+j)%len(testValues)]
				
				// Add variation to create unique values
				testValue += uint64(goroutineID*encodingsPerGoroutine + j)

				// Encode value
				encoded := utils.VarIntEncode(testValue)
				
				// Verify encoding properties
				if encoded != nil && len(encoded.Bytes()) > 0 {
					// Verify proper encoding size based on value range
					expectedSize := getExpectedEncodingSize(testValue)
					actualSize := len(encoded.Bytes())
					
					if actualSize == expectedSize {
						atomic.AddInt64(&successfulEncodings, 1)
					} else {
						t.Errorf("Goroutine %d: Wrong encoding size for value %d. Expected: %d, Got: %d",
							goroutineID, testValue, expectedSize, actualSize)
					}
				}
			}
		}(i)
	}

	wg.Wait()

	expectedTotal := int64(numGoroutines * encodingsPerGoroutine)
	t.Logf("Successful encodings: %d out of %d", successfulEncodings, expectedTotal)
	
	assert.Equal(t, expectedTotal, successfulEncodings, "All encodings should be successful")
}

// Test concurrent VarInt decoding - THREAD SAFETY CRITICAL
func TestVarInt_ConcurrentDecoding(t *testing.T) {
	const numGoroutines = 200
	const decodingsPerGoroutine = 500

	var wg sync.WaitGroup
	var successfulDecodings int64

	// Pre-encoded test data
	testData := []struct {
		original uint64
		encoded  buf.ByteBuf
	}{}

	// Prepare test data
	testValues := []uint64{
		0, 1, 100, 252, 253, 1000, 65535, 65536, 1000000, 
		math.MaxUint32, math.MaxUint32 + 1, math.MaxUint64,
	}

	for _, value := range testValues {
		encoded := utils.VarIntEncode(value)
		testData = append(testData, struct {
			original uint64
			encoded  buf.ByteBuf
		}{value, encoded})
	}

	// Concurrent decoding
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < decodingsPerGoroutine; j++ {
				// Select test data based on iteration
				testEntry := testData[(goroutineID+j)%len(testData)]
				
				// Create a copy for concurrent access
				encodedCopy := buf.NewByteBuf(testEntry.encoded.Bytes())
				
				// Decode value
				flag := encodedCopy.MustReadByte()
				decoded := utils.VarIntDecode(flag, encodedCopy)
				
				// Verify correctness
				if decoded == testEntry.original {
					atomic.AddInt64(&successfulDecodings, 1)
				} else {
					t.Errorf("Goroutine %d: Decoding mismatch. Expected: %d, Got: %d",
						goroutineID, testEntry.original, decoded)
				}
			}
		}(i)
	}

	wg.Wait()

	expectedTotal := int64(numGoroutines * decodingsPerGoroutine)
	t.Logf("Successful decodings: %d out of %d", successfulDecodings, expectedTotal)
	
	assert.Equal(t, expectedTotal, successfulDecodings, "All decodings should be successful")
}

// Test concurrent encode-decode cycles - MEMORY CONSISTENCY TEST
func TestVarInt_ConcurrentEncodeDecode(t *testing.T) {
	const numGoroutines = 150
	const cyclesPerGoroutine = 300

	var wg sync.WaitGroup
	var successfulCycles int64

	// Concurrent encode-decode cycles
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < cyclesPerGoroutine; j++ {
				// Generate test value
				testValue := uint64(goroutineID*cyclesPerGoroutine + j)
				
				// Apply different scaling factors to test various ranges
				switch j % 4 {
				case 0: // Small values (1-byte encoding)
					testValue = testValue % 253
				case 1: // Medium values (2-byte encoding)
					testValue = 253 + (testValue % (math.MaxUint16 - 253))
				case 2: // Large values (4-byte encoding)
					testValue = math.MaxUint16 + 1 + (testValue % (math.MaxUint32 - math.MaxUint16))
				case 3: // Very large values (8-byte encoding)
					testValue = math.MaxUint32 + 1 + testValue
				}

				// Encode
				encoded := utils.VarIntEncode(testValue)
				if encoded == nil || len(encoded.Bytes()) == 0 {
					t.Errorf("Goroutine %d: Encoding failed for value %d", goroutineID, testValue)
					continue
				}

				// Decode
				flag := encoded.MustReadByte()
				decoded := utils.VarIntDecode(flag, encoded)

				// Verify round-trip correctness
				if decoded == testValue {
					atomic.AddInt64(&successfulCycles, 1)
				} else {
					t.Errorf("Goroutine %d: Round-trip failed. Original: %d, Decoded: %d", 
						goroutineID, testValue, decoded)
				}
			}
		}(i)
	}

	wg.Wait()

	expectedTotal := int64(numGoroutines * cyclesPerGoroutine)
	t.Logf("Successful encode-decode cycles: %d out of %d", successfulCycles, expectedTotal)
	
	assert.Equal(t, expectedTotal, successfulCycles, "All encode-decode cycles should be successful")
}

// Test VarInt under high-frequency operations - STRESS TEST
func TestVarInt_HighFrequencyOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping high-frequency test in short mode")
	}

	const numGoroutines = 100
	const operationsPerGoroutine = 1000

	var wg sync.WaitGroup
	var totalOperations int64

	// High-frequency mixed operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < operationsPerGoroutine; j++ {
				operationType := (goroutineID + j) % 3
				testValue := uint64(goroutineID*operationsPerGoroutine + j)

				switch operationType {
				case 0: // Pure encoding
					encoded := utils.VarIntEncode(testValue)
					if encoded != nil && len(encoded.Bytes()) > 0 {
						atomic.AddInt64(&totalOperations, 1)
					}

				case 1: // Pure decoding from pre-encoded data
					preEncoded := utils.VarIntEncode(testValue)
					if preEncoded != nil {
						flag := preEncoded.MustReadByte()
						decoded := utils.VarIntDecode(flag, preEncoded)
						if decoded == testValue {
							atomic.AddInt64(&totalOperations, 1)
						}
					}

				case 2: // Mixed encode-decode with verification
					encoded := utils.VarIntEncode(testValue)
					if encoded != nil {
						flag := encoded.MustReadByte()
						decoded := utils.VarIntDecode(flag, encoded)
						if decoded == testValue {
							atomic.AddInt64(&totalOperations, 1)
						}
					}
				}
			}
		}(i)
	}

	wg.Wait()

	expectedTotal := int64(numGoroutines * operationsPerGoroutine)
	t.Logf("Successful operations: %d out of %d", totalOperations, expectedTotal)
	
	assert.Equal(t, expectedTotal, totalOperations, "All high-frequency operations should be successful")
}

// Test VarInt boundary values under concurrency
func TestVarInt_ConcurrentBoundaryValues(t *testing.T) {
	const numGoroutines = 50
	const iterationsPerGoroutine = 100

	// Critical boundary values
	boundaryValues := []uint64{
		0, 1, 252, 253, 254, 255, 256,
		math.MaxUint16 - 1, math.MaxUint16, math.MaxUint16 + 1,
		math.MaxUint32 - 1, math.MaxUint32, math.MaxUint32 + 1,
		math.MaxUint64 - 1, math.MaxUint64,
	}

	var wg sync.WaitGroup
	var successfulBoundaryTests int64

	// Test boundary values concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < iterationsPerGoroutine; j++ {
				// Select boundary value
				boundaryValue := boundaryValues[j%len(boundaryValues)]

				// Test encoding
				encoded := utils.VarIntEncode(boundaryValue)
				if encoded == nil || len(encoded.Bytes()) == 0 {
					t.Errorf("Goroutine %d: Failed to encode boundary value %d", goroutineID, boundaryValue)
					continue
				}

				// Test decoding
				flag := encoded.MustReadByte()
				decoded := utils.VarIntDecode(flag, encoded)

				// Verify correctness
				if decoded == boundaryValue {
					atomic.AddInt64(&successfulBoundaryTests, 1)
				} else {
					t.Errorf("Goroutine %d: Boundary value test failed. Expected: %d, Got: %d",
						goroutineID, boundaryValue, decoded)
				}
			}
		}(i)
	}

	wg.Wait()

	expectedTotal := int64(numGoroutines * iterationsPerGoroutine)
	t.Logf("Successful boundary tests: %d out of %d", successfulBoundaryTests, expectedTotal)
	
	assert.Equal(t, expectedTotal, successfulBoundaryTests, "All boundary value tests should pass")
}

// Benchmark concurrent VarInt encoding
func BenchmarkVarInt_ConcurrentEncoding(b *testing.B) {
	testValues := []uint64{100, 1000, 100000, math.MaxUint32, math.MaxUint64}
	
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			testValue := testValues[i%len(testValues)] + uint64(i)
			encoded := utils.VarIntEncode(testValue)
			_ = len(encoded.Bytes())
			i++
		}
	})
}

// Benchmark concurrent VarInt decoding
func BenchmarkVarInt_ConcurrentDecoding(b *testing.B) {
	// Pre-encoded test data
	testData := []buf.ByteBuf{
		utils.VarIntEncode(100),
		utils.VarIntEncode(1000),
		utils.VarIntEncode(100000),
		utils.VarIntEncode(math.MaxUint32),
		utils.VarIntEncode(math.MaxUint64),
	}
	
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			encoded := testData[i%len(testData)]
			encodedCopy := buf.NewByteBuf(encoded.Bytes())
			flag := encodedCopy.MustReadByte()
			decoded := utils.VarIntDecode(flag, encodedCopy)
			_ = decoded
			i++
		}
	})
}

// Benchmark full encode-decode cycle
func BenchmarkVarInt_ConcurrentEncodeDecode(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			testValue := uint64(i + 12345)
			encoded := utils.VarIntEncode(testValue)
			flag := encoded.MustReadByte()
			decoded := utils.VarIntDecode(flag, encoded)
			_ = decoded
			i++
		}
	})
}

// Helper function to determine expected encoding size
func getExpectedEncodingSize(value uint64) int {
	if value < 0xfd {
		return 1
	} else if value <= math.MaxUint16 {
		return 3 // 1 flag byte + 2 data bytes
	} else if value <= math.MaxUint32 {
		return 5 // 1 flag byte + 4 data bytes
	} else {
		return 9 // 1 flag byte + 8 data bytes
	}
}

// Test VarInt with edge cases and error conditions
func TestVarInt_EdgeCasesAndErrors(t *testing.T) {
	const numGoroutines = 50

	var wg sync.WaitGroup
	var successfulTests int64

	// Test edge cases concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			// Test various edge cases
			edgeCases := []uint64{
				0, 1, 252, 253, 254, 255,
				math.MaxUint16, math.MaxUint16 + 1,
				math.MaxUint32, math.MaxUint32 + 1,
				math.MaxUint64,
			}

			for _, testValue := range edgeCases {
				// Test normal encoding/decoding
				encoded := utils.VarIntEncode(testValue)
				if encoded != nil && len(encoded.Bytes()) > 0 {
					flag := encoded.MustReadByte()
					decoded := utils.VarIntDecode(flag, encoded)
					
					if decoded == testValue {
						atomic.AddInt64(&successfulTests, 1)
					}
				}
			}
		}(i)
	}

	wg.Wait()

	// Each goroutine tests 11 edge cases
	expectedTotal := int64(numGoroutines * 11)
	t.Logf("Successful edge case tests: %d out of %d", successfulTests, expectedTotal)
	
	assert.Equal(t, expectedTotal, successfulTests, "All edge case tests should pass")
}