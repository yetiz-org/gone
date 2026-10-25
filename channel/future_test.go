package channel

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	concurrent "github.com/yetiz-org/goth-concurrent"
)

// Test Future interface compliance and basic functionality
func TestDefaultFuture_InterfaceCompliance(t *testing.T) {
	channel := &DefaultChannel{}
	channel.init(channel)

	future := NewFuture(channel)

	// Verify Future interface compliance
	var _ Future = future

	// Test basic properties
	assert.False(t, future.IsDone(), "New future should not be done")
}

// Test concurrent future completion - THREAD SAFETY CRITICAL
func TestDefaultFuture_ConcurrentCompletion(t *testing.T) {
	channel := &DefaultChannel{}
	channel.init(channel)

	const numGoroutines = 100
	const futuresPerGoroutine = 50

	var wg sync.WaitGroup
	var successCount, failureCount int64

	// Concurrent future completion
	for i := range numGoroutines {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for j := range futuresPerGoroutine {
				future := NewFuture(channel)

				// Randomly complete with success or failure
				if (goroutineID+j)%2 == 0 {
					// Complete with success
					if future.Completable().Complete(channel) {
						atomic.AddInt64(&successCount, 1)
					}
				} else {
					// Complete with failure
					testError := fmt.Errorf("test error %d-%d", goroutineID, j)
					if future.Completable().Fail(testError) {
						atomic.AddInt64(&failureCount, 1)
					}
				}

				// Verify completion state
				assert.True(t, future.IsDone(), "Future should be done after completion")
			}
		}(i)
	}

	wg.Wait()

	expectedTotal := int64(numGoroutines * futuresPerGoroutine)
	actualTotal := successCount + failureCount

	t.Logf("Successful completions: %d, Failed completions: %d, Total: %d",
		successCount, failureCount, actualTotal)

	assert.Equal(t, expectedTotal, actualTotal, "All futures should be completed")
	assert.Greater(t, successCount, int64(0), "Should have successful completions")
	assert.Greater(t, failureCount, int64(0), "Should have failed completions")
}

// Test concurrent future waiting - THREAD SAFETY CRITICAL
func TestDefaultFuture_ConcurrentWaiting(t *testing.T) {
	channel := &DefaultChannel{}
	channel.init(channel)

	const numFutures = 100
	const waitersPerFuture = 10

	futures := make([]Future, numFutures)

	// Create futures
	for i := range numFutures {
		futures[i] = NewFuture(channel)
	}

	var wg sync.WaitGroup
	var totalWaits int64

	// Start waiters
	for i := range numFutures {
		for j := range waitersPerFuture {
			wg.Add(1)
			go func(futureIndex, waiterID int) {
				defer wg.Done()

				future := futures[futureIndex]

				// Wait for completion with timeout
				startTime := time.Now()
				done := make(chan bool, 1)

				go func() {
					// Use Await() to wait for completion
					future.Await()
					done <- true
				}()

				select {
				case <-done:
					duration := time.Since(startTime)
					if duration < time.Second { // Reasonable completion time
						atomic.AddInt64(&totalWaits, 1)
					}
				case <-time.After(time.Second):
					t.Errorf("Future %d timeout for waiter %d", futureIndex, waiterID)
				}
			}(i, j)
		}
	}

	// Complete futures after a short delay
	go func() {
		time.Sleep(time.Millisecond * 10)
		for i, future := range futures {
			if i%2 == 0 {
				future.Completable().Complete(channel)
			} else {
				future.Completable().Fail(fmt.Errorf("test failure %d", i))
			}
		}
	}()

	wg.Wait()

	expectedWaits := int64(numFutures * waitersPerFuture)
	t.Logf("Successful waits: %d out of %d", totalWaits, expectedWaits)

	assert.Equal(t, expectedWaits, totalWaits, "All waiters should complete successfully")
}

// Test concurrent listener registration - HIGH CONCURRENCY STRESS TEST
func TestDefaultFuture_ConcurrentListeners(t *testing.T) {
	channel := &DefaultChannel{}
	channel.init(channel)

	const numFutures = 50
	const listenersPerFuture = 20
	const expectedCalls = numFutures * listenersPerFuture
	futures := make([]Future, numFutures)
	results := make([]any, numFutures)
	errors := make([]error, numFutures)
	calls := make([]atomic.Int32, expectedCalls)
	completed := make(chan struct{}, expectedCalls)
	complete := func(i int) {
		if errors[i] == nil {
			assert.True(t, futures[i].Completable().Complete(results[i]))
		} else {
			assert.True(t, futures[i].Completable().Fail(errors[i]))
		}
	}
	for i := range numFutures {
		futures[i] = NewFuture(channel)
		if i%3 == 0 {
			results[i] = fmt.Sprintf("result-%d", i)
		} else {
			errors[i] = fmt.Errorf("listener failure %d", i)
		}
		if i%2 == 0 {
			complete(i)
		}
	}

	var registrations sync.WaitGroup
	for i := range numFutures {
		for j := range listenersPerFuture {
			registrations.Go(func() {
				futures[i].AddListener(concurrent.NewFutureListener(func(result concurrent.Future) {
					assert.True(t, result.IsDone(), "future %d listener %d", i, j)
					assert.Equal(t, errors[i] == nil, result.IsSuccess())
					assert.Equal(t, results[i], result.Get())
					assert.Equal(t, results[i], result.GetNow())
					assert.ErrorIs(t, result.Error(), errors[i])
					calls[i*listenersPerFuture+j].Add(1)
					select {
					case completed <- struct{}{}:
					default:
						t.Errorf("extra callback for future %d listener %d", i, j)
					}
				}))
			})
		}
	}
	registrations.Wait()
	for i := range numFutures {
		if i%2 != 0 {
			complete(i)
		}
		assert.False(t, futures[i].Completable().Complete(nil))
		assert.False(t, futures[i].Completable().Fail(fmt.Errorf("second completion")))
	}

	timeout := time.NewTimer(time.Second)
	defer timeout.Stop()
	for i := range expectedCalls {
		select {
		case <-completed:
		case <-timeout.C:
			t.Fatalf("received %d of %d listener callbacks", i, expectedCalls)
		}
	}
	for i := range calls {
		assert.Equal(t, int32(1), calls[i].Load(), "listener %d must run exactly once", i)
	}
}

// Test future cancellation under concurrent access
func TestDefaultFuture_ConcurrentCancellation(t *testing.T) {
	channel := &DefaultChannel{}
	channel.init(channel)

	const numFutures = 200
	const operationsPerFuture = 10

	var wg sync.WaitGroup
	var totalOperations atomic.Int64
	for i := range numFutures {
		wg.Go(func() {
			future := NewFuture(channel)
			var accepted [operationsPerFuture]bool
			var errors [operationsPerFuture]error
			var operations sync.WaitGroup
			for j := range operationsPerFuture {
				operations.Go(func() {
					totalOperations.Add(1)
					switch (i + j) % 4 {
					case 0:
						accepted[j] = future.Completable().Cancel()
					case 1:
						accepted[j] = future.Completable().Complete(i*operationsPerFuture + j)
					case 2:
						errors[j] = fmt.Errorf("operation error %d-%d", i, j)
						accepted[j] = future.Completable().Fail(errors[j])
					case 3:
						if future.IsDone() {
							future.Await()
							assert.True(t, future.IsDone(), "future %d remains done", i)
						}
					}
				})
			}
			operations.Wait()

			winner, winners := -1, 0
			for j, ok := range accepted {
				if ok {
					winner = j
					winners++
				}
			}
			if !assert.Equal(t, 1, winners, "future %d must accept exactly one terminal transition", i) {
				return
			}
			select {
			case <-future.Done():
			default:
				t.Errorf("future %d winner %d did not close Done", i, winner)
				return
			}

			opType := (i + winner) % 4
			var result any
			if opType == 1 {
				result = i*operationsPerFuture + winner
			}
			assert.True(t, future.IsDone(), "future %d", i)
			assert.Equal(t, opType == 0, future.IsCancelled(), "future %d", i)
			assert.Equal(t, opType == 1, future.IsSuccess(), "future %d", i)
			assert.Equal(t, opType == 2, future.IsFail(), "future %d", i)
			assert.Equal(t, result, future.Get(), "future %d winner %d", i, winner)
			assert.Equal(t, result, future.GetNow(), "future %d winner %d", i, winner)
			assert.ErrorIs(t, future.Error(), errors[winner], "future %d winner %d", i, winner)
			if opType == 1 {
				assert.Same(t, channel, future.Channel(), "future %d", i)
			} else {
				assert.Nil(t, future.Channel(), "future %d", i)
			}
		})
	}
	wg.Wait()
	assert.Equal(t, int64(numFutures*operationsPerFuture), totalOperations.Load())
}

// Test future result retrieval under concurrent access
func TestDefaultFuture_ConcurrentResultRetrieval(t *testing.T) {
	channel := &DefaultChannel{}
	channel.init(channel)

	const numFutures = 100
	const retrieversPerFuture = 15
	futures := make([]Future, numFutures)
	expectedResults := make([]any, numFutures)
	expectedErrors := make([]error, numFutures)
	for i := range numFutures {
		futures[i] = NewFuture(channel)
		if i%2 == 0 {
			expectedResults[i] = fmt.Sprintf("result-%d", i)
			assert.True(t, futures[i].Completable().Complete(expectedResults[i]))
		} else {
			expectedErrors[i] = fmt.Errorf("error-%d", i)
			assert.True(t, futures[i].Completable().Fail(expectedErrors[i]))
		}
	}

	var wg sync.WaitGroup
	var successfulRetrievals atomic.Int64
	for i := range numFutures {
		for range retrieversPerFuture {
			wg.Go(func() {
				future := futures[i]
				valid := assert.True(t, future.IsDone())
				valid = assert.Equal(t, expectedResults[i], future.Get()) && valid
				valid = assert.Equal(t, expectedResults[i], future.GetNow()) && valid
				valid = assert.ErrorIs(t, future.Error(), expectedErrors[i]) && valid
				if expectedErrors[i] == nil {
					valid = assert.True(t, future.IsSuccess()) && valid
					valid = assert.Same(t, channel, future.Channel()) && valid
				} else {
					valid = assert.True(t, future.IsFail()) && valid
					valid = assert.Nil(t, future.Channel()) && valid
				}
				if valid {
					successfulRetrievals.Add(1)
				}
			})
		}
	}
	wg.Wait()
	assert.Equal(t, int64(numFutures*retrieversPerFuture), successfulRetrievals.Load())
}

// Test memory consistency in future operations
func TestDefaultFuture_MemoryConsistency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory consistency test in short mode")
	}

	channel := &DefaultChannel{}
	channel.init(channel)

	const numGoroutines = 200
	const operationsPerGoroutine = 100
	rejectedError := fmt.Errorf("second completion")

	var wg sync.WaitGroup
	var operationCount atomic.Int64
	for i := range numGoroutines {
		wg.Go(func() {
			requests := make(chan Future)
			var publisher sync.WaitGroup
			var payload [2]int
			var expectedError error
			publisher.Go(func() {
				j := 0
				for future := range requests {
					payload = [2]int{i, j}
					switch (i + j) % 6 {
					case 0, 3, 4, 5:
						assert.True(t, future.Completable().Complete(&payload))
					case 1:
						assert.True(t, future.Completable().Fail(expectedError))
					case 2:
						assert.True(t, future.Completable().Cancel())
					}
					j++
				}
			})
			defer func() {
				close(requests)
				publisher.Wait()
			}()
			timeout := time.NewTimer(5 * time.Second)
			defer timeout.Stop()
			for j := range operationsPerGoroutine {
				future := NewFuture(channel)
				assert.False(t, future.IsDone())
				assert.False(t, future.IsSuccess())
				assert.False(t, future.IsFail())
				assert.False(t, future.IsCancelled())
				assert.Nil(t, future.GetNow())
				assert.NoError(t, future.Error())
				assert.Nil(t, future.Channel())
				select {
				case <-future.Done():
					t.Errorf("future %d-%d closed Done before a transition", i, j)
					return
				default:
				}

				opType := (i + j) % 6
				expectedError = nil
				if opType == 1 {
					expectedError = fmt.Errorf("error %d-%d", i, j)
				}
				requests <- future
				select {
				case <-future.Done():
				case <-timeout.C:
					t.Errorf("future %d-%d did not publish completion", i, j)
					return
				}

				// Done, not a separate producer join, publishes these writes.
				assert.Equal(t, [2]int{i, j}, payload)
				assert.False(t, future.Completable().Complete(nil))
				assert.False(t, future.Completable().Fail(rejectedError))
				assert.False(t, future.Completable().Cancel())
				assert.True(t, future.IsDone())
				assert.Equal(t, opType != 1 && opType != 2, future.IsSuccess())
				assert.Equal(t, opType == 1, future.IsFail())
				assert.Equal(t, opType == 2, future.IsCancelled())
				assert.ErrorIs(t, future.Error(), expectedError)
				var result any
				if opType != 1 && opType != 2 {
					result = &payload
					assert.Same(t, channel, future.Channel())
				} else {
					assert.Nil(t, future.Channel())
				}
				assert.Equal(t, result, future.Get())
				assert.Equal(t, result, future.GetNow())
				operationCount.Add(1)
			}
		})
	}
	wg.Wait()
	assert.Equal(t, int64(numGoroutines*operationsPerGoroutine), operationCount.Load())
}

// Benchmark concurrent future operations
func BenchmarkDefaultFuture_ConcurrentOperations(b *testing.B) {
	channel := &DefaultChannel{}
	channel.init(channel)

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			future := NewFuture(channel)

			if i%2 == 0 {
				future.Completable().Complete(channel)
			} else {
				future.Completable().Fail(fmt.Errorf("benchmark error %d", i))
				_ = future.Error()
			}

			_ = future.IsDone()
			i++
		}
	})
}

// Benchmark future creation
func BenchmarkDefaultFuture_Creation(b *testing.B) {
	channel := &DefaultChannel{}
	channel.init(channel)

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			future := NewFuture(channel)
			_ = future.Channel()
		}
	})
}

// Test edge case: rapid completion cycles
func TestDefaultFuture_RapidCompletionCycles(t *testing.T) {
	channel := &DefaultChannel{}
	channel.init(channel)

	const cycles = 1000

	for i := range cycles {
		future := NewFuture(channel)

		// Rapid complete and check
		if i%2 == 0 {
			future.Completable().Complete(channel)
			assert.Nil(t, future.Error(), "Should be successful")
		} else {
			future.Completable().Fail(fmt.Errorf("rapid error %d", i))
			assert.NotNil(t, future.Error(), "Should be failed")
		}

		assert.True(t, future.IsDone(), "Should be done")
	}
}

// Test error handling consistency
func TestDefaultFuture_ErrorHandlingConsistency(t *testing.T) {
	channel := &DefaultChannel{}
	channel.init(channel)

	const numErrors = 100

	futures := make([]Future, numErrors)
	expectedErrors := make([]error, numErrors)

	// Create futures with different error types
	for i := range numErrors {
		futures[i] = NewFuture(channel)
		expectedErrors[i] = fmt.Errorf("error type %d: %s", i%5, "test error message")

		futures[i].Completable().Fail(expectedErrors[i])
	}

	// Verify error consistency
	for i, future := range futures {
		assert.True(t, future.IsDone(), "Future should be done")
		assert.NotNil(t, future.Error(), "Should have error")
		assert.Equal(t, expectedErrors[i].Error(), future.Error().Error(), "Error message should match")
	}
}
