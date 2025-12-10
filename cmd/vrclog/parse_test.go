package main

import (
	"strings"
	"testing"
	"time"
)

func TestParseTimeRange(t *testing.T) {
	tests := []struct {
		name      string
		since     string
		until     string
		wantSince time.Time
		wantUntil time.Time
		wantErr   bool
	}{
		{
			name:      "empty strings",
			since:     "",
			until:     "",
			wantSince: time.Time{},
			wantUntil: time.Time{},
			wantErr:   false,
		},
		{
			name:      "valid since only",
			since:     "2024-01-15T12:00:00Z",
			until:     "",
			wantSince: time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC),
			wantUntil: time.Time{},
			wantErr:   false,
		},
		{
			name:      "valid until only",
			since:     "",
			until:     "2024-01-16T00:00:00Z",
			wantSince: time.Time{},
			wantUntil: time.Date(2024, 1, 16, 0, 0, 0, 0, time.UTC),
			wantErr:   false,
		},
		{
			name:      "valid range",
			since:     "2024-01-15T12:00:00Z",
			until:     "2024-01-16T00:00:00Z",
			wantSince: time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC),
			wantUntil: time.Date(2024, 1, 16, 0, 0, 0, 0, time.UTC),
			wantErr:   false,
		},
		{
			name:    "invalid since format",
			since:   "2024-01-15",
			until:   "",
			wantErr: true,
		},
		{
			name:    "invalid until format",
			since:   "",
			until:   "not-a-date",
			wantErr: true,
		},
		{
			name:    "since after until",
			since:   "2024-01-16T00:00:00Z",
			until:   "2024-01-15T00:00:00Z",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSince, gotUntil, err := parseTimeRange(tt.since, tt.until)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseTimeRange() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if !gotSince.Equal(tt.wantSince) {
					t.Errorf("parseTimeRange() since = %v, want %v", gotSince, tt.wantSince)
				}
				if !gotUntil.Equal(tt.wantUntil) {
					t.Errorf("parseTimeRange() until = %v, want %v", gotUntil, tt.wantUntil)
				}
			}
		})
	}
}

func TestRunParseInvalidEventType(t *testing.T) {
	// Save and restore original values
	origInclude := parseIncludeTypes
	origExclude := parseExcludeTypes
	origFormat := parseFormat
	defer func() {
		parseIncludeTypes = origInclude
		parseExcludeTypes = origExclude
		parseFormat = origFormat
	}()

	// Set up test conditions
	parseFormat = "jsonl"
	parseIncludeTypes = []string{"invalid_type"}
	parseExcludeTypes = nil

	err := runParse(parseCmd, nil)
	if err == nil {
		t.Error("expected error for invalid event type, got nil")
		return
	}
	if !strings.Contains(err.Error(), "unknown event type") {
		t.Errorf("expected 'unknown event type' error, got: %v", err)
	}
}

func TestRunParseOverlapEventTypes(t *testing.T) {
	// Save and restore original values
	origInclude := parseIncludeTypes
	origExclude := parseExcludeTypes
	origFormat := parseFormat
	defer func() {
		parseIncludeTypes = origInclude
		parseExcludeTypes = origExclude
		parseFormat = origFormat
	}()

	// Set up test conditions
	parseFormat = "jsonl"
	parseIncludeTypes = []string{"player_join"}
	parseExcludeTypes = []string{"player_join"}

	err := runParse(parseCmd, nil)
	if err == nil {
		t.Error("expected error for overlapping event types, got nil")
		return
	}
	if !strings.Contains(err.Error(), "cannot be both included and excluded") {
		t.Errorf("expected overlap error, got: %v", err)
	}
}
