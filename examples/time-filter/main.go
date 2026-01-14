// Package main demonstrates time-based filtering of VRChat log events.
//
// This example shows how to filter events by timestamp using WithParseSince,
// WithParseUntil, and WithParseTimeRange - useful for analyzing specific time periods.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/vrclog/vrclog-go/pkg/vrclog"
)

// Test data with events at different times (4-hour span)
// VRChat log format: "Log" + 10 spaces + "-" + 2 spaces
const testLogData = `2024.01.15 10:00:00 Log        -  [Behaviour] OnPlayerJoined Alice
2024.01.15 12:30:00 Log        -  [Behaviour] OnPlayerJoined Bob
2024.01.15 15:00:00 Log        -  [Behaviour] OnPlayerJoined Charlie
2024.01.15 18:30:00 Log        -  [Behaviour] OnPlayerJoined Diana
`

func main() {
	ctx := context.Background()

	fmt.Println("Time-Based Filtering Example")
	fmt.Println("============================")
	fmt.Println()

	// Create temporary log file
	tmpFile, err := createTempLogFile(testLogData)
	if err != nil {
		log.Fatal(err)
	}
	defer os.Remove(tmpFile)

	// ========================================
	// Show all events first (no filter)
	// ========================================
	fmt.Println("All events (no filter):")
	fmt.Println("-----------------------")
	printEvents(ctx, tmpFile)
	fmt.Println()

	// ========================================
	// Example 1: WithParseSince - Events after a time
	// ========================================
	fmt.Println("Example 1: WithParseSince (>= 12:00)")
	fmt.Println("-------------------------------------")

	since := time.Date(2024, 1, 15, 12, 0, 0, 0, time.Local)
	fmt.Printf("Filter: Events at or after %s\n\n", since.Format("15:04:05"))

	eventCount := 0
	for ev, err := range vrclog.ParseFile(ctx, tmpFile,
		vrclog.WithParseSince(since),
	) {
		if err != nil {
			log.Fatal(err)
		}
		eventCount++
		fmt.Printf("%s - %s joined\n",
			ev.Timestamp.Format("15:04:05"),
			ev.PlayerName)
	}
	fmt.Printf("\nMatched: %d events (Bob, Charlie, Diana)\n\n", eventCount)

	// ========================================
	// Example 2: WithParseUntil - Events before a time
	// ========================================
	fmt.Println("Example 2: WithParseUntil (< 16:00)")
	fmt.Println("------------------------------------")
	fmt.Println("Note: Assumes timestamps are monotonically increasing")
	fmt.Println("      Out-of-order timestamps may be skipped")
	fmt.Println()

	until := time.Date(2024, 1, 15, 16, 0, 0, 0, time.Local)
	fmt.Printf("Filter: Events before %s\n\n", until.Format("15:04:05"))

	eventCount = 0
	for ev, err := range vrclog.ParseFile(ctx, tmpFile,
		vrclog.WithParseUntil(until),
	) {
		if err != nil {
			log.Fatal(err)
		}
		eventCount++
		fmt.Printf("%s - %s joined\n",
			ev.Timestamp.Format("15:04:05"),
			ev.PlayerName)
	}
	fmt.Printf("\nMatched: %d events (Alice, Bob, Charlie)\n\n", eventCount)

	// ========================================
	// Example 3: WithParseTimeRange - Events in a range
	// ========================================
	fmt.Println("Example 3: WithParseTimeRange (12:00 <= t < 16:00)")
	fmt.Println("---------------------------------------------------")
	fmt.Println("Combines Since (inclusive) and Until (exclusive)")
	fmt.Println()

	since = time.Date(2024, 1, 15, 12, 0, 0, 0, time.Local)
	until = time.Date(2024, 1, 15, 16, 0, 0, 0, time.Local)
	fmt.Printf("Filter: %s <= Events < %s\n\n",
		since.Format("15:04:05"),
		until.Format("15:04:05"))

	eventCount = 0
	for ev, err := range vrclog.ParseFile(ctx, tmpFile,
		vrclog.WithParseTimeRange(since, until),
	) {
		if err != nil {
			log.Fatal(err)
		}
		eventCount++
		fmt.Printf("%s - %s joined\n",
			ev.Timestamp.Format("15:04:05"),
			ev.PlayerName)
	}
	fmt.Printf("\nMatched: %d events (Bob, Charlie)\n\n", eventCount)

	// ========================================
	// Use case: Analyze specific session
	// ========================================
	fmt.Println("Use Case: Analyze afternoon session (15:00-19:00)")
	fmt.Println("--------------------------------------------------")

	since = time.Date(2024, 1, 15, 15, 0, 0, 0, time.Local)
	until = time.Date(2024, 1, 15, 19, 0, 0, 0, time.Local)

	var players []string
	for ev, err := range vrclog.ParseFile(ctx, tmpFile,
		vrclog.WithParseTimeRange(since, until),
		vrclog.WithParseIncludeTypes(vrclog.EventPlayerJoin),
	) {
		if err != nil {
			log.Fatal(err)
		}
		players = append(players, ev.PlayerName)
	}

	fmt.Printf("Players who joined in afternoon: %v\n", players)
	fmt.Println()

	// ========================================
	// Summary
	// ========================================
	fmt.Println("Summary:")
	fmt.Println("--------")
	fmt.Println("WithParseSince(t)        : Events >= t (inclusive)")
	fmt.Println("WithParseUntil(t)        : Events < t (exclusive)")
	fmt.Println("WithParseTimeRange(s, u) : s <= Events < u")
	fmt.Println()
	fmt.Println("Note: Time filters use ParseOption, not WatchOption")
}

// printEvents prints all events from a file
func printEvents(ctx context.Context, path string) {
	for ev, err := range vrclog.ParseFile(ctx, path) {
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%s - %s joined\n",
			ev.Timestamp.Format("15:04:05"),
			ev.PlayerName)
	}
}

// createTempLogFile creates a temporary log file with test data
func createTempLogFile(data string) (string, error) {
	tmpFile, err := os.CreateTemp("", "output_log_*.txt")
	if err != nil {
		return "", err
	}

	if _, err := tmpFile.WriteString(data); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return "", err
	}

	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpFile.Name())
		return "", err
	}

	return tmpFile.Name(), nil
}
