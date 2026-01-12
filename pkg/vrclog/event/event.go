// Package event defines the core Event type for VRChat log parsing.
//
// This package is separated from the main vrclog package to avoid import cycles
// between pkg/vrclog and internal/parser.
package event

import (
	"sort"
	"strings"
	"time"
)

// Type represents the type of VRChat log event.
type Type string

const (
	// WorldJoin indicates the user has joined a world/instance.
	WorldJoin Type = "world_join"

	// PlayerJoin indicates another player has joined the instance.
	PlayerJoin Type = "player_join"

	// PlayerLeft indicates another player has left the instance.
	PlayerLeft Type = "player_left"
)

// allTypes is the canonical list of all event types.
// Add new event types here when extending the parser.
var allTypes = []Type{WorldJoin, PlayerJoin, PlayerLeft}

// TypeNames returns a sorted list of all valid event type names.
// This is the single source of truth for event type enumeration.
func TypeNames() []string {
	names := make([]string, len(allTypes))
	for i, t := range allTypes {
		names[i] = string(t)
	}
	sort.Strings(names)
	return names
}

// typeByName maps lowercase string names to Type for efficient lookup.
// Built once from allTypes at package initialization.
var typeByName = func() map[string]Type {
	m := make(map[string]Type, len(allTypes))
	for _, t := range allTypes {
		m[string(t)] = t
	}
	return m
}()

// ParseType converts a string to Type if valid.
// It is case-insensitive and trims leading/trailing whitespace.
// Returns the type and true if found, zero value and false otherwise.
func ParseType(name string) (Type, bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	t, ok := typeByName[name]
	return t, ok
}

// Event represents a parsed VRChat log event.
type Event struct {
	// Type is the event type.
	Type Type `json:"type"`

	// Timestamp is when the event occurred (local time from log).
	Timestamp time.Time `json:"timestamp"`

	// PlayerName is the display name of the player (for player events).
	PlayerName string `json:"player_name,omitempty"`

	// PlayerID is the VRChat user ID (usr_xxx format, if available).
	PlayerID string `json:"player_id,omitempty"`

	// WorldID is the VRChat world ID (wrld_xxx format).
	WorldID string `json:"world_id,omitempty"`

	// WorldName is the display name of the world.
	WorldName string `json:"world_name,omitempty"`

	// InstanceID is the instance identifier (e.g., "12345~region(us)").
	InstanceID string `json:"instance_id,omitempty"`

	// RawLine is the original log line (only included if requested).
	RawLine string `json:"raw_line,omitempty"`

	// Data contains additional key-value pairs extracted by custom parsers.
	// For example, a regex parser with named capture groups (?P<key>...) will
	// populate this map with the captured values.
	// This field is nil when no custom data is available.
	Data map[string]string `json:"data,omitempty"`
}
