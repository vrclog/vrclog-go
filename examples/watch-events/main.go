// Package main demonstrates real-time VRChat log monitoring using WatchWithOptions.
//
// This example shows how to watch VRChat logs for player join/leave events
// in real-time and display them as they occur.
//
// Note: This example requires VRChat to be running and generating log files
// in the standard Windows location (%LOCALAPPDATA%Low\VRChat\VRChat\).
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/vrclog/vrclog-go/pkg/vrclog"
)

func main() {
	fmt.Println("VRChat Log Watcher Example")
	fmt.Println("===========================")
	fmt.Println()
	fmt.Println("Monitoring VRChat logs for player join/leave events...")
	fmt.Println("Press Ctrl+C to stop.")
	fmt.Println()

	// Create context with cancellation for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt signal for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\n\nShutting down gracefully...")
		cancel()
	}()

	// Start watching with options
	events, errs, err := vrclog.WatchWithOptions(ctx,
		// Filter to only player join/leave events
		vrclog.WithIncludeTypes(vrclog.EventPlayerJoin, vrclog.EventPlayerLeft),

		// Replay last 10 events from current log file
		vrclog.WithReplayLastN(10),

		// Poll every 2 seconds (default is 5 seconds)
		vrclog.WithPollInterval(2*time.Second),

		// Include raw log line in events (for debugging)
		vrclog.WithIncludeRawLine(true),
	)
	if err != nil {
		log.Fatalf("Failed to start watcher: %v", err)
	}

	fmt.Println("Watcher started successfully!")
	fmt.Println()

	eventCount := 0

	// Process events
	for {
		select {
		case event, ok := <-events:
			if !ok {
				// Channel closed, watcher stopped
				fmt.Printf("\nTotal events processed: %d\n", eventCount)
				return
			}

			eventCount++

			// Format timestamp
			timestamp := event.Timestamp.Format("15:04:05")

			// Display event
			switch event.Type {
			case vrclog.EventPlayerJoin:
				fmt.Printf("[%s] ✓ %s joined\n", timestamp, event.PlayerName)
			case vrclog.EventPlayerLeft:
				fmt.Printf("[%s] ✗ %s left\n", timestamp, event.PlayerName)
			}

			// Uncomment to show raw log line (requires WithIncludeRawLine option):
			// fmt.Printf("     Raw: %s\n", event.RawLine)

		case err, ok := <-errs:
			if !ok {
				// Error channel closed
				return
			}

			// Handle errors (e.g., file rotation, read errors)
			log.Printf("Error: %v", err)

			// Check for specific error types using errors.As
			var watchErr *vrclog.WatchError
			if errors.As(err, &watchErr) {
				log.Printf("Watch operation: %s, Path: %s", watchErr.Op, watchErr.Path)
			}

		case <-ctx.Done():
			// Context cancelled (Ctrl+C pressed)
			fmt.Printf("\nTotal events processed: %d\n", eventCount)
			return
		}
	}
}
