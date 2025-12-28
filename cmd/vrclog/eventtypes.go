package main

import (
	"fmt"
	"strings"

	"github.com/vrclog/vrclog-go/pkg/vrclog"
	"github.com/vrclog/vrclog-go/pkg/vrclog/event"
)

// ValidEventTypeNames returns a sorted list of valid event type names.
// Delegates to event.TypeNames() as the single source of truth.
func ValidEventTypeNames() []string {
	return event.TypeNames()
}

// NormalizeEventTypes converts CLI string values to vrclog.EventType slice.
// It handles case-insensitivity, whitespace trimming, and duplicate removal.
// Delegates to event.ParseType() for parsing.
func NormalizeEventTypes(values []string) ([]vrclog.EventType, error) {
	if len(values) == 0 {
		return nil, nil
	}

	result := make([]vrclog.EventType, 0, len(values))
	seen := make(map[vrclog.EventType]struct{})

	for _, raw := range values {
		t, ok := event.ParseType(raw)
		if !ok {
			return nil, fmt.Errorf("unknown event type %q (valid: %s)", raw, strings.Join(ValidEventTypeNames(), ", "))
		}

		if _, dup := seen[t]; dup {
			continue // ignore duplicates silently
		}
		seen[t] = struct{}{}
		result = append(result, t)
	}

	return result, nil
}

// RejectOverlap returns an error if any event type is in both includes and excludes.
func RejectOverlap(includes, excludes []vrclog.EventType) error {
	ex := make(map[vrclog.EventType]struct{}, len(excludes))
	for _, t := range excludes {
		ex[t] = struct{}{}
	}
	for _, t := range includes {
		if _, ok := ex[t]; ok {
			return fmt.Errorf("event type %q cannot be both included and excluded", t)
		}
	}
	return nil
}
