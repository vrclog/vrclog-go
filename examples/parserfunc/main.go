// Package main demonstrates using ParserFunc to create custom parsers in Go code.
//
// This example shows how to implement a custom parser without YAML patterns,
// parsing game score events from VRChat world logs using pure Go logic.
package main

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"time"

	"github.com/vrclog/vrclog-go/pkg/vrclog"
	"github.com/vrclog/vrclog-go/pkg/vrclog/event"
)

// Custom event types can be any string (not limited to built-in constants)
const (
	TypeGameScore event.Type = "game_score"
	TypeGameWin   event.Type = "game_win"
)

// Regex patterns for matching log lines
var (
	// Matches: "Player Alice scored 50 points"
	scorePattern = regexp.MustCompile(`Player (\w+) scored (\d+) points`)

	// Matches: "Player Bob won the game!"
	winPattern = regexp.MustCompile(`Player (\w+) won the game!`)

	// VRChat log timestamp format: "2024.01.15 12:00:00"
	timestampPattern = regexp.MustCompile(`^(\d{4}\.\d{2}\.\d{2} \d{2}:\d{2}:\d{2})`)
)

func main() {
	// Sample VRChat log lines with custom game events
	logLines := []string{
		"2024.01.15 12:00:00 Log        -  Player Alice scored 50 points",
		"2024.01.15 12:00:15 Log        -  Player Bob scored 75 points",
		"2024.01.15 12:00:30 Log        -  Player Alice won the game!",
		"2024.01.15 12:00:45 Log        -  [Behaviour] OnPlayerJoined Charlie",
		"2024.01.15 12:01:00 Log        -  Player Charlie scored 100 points",
	}

	// Create a custom parser using ParserFunc
	gameParser := vrclog.ParserFunc(parseGameEvent)

	ctx := context.Background()

	fmt.Println("ParserFunc Custom Parser Example")
	fmt.Println("==================================")
	fmt.Println()
	fmt.Println("This example demonstrates:")
	fmt.Println("1. Creating custom parsers with ParserFunc (no YAML)")
	fmt.Println("2. Constructing event.Event manually")
	fmt.Println("3. Using Data map for custom fields")
	fmt.Println("4. Proper Matched return value handling")
	fmt.Println()

	// Parse each line
	matchCount := 0
	for i, line := range logLines {
		result, err := gameParser.ParseLine(ctx, line)
		if err != nil {
			log.Printf("Error parsing line %d: %v", i+1, err)
			continue
		}

		if result.Matched {
			matchCount++
			for _, ev := range result.Events {
				fmt.Printf("[%d] ✓ %s\n", i+1, ev.Type)
				fmt.Printf("    Time: %s\n", ev.Timestamp.Format("15:04:05"))
				if ev.Data != nil {
					fmt.Printf("    Data:\n")
					for key, value := range ev.Data {
						fmt.Printf("      %s: %s\n", key, value)
					}
				}
				fmt.Println()
			}
		} else {
			// Matched: false means the line is not recognized by this parser
			// This is normal and not an error - other parsers might handle it
			fmt.Printf("[%d] - No match (not a game event)\n", i+1)
			fmt.Printf("    Line: %s\n", line[20:]) // Skip timestamp for brevity
			fmt.Println()
		}
	}

	fmt.Printf("Summary: Matched %d/%d lines\n", matchCount, len(logLines))
	fmt.Println()
	fmt.Println("Note: To combine this with the default VRChat parser,")
	fmt.Println("use ParserChain (see examples/parser-chain)")
}

// parseGameEvent is a custom parser function that parses game events.
//
// Key concepts demonstrated:
// - Return Matched: true when the line is recognized (even if Events is empty)
// - Return Matched: false when the line is not recognized (not an error)
// - Return error only for unexpected failures (e.g., context cancelled)
// - Always check ctx.Err() for cancellation support
func parseGameEvent(ctx context.Context, line string) (vrclog.ParseResult, error) {
	// 1. Check for context cancellation
	//    Important: Always check ctx.Err() to support operation cancellation
	if err := ctx.Err(); err != nil {
		return vrclog.ParseResult{}, err
	}

	// 2. Extract timestamp from VRChat log line
	//    Format: "2024.01.15 12:00:00 Log        -  [content]"
	ts, rest, ok := extractTimestamp(line)
	if !ok {
		// No timestamp means this is not a valid VRChat log line
		// Return Matched: false (not an error - just not recognized)
		return vrclog.ParseResult{Matched: false}, nil
	}

	// 3. Try to match game score pattern
	if match := scorePattern.FindStringSubmatch(rest); match != nil {
		player := match[1]
		score := match[2]

		// 4. Construct event.Event manually
		//    - Type: custom event type (any string, not limited to built-in constants)
		//    - Timestamp: required field, extracted from log line
		//    - Data: map for custom key-value pairs
		ev := event.Event{
			Type:      TypeGameScore,
			Timestamp: ts,
			Data: map[string]string{
				"player": player,
				"score":  score,
			},
		}

		// Return Matched: true with the parsed event
		return vrclog.ParseResult{
			Events:  []event.Event{ev},
			Matched: true,
		}, nil
	}

	// 5. Try to match game win pattern
	if match := winPattern.FindStringSubmatch(rest); match != nil {
		player := match[1]

		ev := event.Event{
			Type:      TypeGameWin,
			Timestamp: ts,
			Data: map[string]string{
				"player": player,
			},
		}

		return vrclog.ParseResult{
			Events:  []event.Event{ev},
			Matched: true,
		}, nil
	}

	// 6. No pattern matched - return Matched: false
	//    This is NOT an error - it just means this parser doesn't recognize the line
	//    Other parsers in a chain might handle it
	return vrclog.ParseResult{Matched: false}, nil
}

// extractTimestamp extracts the timestamp and remaining content from a VRChat log line.
// Returns (timestamp, remaining content, success).
func extractTimestamp(line string) (time.Time, string, bool) {
	match := timestampPattern.FindStringSubmatch(line)
	if match == nil {
		return time.Time{}, "", false
	}

	// Parse VRChat timestamp format
	ts, err := time.ParseInLocation("2006.01.02 15:04:05", match[1], time.Local)
	if err != nil {
		return time.Time{}, "", false
	}

	// Return timestamp and remaining content
	remaining := line[len(match[0]):]
	return ts, remaining, true
}

// Alternative approach: Multiple events from a single line
//
// A parser can return multiple events from a single log line:
//
//	return vrclog.ParseResult{
//	    Events: []event.Event{event1, event2},
//	    Matched: true,
//	}, nil
//
// This is useful for parsers that extract multiple pieces of information.

// Alternative approach: Using standard Event fields
//
// Instead of Data map, you can use standard fields if they fit:
//
//	ev := event.Event{
//	    Type:       "custom_player_event",
//	    Timestamp:  ts,
//	    PlayerName: "Alice",     // Standard field
//	    PlayerID:   "usr_12345", // Standard field
//	}
//
// Use Data map when the standard fields don't fit your use case.

// Alternative approach: Error handling
//
// Return error only for unexpected failures, not for unrecognized lines:
//
//	// Good: Return Matched: false for unrecognized lines
//	return vrclog.ParseResult{Matched: false}, nil
//
//	// Good: Return error for unexpected failures
//	if ctx.Err() != nil {
//	    return vrclog.ParseResult{}, ctx.Err()
//	}
//
//	// Bad: Don't return error for lines that don't match
//	if !scorePattern.MatchString(line) {
//	    return vrclog.ParseResult{}, fmt.Errorf("not a score line")
//	}

// Additional notes:
//
// - Matched: true can be returned with empty Events slice if needed
//   (e.g., a filter that matches but produces no output)
//
// - Context cancellation is important for long-running operations
//   Always check ctx.Err() to support cancellation
//
// - ParserFunc can be combined with other parsers using ParserChain
//   See examples/parser-chain for details
//
// - For simple regex patterns, consider using RegexParser with YAML
//   See examples/custom-parser for details

// Example: Validating parsed data
//
//	func validateScore(scoreStr string) error {
//		score, err := strconv.Atoi(scoreStr)
//		if err != nil {
//			return fmt.Errorf("invalid score: %w", err)
//		}
//		if score < 0 {
//			return fmt.Errorf("score must be non-negative: %d", score)
//		}
//		return nil
//	}

// Example: Returning validation errors
//
// If validation fails, you can either:
// 1. Return Matched: false (skip this line)
// 2. Return error (stop processing)
//
// Choose based on whether invalid data should stop processing:
//
//	if err := validateScore(scoreStr); err != nil {
//	    // Option 1: Skip invalid lines
//	    return vrclog.ParseResult{Matched: false}, nil
//
//	    // Option 2: Stop on invalid data
//	    return vrclog.ParseResult{}, err
//	}
