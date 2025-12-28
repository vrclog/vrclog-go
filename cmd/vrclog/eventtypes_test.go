package main

import (
	"testing"

	"github.com/vrclog/vrclog-go/pkg/vrclog"
	"github.com/vrclog/vrclog-go/pkg/vrclog/event"
)

func TestValidEventTypeNames(t *testing.T) {
	names := ValidEventTypeNames()

	// Should delegate to event.TypeNames()
	eventNames := event.TypeNames()
	if len(names) != len(eventNames) {
		t.Errorf("ValidEventTypeNames() returned %d names, want %d", len(names), len(eventNames))
	}

	// Should be sorted
	for i := 1; i < len(names); i++ {
		if names[i-1] > names[i] {
			t.Errorf("ValidEventTypeNames() not sorted: %q > %q", names[i-1], names[i])
		}
	}

	// Should contain all expected names
	expected := []string{"player_join", "player_left", "world_join"}
	for _, name := range expected {
		found := false
		for _, n := range names {
			if n == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("ValidEventTypeNames() missing %q", name)
		}
	}
}

func TestNormalizeEventTypes(t *testing.T) {
	tests := []struct {
		name    string
		input   []string
		want    []vrclog.EventType
		wantErr bool
	}{
		{
			name:    "empty input",
			input:   nil,
			want:    nil,
			wantErr: false,
		},
		{
			name:    "single valid type",
			input:   []string{"player_join"},
			want:    []vrclog.EventType{vrclog.EventPlayerJoin},
			wantErr: false,
		},
		{
			name:    "multiple valid types",
			input:   []string{"player_join", "player_left", "world_join"},
			want:    []vrclog.EventType{vrclog.EventPlayerJoin, vrclog.EventPlayerLeft, vrclog.EventWorldJoin},
			wantErr: false,
		},
		{
			name:    "case insensitive",
			input:   []string{"PLAYER_JOIN", "Player_Left"},
			want:    []vrclog.EventType{vrclog.EventPlayerJoin, vrclog.EventPlayerLeft},
			wantErr: false,
		},
		{
			name:    "with whitespace",
			input:   []string{" player_join ", "  world_join  "},
			want:    []vrclog.EventType{vrclog.EventPlayerJoin, vrclog.EventWorldJoin},
			wantErr: false,
		},
		{
			name:    "duplicates removed",
			input:   []string{"player_join", "player_join", "player_left"},
			want:    []vrclog.EventType{vrclog.EventPlayerJoin, vrclog.EventPlayerLeft},
			wantErr: false,
		},
		{
			name:    "invalid type",
			input:   []string{"invalid_type"},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "mixed valid and invalid",
			input:   []string{"player_join", "invalid"},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "empty string error",
			input:   []string{""},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "empty between values error",
			input:   []string{"player_join", "", "player_left"},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "whitespace only error",
			input:   []string{"   "},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeEventTypes(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("NormalizeEventTypes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if len(got) != len(tt.want) {
					t.Errorf("NormalizeEventTypes() len = %v, want %v", len(got), len(tt.want))
					return
				}
				for i := range got {
					if got[i] != tt.want[i] {
						t.Errorf("NormalizeEventTypes()[%d] = %v, want %v", i, got[i], tt.want[i])
					}
				}
			}
		})
	}
}

func TestRejectOverlap(t *testing.T) {
	tests := []struct {
		name     string
		includes []vrclog.EventType
		excludes []vrclog.EventType
		wantErr  bool
	}{
		{
			name:     "no overlap",
			includes: []vrclog.EventType{vrclog.EventPlayerJoin},
			excludes: []vrclog.EventType{vrclog.EventPlayerLeft},
			wantErr:  false,
		},
		{
			name:     "empty lists",
			includes: nil,
			excludes: nil,
			wantErr:  false,
		},
		{
			name:     "overlap",
			includes: []vrclog.EventType{vrclog.EventPlayerJoin, vrclog.EventWorldJoin},
			excludes: []vrclog.EventType{vrclog.EventPlayerJoin},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := RejectOverlap(tt.includes, tt.excludes)
			if (err != nil) != tt.wantErr {
				t.Errorf("RejectOverlap() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
