package main

import (
	"strings"
	"testing"
)

func TestValidFormats(t *testing.T) {
	tests := []struct {
		format string
		valid  bool
	}{
		{"jsonl", true},
		{"pretty", true},
		{"json", false},
		{"xml", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			got := ValidFormats[tt.format]
			if got != tt.valid {
				t.Errorf("ValidFormats[%q] = %v, want %v", tt.format, got, tt.valid)
			}
		})
	}
}

func TestRunTailInvalidEventType(t *testing.T) {
	// Save and restore original values
	origInclude := tailIncludeTypes
	origExclude := tailExcludeTypes
	origFormat := format
	defer func() {
		tailIncludeTypes = origInclude
		tailExcludeTypes = origExclude
		format = origFormat
	}()

	// Set up test conditions
	format = "jsonl"
	tailIncludeTypes = []string{"invalid_type"}
	tailExcludeTypes = nil

	err := runTail(tailCmd, nil)
	if err == nil {
		t.Error("expected error for invalid event type, got nil")
		return
	}
	if !strings.Contains(err.Error(), "unknown event type") {
		t.Errorf("expected 'unknown event type' error, got: %v", err)
	}
}

func TestRunTailOverlapEventTypes(t *testing.T) {
	// Save and restore original values
	origInclude := tailIncludeTypes
	origExclude := tailExcludeTypes
	origFormat := format
	defer func() {
		tailIncludeTypes = origInclude
		tailExcludeTypes = origExclude
		format = origFormat
	}()

	// Set up test conditions
	format = "jsonl"
	tailIncludeTypes = []string{"player_join"}
	tailExcludeTypes = []string{"player_join"}

	err := runTail(tailCmd, nil)
	if err == nil {
		t.Error("expected error for overlapping event types, got nil")
		return
	}
	if !strings.Contains(err.Error(), "cannot be both included and excluded") {
		t.Errorf("expected overlap error, got: %v", err)
	}
}
