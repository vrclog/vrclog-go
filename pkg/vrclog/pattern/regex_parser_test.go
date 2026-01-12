package pattern_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vrclog/vrclog-go/pkg/vrclog"
	"github.com/vrclog/vrclog-go/pkg/vrclog/event"
	"github.com/vrclog/vrclog-go/pkg/vrclog/pattern"
)

func TestNewRegexParser_Valid(t *testing.T) {
	pf, err := pattern.Load("testdata/valid.yaml")
	require.NoError(t, err)

	parser, err := pattern.NewRegexParser(pf)
	require.NoError(t, err)
	assert.NotNil(t, parser)
}

func TestNewRegexParser_InvalidRegex(t *testing.T) {
	pf, err := pattern.Load("testdata/invalid_regex.yaml")
	require.NoError(t, err) // Load succeeds (validation doesn't compile regex)

	_, err = pattern.NewRegexParser(pf)
	require.Error(t, err) // NewRegexParser fails on invalid regex
	var patErr *pattern.PatternError
	require.True(t, errors.As(err, &patErr))
	assert.Contains(t, err.Error(), "invalid regular expression")
}

func TestNewRegexParser_Nil(t *testing.T) {
	_, err := pattern.NewRegexParser(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil")
}

func TestNewRegexParserFromFile_Valid(t *testing.T) {
	parser, err := pattern.NewRegexParserFromFile("testdata/valid.yaml")
	require.NoError(t, err)
	assert.NotNil(t, parser)
}

func TestNewRegexParserFromFile_InvalidFile(t *testing.T) {
	_, err := pattern.NewRegexParserFromFile("testdata/nonexistent.yaml")
	require.Error(t, err)
}

func TestRegexParser_ParseLine_Match(t *testing.T) {
	pf := &pattern.PatternFile{
		Version: 1,
		Patterns: []pattern.Pattern{
			{
				ID:        "test",
				EventType: "test_event",
				Regex:     `Test Pattern: (?P<value>\w+)`,
			},
		},
	}
	parser, err := pattern.NewRegexParser(pf)
	require.NoError(t, err)

	line := "2024.01.15 23:59:59 Log - Test Pattern: ABC123"
	result, err := parser.ParseLine(context.Background(), line)
	require.NoError(t, err)
	assert.True(t, result.Matched)
	require.Len(t, result.Events, 1)

	ev := result.Events[0]
	assert.Equal(t, event.Type("test_event"), ev.Type)
	assert.NotNil(t, ev.Data)
	assert.Equal(t, "ABC123", ev.Data["value"])

	// Verify timestamp was extracted
	expectedTime := time.Date(2024, 1, 15, 23, 59, 59, 0, time.Local)
	assert.True(t, ev.Timestamp.Equal(expectedTime))
}

func TestRegexParser_ParseLine_NoMatch(t *testing.T) {
	pf := &pattern.PatternFile{
		Version: 1,
		Patterns: []pattern.Pattern{
			{
				ID:        "test",
				EventType: "test_event",
				Regex:     `Test Pattern: \w+`,
			},
		},
	}
	parser, err := pattern.NewRegexParser(pf)
	require.NoError(t, err)

	line := "This line does not match"
	result, err := parser.ParseLine(context.Background(), line)
	require.NoError(t, err)
	assert.False(t, result.Matched)
	assert.Empty(t, result.Events)
}

func TestRegexParser_ParseLine_MultiplePatterns(t *testing.T) {
	pf := &pattern.PatternFile{
		Version: 1,
		Patterns: []pattern.Pattern{
			{
				ID:        "pattern1",
				EventType: "event1",
				Regex:     `Pattern1: (?P<val1>\w+)`,
			},
			{
				ID:        "pattern2",
				EventType: "event2",
				Regex:     `Pattern2: (?P<val2>\w+)`,
			},
		},
	}
	parser, err := pattern.NewRegexParser(pf)
	require.NoError(t, err)

	// Line matches only pattern1
	line1 := "Pattern1: ABC"
	result1, err := parser.ParseLine(context.Background(), line1)
	require.NoError(t, err)
	assert.True(t, result1.Matched)
	require.Len(t, result1.Events, 1)
	assert.Equal(t, event.Type("event1"), result1.Events[0].Type)
	assert.Equal(t, "ABC", result1.Events[0].Data["val1"])

	// Line matches both patterns
	line2 := "Pattern1: XYZ and Pattern2: 123"
	result2, err := parser.ParseLine(context.Background(), line2)
	require.NoError(t, err)
	assert.True(t, result2.Matched)
	require.Len(t, result2.Events, 2)
	// Events should be in pattern definition order
	assert.Equal(t, event.Type("event1"), result2.Events[0].Type)
	assert.Equal(t, "XYZ", result2.Events[0].Data["val1"])
	assert.Equal(t, event.Type("event2"), result2.Events[1].Type)
	assert.Equal(t, "123", result2.Events[1].Data["val2"])
}

func TestRegexParser_ParseLine_NamedCaptureGroups(t *testing.T) {
	pf := &pattern.PatternFile{
		Version: 1,
		Patterns: []pattern.Pattern{
			{
				ID:        "multi_capture",
				EventType: "multi_capture",
				Regex:     `User: (?P<username>\w+), Score: (?P<score>\d+), Level: (?P<level>\d+)`,
			},
		},
	}
	parser, err := pattern.NewRegexParser(pf)
	require.NoError(t, err)

	line := "User: Alice, Score: 9999, Level: 42"
	result, err := parser.ParseLine(context.Background(), line)
	require.NoError(t, err)
	assert.True(t, result.Matched)
	require.Len(t, result.Events, 1)

	ev := result.Events[0]
	require.NotNil(t, ev.Data)
	assert.Equal(t, "Alice", ev.Data["username"])
	assert.Equal(t, "9999", ev.Data["score"])
	assert.Equal(t, "42", ev.Data["level"])
}

func TestRegexParser_ParseLine_NoCaptureGroups(t *testing.T) {
	pf := &pattern.PatternFile{
		Version: 1,
		Patterns: []pattern.Pattern{
			{
				ID:        "no_capture",
				EventType: "no_capture",
				Regex:     `Simple pattern without captures`,
			},
		},
	}
	parser, err := pattern.NewRegexParser(pf)
	require.NoError(t, err)

	line := "Simple pattern without captures"
	result, err := parser.ParseLine(context.Background(), line)
	require.NoError(t, err)
	assert.True(t, result.Matched)
	require.Len(t, result.Events, 1)

	ev := result.Events[0]
	// Data should be nil (not empty map) when there are no named capture groups
	assert.Nil(t, ev.Data)
}

func TestRegexParser_ParseLine_NoTimestamp(t *testing.T) {
	pf := &pattern.PatternFile{
		Version: 1,
		Patterns: []pattern.Pattern{
			{
				ID:        "test",
				EventType: "test_event",
				Regex:     `Test: (?P<value>\w+)`,
			},
		},
	}
	parser, err := pattern.NewRegexParser(pf)
	require.NoError(t, err)

	// Line without VRChat timestamp format
	line := "Test: ABC"
	result, err := parser.ParseLine(context.Background(), line)
	require.NoError(t, err)
	assert.True(t, result.Matched)
	require.Len(t, result.Events, 1)

	ev := result.Events[0]
	// Timestamp should be zero value when not present
	assert.True(t, ev.Timestamp.IsZero())
}

func TestRegexParser_ParseLine_InvalidTimestamp(t *testing.T) {
	pf := &pattern.PatternFile{
		Version: 1,
		Patterns: []pattern.Pattern{
			{
				ID:        "test",
				EventType: "test_event",
				Regex:     `Test: \w+`,
			},
		},
	}
	parser, err := pattern.NewRegexParser(pf)
	require.NoError(t, err)

	// Line with invalid timestamp format
	line := "2024-99-99 99:99:99 Test: ABC"
	result, err := parser.ParseLine(context.Background(), line)
	require.NoError(t, err)
	assert.True(t, result.Matched)
	require.Len(t, result.Events, 1)

	ev := result.Events[0]
	// Invalid timestamp should result in zero value
	assert.True(t, ev.Timestamp.IsZero())
}

func TestRegexParser_ParseLine_TimestampWithTab(t *testing.T) {
	pf := &pattern.PatternFile{
		Version: 1,
		Patterns: []pattern.Pattern{
			{
				ID:        "test",
				EventType: "test_event",
				Regex:     `Test: \w+`,
			},
		},
	}
	parser, err := pattern.NewRegexParser(pf)
	require.NoError(t, err)

	// Line with tab after timestamp
	line := "2024.01.15 23:59:59\tTest: ABC"
	result, err := parser.ParseLine(context.Background(), line)
	require.NoError(t, err)
	assert.True(t, result.Matched)
	require.Len(t, result.Events, 1)

	ev := result.Events[0]
	// Timestamp should be extracted even with tab separator
	expectedTime := time.Date(2024, 1, 15, 23, 59, 59, 0, time.Local)
	assert.True(t, ev.Timestamp.Equal(expectedTime))
}

func TestRegexParser_ParserInterface(t *testing.T) {
	pf := &pattern.PatternFile{
		Version: 1,
		Patterns: []pattern.Pattern{
			{
				ID:        "test",
				EventType: "test_event",
				Regex:     `test`,
			},
		},
	}
	parser, err := pattern.NewRegexParser(pf)
	require.NoError(t, err)

	// Verify that RegexParser implements vrclog.Parser interface
	var _ vrclog.Parser = parser

	// Test with context
	ctx := context.Background()
	result, err := parser.ParseLine(ctx, "test")
	require.NoError(t, err)
	assert.True(t, result.Matched)
}

func TestRegexParser_RealWorldPokerExample(t *testing.T) {
	parser, err := pattern.NewRegexParserFromFile("testdata/valid.yaml")
	require.NoError(t, err)

	tests := []struct {
		name          string
		line          string
		wantMatched   bool
		wantEventType event.Type
		wantDataKeys  []string
	}{
		{
			name:          "hole_cards",
			line:          "2024.01.15 23:59:59 Log - [Seat]: Draw Local Hole Cards: AH, KD",
			wantMatched:   true,
			wantEventType: "poker_hole_cards",
			wantDataKeys:  []string{"card1", "card2"},
		},
		{
			name:          "winner",
			line:          "2024.01.15 23:59:59 Log - [PotManager]: Round complete, player 3 won 500",
			wantMatched:   true,
			wantEventType: "poker_winner",
			wantDataKeys:  []string{"seat_id", "amount"},
		},
		{
			name:        "no_match",
			line:        "2024.01.15 23:59:59 Log - Some other event",
			wantMatched: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.ParseLine(context.Background(), tt.line)
			require.NoError(t, err)
			assert.Equal(t, tt.wantMatched, result.Matched)

			if tt.wantMatched {
				require.Len(t, result.Events, 1)
				ev := result.Events[0]
				assert.Equal(t, tt.wantEventType, ev.Type)

				if len(tt.wantDataKeys) > 0 {
					require.NotNil(t, ev.Data)
					for _, key := range tt.wantDataKeys {
						assert.Contains(t, ev.Data, key)
					}
				}

				// Verify timestamp
				expectedTime := time.Date(2024, 1, 15, 23, 59, 59, 0, time.Local)
				assert.True(t, ev.Timestamp.Equal(expectedTime))
			}
		})
	}
}

func TestRegexParser_MixedCaptureGroups(t *testing.T) {
	// Test pattern with both unnamed (\d+) and named (?P<name>\w+) capture groups.
	// This test ensures the fix for H1 bug: correct indexing when unnamed and named groups are mixed.
	pf := &pattern.PatternFile{
		Version: 1,
		Patterns: []pattern.Pattern{
			{
				ID:        "mixed_groups",
				EventType: "mixed_test",
				Regex:     `foo (\d+) bar (?P<name>\w+) baz (?P<value>\d+)`,
			},
		},
	}
	parser, err := pattern.NewRegexParser(pf)
	require.NoError(t, err)

	line := "foo 123 bar Alice baz 456"
	result, err := parser.ParseLine(context.Background(), line)
	require.NoError(t, err)
	assert.True(t, result.Matched)
	require.Len(t, result.Events, 1)

	ev := result.Events[0]
	require.NotNil(t, ev.Data)
	// Should only contain named groups, not the unnamed (\d+)
	assert.Equal(t, 2, len(ev.Data))
	assert.Equal(t, "Alice", ev.Data["name"])
	assert.Equal(t, "456", ev.Data["value"])
	// The unnamed group (123) should NOT be in Data
	assert.NotContains(t, ev.Data, "")
}

func TestNewRegexParser_CallsValidate(t *testing.T) {
	// Test that NewRegexParser enforces validation
	// even when passed a programmatically-created PatternFile
	pf := &pattern.PatternFile{
		Version: 1,
		Patterns: []pattern.Pattern{
			{
				ID:        "test",
				EventType: "test",
				Regex:     strings.Repeat("a", pattern.MaxPatternLength+1), // Over limit
			},
		},
	}

	_, err := pattern.NewRegexParser(pf)
	require.Error(t, err)
	var patErr *pattern.PatternError
	require.True(t, errors.As(err, &patErr))
	assert.Contains(t, err.Error(), "pattern too long")
}

func TestPatternError_Unwrap(t *testing.T) {
	// Test that regex compile errors can be unwrapped from PatternError
	pf := &pattern.PatternFile{
		Version: 1,
		Patterns: []pattern.Pattern{
			{
				ID:        "broken",
				EventType: "test",
				Regex:     "[invalid",
			},
		},
	}

	_, err := pattern.NewRegexParser(pf)
	require.Error(t, err)

	var patErr *pattern.PatternError
	require.True(t, errors.As(err, &patErr))
	assert.NotNil(t, patErr.Cause, "PatternError should have a Cause")

	// Test that Unwrap works
	unwrapped := errors.Unwrap(err)
	assert.NotNil(t, unwrapped, "errors.Unwrap should return the cause")
}
