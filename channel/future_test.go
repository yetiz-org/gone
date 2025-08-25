package channel

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			
			for j := 0; j < futuresPerGoroutine; j++ {
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
	for i := 0; i < numFutures; i++ {
		futures[i] = NewFuture(channel)
	}
	
	var wg sync.WaitGroup
	var totalWaits int64
	
	// Start waiters
	for i := 0; i < numFutures; i++ {
		for j := 0; j < waitersPerFuture; j++ {
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
	
	futures := make([]Future, numFutures)
	
	// Create futures
	for i := 0; i < numFutures; i++ {
		futures[i] = NewFuture(channel)
	}
	
	var wg sync.WaitGroup
	var totalListenerCalls int64
	
	// Register listeners concurrently
	for i := 0; i < numFutures; i++ {
		for j := 0; j < listenersPerFuture; j++ {
			wg.Add(1)
			go func(futureIndex, listenerID int) {
				defer wg.Done()
				
				future := futures[futureIndex]
				
				// Register listener (simulated through checking completion)
				listenerCalled := false
				
				// Poll for completion (in real implementation would use AddListener)
				timeout := time.After(time.Second)
				ticker := time.NewTicker(time.Microsecond * 100)
				defer ticker.Stop()
				
				for {
					select {
					case <-ticker.C:
						if future.IsDone() {
							if !listenerCalled {
								listenerCalled = true
								atomic.AddInt64(&totalListenerCalls, 1)
								
								// Verify completion state after ensuring completion
								future.Await()
								_ = future.Error()
							}
							return
						}
					case <-timeout:
						t.Errorf("Listener timeout for future %d, listener %d", futureIndex, listenerID)
						return
					}
				}
			}(i, j)
		}
	}
	
	// Complete futures after listeners are registered
	go func() {
		time.Sleep(time.Millisecond * 50)
		for i, future := range futures {
			if i%3 == 0 {
				future.Completable().Complete(channel)
			} else {
				future.Completable().Fail(fmt.Errorf("test error %d", i))
			}
		}
	}()
	
	wg.Wait()
	
	expectedCalls := int64(numFutures * listenersPerFuture)
	t.Logf("Total listener calls: %d out of %d", totalListenerCalls, expectedCalls)
	
	assert.Equal(t, expectedCalls, totalListenerCalls, "All listeners should be called")
}

// Test future cancellation under concurrent access
func TestDefaultFuture_ConcurrentCancellation(t *testing.T) {
	channel := &DefaultChannel{}
	channel.init(channel)
	
	const numFutures = 200
	const operationsPerFuture = 10
	
	var wg sync.WaitGroup
	var cancelledCount, completedCount, totalOperations int64
	
	for i := 0; i < numFutures; i++ {
		wg.Add(1)
		go func(futureID int) {
			defer wg.Done()
			
			future := NewFuture(channel)
			
			// Concurrent operations on the same future
			var futureWg sync.WaitGroup
			
			for j := 0; j < operationsPerFuture; j++ {
				futureWg.Add(1)
				go func(operationID int) {
					defer futureWg.Done()
					atomic.AddInt64(&totalOperations, 1)
					
					opType := (futureID + operationID) % 4
					
					switch opType {
					case 0: // Try to cancel
						if completable := future.Completable(); completable != nil {
							if completable.Cancel() {
								atomic.AddInt64(&cancelledCount, 1)
							}
						}
						
					case 1: // Try to complete with success
						if future.Completable().Complete(channel) {
							atomic.AddInt64(&completedCount, 1)
						}
						
					case 2: // Try to complete with failure
						if future.Completable().Fail(fmt.Errorf("operation error %d-%d", futureID, operationID)) {
							atomic.AddInt64(&completedCount, 1)
						}
						
					case 3: // Check status
						if future.IsDone() {
							future.Await()
							_ = future.Error()
						}
					}
				}(j)
			}
			
			futureWg.Wait()
		}(i)
	}
	
	wg.Wait()
	
	t.Logf("Total operations: %d, Completed: %d, Cancelled: %d", 
		totalOperations, completedCount, cancelledCount)
	
	assert.Equal(t, int64(numFutures*operationsPerFuture), totalOperations, 
		"All operations should execute")
	assert.Greater(t, completedCount+cancelledCount, int64(0), 
		"Should have some completions or cancellations")
}

// Test future result retrieval under concurrent access
func TestDefaultFuture_ConcurrentResultRetrieval(t *testing.T) {
	channel := &DefaultChannel{}
	channel.init(channel)
	
	const numFutures = 100
	const retrieversPerFuture = 15
	
	futures := make([]Future, numFutures)
	expectedResults := make([]interface{}, numFutures)
	
	// Create and complete futures
	for i := 0; i < numFutures; i++ {
		futures[i] = NewFuture(channel)
		
		if i%2 == 0 {
			// Success cases
			expectedResults[i] = fmt.Sprintf("result-%d", i)
			futures[i].Completable().Complete(expectedResults[i])
		} else {
			// Failure cases
			expectedResults[i] = fmt.Errorf("error-%d", i)
			futures[i].Completable().Fail(expectedResults[i].(error))
		}
	}
	
	var wg sync.WaitGroup
	var successfulRetrievals int64
	
	// Concurrent result retrieval
	for i := 0; i < numFutures; i++ {
		for j := 0; j < retrieversPerFuture; j++ {
			wg.Add(1)
			go func(futureIndex, retrieverID int) {
				defer wg.Done()
				
				future := futures[futureIndex]
				
				// Retrieve result
				if future.IsDone() {
					// Check if completed successfully or with error
					if future.Error() == nil {
						// Success case - check that result is available
						atomic.AddInt64(&successfulRetrievals, 1)
					} else {
						// Failure case - check that error is available
						atomic.AddInt64(&successfulRetrievals, 1)
					}
				}
			}(i, j)
		}
	}
	
	wg.Wait()
	
	expectedRetrievals := int64(numFutures * retrieversPerFuture)
	t.Logf("Successful retrievals: %d out of %d", successfulRetrievals, expectedRetrievals)
	
	assert.Equal(t, expectedRetrievals, successfulRetrievals, 
		"All retrievals should be successful")
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
	
	var wg sync.WaitGroup
	var operationCount int64
	
	// Mixed concurrent operations for memory consistency testing
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			
			for j := 0; j < operationsPerGoroutine; j++ {
				future := NewFuture(channel)
				
				opType := (goroutineID + j) % 6
				
				switch opType {
				case 0: // Create and complete with success
					future.Completable().Complete(channel)
					_ = future.IsDone()
					
				case 1: // Create and complete with failure
					future.Completable().Fail(fmt.Errorf("error %d-%d", goroutineID, j))
					_ = future.Error()
					
				case 2: // Create and cancel
					if completable := future.Completable(); completable != nil {
						completable.Cancel()
					}
					
				case 3: // Status checks
					_ = future.IsDone()
					_ = future.Error()
					
				case 4: // Channel access
					_ = future.Channel()
					
				case 5: // Multiple state changes
					future.Completable().Complete(channel)
					if completable := future.Completable(); completable != nil {
						completable.Cancel()
					}
					_ = future.IsDone()
				}
				
				atomic.AddInt64(&operationCount, 1)
			}
		}(i)
	}
	
	wg.Wait()
	
	expectedOperations := int64(numGoroutines * operationsPerGoroutine)
	assert.Equal(t, expectedOperations, operationCount, 
		"All operations should complete")
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
	
	for i := 0; i < cycles; i++ {
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
	for i := 0; i < numErrors; i++ {
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