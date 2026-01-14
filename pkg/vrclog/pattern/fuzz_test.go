package pattern

import (
	"context"
	"testing"
)

// FuzzRegexParser_ParseLine tests RegexParser.ParseLine with arbitrary input
// to ensure it never panics and handles all edge cases gracefully.
func FuzzRegexParser_ParseLine(f *testing.F) {
	// Create a simple parser with a few patterns for fuzzing
	pf := &PatternFile{
		Version: 1,
		Patterns: []Pattern{
			{
				ID:        "test_basic",
				EventType: "test_basic",
				Regex:     `Test: (\w+)`,
			},
			{
				ID:        "test_named",
				EventType: "test_named",
				Regex:     `Player (?P<name>\w+) score (?P<points>\d+)`,
			},
			{
				ID:        "test_complex",
				EventType: "test_complex",
				Regex:     `\[(\w+)\]: .* player (?P<seat_id>\d+) won (?P<amount>\d+)`,
			},
		},
	}

	parser, err := NewRegexParser(pf)
	if err != nil {
		f.Fatalf("Failed to create parser: %v", err)
	}

	// Seed corpus with valid VRChat log lines
	f.Add("2024.01.15 23:59:59 Log - Test: ABC123")
	f.Add("2024.01.15 23:59:59 Log - Player Alice score 100")
	f.Add("2024.01.15 23:59:59 Log - [Game]: Round complete, player 3 won 500")

	// Seed with edge cases
	f.Add("") // Empty string
	f.Add("no timestamp here")
	f.Add("2024.01.15 23:59:59")                   // Timestamp only
	f.Add("9999.99.99 99:99:99 Log - Test: Value") // Invalid timestamp
	f.Add(string([]byte{0xff, 0xfe, 0xfd}))        // Invalid UTF-8

	// Seed with long strings
	f.Add(string(make([]byte, 2048)))                                // 2KB of null bytes
	f.Add("2024.01.15 23:59:59 Log - " + string(make([]byte, 1024))) // Long content after timestamp

	// Seed with special characters
	f.Add("2024.01.15 23:59:59 Log - Test: \x00\x01\x02\r\n\t")
	f.Add("2024.01.15 23:59:59 Log - Player \uFFFD score 999")

	ctx := context.Background()

	f.Fuzz(func(t *testing.T, line string) {
		// ParseLine should never panic, regardless of input
		result, err := parser.ParseLine(ctx, line)

		// Error should only occur on context cancellation (not in this test)
		if err != nil {
			t.Errorf("ParseLine returned unexpected error: %v", err)
		}

		// Check invariants
		// Note: ParseResult.Matched can be true even when Events is empty
		// (e.g., filter that matches but outputs nothing - see CLAUDE.md)
		if !result.Matched && len(result.Events) > 0 {
			t.Error("ParseResult.Matched is false but Events is not empty")
		}

		// Validate each event
		for i, ev := range result.Events {
			// Type should not be empty
			if ev.Type == "" {
				t.Errorf("Event[%d] has empty Type", i)
			}

			// If Data is not nil, validate its contents
			if ev.Data != nil {
				for key, value := range ev.Data {
					if key == "" {
						t.Errorf("Event[%d] has Data with empty key", i)
					}
					// Value can be empty (valid capture of empty string)
					_ = value
				}
			}
		}
	})
}

// FuzzLoadBytes tests LoadBytes with arbitrary YAML input to ensure
// it never panics and properly validates input.
func FuzzLoadBytes(f *testing.F) {
	// Seed with valid YAML
	f.Add([]byte(`version: 1
patterns:
  - id: test
    event_type: test_event
    regex: 'test pattern'`))

	// Seed with edge cases
	f.Add([]byte(""))                         // Empty
	f.Add([]byte("not yaml"))                 // Invalid YAML
	f.Add([]byte("version: 999"))             // Unsupported version
	f.Add([]byte("version: 1"))               // No patterns
	f.Add(make([]byte, MaxPatternFileSize+1)) // Too large

	// Seed with invalid UTF-8
	f.Add([]byte{0xff, 0xfe, 0xfd})

	f.Fuzz(func(t *testing.T, data []byte) {
		// LoadBytes should never panic, regardless of input
		pf, err := LoadBytes(data)

		// Either both should be nil (error case) or both should be non-nil (success case)
		if (pf == nil) != (err != nil) {
			t.Errorf("LoadBytes inconsistent: pf=%v, err=%v", pf != nil, err)
		}

		// If successful, validate the pattern file
		if pf != nil {
			if pf.Version != SupportedVersion {
				t.Errorf("LoadBytes succeeded with unsupported version: %d", pf.Version)
			}
			if len(pf.Patterns) == 0 {
				t.Error("LoadBytes succeeded with no patterns")
			}

			// Validate each pattern
			for i, p := range pf.Patterns {
				if p.ID == "" {
					t.Errorf("Pattern[%d] has empty ID", i)
				}
				if p.EventType == "" {
					t.Errorf("Pattern[%d] has empty EventType", i)
				}
				if p.Regex == "" {
					t.Errorf("Pattern[%d] has empty Regex", i)
				}
				if len(p.Regex) > MaxPatternLength {
					t.Errorf("Pattern[%d] regex too long: %d (max %d)", i, len(p.Regex), MaxPatternLength)
				}
			}
		}
	})
}
