// Package parser provides VRChat log line parsing functionality.
package parser

import (
	"fmt"
	"strings"
	"time"

	"github.com/vrclog/vrclog-go/pkg/vrclog/event"
)

// Parse parses a VRChat log line into an Event.
//
// Returns:
//   - (*Event, nil): Successfully parsed
//   - (nil, nil): Not a recognized event pattern
//   - (nil, error): Malformed line
func Parse(line string) (*event.Event, error) {
	// Trim trailing CR for Windows CRLF compatibility
	line = strings.TrimRight(line, "\r")

	// Sanitize invalid UTF-8 sequences to prevent issues in JSON output
	line = strings.ToValidUTF8(line, "\uFFFD")

	// Quick exclusion check
	for _, pattern := range exclusionPatterns {
		if strings.Contains(line, pattern) {
			return nil, nil
		}
	}

	// Extract timestamp
	ts, err := parseTimestamp(line)
	if err != nil {
		// No timestamp means not a standard log line
		return nil, nil
	}

	// Try each event pattern
	if ev := parsePlayerJoin(line, ts); ev != nil {
		return ev, nil
	}
	if ev := parsePlayerLeft(line, ts); ev != nil {
		return ev, nil
	}
	if ev := parseWorldJoin(line, ts); ev != nil {
		return ev, nil
	}

	// Not a recognized event
	return nil, nil
}

// timestampLen is the length of VRChat log timestamps ("2024.01.15 23:59:59")
const timestampLen = 19

func parseTimestamp(line string) (time.Time, error) {
	// VRChat log timestamps are always 19 characters at the start
	// Format: "2024.01.15 23:59:59"
	if len(line) < timestampLen {
		return time.Time{}, fmt.Errorf("line too short for timestamp")
	}

	// Quick validation: check for expected format markers
	ts := line[:timestampLen]
	if ts[4] != '.' || ts[7] != '.' || ts[10] != ' ' || ts[13] != ':' || ts[16] != ':' {
		return time.Time{}, fmt.Errorf("invalid timestamp format")
	}

	return time.ParseInLocation(timestampLayout, ts, time.Local)
}

func parsePlayerJoin(line string, ts time.Time) *event.Event {
	match := playerJoinPattern.FindStringSubmatch(line)
	if match == nil {
		return nil
	}

	ev := &event.Event{
		Type:       event.PlayerJoin,
		Timestamp:  ts,
		PlayerName: strings.TrimSpace(match[1]),
	}

	if len(match) > 2 && match[2] != "" {
		ev.PlayerID = match[2]
	}

	return ev
}

func parsePlayerLeft(line string, ts time.Time) *event.Event {
	match := playerLeftPattern.FindStringSubmatch(line)
	if match == nil {
		return nil
	}

	return &event.Event{
		Type:       event.PlayerLeft,
		Timestamp:  ts,
		PlayerName: strings.TrimSpace(match[1]),
	}
}

func parseWorldJoin(line string, ts time.Time) *event.Event {
	// Try "Entering Room" first (has world name)
	if match := enteringRoomPattern.FindStringSubmatch(line); match != nil {
		return &event.Event{
			Type:      event.WorldJoin,
			Timestamp: ts,
			WorldName: strings.TrimSpace(match[1]),
		}
	}

	// Try "Joining" (has world ID and instance ID)
	if match := joiningPattern.FindStringSubmatch(line); match != nil {
		return &event.Event{
			Type:       event.WorldJoin,
			Timestamp:  ts,
			WorldID:    match[1],
			InstanceID: match[2],
		}
	}

	return nil
}
