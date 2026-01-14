// Package main demonstrates Watcher lifecycle management and graceful shutdown patterns.
//
// This example shows:
// - NewWatcherWithOptions() + Watcher.Watch() (two-phase API)
// - Watcher.Close() for synchronous shutdown
// - WithLogger() for slog integration
// - sync.WaitGroup for coordinating multiple goroutines
// - Comparing context cancellation vs Watcher.Close()
//
// Key pattern: Watcher.Close() is synchronous and blocks until all resources are released.
package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/vrclog/vrclog-go/pkg/vrclog"
)

const testLogData = `2024.01.15 10:00:00 Log        -  [Behaviour] OnPlayerJoined Alice
2024.01.15 10:01:00 Log        -  [Behaviour] OnPlayerJoined Bob
2024.01.15 10:02:00 Log        -  [Behaviour] Entering Room: Test World
2024.01.15 10:03:00 Log        -  [Behaviour] OnPlayerLeft Alice
`

func main() {
	fmt.Println("Graceful Shutdown Example")
	fmt.Println("=========================")
	fmt.Println()

	// ========================================
	// 1. Setup: Create slog Logger
	// ========================================
	fmt.Println("1. Setting up structured logging (slog)")
	fmt.Println("---------------------------------------")
	fmt.Println()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	fmt.Println("→ Created logger with Debug level")
	fmt.Println("→ Watcher will log internal operations")
	fmt.Println()

	// ========================================
	// 2. Create Temporary Log Directory
	// ========================================
	tmpDir, err := createMockLogDir()
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	fmt.Printf("→ Created mock log directory: %s\n", tmpDir)
	fmt.Println()

	// ========================================
	// 3. Two-Phase Watcher API
	// ========================================
	fmt.Println("2. Two-Phase Watcher API")
	fmt.Println("------------------------")
	fmt.Println()

	fmt.Println("→ Phase 1: NewWatcherWithOptions()")
	fmt.Println("  - Validates options")
	fmt.Println("  - Finds log directory")
	fmt.Println("  - Returns error on configuration failure")
	fmt.Println()

	watcher, err := vrclog.NewWatcherWithOptions(
		vrclog.WithLogDir(tmpDir),
		vrclog.WithLogger(logger),
		vrclog.WithReplayFromStart(), // Replay existing events for demo
		// vrclog.WithWaitForLogs(true), // Uncomment to wait for logs before VRChat starts
	)
	if err != nil {
		log.Fatalf("Failed to create watcher: %v", err)
	}

	fmt.Println("→ Watcher created successfully")
	fmt.Println()

	fmt.Println("→ Phase 2: watcher.Watch(ctx)")
	fmt.Println("  - Starts internal goroutines")
	fmt.Println("  - Returns event and error channels")
	fmt.Println("  - Non-blocking (returns immediately)")
	fmt.Println()

	ctx := context.Background()
	events, errs, err := watcher.Watch(ctx)
	if err != nil {
		log.Fatalf("Failed to start watch: %v", err)
	}

	fmt.Println("→ Watch started (goroutines running)")
	fmt.Println()

	// ========================================
	// 4. Process Events with WaitGroup
	// ========================================
	fmt.Println("3. Processing Events with sync.WaitGroup")
	fmt.Println("-----------------------------------------")
	fmt.Println()

	var wg sync.WaitGroup

	// Event processing goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		fmt.Println("→ Event processor goroutine started")
		eventCount := 0
		for ev := range events {
			eventCount++
			fmt.Printf("  [Event %d] %s", eventCount, ev.Type)
			if ev.PlayerName != "" {
				fmt.Printf(" - %s", ev.PlayerName)
			}
			if ev.WorldName != "" {
				fmt.Printf(" - %s", ev.WorldName)
			}
			fmt.Println()
		}
		fmt.Println("→ Event channel closed, processor exiting")
	}()

	// Error handling goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		fmt.Println("→ Error handler goroutine started")
		for err := range errs {
			if err != nil {
				fmt.Printf("  [Error] %v\n", err)
			}
		}
		fmt.Println("→ Error channel closed, handler exiting")
	}()

	fmt.Println()

	// ========================================
	// 5. Simulate Work and Shutdown
	// ========================================
	fmt.Println("4. Graceful Shutdown Patterns")
	fmt.Println("------------------------------")
	fmt.Println()

	// Set up signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	fmt.Println("→ Waiting for replay events to process...")
	fmt.Println("  (Press Ctrl+C to trigger shutdown, or wait 2 seconds)")
	fmt.Println()

	// Wait for signal or timeout
	select {
	case <-sigCh:
		fmt.Println("\n→ Received interrupt signal")
	case <-time.After(2 * time.Second):
		fmt.Println("→ Timeout reached (simulating shutdown)")
	}
	fmt.Println()

	// ========================================
	// 6. Watcher.Close() - Synchronous Shutdown
	// ========================================
	fmt.Println("5. Comparing Shutdown Patterns")
	fmt.Println("-------------------------------")
	fmt.Println()

	fmt.Println("→ Pattern A: context.Cancel() (asynchronous)")
	fmt.Println("  cancel()")
	fmt.Println("  // Goroutines may still be running here!")
	fmt.Println("  // Must wait on channels or WaitGroup")
	fmt.Println()

	fmt.Println("→ Pattern B: Watcher.Close() (synchronous, blocking)")
	fmt.Println("  watcher.Close()")
	fmt.Println("  // Blocks until all goroutines exit")
	fmt.Println("  // Channels are guaranteed closed")
	fmt.Println()

	fmt.Println("→ Using Watcher.Close():")
	startTime := time.Now()
	fmt.Println("  Calling Close()...")

	// Close() blocks until watcher fully stops
	watcher.Close()

	elapsed := time.Since(startTime)
	fmt.Printf("  Close() returned after %dms\n", elapsed.Milliseconds())
	fmt.Println("  → All goroutines have exited")
	fmt.Println("  → Channels are closed")
	fmt.Println()

	// ========================================
	// 7. Wait for Processing Goroutines
	// ========================================
	fmt.Println("6. Waiting for Processing Goroutines")
	fmt.Println("-------------------------------------")
	fmt.Println()

	fmt.Println("→ Waiting for event processor and error handler...")
	wg.Wait()
	fmt.Println("→ All goroutines finished")
	fmt.Println()

	// ========================================
	// Summary
	// ========================================
	fmt.Println("Summary:")
	fmt.Println("--------")
	fmt.Println("Two-Phase API:")
	fmt.Println("  NewWatcherWithOptions() : Create and validate (returns error)")
	fmt.Println("  watcher.Watch(ctx)      : Start watching (returns channels)")
	fmt.Println()
	fmt.Println("Shutdown Patterns:")
	fmt.Println("  context.Cancel()        : Asynchronous signal (non-blocking)")
	fmt.Println("  Watcher.Close()         : Synchronous shutdown (blocks until done)")
	fmt.Println()
	fmt.Println("Best Practices:")
	fmt.Println("  - Use sync.WaitGroup to coordinate multiple goroutines")
	fmt.Println("  - Use slog.Logger (WithLogger) for debugging")
	fmt.Println("  - Call Close() for clean resource release")
	fmt.Println("  - Wait on WaitGroup after Close() for full cleanup")
	fmt.Println()
	fmt.Println("WithWaitForLogs(true):")
	fmt.Println("  - Use when starting watcher before VRChat launches")
	fmt.Println("  - Watcher will poll until log files appear")
	fmt.Println("  - Useful for background monitoring tools")
}

// createMockLogDir creates a temporary directory with a mock VRChat log file
func createMockLogDir() (string, error) {
	tmpDir, err := os.MkdirTemp("", "vrclog_test_*")
	if err != nil {
		return "", err
	}

	// Create log file with VRChat naming pattern: output_log_*.txt
	logPath := filepath.Join(tmpDir, "output_log_test.txt")
	if err := os.WriteFile(logPath, []byte(testLogData), 0644); err != nil {
		os.RemoveAll(tmpDir)
		return "", err
	}

	return tmpDir, nil
}
