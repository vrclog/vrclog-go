package pattern

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/vrclog/vrclog-go/pkg/vrclog"
	"github.com/vrclog/vrclog-go/pkg/vrclog/event"
)

// timestampLayout is the Go time layout for VRChat log timestamps.
// VRChat format: "2024.01.15 23:59:59"
const timestampLayout = "2006.01.02 15:04:05"

// RegexParser is a Parser implementation that matches log lines using
// user-defined regular expression patterns from a YAML file.
//
// Named capture groups (?P<name>...) in patterns are extracted into the
// Event.Data field. The parser checks all patterns and can generate multiple
// events from a single line if multiple patterns match.
//
// RegexParser is safe for concurrent use by multiple goroutines.
type RegexParser struct {
	patterns []*compiledPattern
}

// compiledPattern represents a single compiled pattern with its metadata.
type compiledPattern struct {
	id         string
	eventType  event.Type
	regex      *regexp.Regexp
	groupNames []string // Named capture group names (excluding empty string at index 0)
	groupIndex []int    // Indices of named groups in matches array (parallel to groupNames)
}

// NewRegexParser creates a RegexParser from a PatternFile.
// This function compiles all regular expressions and validates their syntax.
// Returns an error if any pattern has invalid regex syntax.
//
// Example:
//
//	pf, err := pattern.Load("patterns.yaml")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	parser, err := pattern.NewRegexParser(pf)
//	if err != nil {
//	    log.Fatal(err)
//	}
func NewRegexParser(pf *PatternFile) (*RegexParser, error) {
	if pf == nil {
		return nil, fmt.Errorf("pattern file is nil")
	}

	// Validate the pattern file to enforce security constraints
	// (max pattern length, required fields, etc.)
	if err := pf.Validate(); err != nil {
		return nil, err
	}

	patterns := make([]*compiledPattern, 0, len(pf.Patterns))
	for i, p := range pf.Patterns {
		re, err := regexp.Compile(p.Regex)
		if err != nil {
			return nil, &PatternError{
				Index:   i,
				ID:      p.ID,
				Field:   "regex",
				Message: fmt.Sprintf("invalid regular expression: %v", err),
				Cause:   err,
			}
		}

		// Extract named capture group names and their indices.
		// Note: SubexpNames()[0] is always an empty string (the whole match),
		// so we skip it when collecting named groups.
		// We store both the names and indices to avoid calling SubexpNames() on every match.
		allNames := re.SubexpNames()
		groupNames := make([]string, 0, len(allNames)-1)
		groupIndex := make([]int, 0, len(allNames)-1)
		for j := 1; j < len(allNames); j++ {
			if allNames[j] != "" {
				groupNames = append(groupNames, allNames[j])
				groupIndex = append(groupIndex, j)
			}
		}

		patterns = append(patterns, &compiledPattern{
			id:         p.ID,
			eventType:  event.Type(p.EventType),
			regex:      re,
			groupNames: groupNames,
			groupIndex: groupIndex,
		})
	}

	return &RegexParser{patterns: patterns}, nil
}

// NewRegexParserFromFile is a convenience function that loads a pattern file
// and creates a RegexParser in one step.
//
// Example:
//
//	parser, err := pattern.NewRegexParserFromFile("patterns.yaml")
//	if err != nil {
//	    log.Fatal(err)
//	}
func NewRegexParserFromFile(path string) (*RegexParser, error) {
	pf, err := Load(path)
	if err != nil {
		return nil, err
	}
	return NewRegexParser(pf)
}

// ParseLine implements the vrclog.Parser interface.
// It matches the line against all patterns and returns all matching events.
// Events are returned in the order patterns were defined in the file.
//
// Respects context cancellation for large pattern sets.
func (p *RegexParser) ParseLine(ctx context.Context, line string) (vrclog.ParseResult, error) {
	var allEvents []event.Event

	// Extract timestamp from line (VRChat format: "2024.01.15 23:59:59 ...")
	timestamp, hasTimestamp := extractTimestamp(line)

	// Check all patterns (similar to ChainAll mode)
	for _, cp := range p.patterns {
		// Check for context cancellation when processing many patterns
		if err := ctx.Err(); err != nil {
			// Return partial results and cancellation error
			if len(allEvents) > 0 {
				return vrclog.ParseResult{Events: allEvents, Matched: true}, err
			}
			return vrclog.ParseResult{Matched: false}, err
		}

		matches := cp.regex.FindStringSubmatch(line)
		if matches == nil {
			continue
		}

		// Create event
		ev := event.Event{
			Type: cp.eventType,
		}

		// Set timestamp if available
		if hasTimestamp {
			ev.Timestamp = timestamp
		}

		// Extract named capture groups into Data field
		if len(cp.groupNames) > 0 {
			data := make(map[string]string, len(cp.groupNames))
			// Use pre-computed groupIndex to avoid calling SubexpNames() on every match.
			// This correctly handles patterns with mixed unnamed and named capture groups.
			for i, name := range cp.groupNames {
				idx := cp.groupIndex[i]
				if idx < len(matches) {
					data[name] = matches[idx]
				}
			}
			ev.Data = data
		}
		// If no named groups, leave Data as nil (not empty map)

		allEvents = append(allEvents, ev)
	}

	if len(allEvents) == 0 {
		return vrclog.ParseResult{Matched: false}, nil
	}

	return vrclog.ParseResult{
		Events:  allEvents,
		Matched: true,
	}, nil
}

// extractTimestamp attempts to extract a VRChat log timestamp from the beginning
// of a line. VRChat format: "2024.01.15 23:59:59 Log - ..."
//
// Returns (timestamp, true) if successfully parsed, or (zero, false) if the line
// doesn't have a valid timestamp.
func extractTimestamp(line string) (time.Time, bool) {
	if len(line) < 19 {
		return time.Time{}, false
	}

	tsStr := line[:19]
	// Use time.ParseInLocation to interpret the timestamp as local time,
	// matching VRChat's behavior. Using time.Parse would interpret as UTC.
	ts, err := time.ParseInLocation(timestampLayout, tsStr, time.Local)
	if err != nil {
		return time.Time{}, false
	}

	// Verify there's a space or tab after the timestamp
	if len(line) > 19 {
		c := line[19]
		if c != ' ' && c != '\t' {
			return time.Time{}, false
		}
	}

	return ts, true
}
