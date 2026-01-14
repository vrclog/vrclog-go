// Package main demonstrates event filtering patterns in vrclog-go.
//
// This example shows how to filter events by type using:
// - WithExcludeTypes: Block specific event types
// - WithFilter: Combine include and exclude filters
// - event.TypeNames: List all valid event types
// - event.ParseType: Validate user input
//
// Key concept: Exclude filters take precedence over include filters.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/vrclog/vrclog-go/pkg/vrclog"
	"github.com/vrclog/vrclog-go/pkg/vrclog/event"
)

const testLogData = `2024.01.15 10:00:00 Log        -  [Behaviour] OnPlayerJoined Alice
2024.01.15 10:01:00 Log        -  [Behaviour] OnPlayerJoined Bob
2024.01.15 10:02:00 Log        -  [Behaviour] Entering Room: Test World
2024.01.15 10:03:00 Log        -  [Behaviour] OnPlayerLeft Alice
2024.01.15 10:04:00 Log        -  [Behaviour] OnPlayerJoined Charlie
2024.01.15 10:05:00 Log        -  [Behaviour] OnPlayerLeft Bob
2024.01.15 10:06:00 Log        -  [Behaviour] Entering Room: Another World
2024.01.15 10:07:00 Log        -  [Behaviour] OnPlayerLeft Charlie
`

func main() {
	fmt.Println("Event Filtering Example")
	fmt.Println("=======================")
	fmt.Println()

	ctx := context.Background()

	// Create temp log file
	tmpFile := createTempFile(testLogData)
	defer os.Remove(tmpFile)

	fmt.Println("Test data contains:")
	fmt.Println("  - 3 player_join events (Alice, Bob, Charlie)")
	fmt.Println("  - 3 player_left events (Alice, Bob, Charlie)")
	fmt.Println("  - 2 world_join events (Test World, Another World)")
	fmt.Println()

	// ========================================
	// 1. Basic Exclude Filter
	// ========================================
	fmt.Println("1. WithExcludeTypes - Block specific events")
	fmt.Println("--------------------------------------------")
	fmt.Println()

	fmt.Println("→ Exclude player_left events:")
	var events []event.Event
	for ev, err := range vrclog.ParseFile(ctx, tmpFile,
		vrclog.WithParseExcludeTypes(vrclog.EventPlayerLeft),
	) {
		if err != nil {
			log.Fatal(err)
		}
		events = append(events, ev)
	}
	printEventSummary(events)
	fmt.Println()

	// ========================================
	// 2. Include + Exclude Precedence
	// ========================================
	fmt.Println("2. WithFilter - Include + Exclude Precedence")
	fmt.Println("---------------------------------------------")
	fmt.Println("Key rule: Exclude takes precedence over include")
	fmt.Println()

	fmt.Println("→ Include: player_join + player_left")
	fmt.Println("→ Exclude: player_left")
	fmt.Println("→ Result: Only player_join (exclude wins)")
	fmt.Println()

	events = nil
	for ev, err := range vrclog.ParseFile(ctx, tmpFile,
		vrclog.WithParseFilter(
			[]vrclog.EventType{vrclog.EventPlayerJoin, vrclog.EventPlayerLeft}, // include
			[]vrclog.EventType{vrclog.EventPlayerLeft},                         // exclude (wins!)
		),
	) {
		if err != nil {
			log.Fatal(err)
		}
		events = append(events, ev)
	}
	printEventSummary(events)
	fmt.Println()

	// ========================================
	// 3. event.TypeNames() - Dynamic Discovery
	// ========================================
	fmt.Println("3. event.TypeNames() - Discover Available Types")
	fmt.Println("------------------------------------------------")
	fmt.Println()

	allTypes := event.TypeNames()
	fmt.Println("→ All valid event types:")
	for i, typeName := range allTypes {
		fmt.Printf("  %d. %s\n", i+1, typeName)
	}
	fmt.Println()

	fmt.Println("→ Use case: Building dynamic filters from user input")
	fmt.Println()

	// ========================================
	// 4. event.ParseType() - Input Validation
	// ========================================
	fmt.Println("4. event.ParseType() - Validate User Input")
	fmt.Println("-------------------------------------------")
	fmt.Println()

	// Simulate CLI arguments
	userInputs := []string{"player_join", "world_join", "invalid_type", "PLAYER_LEFT"}

	fmt.Println("→ Parsing user-provided event types:")
	for _, input := range userInputs {
		parsedType, ok := event.ParseType(input)
		if ok {
			fmt.Printf("  ✓ \"%s\" → %s (valid)\n", input, parsedType)
		} else {
			fmt.Printf("  ✗ \"%s\" → invalid type\n", input)
		}
	}
	fmt.Println()

	// ========================================
	// 5. Practical Example: CLI-style Filtering
	// ========================================
	fmt.Println("5. Practical Example: CLI-style Filtering")
	fmt.Println("------------------------------------------")
	fmt.Println()

	// Simulate: myapp --include player_join,world_join --exclude world_join
	includeArgs := "player_join,world_join"
	excludeArgs := "world_join"

	fmt.Printf("→ User input:\n")
	fmt.Printf("  --include %s\n", includeArgs)
	fmt.Printf("  --exclude %s\n", excludeArgs)
	fmt.Println()

	includeTypes, err := parseEventTypes(includeArgs)
	if err != nil {
		log.Fatalf("Invalid include types: %v", err)
	}

	excludeTypes, err := parseEventTypes(excludeArgs)
	if err != nil {
		log.Fatalf("Invalid exclude types: %v", err)
	}

	fmt.Println("→ Parsed filter config:")
	fmt.Printf("  Include: %v\n", includeTypes)
	fmt.Printf("  Exclude: %v\n", excludeTypes)
	fmt.Println("→ Expected: Only player_join (world_join excluded)")
	fmt.Println()

	events = nil
	for ev, err := range vrclog.ParseFile(ctx, tmpFile,
		vrclog.WithParseFilter(includeTypes, excludeTypes),
	) {
		if err != nil {
			log.Fatal(err)
		}
		events = append(events, ev)
	}
	printEventSummary(events)
	fmt.Println()

	// ========================================
	// Summary
	// ========================================
	fmt.Println("Summary:")
	fmt.Println("--------")
	fmt.Println("WithExcludeTypes()    : Block specific event types")
	fmt.Println("WithFilter()          : Combine include + exclude (exclude wins)")
	fmt.Println("event.TypeNames()     : List all valid event types")
	fmt.Println("event.ParseType()     : Validate and parse event type strings")
	fmt.Println()
	fmt.Println("Filter precedence: Exclude > Include")
	fmt.Println("Use case: Building CLI tools with user-configurable filters")
}

// parseEventTypes parses a comma-separated string of event types
func parseEventTypes(s string) ([]vrclog.EventType, error) {
	if s == "" {
		return nil, nil
	}

	parts := strings.Split(s, ",")
	types := make([]vrclog.EventType, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		eventType, ok := event.ParseType(part)
		if !ok {
			return nil, fmt.Errorf("unknown event type: %s (available: %v)",
				part, event.TypeNames())
		}

		types = append(types, vrclog.EventType(eventType))
	}

	return types, nil
}

// printEventSummary prints a summary of collected events
func printEventSummary(events []event.Event) {
	typeCounts := make(map[event.Type]int)
	for _, ev := range events {
		typeCounts[ev.Type]++
	}

	fmt.Printf("→ Collected %d events:\n", len(events))
	for eventType, count := range typeCounts {
		fmt.Printf("  - %s: %d\n", eventType, count)
	}
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
