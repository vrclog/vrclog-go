// Package pattern provides custom pattern matching for VRChat log files.
// It allows users to define their own event types via YAML configuration files
// with regular expression patterns.
package pattern

// PatternFile represents the structure of a YAML pattern file.
// Pattern files allow users to define custom log parsing rules using regular expressions.
//
// Example YAML file:
//
//	version: 1
//	patterns:
//	  - id: poker_hole_cards
//	    event_type: poker_hole_cards
//	    regex: '\[Seat\]: Draw Local Hole Cards: (?P<card1>\w+), (?P<card2>\w+)'
//	  - id: poker_winner
//	    event_type: poker_winner
//	    regex: '\[PotManager\]: .* player (?P<seat_id>\d+) won (?P<amount>\d+)'
type PatternFile struct {
	// Version is the pattern file format version. Currently only version 1 is supported.
	Version int `yaml:"version"`

	// Patterns is the list of pattern definitions.
	Patterns []Pattern `yaml:"patterns"`
}

// Pattern represents a single log pattern definition.
// Each pattern consists of a unique identifier, an event type, and a regular expression.
// The regex may contain named capture groups (?P<name>...) which will be extracted
// into the Event.Data field.
type Pattern struct {
	// ID is a unique identifier for this pattern (e.g., "poker_hole_cards").
	// IDs must be unique within a pattern file.
	ID string `yaml:"id"`

	// EventType is the value to use for the Event.Type field when this pattern matches.
	EventType string `yaml:"event_type"`

	// Regex is the regular expression pattern to match against log lines.
	// Named capture groups (?P<name>...) will be extracted into Event.Data.
	Regex string `yaml:"regex"`
}
