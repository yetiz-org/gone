package utils

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Test Queue interface compliance and basic functionality
func TestQueue_BasicFunctionality(t *testing.T) {
	queue := &Queue{}

	// Test empty queue
	assert.Nil(t, queue.Pop(), "Pop from empty queue should return nil")

	// Test single push/pop
	testValue := "test-value"
	queue.Push(testValue)
	popped := queue.Pop()
	assert.Equal(t, testValue, popped, "Should retrieve the same value that was pushed")

	// Test queue is empty after pop
	assert.Nil(t, queue.Pop(), "Queue should be empty after popping the only item")
}

// Test FIFO behavior under normal conditions
func TestQueue_FIFOBehavior(t *testing.T) {
	queue := &Queue{}

	// Push multiple values
	values := []string{"first", "second", "third", "fourth", "fifth"}
	for _, value := range values {
		queue.Push(value)
	}

	// Pop values and verify FIFO order
	for _, expectedValue := range values {
		popped := queue.Pop()
		assert.Equal(t, expectedValue, popped, "Should maintain FIFO order")
	}

	// Queue should be empty
	assert.Nil(t, queue.Pop(), "Queue should be empty after all items popped")
}

// Test concurrent push operations - THREAD SAFETY CRITICAL
func TestQueue_ConcurrentPushOperations(t *testing.T) {
	queue := &Queue{}

	const numGoroutines = 100
	const pushesPerGoroutine = 100
	const totalExpectedPushes = numGoroutines * pushesPerGoroutine

	var wg sync.WaitGroup

	// Concurrent push operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < pushesPerGoroutine; j++ {
				value := map[string]interface{}{
					"goroutine": goroutineID,
					"iteration": j,
					"timestamp": time.Now().UnixNano(),
					"data":      "test-data",
				}
				queue.Push(value)
			}
		}(i)
	}

	wg.Wait()

	// Verify all items were pushed by counting pops
	popCount := 0
	for queue.Pop() != nil {
		popCount++
	}

	assert.Equal(t, totalExpectedPushes, popCount,
		"All pushed items should be retrievable")
}

// Test concurrent pop operations - THREAD SAFETY CRITICAL
func TestQueue_ConcurrentPopOperations(t *testing.T) {
	queue := &Queue{}

	const numItems = 1000
	const numPoppers = 50

	// Pre-populate queue
	for i := 0; i < numItems; i++ {
		queue.Push(map[string]interface{}{
			"id":   i,
			"data": "test-data-" + string(rune(i)),
		})
	}

	var wg sync.WaitGroup
	var totalPopped int64
	var successfulPops int64

	// Concurrent pop operations
	for i := 0; i < numPoppers; i++ {
		wg.Add(1)
		go func(popperID int) {
			defer wg.Done()

			poppedCount := int64(0)
			for {
				item := queue.Pop()
				if item == nil {
					break // Queue is empty
				}

				// Verify item structure
				if itemMap, ok := item.(map[string]interface{}); ok {
					if _, hasID := itemMap["id"]; hasID {
						atomic.AddInt64(&successfulPops, 1)
					}
				}
				poppedCount++
			}
			atomic.AddInt64(&totalPopped, poppedCount)
		}(i)
	}

	wg.Wait()

	assert.Equal(t, int64(numItems), totalPopped,
		"Should pop exactly the number of items that were pushed")
	assert.Equal(t, int64(numItems), successfulPops,
		"All popped items should have valid structure")

	// Queue should be empty
	assert.Nil(t, queue.Pop(), "Queue should be empty after concurrent pops")
}

// Test mixed concurrent push/pop operations - HIGH CONCURRENCY STRESS TEST
func TestQueue_ConcurrentPushPopOperations(t *testing.T) {
	queue := &Queue{}

	const numProducers = 50
	const numConsumers = 50
	const itemsPerProducer = 200
	const totalExpectedItems = numProducers * itemsPerProducer

	var wg sync.WaitGroup
	var producedCount, consumedCount int64

	startSignal := make(chan struct{})

	// Start producers
	for i := 0; i < numProducers; i++ {
		wg.Add(1)
		go func(producerID int) {
			defer wg.Done()

			<-startSignal // Wait for start signal

			for j := 0; j < itemsPerProducer; j++ {
				item := map[string]interface{}{
					"producer":  producerID,
					"sequence":  j,
					"timestamp": time.Now().UnixNano(),
					"data":      "producer-data",
				}
				queue.Push(item)
				atomic.AddInt64(&producedCount, 1)
			}
		}(i)
	}

	// Start consumers
	for i := 0; i < numConsumers; i++ {
		wg.Add(1)
		go func(consumerID int) {
			defer wg.Done()

			<-startSignal // Wait for start signal

			consumedThisConsumer := int64(0)
			maxAttempts := totalExpectedItems/numConsumers + 100 // Allow some buffer

			for attempts := 0; attempts < int(maxAttempts); attempts++ {
				item := queue.Pop()
				if item != nil {
					// Verify item structure
					if itemMap, ok := item.(map[string]interface{}); ok {
						if _, hasProducer := itemMap["producer"]; hasProducer {
							consumedThisConsumer++
						}
					}
				} else {
					// No item available, short sleep and retry
					time.Sleep(time.Microsecond)
				}
			}

			atomic.AddInt64(&consumedCount, consumedThisConsumer)
		}(i)
	}

	// Start all goroutines simultaneously
	close(startSignal)
	wg.Wait()

	// Allow a brief moment for any remaining operations
	time.Sleep(time.Millisecond * 10)

	// Count remaining items in queue
	remainingItems := int64(0)
	for queue.Pop() != nil {
		remainingItems++
	}

	totalItems := consumedCount + remainingItems

	t.Logf("Produced: %d, Consumed: %d, Remaining: %d, Total: %d",
		producedCount, consumedCount, remainingItems, totalItems)

	assert.Equal(t, int64(totalExpectedItems), producedCount,
		"Should produce exactly expected number of items")
	assert.Equal(t, int64(totalExpectedItems), totalItems,
		"Total consumed + remaining should equal produced")
}

// Test queue behavior with different data types - THREAD SAFETY
func TestQueue_ConcurrentMixedDataTypes(t *testing.T) {
	queue := &Queue{}

	const numGoroutines = 100

	var wg sync.WaitGroup
	dataTypes := []interface{}{
		"string-value",
		42,
		3.14159,
		true,
		map[string]string{"key": "value"},
		[]int{1, 2, 3, 4, 5},
		struct{ Name string }{"test-struct"},
	}

	// Concurrent push with different data types
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			dataType := dataTypes[goroutineID%len(dataTypes)]
			queue.Push(dataType)
		}(i)
	}

	wg.Wait()

	// Verify all items can be popped
	popCount := 0
	typeCount := make(map[string]int)

	for {
		item := queue.Pop()
		if item == nil {
			break
		}

		popCount++
		switch item.(type) {
		case string:
			typeCount["string"]++
		case int:
			typeCount["int"]++
		case float64:
			typeCount["float64"]++
		case bool:
			typeCount["bool"]++
		case map[string]string:
			typeCount["map"]++
		case []int:
			typeCount["slice"]++
		default:
			typeCount["struct"]++
		}
	}

	assert.Equal(t, numGoroutines, popCount, "Should pop all pushed items")
	assert.Greater(t, len(typeCount), 1, "Should have multiple data types")
}

// Test memory consistency under high concurrent stress
func TestQueue_MemoryConsistencyStress(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory consistency test in short mode")
	}

	queue := &Queue{}

	const numGoroutines = 200
	const operationsPerGoroutine = 500

	var wg sync.WaitGroup
	var pushCount, popCount int64

	// Mixed concurrent operations with high stress
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < operationsPerGoroutine; j++ {
				if j%2 == 0 {
					// Push operation
					item := map[string]interface{}{
						"id":    goroutineID*operationsPerGoroutine + j,
						"value": "stress-test-data",
					}
					queue.Push(item)
					atomic.AddInt64(&pushCount, 1)
				} else {
					// Pop operation
					item := queue.Pop()
					if item != nil {
						atomic.AddInt64(&popCount, 1)
					}
				}
			}
		}(i)
	}

	wg.Wait()

	// Count remaining items
	remainingItems := int64(0)
	for queue.Pop() != nil {
		remainingItems++
	}

	totalProcessed := popCount + remainingItems

	t.Logf("Pushes: %d, Pops: %d, Remaining: %d, Total Processed: %d",
		pushCount, popCount, remainingItems, totalProcessed)

	assert.Equal(t, pushCount, totalProcessed,
		"All pushed items should be accounted for (popped + remaining)")
}

// Benchmark concurrent queue operations
func BenchmarkQueue_ConcurrentOperations(b *testing.B) {
	queue := &Queue{}

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if i%2 == 0 {
				// Push operation
				queue.Push(map[string]interface{}{
					"id":   i,
					"data": "benchmark-data",
				})
			} else {
				// Pop operation
				_ = queue.Pop()
			}
			i++
		}
	})
}

// Benchmark push-only operations
func BenchmarkQueue_PushOnly(b *testing.B) {
	queue := &Queue{}

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			queue.Push("test-data")
			i++
		}
	})
}

// Benchmark pop-only operations (pre-populated queue)
func BenchmarkQueue_PopOnly(b *testing.B) {
	queue := &Queue{}

	// Pre-populate queue
	for i := 0; i < b.N*2; i++ {
		queue.Push("benchmark-data")
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = queue.Pop()
		}
	})
}

// Test edge case: rapid push/pop cycles
func TestQueue_RapidCycles(t *testing.T) {
	queue := &Queue{}

	const cycles = 10000

	for i := 0; i < cycles; i++ {
		// Push and immediately pop
		testValue := i
		queue.Push(testValue)
		popped := queue.Pop()

		assert.Equal(t, testValue, popped,
			"Should maintain consistency in rapid push/pop cycles")
	}
}

// Test nil value handling
func TestQueue_NilValueHandling(t *testing.T) {
	queue := &Queue{}

	// Push nil value
	queue.Push(nil)

	// Pop should return nil, but queue should not be empty
	popped := queue.Pop()
	assert.Nil(t, popped, "Should be able to push and pop nil values")

	// Queue should now be empty
	nextPop := queue.Pop()
	assert.Nil(t, nextPop, "Queue should be empty after popping nil value")
}
