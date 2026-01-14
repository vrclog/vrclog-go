package pattern

import (
	"context"
	"testing"
)

// BenchmarkRegexParser_SinglePattern benchmarks parsing with a single pattern.
func BenchmarkRegexParser_SinglePattern(b *testing.B) {
	pf := &PatternFile{
		Version: 1,
		Patterns: []Pattern{
			{
				ID:        "test",
				EventType: "test_event",
				Regex:     `Test Pattern: (?P<value>\w+)`,
			},
		},
	}
	parser, err := NewRegexParser(pf)
	if err != nil {
		b.Fatalf("Failed to create parser: %v", err)
	}

	line := "2024.01.15 23:59:59 Log - Test Pattern: ABC123"
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = parser.ParseLine(ctx, line)
	}
}

// BenchmarkRegexParser_SinglePattern_NoMatch benchmarks parsing with no match.
func BenchmarkRegexParser_SinglePattern_NoMatch(b *testing.B) {
	pf := &PatternFile{
		Version: 1,
		Patterns: []Pattern{
			{
				ID:        "test",
				EventType: "test_event",
				Regex:     `Test Pattern: \w+`,
			},
		},
	}
	parser, err := NewRegexParser(pf)
	if err != nil {
		b.Fatalf("Failed to create parser: %v", err)
	}

	line := "2024.01.15 23:59:59 Log - This line does not match"
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = parser.ParseLine(ctx, line)
	}
}

// BenchmarkRegexParser_MultiplePatterns benchmarks parsing with multiple patterns.
func BenchmarkRegexParser_MultiplePatterns(b *testing.B) {
	pf := &PatternFile{
		Version: 1,
		Patterns: []Pattern{
			{
				ID:        "poker_hand",
				EventType: "poker_hand",
				Regex:     `Draw Local Hole Cards: (?P<card1>\w+), (?P<card2>\w+)`,
			},
			{
				ID:        "poker_winner",
				EventType: "poker_winner",
				Regex:     `player (?P<seat_id>\d+) won (?P<amount>\d+)`,
			},
			{
				ID:        "poker_bet",
				EventType: "poker_bet",
				Regex:     `Player (?P<name>\w+) bet (?P<amount>\d+)`,
			},
			{
				ID:        "quest_complete",
				EventType: "quest_complete",
				Regex:     `Quest "(?P<quest_name>[^"]+)" completed`,
			},
			{
				ID:        "score",
				EventType: "score",
				Regex:     `Player (?P<name>\w+) scored (?P<points>\d+) points`,
			},
		},
	}
	parser, err := NewRegexParser(pf)
	if err != nil {
		b.Fatalf("Failed to create parser: %v", err)
	}

	// Line that matches the second pattern
	line := "2024.01.15 23:59:59 Log - [Game] player 3 won 150"
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = parser.ParseLine(ctx, line)
	}
}

// BenchmarkRegexParser_MultiplePatterns_FirstMatch benchmarks parsing when first pattern matches.
func BenchmarkRegexParser_MultiplePatterns_FirstMatch(b *testing.B) {
	pf := &PatternFile{
		Version: 1,
		Patterns: []Pattern{
			{
				ID:        "poker_hand",
				EventType: "poker_hand",
				Regex:     `Draw Local Hole Cards: (?P<card1>\w+), (?P<card2>\w+)`,
			},
			{
				ID:        "poker_winner",
				EventType: "poker_winner",
				Regex:     `player (?P<seat_id>\d+) won (?P<amount>\d+)`,
			},
		},
	}
	parser, err := NewRegexParser(pf)
	if err != nil {
		b.Fatalf("Failed to create parser: %v", err)
	}

	// Line that matches the first pattern
	line := "2024.01.15 23:59:59 Log - Draw Local Hole Cards: AceSpades, KingHearts"
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = parser.ParseLine(ctx, line)
	}
}

// BenchmarkRegexParser_MultiplePatterns_NoMatch benchmarks parsing with no match across multiple patterns.
func BenchmarkRegexParser_MultiplePatterns_NoMatch(b *testing.B) {
	pf := &PatternFile{
		Version: 1,
		Patterns: []Pattern{
			{
				ID:        "poker_hand",
				EventType: "poker_hand",
				Regex:     `Draw Local Hole Cards: (?P<card1>\w+), (?P<card2>\w+)`,
			},
			{
				ID:        "poker_winner",
				EventType: "poker_winner",
				Regex:     `player (?P<seat_id>\d+) won (?P<amount>\d+)`,
			},
			{
				ID:        "poker_bet",
				EventType: "poker_bet",
				Regex:     `Player (?P<name>\w+) bet (?P<amount>\d+)`,
			},
		},
	}
	parser, err := NewRegexParser(pf)
	if err != nil {
		b.Fatalf("Failed to create parser: %v", err)
	}

	line := "2024.01.15 23:59:59 Log - This line does not match any pattern"
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = parser.ParseLine(ctx, line)
	}
}

// BenchmarkRegexParser_ComplexPattern benchmarks parsing with a complex regex pattern.
func BenchmarkRegexParser_ComplexPattern(b *testing.B) {
	pf := &PatternFile{
		Version: 1,
		Patterns: []Pattern{
			{
				ID:        "complex",
				EventType: "complex_event",
				Regex:     `\[(?P<component>\w+)\]: (?P<action>\w+) .* player (?P<seat_id>\d+) (?P<verb>\w+) (?P<amount>\d+) (?P<unit>\w+)`,
			},
		},
	}
	parser, err := NewRegexParser(pf)
	if err != nil {
		b.Fatalf("Failed to create parser: %v", err)
	}

	line := "2024.01.15 23:59:59 Log - [PotManager]: Round complete, player 3 won 500 chips"
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = parser.ParseLine(ctx, line)
	}
}

// BenchmarkRegexParser_LongLine benchmarks parsing a very long log line.
func BenchmarkRegexParser_LongLine(b *testing.B) {
	pf := &PatternFile{
		Version: 1,
		Patterns: []Pattern{
			{
				ID:        "test",
				EventType: "test_event",
				Regex:     `Value: (?P<value>\w+)`,
			},
		},
	}
	parser, err := NewRegexParser(pf)
	if err != nil {
		b.Fatalf("Failed to create parser: %v", err)
	}

	// Create a 2KB line
	longContent := string(make([]byte, 2000))
	line := "2024.01.15 23:59:59 Log - " + longContent + " Value: ABC123"
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = parser.ParseLine(ctx, line)
	}
}

// BenchmarkNewRegexParser benchmarks the parser creation from a PatternFile.
func BenchmarkNewRegexParser(b *testing.B) {
	pf := &PatternFile{
		Version: 1,
		Patterns: []Pattern{
			{
				ID:        "test1",
				EventType: "test1",
				Regex:     `Pattern 1: (?P<value>\w+)`,
			},
			{
				ID:        "test2",
				EventType: "test2",
				Regex:     `Pattern 2: (?P<name>\w+) (?P<score>\d+)`,
			},
			{
				ID:        "test3",
				EventType: "test3",
				Regex:     `Pattern 3: \[(?P<tag>\w+)\]`,
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = NewRegexParser(pf)
	}
}

// BenchmarkLoadBytes benchmarks loading a pattern file from bytes.
func BenchmarkLoadBytes(b *testing.B) {
	data := []byte(`version: 1
patterns:
  - id: poker_hand
    event_type: poker_hand
    regex: 'Draw Local Hole Cards: (?P<card1>\w+), (?P<card2>\w+)'
  - id: poker_winner
    event_type: poker_winner
    regex: 'player (?P<seat_id>\d+) won (?P<amount>\d+)'
`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = LoadBytes(data)
	}
}
