package pattern_test

import (
	"context"
	"fmt"
	"log"

	"github.com/vrclog/vrclog-go/pkg/vrclog"
	"github.com/vrclog/vrclog-go/pkg/vrclog/pattern"
)

// Example demonstrates basic usage of the pattern package with in-memory YAML.
func Example() {
	// Define custom patterns in YAML format
	yamlData := []byte(`version: 1
patterns:
  - id: poker_hand
    event_type: poker_hand
    regex: 'Draw Local Hole Cards: (?P<card1>\w+), (?P<card2>\w+)'
`)

	// Load pattern file from bytes
	pf, err := pattern.LoadBytes(yamlData)
	if err != nil {
		log.Fatal(err)
	}

	// Create regex parser from pattern file
	parser, err := pattern.NewRegexParser(pf)
	if err != nil {
		log.Fatal(err)
	}

	// Parse a log line
	line := "2024.01.15 23:59:59 Log - [Seat]: Draw Local Hole Cards: AceSpades, KingHearts"
	result, err := parser.ParseLine(context.Background(), line)
	if err != nil {
		log.Fatal(err)
	}

	if result.Matched {
		ev := result.Events[0]
		fmt.Printf("Type: %s\n", ev.Type)
		fmt.Printf("Card 1: %s\n", ev.Data["card1"])
		fmt.Printf("Card 2: %s\n", ev.Data["card2"])
	}
	// Output:
	// Type: poker_hand
	// Card 1: AceSpades
	// Card 2: KingHearts
}

// ExampleNewRegexParserFromFile demonstrates loading patterns from a file.
func ExampleNewRegexParserFromFile() {
	// Load patterns from a YAML file
	parser, err := pattern.NewRegexParserFromFile("testdata/valid.yaml")
	if err != nil {
		log.Fatal(err)
	}

	// Parse a poker event
	line := "2024.01.15 23:59:59 Log - [Seat]: Draw Local Hole Cards: AceSpades, KingHearts"
	result, err := parser.ParseLine(context.Background(), line)
	if err != nil {
		log.Fatal(err)
	}

	if result.Matched {
		fmt.Printf("Matched: %v\n", result.Matched)
		fmt.Printf("Events: %d\n", len(result.Events))
	}
	// Output:
	// Matched: true
	// Events: 1
}

// ExampleLoad demonstrates loading and validating a pattern file.
func ExampleLoad() {
	// Load pattern file (validates schema but doesn't compile regexes yet)
	pf, err := pattern.Load("testdata/valid.yaml")
	if err != nil {
		log.Fatal(err)
	}

	// Display pattern file info
	fmt.Printf("Version: %d\n", pf.Version)
	fmt.Printf("Patterns: %d\n", len(pf.Patterns))
	fmt.Printf("First pattern ID: %s\n", pf.Patterns[0].ID)

	// Now create the parser (compiles regexes)
	parser, err := pattern.NewRegexParser(pf)
	if err != nil {
		log.Fatal(err)
	}

	// Use parser...
	_ = parser

	// Output:
	// Version: 1
	// Patterns: 2
	// First pattern ID: poker_hole_cards
}

// ExampleRegexParser_withDefaultParser demonstrates combining RegexParser
// with DefaultParser to handle both custom and built-in VRChat events.
func ExampleRegexParser_withDefaultParser() {
	ctx := context.Background()

	// Create custom parser for poker events
	yamlData := []byte(`version: 1
patterns:
  - id: poker_winner
    event_type: poker_winner
    regex: 'player (?P<seat_id>\d+) won (?P<amount>\d+)'
`)
	pf, err := pattern.LoadBytes(yamlData)
	if err != nil {
		log.Fatal(err)
	}
	customParser, err := pattern.NewRegexParser(pf)
	if err != nil {
		log.Fatal(err)
	}

	// Combine with DefaultParser using ChainAll mode
	chain := &vrclog.ParserChain{
		Mode: vrclog.ChainAll,
		Parsers: []vrclog.Parser{
			vrclog.DefaultParser{}, // Built-in VRChat events
			customParser,           // Custom poker events
		},
	}

	// Parse a VRChat player join event (handled by DefaultParser)
	result1, _ := chain.ParseLine(ctx, "2024.01.15 23:59:59 Log - [Behaviour] OnPlayerJoined TestUser")
	if result1.Matched {
		fmt.Printf("VRChat event: %s\n", result1.Events[0].Type)
	}

	// Parse a custom poker event (handled by RegexParser)
	result2, _ := chain.ParseLine(ctx, "2024.01.15 23:59:59 Log - [Game] player 3 won 500")
	if result2.Matched {
		ev := result2.Events[0]
		fmt.Printf("Custom event: %s (seat=%s, amount=%s)\n",
			ev.Type, ev.Data["seat_id"], ev.Data["amount"])
	}
	// Output:
	// VRChat event: player_join
	// Custom event: poker_winner (seat=3, amount=500)
}

// Example_captureGroups demonstrates how named capture groups in regex patterns
// populate the Event.Data field.
func Example_captureGroups() {
	ctx := context.Background()

	// Pattern with named capture groups
	yamlData := []byte(`version: 1
patterns:
  - id: player_score
    event_type: player_score
    regex: 'Player (?P<name>\w+) scored (?P<points>\d+) points'
`)

	pf, err := pattern.LoadBytes(yamlData)
	if err != nil {
		log.Fatal(err)
	}

	parser, err := pattern.NewRegexParser(pf)
	if err != nil {
		log.Fatal(err)
	}

	// Parse a line with capture groups
	line := "2024.01.15 23:59:59 Log - Player Alice scored 100 points"
	result, err := parser.ParseLine(ctx, line)
	if err != nil {
		log.Fatal(err)
	}

	if result.Matched {
		ev := result.Events[0]
		fmt.Printf("Event type: %s\n", ev.Type)
		fmt.Printf("Player name: %s\n", ev.Data["name"])
		fmt.Printf("Points: %s\n", ev.Data["points"])
		fmt.Printf("Data is nil: %v\n", ev.Data == nil)
	}

	// Pattern without named capture groups results in nil Data
	yamlData2 := []byte(`version: 1
patterns:
  - id: simple_event
    event_type: simple_event
    regex: 'Simple pattern without captures'
`)
	pf2, _ := pattern.LoadBytes(yamlData2)
	parser2, _ := pattern.NewRegexParser(pf2)

	result2, _ := parser2.ParseLine(ctx, "2024.01.15 23:59:59 Log - Simple pattern without captures")
	if result2.Matched {
		ev := result2.Events[0]
		fmt.Printf("Data is nil (no captures): %v\n", ev.Data == nil)
	}

	// Output:
	// Event type: player_score
	// Player name: Alice
	// Points: 100
	// Data is nil: false
	// Data is nil (no captures): true
}
