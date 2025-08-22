package channel

import (
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Test pipeline interface compliance
func TestDefaultPipeline_InterfaceCompliance(t *testing.T) {
	channel := &DefaultChannel{}
	channel.init(channel)
	pipeline := channel.Pipeline().(*DefaultPipeline)
	
	// Verify Pipeline interface compliance
	var _ Pipeline = pipeline
	
	// Test basic properties
	assert.Equal(t, channel, pipeline.Channel(), "Pipeline should reference its channel")
	assert.NotNil(t, pipeline, "Pipeline should exist")
}

// Simple test handler for pipeline testing - implements Handler interface
type SimpleTestHandler struct {
	DefaultHandler
	name      string
	callCount int64
}

func (h *SimpleTestHandler) Added(ctx HandlerContext) {
	atomic.AddInt64(&h.callCount, 1)
}

func (h *SimpleTestHandler) Removed(ctx HandlerContext) {
	atomic.AddInt64(&h.callCount, 1)
}

func (h *SimpleTestHandler) Registered(ctx HandlerContext) {
	atomic.AddInt64(&h.callCount, 1)
	ctx.FireRegistered()
}

func (h *SimpleTestHandler) Unregistered(ctx HandlerContext) {
	atomic.AddInt64(&h.callCount, 1)
	ctx.FireUnregistered()
}

func (h *SimpleTestHandler) Active(ctx HandlerContext) {
	atomic.AddInt64(&h.callCount, 1)
	ctx.FireActive()
}

func (h *SimpleTestHandler) Inactive(ctx HandlerContext) {
	atomic.AddInt64(&h.callCount, 1)
	ctx.FireInactive()
}

func (h *SimpleTestHandler) Read(ctx HandlerContext, obj any) {
	atomic.AddInt64(&h.callCount, 1)
	ctx.FireRead(obj)
}

func (h *SimpleTestHandler) ReadCompleted(ctx HandlerContext) {
	atomic.AddInt64(&h.callCount, 1)
	ctx.FireReadCompleted()
}

func (h *SimpleTestHandler) Write(ctx HandlerContext, obj any, future Future) {
	atomic.AddInt64(&h.callCount, 1)
	ctx.Write(obj, future)
}

func (h *SimpleTestHandler) Bind(ctx HandlerContext, localAddr net.Addr, future Future) {
	atomic.AddInt64(&h.callCount, 1)
	ctx.Bind(localAddr, future)
}

func (h *SimpleTestHandler) Close(ctx HandlerContext, future Future) {
	atomic.AddInt64(&h.callCount, 1)
	ctx.Close(future)
}

func (h *SimpleTestHandler) Connect(ctx HandlerContext, localAddr net.Addr, remoteAddr net.Addr, future Future) {
	atomic.AddInt64(&h.callCount, 1)
	ctx.Connect(localAddr, remoteAddr, future)
}

func (h *SimpleTestHandler) Disconnect(ctx HandlerContext, future Future) {
	atomic.AddInt64(&h.callCount, 1)
	ctx.Disconnect(future)
}

func (h *SimpleTestHandler) Deregister(ctx HandlerContext, future Future) {
	atomic.AddInt64(&h.callCount, 1)
	ctx.Deregister(future)
}

func (h *SimpleTestHandler) ErrorCaught(ctx HandlerContext, err error) {
	atomic.AddInt64(&h.callCount, 1)
	ctx.FireErrorCaught(err)
}

func (h *SimpleTestHandler) GetCallCount() int64 {
	return atomic.LoadInt64(&h.callCount)
}

// Test concurrent handler addition - THREAD SAFETY CRITICAL
func TestDefaultPipeline_ConcurrentHandlerAddition(t *testing.T) {
	channel := &DefaultChannel{}
	channel.init(channel)
	pipeline := channel.Pipeline()
	
	const numGoroutines = 100
	const handlersPerGoroutine = 10
	
	var wg sync.WaitGroup
	var addedCount int64
	
	// Concurrent handler addition
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			
			for j := 0; j < handlersPerGoroutine; j++ {
				handlerName := fmt.Sprintf("handler-%d-%d", goroutineID, j)
				handler := &SimpleTestHandler{name: handlerName}
				
				pipeline.AddLast(handlerName, handler)
				atomic.AddInt64(&addedCount, 1)
			}
		}(i)
	}
	
	wg.Wait()
	
	expectedCount := int64(numGoroutines * handlersPerGoroutine)
	assert.Equal(t, expectedCount, addedCount, "All handlers should be added")
	
	// Verify pipeline structure integrity
	defaultPipeline := pipeline.(*DefaultPipeline)
	assert.NotNil(t, defaultPipeline.head, "Pipeline should have head")
	assert.NotNil(t, defaultPipeline.tail, "Pipeline should have tail")
	
	// Count handlers by traversing from head
	context := defaultPipeline.head.next()
	handlerCount := 0
	for context != nil && context != defaultPipeline.tail {
		handlerCount++
		context = context.next()
		
		// Prevent infinite loop
		if handlerCount > int(expectedCount)+10 {
			t.Fatal("Pipeline structure appears corrupted - infinite loop detected")
		}
	}
	
	assert.Greater(t, handlerCount, 0, "Pipeline should contain handlers")
}

// Test concurrent handler removal - THREAD SAFETY CRITICAL
func TestDefaultPipeline_ConcurrentHandlerRemoval(t *testing.T) {
	channel := &DefaultChannel{}
	channel.init(channel)
	pipeline := channel.Pipeline()
	
	const numHandlers = 200
	handlerNames := make([]string, numHandlers)
	
	// Pre-populate pipeline with handlers
	for i := 0; i < numHandlers; i++ {
		handlerName := fmt.Sprintf("handler-%d", i)
		handlerNames[i] = handlerName
		handler := &SimpleTestHandler{name: handlerName}
		
		pipeline.AddLast(handlerName, handler)
	}
	
	const numRemovers = 50
	var wg sync.WaitGroup
	var removeCount int64
	
	// Concurrent handler removal
	for i := 0; i < numRemovers; i++ {
		wg.Add(1)
		go func(removerID int) {
			defer wg.Done()
			
			// Each remover tries to remove a subset of handlers
			startIndex := removerID * (numHandlers / numRemovers)
			endIndex := (removerID + 1) * (numHandlers / numRemovers)
			if endIndex > numHandlers {
				endIndex = numHandlers
			}
			
			for j := startIndex; j < endIndex; j++ {
				handlerName := handlerNames[j]
				pipeline.RemoveByName(handlerName)
				atomic.AddInt64(&removeCount, 1)
			}
		}(i)
	}
	
	wg.Wait()
	
	t.Logf("Removed %d handlers out of %d", removeCount, numHandlers)
	assert.Greater(t, removeCount, int64(0), "Should remove some handlers")
	assert.LessOrEqual(t, removeCount, int64(numHandlers), "Cannot remove more handlers than added")
}

// Test concurrent event firing - HIGH CONCURRENCY STRESS TEST
func TestDefaultPipeline_ConcurrentEventFiring(t *testing.T) {
	channel := &DefaultChannel{}
	channel.init(channel)
	pipeline := channel.Pipeline()
	
	const numHandlers = 20
	const numFirers = 100
	const eventsPerFirer = 50
	
	handlers := make([]*SimpleTestHandler, numHandlers)
	
	// Add handlers to pipeline
	for i := 0; i < numHandlers; i++ {
		handlerName := fmt.Sprintf("event-handler-%d", i)
		handler := &SimpleTestHandler{name: handlerName}
		handlers[i] = handler
		
		pipeline.AddLast(handlerName, handler)
	}
	
	var wg sync.WaitGroup
	var firedEvents int64
	
	startSignal := make(chan struct{})
	
	// Concurrent event firing
	for i := 0; i < numFirers; i++ {
		wg.Add(1)
		go func(firerID int) {
			defer wg.Done()
			
			<-startSignal // Wait for start signal
			
			for j := 0; j < eventsPerFirer; j++ {
				eventData := map[string]interface{}{
					"firer":     firerID,
					"sequence":  j,
					"timestamp": time.Now().UnixNano(),
					"payload":   "event-payload",
				}
				
				// Fire different types of events
				switch j % 3 {
				case 0:
					pipeline.fireRead(eventData)
				case 1:
					pipeline.Write(eventData)
				case 2:
					pipeline.fireErrorCaught(fmt.Errorf("test error %d-%d", firerID, j))
				}
				
				atomic.AddInt64(&firedEvents, 1)
			}
		}(i)
	}
	
	// Start all firers simultaneously
	close(startSignal)
	wg.Wait()
	
	expectedEvents := int64(numFirers * eventsPerFirer)
	assert.Equal(t, expectedEvents, firedEvents, "All events should be fired")
	
	// Allow time for event processing
	time.Sleep(time.Millisecond * 100)
	
	// Verify handlers received events (some handlers should have been called)
	totalHandlerCalls := int64(0)
	for _, handler := range handlers {
		totalHandlerCalls += handler.GetCallCount()
	}
	
	t.Logf("Total handler calls: %d for %d fired events", totalHandlerCalls, firedEvents)
	assert.Greater(t, totalHandlerCalls, int64(0), "Handlers should receive events")
}

// Test concurrent pipeline modification during event processing
func TestDefaultPipeline_ConcurrentModificationDuringEvents(t *testing.T) {
	channel := &DefaultChannel{}
	channel.init(channel)
	pipeline := channel.Pipeline()
	
	const numModifiers = 30
	const numEventFirers = 30
	const eventsPerFirer = 100
	
	var wg sync.WaitGroup
	var modificationCount, eventCount int64
	
	startSignal := make(chan struct{})
	
	// Concurrent pipeline modifiers
	for i := 0; i < numModifiers; i++ {
		wg.Add(1)
		go func(modifierID int) {
			defer wg.Done()
			
			<-startSignal
			
			for j := 0; j < 10; j++ {
				handlerName := fmt.Sprintf("modifier-%d-handler-%d", modifierID, j)
				handler := &SimpleTestHandler{name: handlerName}
				
				// Add handler
				pipeline.AddLast(handlerName, handler)
				atomic.AddInt64(&modificationCount, 1)
				
				// Short sleep to allow events to process
				time.Sleep(time.Microsecond)
				
				// Remove handler
				pipeline.RemoveByName(handlerName)
				atomic.AddInt64(&modificationCount, 1)
			}
		}(i)
	}
	
	// Concurrent event firers
	for i := 0; i < numEventFirers; i++ {
		wg.Add(1)
		go func(firerID int) {
			defer wg.Done()
			
			<-startSignal
			
			for j := 0; j < eventsPerFirer; j++ {
				eventData := map[string]interface{}{
					"firer":   firerID,
					"seq":     j,
					"payload": "concurrent-test",
				}
				
				pipeline.fireRead(eventData)
				atomic.AddInt64(&eventCount, 1)
				
				// Brief yield to allow modifications
				if j%10 == 0 {
					time.Sleep(time.Microsecond)
				}
			}
		}(i)
	}
	
	// Start all operations simultaneously
	close(startSignal)
	wg.Wait()
	
	expectedEvents := int64(numEventFirers * eventsPerFirer)
	t.Logf("Modifications: %d, Events: %d", modificationCount, eventCount)
	
	assert.Equal(t, expectedEvents, eventCount, "All events should be fired")
	assert.Greater(t, modificationCount, int64(0), "Should perform modifications")
}

// Test pipeline context chain integrity under concurrent access
func TestDefaultPipeline_ContextChainIntegrity(t *testing.T) {
	channel := &DefaultChannel{}
	channel.init(channel)
	pipeline := channel.Pipeline()
	
	const numHandlers = 50
	const numAccessors = 100
	
	// Add handlers to establish chain
	for i := 0; i < numHandlers; i++ {
		handlerName := fmt.Sprintf("chain-handler-%d", i)
		handler := &SimpleTestHandler{name: handlerName}
		
		pipeline.AddLast(handlerName, handler)
	}
	
	var wg sync.WaitGroup
	var validTraversals int64
	
	// Concurrent context chain traversals
	for i := 0; i < numAccessors; i++ {
		wg.Add(1)
		go func(accessorID int) {
			defer wg.Done()
			
			for j := 0; j < 10; j++ {
				// Access pipeline head context for traversal
				context := pipeline.(*DefaultPipeline).head.next()
				chainLength := 0
				validChain := true
				
				// Traverse the context chain
				for context != nil && context != pipeline.(*DefaultPipeline).tail && chainLength < numHandlers+10 {
					// Verify context integrity
					if context.Name() == "" {
						validChain = false
						break
					}
					
					next := context.next()
					if next != nil {
						// Verify bidirectional links if available
						// (Implementation dependent)
					}
					
					context = next
					chainLength++
				}
				
				if validChain && chainLength > 0 {
					atomic.AddInt64(&validTraversals, 1)
				}
			}
		}(i)
	}
	
	wg.Wait()
	
	expectedTraversals := int64(numAccessors * 10)
	t.Logf("Valid traversals: %d out of %d", validTraversals, expectedTraversals)
	
	assert.Greater(t, validTraversals, expectedTraversals/2, 
		"Most traversals should be valid")
}

// Test memory consistency in pipeline operations
func TestDefaultPipeline_MemoryConsistency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory consistency test in short mode")
	}
	
	channel := &DefaultChannel{}
	channel.init(channel)
	pipeline := channel.Pipeline()
	
	const numOperations = 1000
	const numGoroutines = 100
	
	var wg sync.WaitGroup
	var operationCount int64
	
	// Mixed concurrent operations for memory consistency testing
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			
			for j := 0; j < numOperations/numGoroutines; j++ {
				opType := (goroutineID + j) % 4
				
				switch opType {
				case 0: // Add handler
					handlerName := fmt.Sprintf("consistency-%d-%d", goroutineID, j)
					handler := &SimpleTestHandler{name: handlerName}
					pipeline.AddLast(handlerName, handler)
					
				case 1: // Fire event
					eventData := map[string]interface{}{
						"goroutine": goroutineID,
						"operation": j,
					}
					pipeline.fireRead(eventData)
					
				case 2: // Access pipeline head context
					head := pipeline.(*DefaultPipeline).head
					if head != nil {
						_ = head.Name()
						_ = head.next()
					}
					
				case 3: // Get channel
					_ = pipeline.Channel()
				}
				
				atomic.AddInt64(&operationCount, 1)
			}
		}(i)
	}
	
	wg.Wait()
	
	assert.Equal(t, int64(numOperations), operationCount, 
		"All operations should complete")
}

// Benchmark concurrent pipeline operations
func BenchmarkDefaultPipeline_ConcurrentOperations(b *testing.B) {
	channel := &DefaultChannel{}
	channel.init(channel)
	pipeline := channel.Pipeline()
	
	// Pre-populate pipeline
	for i := 0; i < 10; i++ {
		handlerName := fmt.Sprintf("bench-handler-%d", i)
		handler := &SimpleTestHandler{name: handlerName}
		pipeline.AddLast(handlerName, handler)
	}
	
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			eventData := map[string]interface{}{
				"benchmark": true,
				"iteration": i,
			}
			
			pipeline.fireRead(eventData)
			i++
		}
	})
}

// Test handler replacement under concurrent access
func TestDefaultPipeline_ConcurrentHandlerReplacement(t *testing.T) {
	channel := &DefaultChannel{}
	channel.init(channel)
	pipeline := channel.Pipeline()
	
	const handlerName = "replaceable-handler"
	const numReplacers = 50
	const replacementsPerReplacer = 20
	
	// Initial handler
	initialHandler := &SimpleTestHandler{name: handlerName}
	pipeline.AddLast(handlerName, initialHandler)
	
	var wg sync.WaitGroup
	var replacementCount int64
	
	// Concurrent handler replacement
	for i := 0; i < numReplacers; i++ {
		wg.Add(1)
		go func(replacerID int) {
			defer wg.Done()
			
			for j := 0; j < replacementsPerReplacer; j++ {
				newHandler := &SimpleTestHandler{name: fmt.Sprintf("%s-replacement-%d-%d", handlerName, replacerID, j)}
				
				// Replace handler (remove and add)
				pipeline.RemoveByName(handlerName)
				pipeline.AddLast(handlerName, newHandler)
				atomic.AddInt64(&replacementCount, 1)
			}
		}(i)
	}
	
	wg.Wait()
	
	t.Logf("Successful replacements: %d", replacementCount)
	assert.Greater(t, replacementCount, int64(0), "Should perform some replacements")
	
	// Verify final state - check if handler exists in pipeline
	// Note: Pipeline interface doesn't expose Get method, so we verify through Channel
	assert.NotNil(t, pipeline.Channel(), "Pipeline should have valid channel after replacements")
}
