// Package main demonstrates using RegexParser to extract custom events from VRChat logs.
//
// This example shows how to define custom patterns for game-specific events
// (e.g., poker game events) using YAML patterns and parse them from log lines.
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/vrclog/vrclog-go/pkg/vrclog/pattern"
)

func main() {
	// Define custom patterns for a poker game in VRChat
	yamlData := []byte(`version: 1
patterns:
  - id: poker_hole_cards
    event_type: poker_hole_cards
    regex: '\[Seat\]: Draw Local Hole Cards: (?P<card1>\w+), (?P<card2>\w+)'
  - id: poker_winner
    event_type: poker_winner
    regex: '\[PotManager\]: .* player (?P<seat_id>\d+) won (?P<amount>\d+) chips'
  - id: poker_bet
    event_type: poker_bet
    regex: 'Player (?P<player_name>\w+) bet (?P<amount>\d+) chips'
`)

	// Load and compile patterns
	pf, err := pattern.LoadBytes(yamlData)
	if err != nil {
		log.Fatalf("Failed to load patterns: %v", err)
	}

	parser, err := pattern.NewRegexParser(pf)
	if err != nil {
		log.Fatalf("Failed to create parser: %v", err)
	}

	// Sample VRChat log lines (simulated)
	logLines := []string{
		"2024.01.15 12:00:00 Log        -  [Seat]: Draw Local Hole Cards: AceSpades, KingHearts",
		"2024.01.15 12:00:15 Log        -  Player Alice bet 50 chips",
		"2024.01.15 12:00:30 Log        -  [PotManager]: Round complete, player 3 won 200 chips",
		"2024.01.15 12:00:45 Log        -  [Behaviour] OnPlayerJoined Bob",
	}

	ctx := context.Background()

	fmt.Println("Custom Event Parser Example")
	fmt.Println("============================")
	fmt.Println()

	// Parse each line
	for _, line := range logLines {
		result, err := parser.ParseLine(ctx, line)
		if err != nil {
			log.Printf("Error parsing line: %v", err)
			continue
		}

		if result.Matched {
			for _, event := range result.Events {
				fmt.Printf("Event: %s\n", event.Type)
				fmt.Printf("  Timestamp: %s\n", event.Timestamp.Format("15:04:05"))
				if event.Data != nil {
					fmt.Printf("  Data:\n")
					for key, value := range event.Data {
						fmt.Printf("    %s: %s\n", key, value)
					}
				}
				fmt.Println()
			}
		} else {
			// This line doesn't match any custom patterns
			// (e.g., the OnPlayerJoined event is not in our custom patterns)
			fmt.Printf("No match: %s\n\n", line[20:]) // Skip timestamp for brevity
		}
	}

	fmt.Println("\nNote: To handle both custom and built-in VRChat events,")
	fmt.Println("see the parser-chain example.")
}
