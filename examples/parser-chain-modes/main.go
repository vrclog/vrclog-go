// Package main demonstrates ParserChain modes for combining multiple parsers.
//
// This example shows the practical use cases for each mode:
// - ChainAll: Execute all parsers, combine results
// - ChainFirst: Stop at first match (priority/fallback pattern)
// - ChainContinueOnError: Resilient parsing (collect errors, continue)
//
// Key pattern: Strategy pattern - behavior changes based on mode.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/vrclog/vrclog-go/pkg/vrclog"
	"github.com/vrclog/vrclog-go/pkg/vrclog/event"
	"github.com/vrclog/vrclog-go/pkg/vrclog/pattern"
)

const testLogData = `2024.01.15 10:00:00 Log        -  [Behaviour] OnPlayerJoined Alice
2024.01.15 10:01:00 Log        -  [Game] Special event: Alice won the game
2024.01.15 10:02:00 Log        -  [Behaviour] OnPlayerLeft Bob
2024.01.15 10:03:00 Log        -  [Test] ERROR_LINE simulated error
2024.01.15 10:04:00 Log        -  [Game] Special event: Bob scored 100 points
2024.01.15 10:05:00 Log        -  [Behaviour] Entering Room: Test World
`

func main() {
	fmt.Println("Parser Chain Modes Example")
	fmt.Println("==========================")
	fmt.Println()

	ctx := context.Background()

	// Create temp log file
	tmpFile := createTempFile(testLogData)
	defer os.Remove(tmpFile)

	// Define custom parser for game events
	gamePattern := []byte(`version: 1
patterns:
  - id: game_win
    event_type: game_win
    regex: 'Special event: (?P<player>\w+) won the game'
  - id: game_score
    event_type: game_score
    regex: 'Special event: (?P<player>\w+) scored (?P<points>\d+) points'
`)

	pf, err := pattern.LoadBytes(gamePattern)
	if err != nil {
		log.Fatal(err)
	}

	gameParser, err := pattern.NewRegexParser(pf)
	if err != nil {
		log.Fatal(err)
	}

	// Create error-prone parser for demonstration
	errorParser := vrclog.ParserFunc(func(ctx context.Context, line string) (vrclog.ParseResult, error) {
		if strings.Contains(line, "ERROR_LINE") {
			return vrclog.ParseResult{}, fmt.Errorf("simulated parse error")
		}
		return vrclog.ParseResult{Matched: false}, nil
	})

	// ========================================
	// 1. ChainAll - Execute All Parsers
	// ========================================
	fmt.Println("1. ChainAll Mode")
	fmt.Println("----------------")
	fmt.Println("Use case: Parse both built-in VRChat events AND custom game events")
	fmt.Println()

	chainAll := &vrclog.ParserChain{
		Mode: vrclog.ChainAll,
		Parsers: []vrclog.Parser{
			vrclog.DefaultParser{}, // VRChat events
			gameParser,             // Game events
		},
	}

	fmt.Println("→ Parsing with ChainAll:")
	fmt.Println("  - DefaultParser: Handles VRChat events")
	fmt.Println("  - GameParser: Handles game events")
	fmt.Println()

	eventNumber := 0
	for ev, err := range vrclog.ParseFile(ctx, tmpFile,
		vrclog.WithParseParser(chainAll),
	) {
		if err != nil {
			log.Printf("Error: %v", err)
			continue
		}
		eventNumber++
		printEvent(eventNumber, ev)
	}
	fmt.Println()
	fmt.Println("→ Result: All events from both parsers collected")
	fmt.Println()

	// ========================================
	// 2. ChainFirst - Priority Parser
	// ========================================
	fmt.Println("2. ChainFirst Mode")
	fmt.Println("------------------")
	fmt.Println("Use case: Priority parser with fallback (custom events override defaults)")
	fmt.Println()

	chainFirst := &vrclog.ParserChain{
		Mode: vrclog.ChainFirst,
		Parsers: []vrclog.Parser{
			gameParser,             // Try game parser first (higher priority)
			vrclog.DefaultParser{}, // Fallback to default
		},
	}

	fmt.Println("→ Parsing with ChainFirst:")
	fmt.Println("  - GameParser has priority")
	fmt.Println("  - DefaultParser used only if GameParser doesn't match")
	fmt.Println()

	eventNumber = 0
	for ev, err := range vrclog.ParseFile(ctx, tmpFile,
		vrclog.WithParseParser(chainFirst),
	) {
		if err != nil {
			log.Printf("Error: %v", err)
			continue
		}
		eventNumber++
		printEvent(eventNumber, ev)
	}
	fmt.Println()
	fmt.Println("→ Result: First matching parser handles each line")
	fmt.Println()

	// ========================================
	// 3. ChainContinueOnError - Error Resilience
	// ========================================
	fmt.Println("3. ChainContinueOnError Mode")
	fmt.Println("----------------------------")
	fmt.Println("Use case: Resilient parsing - continue even if one parser fails")
	fmt.Println()

	chainContinue := &vrclog.ParserChain{
		Mode: vrclog.ChainContinueOnError,
		Parsers: []vrclog.Parser{
			errorParser,            // May fail on some lines
			vrclog.DefaultParser{}, // Should still process successfully
			gameParser,             // Should still process successfully
		},
	}

	fmt.Println("→ Parsing with ChainContinueOnError:")
	fmt.Println("  - ErrorParser: Fails on ERROR_LINE")
	fmt.Println("  - DefaultParser + GameParser: Continue processing")
	fmt.Println()

	eventNumber = 0
	for ev, err := range vrclog.ParseFile(ctx, tmpFile,
		vrclog.WithParseParser(chainContinue),
	) {
		if err != nil {
			// ChainContinueOnError may return aggregated errors via errors.Join
			fmt.Printf("  [Line %d] Errors occurred, but got events:\n", eventNumber+1)

			// Unwrap aggregated errors
			errs := unwrapJoinedErrors(err)
			for i, e := range errs {
				fmt.Printf("    Error %d: %v\n", i+1, e)
			}
		}
		eventNumber++
		if err == nil {
			printEvent(eventNumber, ev)
		} else {
			// Still got an event despite errors
			if ev.Type != "" {
				fmt.Printf("    But still got event: %s\n", ev.Type)
			}
		}
	}
	fmt.Println()
	fmt.Println("→ Result: Parsing continues despite errors from errorParser")
	fmt.Println()

	// ========================================
	// 4. WithParsers() Shorthand
	// ========================================
	fmt.Println("4. WithParsers() Shorthand")
	fmt.Println("--------------------------")
	fmt.Println("Convenience function: Automatically uses ChainAll mode")
	fmt.Println()

	fmt.Println("→ Equivalent to manually creating a ChainAll:")
	fmt.Println()

	fmt.Println("  Manual:")
	fmt.Println("    chain := &vrclog.ParserChain{")
	fmt.Println("        Mode: vrclog.ChainAll,")
	fmt.Println("        Parsers: []vrclog.Parser{parser1, parser2},")
	fmt.Println("    }")
	fmt.Println()

	fmt.Println("  Shorthand:")
	fmt.Println("    vrclog.WithParsers(parser1, parser2)")
	fmt.Println()

	// Demonstrate with ParseFile
	events := make([]event.Event, 0)
	for ev, err := range vrclog.ParseFile(ctx, tmpFile,
		vrclog.WithParseParser(&vrclog.ParserChain{
			Mode:    vrclog.ChainAll,
			Parsers: []vrclog.Parser{vrclog.DefaultParser{}, gameParser},
		}),
	) {
		if err != nil {
			continue
		}
		events = append(events, ev)
	}
	fmt.Printf("→ Parsed %d events using manual ChainAll\n", len(events))
	fmt.Println()

	// ========================================
	// Summary
	// ========================================
	fmt.Println("Summary:")
	fmt.Println("--------")
	fmt.Println("ChainAll              : Execute all parsers, combine results")
	fmt.Println("                        Use: Parse multiple event types simultaneously")
	fmt.Println()
	fmt.Println("ChainFirst            : Stop at first match (priority order)")
	fmt.Println("                        Use: Custom parser overrides default behavior")
	fmt.Println()
	fmt.Println("ChainContinueOnError  : Continue parsing despite errors")
	fmt.Println("                        Use: Resilient parsing of mixed valid/invalid data")
	fmt.Println()
	fmt.Println("WithParsers()         : Shorthand for ChainAll mode")
	fmt.Println("                        Use: Convenient API for common case")
	fmt.Println()
	fmt.Println("Pattern: Strategy pattern - mode changes behavior at runtime")
}

// printEvent prints a single event
func printEvent(lineNum int, ev event.Event) {
	fmt.Printf("  [Line %d] %s", lineNum, ev.Type)
	if ev.PlayerName != "" {
		fmt.Printf(" - %s", ev.PlayerName)
	}
	if ev.WorldName != "" {
		fmt.Printf(" - %s", ev.WorldName)
	}
	if len(ev.Data) > 0 {
		fmt.Printf(" - Data: %v", ev.Data)
	}
	fmt.Println()
}

// unwrapJoinedErrors unwraps errors created by errors.Join
func unwrapJoinedErrors(err error) []error {
	type unwrapper interface {
		Unwrap() []error
	}

	if u, ok := err.(unwrapper); ok {
		return u.Unwrap()
	}

	return []error{err}
}

// createTempFile creates a temporary VRChat log file
func createTempFile(content string) string {
	tmpFile, err := os.CreateTemp("", "output_log_*.txt")
	if err != nil {
		log.Fatal(err)
	}

	if _, err := tmpFile.WriteString(content); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		log.Fatal(err)
	}

	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpFile.Name())
		log.Fatal(err)
	}

	return tmpFile.Name()
}
