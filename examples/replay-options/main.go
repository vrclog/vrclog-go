// Package main demonstrates replay configuration options for watching VRChat logs.
//
// This example shows how to configure different replay modes when starting a watcher:
// - WithReplayLastN: Replay the last N lines
// - WithReplayFromStart: Replay from the beginning
// - WithReplaySinceTime: Replay since a specific time
// - ReplayNone: Only watch for new events (default)
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/vrclog/vrclog-go/pkg/vrclog"
)

// Test log data with 15 events
const testLogData = `2024.01.15 10:00:00 Log        -  [Behaviour] OnPlayerJoined Event01
2024.01.15 10:01:00 Log        -  [Behaviour] OnPlayerJoined Event02
2024.01.15 10:02:00 Log        -  [Behaviour] OnPlayerJoined Event03
2024.01.15 10:03:00 Log        -  [Behaviour] OnPlayerJoined Event04
2024.01.15 10:04:00 Log        -  [Behaviour] OnPlayerJoined Event05
2024.01.15 10:05:00 Log        -  [Behaviour] OnPlayerJoined Event06
2024.01.15 10:06:00 Log        -  [Behaviour] OnPlayerJoined Event07
2024.01.15 10:07:00 Log        -  [Behaviour] OnPlayerJoined Event08
2024.01.15 10:08:00 Log        -  [Behaviour] OnPlayerJoined Event09
2024.01.15 10:09:00 Log        -  [Behaviour] OnPlayerJoined Event10
2024.01.15 10:10:00 Log        -  [Behaviour] OnPlayerJoined Event11
2024.01.15 10:11:00 Log        -  [Behaviour] OnPlayerJoined Event12
2024.01.15 10:12:00 Log        -  [Behaviour] OnPlayerJoined Event13
2024.01.15 10:13:00 Log        -  [Behaviour] OnPlayerJoined Event14
2024.01.15 10:14:00 Log        -  [Behaviour] OnPlayerJoined Event15
`

func main() {
	fmt.Println("Replay Options Example")
	fmt.Println("======================")
	fmt.Println()

	// Create mock log directory
	tmpDir, err := createMockLogDir()
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	fmt.Printf("Created mock log directory: %s\n\n", tmpDir)

	// ========================================
	// Example 1: WithReplayLastN(N)
	// ========================================
	fmt.Println("Example 1: WithReplayLastN(5)")
	fmt.Println("------------------------------")
	fmt.Println("Replays the last 5 non-empty lines before watching for new events")
	fmt.Println()

	watcher1, err := vrclog.NewWatcherWithOptions(
		vrclog.WithLogDir(tmpDir),
		vrclog.WithReplayLastN(5), // Last 5 events
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("✓ Watcher created with WithReplayLastN(5)")
	fmt.Println("  Will replay: Event11, Event12, Event13, Event14, Event15")
	demoWatch(watcher1, "ReplayLastN")

	// ========================================
	// Example 2: WithReplayFromStart()
	// ========================================
	fmt.Println("\nExample 2: WithReplayFromStart()")
	fmt.Println("----------------------------------")
	fmt.Println("Replays from the beginning of the log file")
	fmt.Println()

	watcher2, err := vrclog.NewWatcherWithOptions(
		vrclog.WithLogDir(tmpDir),
		vrclog.WithReplayFromStart(),
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("✓ Watcher created with WithReplayFromStart()")
	fmt.Println("  Will replay: All 15 events from start")
	demoWatch(watcher2, "ReplayFromStart")

	// ========================================
	// Example 3: WithReplaySinceTime(t)
	// ========================================
	fmt.Println("\nExample 3: WithReplaySinceTime(t)")
	fmt.Println("-----------------------------------")
	fmt.Println("Replays events since a specific timestamp")
	fmt.Println()

	since := time.Date(2024, 1, 15, 10, 8, 0, 0, time.Local)
	watcher3, err := vrclog.NewWatcherWithOptions(
		vrclog.WithLogDir(tmpDir),
		vrclog.WithReplaySinceTime(since),
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("✓ Watcher created with WithReplaySinceTime(%s)\n", since.Format("15:04:05"))
	fmt.Println("  Will replay: Events from 10:08:00 onwards (Event09-Event15)")
	demoWatch(watcher3, "ReplaySinceTime")

	// ========================================
	// Example 4: ReplayNone (default)
	// ========================================
	fmt.Println("\nExample 4: ReplayNone (default)")
	fmt.Println("---------------------------------")
	fmt.Println("No replay - only watches for new events (like tail -f)")
	fmt.Println()

	watcher4, err := vrclog.NewWatcherWithOptions(
		vrclog.WithLogDir(tmpDir),
		vrclog.WithReplay(vrclog.ReplayConfig{Mode: vrclog.ReplayNone}),
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("✓ Watcher created with ReplayNone")
	fmt.Println("  Will replay: Nothing (only new events)")
	watcher4.Close() // No demo needed for ReplayNone

	// ========================================
	// Example 5: Memory Limits
	// ========================================
	fmt.Println("\nExample 5: Memory Limits")
	fmt.Println("-------------------------")
	fmt.Println("Control memory usage during replay with limits")
	fmt.Println()

	watcher5, err := vrclog.NewWatcherWithOptions(
		vrclog.WithLogDir(tmpDir),
		vrclog.WithReplayLastN(100),
		vrclog.WithMaxReplayBytes(1024*1024),   // 1MB total
		vrclog.WithMaxReplayLineBytes(64*1024), // 64KB per line
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("✓ Watcher created with memory limits:")
	fmt.Println("  - WithReplayLastN(100)")
	fmt.Println("  - WithMaxReplayBytes(1MB)")
	fmt.Println("  - WithMaxReplayLineBytes(64KB)")
	watcher5.Close()

	// ========================================
	// Summary
	// ========================================
	fmt.Println("\nSummary:")
	fmt.Println("--------")
	fmt.Println("WithReplayLastN(N)       : Replay last N non-empty lines")
	fmt.Println("WithReplayFromStart()    : Replay from beginning of file")
	fmt.Println("WithReplaySinceTime(t)   : Replay events >= timestamp t")
	fmt.Println("ReplayNone               : No replay (default, tail -f)")
	fmt.Println()
	fmt.Println("Memory protection:")
	fmt.Println("WithMaxReplayBytes(N)    : Limit total bytes read (default: 10MB)")
	fmt.Println("WithMaxReplayLineBytes(N): Limit bytes per line (default: 512KB)")
	fmt.Println()
	fmt.Println("Note: Replay options use WatchOption, not ParseOption")
}

// demoWatch demonstrates the watcher by collecting a few replay events
func demoWatch(watcher *vrclog.Watcher, mode string) {
	// Create a context with short timeout to collect replay events
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	events, errs, err := watcher.Watch(ctx)
	if err != nil {
		log.Printf("Failed to start watch: %v", err)
		return
	}

	// Collect events
	var collected []string
	done := false
	for !done {
		select {
		case ev, ok := <-events:
			if !ok {
				done = true
				break
			}
			collected = append(collected, ev.PlayerName)
		case err, ok := <-errs:
			if !ok {
				done = true
				break
			}
			// Context deadline exceeded is expected
			if err != context.DeadlineExceeded {
				log.Printf("Error: %v", err)
			}
		case <-time.After(150 * time.Millisecond):
			done = true
		}
	}

	watcher.Close()

	fmt.Printf("  Replayed %d events: ", len(collected))
	if len(collected) > 0 {
		fmt.Printf("%s", collected[0])
		for i := 1; i < len(collected) && i < 5; i++ {
			fmt.Printf(", %s", collected[i])
		}
		if len(collected) > 5 {
			fmt.Printf(", ... (total: %d)", len(collected))
		}
	}
	fmt.Println()
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
