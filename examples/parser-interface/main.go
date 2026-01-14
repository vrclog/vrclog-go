// Package main demonstrates implementing the Parser interface directly with a struct.
//
// This example shows how to create a custom parser with state management,
// which is the key advantage over ParserFunc for complex parsing scenarios.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"sync"
	"time"

	"github.com/vrclog/vrclog-go/pkg/vrclog"
	"github.com/vrclog/vrclog-go/pkg/vrclog/event"
)

// Custom event types
const (
	TypeGameScore event.Type = "game_score"
	TypeGameWin   event.Type = "game_win"
)

// Sample VRChat log lines with custom game events
const testLogData = `2024.01.15 12:00:00 Log        -  [Game]: Score: 50 by Alice
2024.01.15 12:00:15 Log        -  [Game]: Score: 75 by Bob
2024.01.15 12:00:30 Log        -  [Game]: Winner: Alice
2024.01.15 12:00:45 Log        -  [Behaviour] OnPlayerJoined Charlie
2024.01.15 12:01:00 Log        -  [Game]: Score: 100 by Charlie
`

func main() {
	fmt.Println("Parser Interface Implementation Example")
	fmt.Println("========================================")
	fmt.Println()
	fmt.Println("This example demonstrates:")
	fmt.Println("1. Implementing Parser interface with a struct (not ParserFunc)")
	fmt.Println("2. State management (match counting)")
	fmt.Println("3. Custom methods (MatchCount, Reset)")
	fmt.Println("4. Compile-time interface verification")
	fmt.Println("5. Thread-safe version with sync.Mutex")
	fmt.Println()

	// Create temporary log file
	tmpFile, err := createTempLogFile(testLogData)
	if err != nil {
		log.Fatal(err)
	}
	defer os.Remove(tmpFile)

	// ========================================
	// Example 1: Simple Parser with State
	// ========================================
	fmt.Println("Example 1: Simple Parser with State Management")
	fmt.Println("-----------------------------------------------")
	fmt.Println()

	// Create a parser instance
	scoreParser := NewScoreParser()

	// Parse the file using our custom parser
	ctx := context.Background()
	eventCount := 0
	for ev, err := range vrclog.ParseFile(ctx, tmpFile,
		vrclog.WithParseParser(scoreParser),
	) {
		if err != nil {
			log.Printf("Error: %v", err)
			break
		}

		eventCount++
		fmt.Printf("[%d] %s\n", eventCount, ev.Type)
		fmt.Printf("    Time: %s\n", ev.Timestamp.Format("15:04:05"))
		if ev.Data != nil {
			fmt.Printf("    Data:\n")
			for key, value := range ev.Data {
				fmt.Printf("      %s: %s\n", key, value)
			}
		}
		fmt.Println()
	}

	// Access parser state
	fmt.Printf("Parser matched %d events\n\n", scoreParser.MatchCount())

	// ========================================
	// Example 2: Parser Chain with State
	// ========================================
	fmt.Println("Example 2: Combining with DefaultParser")
	fmt.Println("----------------------------------------")
	fmt.Println("Show both built-in and custom events")
	fmt.Println()

	// Reset the parser for reuse
	scoreParser.Reset()

	// Create a chain with both default and custom parser
	chain := &vrclog.ParserChain{
		Mode: vrclog.ChainAll, // Try all parsers
		Parsers: []vrclog.Parser{
			&vrclog.DefaultParser{},
			scoreParser,
		},
	}

	eventCount = 0
	for ev, err := range vrclog.ParseFile(ctx, tmpFile,
		vrclog.WithParseParser(chain),
	) {
		if err != nil {
			log.Printf("Error: %v", err)
			break
		}

		eventCount++
		fmt.Printf("[%d] %s", eventCount, ev.Type)
		if ev.PlayerName != "" {
			fmt.Printf(" | Player: %s", ev.PlayerName)
		}
		if ev.Data != nil {
			if score, ok := ev.Data["score"]; ok {
				fmt.Printf(" | Score: %s", score)
			}
			if player, ok := ev.Data["player"]; ok {
				fmt.Printf(" | Player: %s", player)
			}
		}
		fmt.Println()
	}

	fmt.Printf("\nTotal events: %d\n", eventCount)
	fmt.Printf("Custom parser matched: %d events\n\n", scoreParser.MatchCount())

	// ========================================
	// Example 3: Thread-Safe Parser
	// ========================================
	fmt.Println("Example 3: Thread-Safe Parser with Mutex")
	fmt.Println("-----------------------------------------")
	fmt.Println("Useful when parser is shared across goroutines")
	fmt.Println()

	threadSafeParser := NewThreadSafeParser()
	fmt.Printf("Initial stats: total=%d, matched=%d\n",
		threadSafeParser.TotalLines(),
		threadSafeParser.MatchedLines())

	for ev, err := range vrclog.ParseFile(ctx, tmpFile,
		vrclog.WithParseParser(threadSafeParser),
	) {
		if err != nil {
			log.Printf("Error: %v", err)
			break
		}
		// Process event...
		_ = ev
	}

	fmt.Printf("Final stats: total=%d, matched=%d\n",
		threadSafeParser.TotalLines(),
		threadSafeParser.MatchedLines())
	fmt.Println()

	// ========================================
	// Summary
	// ========================================
	fmt.Println("Summary:")
	fmt.Println("--------")
	fmt.Println("Parser Interface vs ParserFunc:")
	fmt.Println()
	fmt.Println("Parser Interface (struct):")
	fmt.Println("  ✓ Can maintain state (counters, caches)")
	fmt.Println("  ✓ Can have custom methods")
	fmt.Println("  ✓ Can be thread-safe with mutex")
	fmt.Println("  ✓ Useful for complex parsing logic")
	fmt.Println()
	fmt.Println("ParserFunc (function):")
	fmt.Println("  ✓ Simpler for stateless parsing")
	fmt.Println("  ✓ Quick inline definitions")
	fmt.Println("  ✓ Good for one-off parsers")
	fmt.Println()
	fmt.Println("See examples/parserfunc for ParserFunc usage")
}

// ============================================================================
// Simple Parser Implementation
// ============================================================================

// Compile-time verification that ScoreParser implements vrclog.Parser
var _ vrclog.Parser = (*ScoreParser)(nil)

// ScoreParser parses custom game score events.
// Demonstrates state management (match counting).
type ScoreParser struct {
	scorePattern  *regexp.Regexp
	winnerPattern *regexp.Regexp
	matchCount    int // State: number of matches found
}

// NewScoreParser creates a new score parser
func NewScoreParser() *ScoreParser {
	return &ScoreParser{
		scorePattern:  regexp.MustCompile(`\[Game\]: Score: (\d+) by (\w+)`),
		winnerPattern: regexp.MustCompile(`\[Game\]: Winner: (\w+)`),
	}
}

// ParseLine implements the vrclog.Parser interface
func (p *ScoreParser) ParseLine(ctx context.Context, line string) (vrclog.ParseResult, error) {
	// 1. Check for context cancellation
	if err := ctx.Err(); err != nil {
		return vrclog.ParseResult{}, err
	}

	// 2. Extract timestamp from VRChat log line
	ts, rest, ok := extractTimestamp(line)
	if !ok {
		return vrclog.ParseResult{Matched: false}, nil
	}

	// 3. Try score pattern
	if match := p.scorePattern.FindStringSubmatch(rest); match != nil {
		// Update state (this is why we use struct instead of ParserFunc)
		p.matchCount++

		ev := event.Event{
			Type:      TypeGameScore,
			Timestamp: ts,
			Data: map[string]string{
				"score":  match[1],
				"player": match[2],
			},
		}
		return vrclog.ParseResult{
			Events:  []event.Event{ev},
			Matched: true,
		}, nil
	}

	// 4. Try winner pattern
	if match := p.winnerPattern.FindStringSubmatch(rest); match != nil {
		p.matchCount++

		ev := event.Event{
			Type:      TypeGameWin,
			Timestamp: ts,
			Data: map[string]string{
				"player": match[1],
			},
		}
		return vrclog.ParseResult{
			Events:  []event.Event{ev},
			Matched: true,
		}, nil
	}

	// 5. No match
	return vrclog.ParseResult{Matched: false}, nil
}

// MatchCount returns the number of lines matched (custom method)
func (p *ScoreParser) MatchCount() int {
	return p.matchCount
}

// Reset resets the match counter (custom method)
func (p *ScoreParser) Reset() {
	p.matchCount = 0
}

// ============================================================================
// Thread-Safe Parser Implementation
// ============================================================================

// Compile-time verification that ThreadSafeParser implements vrclog.Parser
var _ vrclog.Parser = (*ThreadSafeParser)(nil)

// ThreadSafeParser is a thread-safe version with mutex protection.
// Note: ParseFile is typically single-threaded, so this is only needed
// if the parser is shared across multiple goroutines or used in ParserChain
// where parsers might be called concurrently.
type ThreadSafeParser struct {
	pattern *regexp.Regexp
	mu      sync.Mutex
	stats   struct {
		total   int
		matched int
	}
}

// NewThreadSafeParser creates a thread-safe parser
func NewThreadSafeParser() *ThreadSafeParser {
	return &ThreadSafeParser{
		pattern: regexp.MustCompile(`\[Game\]: Score: (\d+) by (\w+)`),
	}
}

// ParseLine implements the vrclog.Parser interface with thread safety
func (p *ThreadSafeParser) ParseLine(ctx context.Context, line string) (vrclog.ParseResult, error) {
	// Check cancellation before taking lock
	if err := ctx.Err(); err != nil {
		return vrclog.ParseResult{}, err
	}

	// Lock for state updates
	p.mu.Lock()
	defer p.mu.Unlock()

	p.stats.total++

	ts, rest, ok := extractTimestamp(line)
	if !ok {
		return vrclog.ParseResult{Matched: false}, nil
	}

	match := p.pattern.FindStringSubmatch(rest)
	if match == nil {
		return vrclog.ParseResult{Matched: false}, nil
	}

	p.stats.matched++

	ev := event.Event{
		Type:      TypeGameScore,
		Timestamp: ts,
		Data: map[string]string{
			"score":  match[1],
			"player": match[2],
		},
	}

	return vrclog.ParseResult{
		Events:  []event.Event{ev},
		Matched: true,
	}, nil
}

// TotalLines returns total lines processed (thread-safe)
func (p *ThreadSafeParser) TotalLines() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.stats.total
}

// MatchedLines returns matched lines (thread-safe)
func (p *ThreadSafeParser) MatchedLines() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.stats.matched
}

// ============================================================================
// Helper Functions
// ============================================================================

// extractTimestamp extracts the timestamp and remaining content from a VRChat log line.
// VRChat format: "2024.01.15 12:00:00 Log        -  [content]"
// Returns (timestamp, remaining content, success)
func extractTimestamp(line string) (time.Time, string, bool) {
	if len(line) < 19 {
		return time.Time{}, "", false
	}

	// Parse timestamp (first 19 characters)
	ts, err := time.ParseInLocation("2006.01.02 15:04:05", line[:19], time.Local)
	if err != nil {
		return time.Time{}, "", false
	}

	// Return timestamp and remaining content
	remaining := line[19:]
	return ts, remaining, true
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
