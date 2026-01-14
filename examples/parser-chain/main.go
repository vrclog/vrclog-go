// Package main demonstrates combining DefaultParser with custom RegexParser
// using ParserChain to handle both built-in and custom VRChat events.
//
// This example shows how to parse both standard VRChat events (player join/leave)
// and custom game events (poker) using a single parser chain.
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/vrclog/vrclog-go/pkg/vrclog"
	"github.com/vrclog/vrclog-go/pkg/vrclog/pattern"
)

func main() {
	// Define custom patterns for poker game
	yamlData := []byte(`version: 1
patterns:
  - id: poker_hand
    event_type: poker_hand
    regex: 'Draw Local Hole Cards: (?P<card1>\w+), (?P<card2>\w+)'
  - id: poker_winner
    event_type: poker_winner
    regex: 'player (?P<seat_id>\d+) won (?P<amount>\d+)'
`)

	pf, err := pattern.LoadBytes(yamlData)
	if err != nil {
		log.Fatalf("Failed to load patterns: %v", err)
	}

	customParser, err := pattern.NewRegexParser(pf)
	if err != nil {
		log.Fatalf("Failed to create custom parser: %v", err)
	}

	// Create parser chain: DefaultParser handles built-in events,
	// customParser handles game-specific events
	chain := &vrclog.ParserChain{
		Mode: vrclog.ChainAll, // Try all parsers
		Parsers: []vrclog.Parser{
			vrclog.DefaultParser{}, // Built-in VRChat events
			customParser,           // Custom poker events
		},
	}

	// Sample log lines with mixed event types
	logLines := []string{
		"2024.01.15 12:00:00 Log        -  [Behaviour] OnPlayerJoined Alice",
		"2024.01.15 12:00:10 Log        -  [Seat]: Draw Local Hole Cards: AceSpades, KingDiamonds",
		"2024.01.15 12:00:30 Log        -  [Behaviour] OnPlayerLeft Bob",
		"2024.01.15 12:00:45 Log        -  [Game] player 2 won 150",
		"2024.01.15 12:01:00 Log        -  [Behaviour] Entering Room: Poker VIP",
	}

	ctx := context.Background()

	fmt.Println("Parser Chain Example")
	fmt.Println("====================")
	fmt.Println()
	fmt.Println("Combining DefaultParser + RegexParser to handle both")
	fmt.Println("built-in VRChat events and custom game events.")
	fmt.Println()

	// Parse each line with the chain
	for i, line := range logLines {
		result, err := chain.ParseLine(ctx, line)
		if err != nil {
			log.Printf("Error parsing line: %v", err)
			continue
		}

		fmt.Printf("[%d] ", i+1)
		if result.Matched {
			for _, event := range result.Events {
				fmt.Printf("✓ %s", event.Type)

				// Print event-specific details
				switch event.Type {
				case vrclog.EventPlayerJoin:
					fmt.Printf(" - Player: %s", event.PlayerName)
				case vrclog.EventPlayerLeft:
					fmt.Printf(" - Player: %s", event.PlayerName)
				case vrclog.EventWorldJoin:
					fmt.Printf(" - World: %s", event.WorldName)
				case "poker_hand":
					fmt.Printf(" - Cards: %s, %s", event.Data["card1"], event.Data["card2"])
				case "poker_winner":
					fmt.Printf(" - Seat %s won %s chips", event.Data["seat_id"], event.Data["amount"])
				}
				fmt.Println()
			}
		} else {
			fmt.Printf("✗ No match\n")
		}
	}

	fmt.Println("\nAll events parsed successfully!")
	fmt.Println("\nChain modes:")
	fmt.Println("  - ChainAll: Try all parsers (current)")
	fmt.Println("  - ChainFirst: Stop at first match")
	fmt.Println("  - ChainContinueOnError: Continue even if a parser errors")
}
