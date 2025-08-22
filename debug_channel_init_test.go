package gone

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/yetiz-org/gone/gtcp"
	"github.com/yetiz-org/gone/channel"
	kklogger "github.com/yetiz-org/goth-kklogger"
)

// Minimal test to isolate channel initialization deadlock
func TestMinimalChannelCreation(t *testing.T) {
	kklogger.SetLogLevel("DEBUG")
	
	fmt.Println("=== Starting minimal channel creation test ===")
	kklogger.DebugJ("debug_test:TestMinimalChannelCreation#test!start", "Test started")
	
	// Step 1: Test basic ServerChannel creation
	fmt.Println("Step 1: Creating ServerChannel...")
	serverChannel := &gtcp.ServerChannel{}
	fmt.Println("Step 1: ServerChannel created successfully")
	
	// Step 2: Test channel.init() call
	fmt.Println("Step 2: Calling serverChannel.init()...")
	serverChannel.Init()  // This should call c.init(c) internally
	fmt.Println("Step 2: serverChannel.Init() completed")
	
	// Step 3: Test pipeline access
	fmt.Println("Step 3: Accessing Pipeline...")
	pipeline := serverChannel.Pipeline()
	if pipeline == nil {
		fmt.Println("ERROR: Pipeline is nil!")
		t.Fatal("Pipeline is nil after Init()")
	}
	fmt.Println("Step 3: Pipeline access successful")
	
	// Step 4: Test basic pipeline operations
	fmt.Println("Step 4: Testing pipeline operations...")
	future := pipeline.NewFuture()
	if future == nil {
		fmt.Println("ERROR: NewFuture returned nil!")
		t.Fatal("NewFuture returned nil")
	}
	fmt.Println("Step 4: Pipeline NewFuture successful")
	
	fmt.Println("=== All steps completed successfully ===")
	kklogger.DebugJ("debug_test:TestMinimalChannelCreation#test!end", "Test completed successfully")
}

// Test ServerBootstrap creation without binding
func TestMinimalBootstrapCreation(t *testing.T) {
	kklogger.SetLogLevel("DEBUG")
	
	fmt.Println("=== Starting minimal bootstrap creation test ===")
	
	// Step 1: Create bootstrap
	fmt.Println("Step 1: Creating ServerBootstrap...")
	bootstrap := channel.NewServerBootstrap()
	fmt.Println("Step 1: ServerBootstrap created")
	
	// Step 2: Set channel type
	fmt.Println("Step 2: Setting ChannelType...")
	bootstrap.ChannelType(&gtcp.ServerChannel{})
	fmt.Println("Step 2: ChannelType set")
	
	// Step 3: Set child handler
	fmt.Println("Step 3: Setting ChildHandler...")
	bootstrap.ChildHandler(channel.NewInitializer(func(ch channel.Channel) {
		fmt.Println("Initializer callback called")
	}))
	fmt.Println("Step 3: ChildHandler set")
	
	fmt.Println("=== Bootstrap creation completed successfully ===")
}

// Test only the Bind operation without Sync
func TestMinimalBindWithoutSync(t *testing.T) {
	kklogger.SetLogLevel("DEBUG")
	
	fmt.Println("=== Starting minimal bind without sync test ===")
	
	bootstrap := channel.NewServerBootstrap()
	bootstrap.ChannelType(&gtcp.ServerChannel{})
	bootstrap.ChildHandler(channel.NewInitializer(func(ch channel.Channel) {
		fmt.Println("Initializer in bind test called")
	}))
	
	fmt.Println("Calling bootstrap.Bind()...")
	future := bootstrap.Bind(&net.TCPAddr{IP: nil, Port: 18084})
	
	if future == nil {
		fmt.Println("ERROR: Bind returned nil future!")
		t.Fatal("Bind returned nil future")
	}
	
	fmt.Println("Bootstrap.Bind() returned future successfully")
	fmt.Println("=== Bind without sync completed successfully ===")
}

// Test manual Future completion to verify Sync() mechanism
func TestManualFutureCompletion(t *testing.T) {
	kklogger.SetLogLevel("DEBUG")
	
	fmt.Println("=== Starting manual future completion test ===")
	
	// Create a simple channel and get a future from its pipeline
	serverChannel := &gtcp.ServerChannel{}
	serverChannel.Init()
	
	fmt.Println("Creating future from pipeline...")
	future := serverChannel.Pipeline().NewFuture()
	
	fmt.Println("Testing future completion manually...")
	
	// Complete the future in a goroutine
	go func() {
		fmt.Println("Goroutine: Completing future manually...")
		// Use the correct completion method via Completable interface
		fmt.Println("Goroutine: Calling future.Completable().Complete()")
		completed := future.Completable().Complete(serverChannel)
		fmt.Printf("Goroutine: Complete() returned: %v\n", completed)
		fmt.Println("Goroutine: Manual completion done")
	}()
	
	fmt.Println("Calling future.Sync() with timeout...")
	
	// Create a timeout to prevent infinite hang
	done := make(chan bool, 1)
	go func() {
		result := future.Sync()
		fmt.Printf("Sync completed! Result: %T\n", result)
		done <- true
	}()
	
	// Wait for either completion or timeout
	select {
	case <-done:
		fmt.Println("SUCCESS: Manual future completion and Sync() worked!")
	case <-time.After(5 * time.Second):
		fmt.Println("TIMEOUT: Manual future completion failed - Sync() still hangs")
		t.Fatal("Manual future completion test timed out")
	}
	
	fmt.Println("=== Manual future completion test completed ===")
}
