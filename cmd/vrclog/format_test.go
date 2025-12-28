package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vrclog/vrclog-go/pkg/vrclog"
)

var updateGolden = flag.Bool("update-golden", false, "update golden files")

func TestOutputJSON(t *testing.T) {
	event := vrclog.Event{
		Type:       vrclog.EventPlayerJoin,
		Timestamp:  time.Date(2024, 1, 15, 12, 30, 45, 0, time.UTC),
		PlayerName: "TestUser",
		PlayerID:   "usr_12345",
	}

	var buf bytes.Buffer
	err := OutputJSON(event, &buf)
	if err != nil {
		t.Fatalf("OutputJSON() error = %v", err)
	}

	// Verify it's valid JSON
	var decoded vrclog.Event
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("OutputJSON() produced invalid JSON: %v", err)
	}

	if decoded.PlayerName != "TestUser" {
		t.Errorf("decoded.PlayerName = %q, want %q", decoded.PlayerName, "TestUser")
	}
}

func TestOutputPretty(t *testing.T) {
	tests := []struct {
		name     string
		event    vrclog.Event
		contains string
	}{
		{
			name: "player_join",
			event: vrclog.Event{
				Type:       vrclog.EventPlayerJoin,
				Timestamp:  time.Date(2024, 1, 15, 12, 30, 45, 0, time.UTC),
				PlayerName: "TestUser",
			},
			contains: "+ TestUser joined",
		},
		{
			name: "player_left",
			event: vrclog.Event{
				Type:       vrclog.EventPlayerLeft,
				Timestamp:  time.Date(2024, 1, 15, 12, 30, 45, 0, time.UTC),
				PlayerName: "TestUser",
			},
			contains: "- TestUser left",
		},
		{
			name: "world_join_with_name",
			event: vrclog.Event{
				Type:      vrclog.EventWorldJoin,
				Timestamp: time.Date(2024, 1, 15, 12, 30, 45, 0, time.UTC),
				WorldName: "Test World",
			},
			contains: "> Joined world: Test World",
		},
		{
			name: "world_join_instance_only",
			event: vrclog.Event{
				Type:       vrclog.EventWorldJoin,
				Timestamp:  time.Date(2024, 1, 15, 12, 30, 45, 0, time.UTC),
				InstanceID: "12345~private",
			},
			contains: "> Joined instance: 12345~private",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := OutputPretty(tt.event, &buf)
			if err != nil {
				t.Fatalf("OutputPretty() error = %v", err)
			}

			if !strings.Contains(buf.String(), tt.contains) {
				t.Errorf("OutputPretty() = %q, want to contain %q", buf.String(), tt.contains)
			}
		})
	}
}

func TestOutputEvent(t *testing.T) {
	event := vrclog.Event{
		Type:       vrclog.EventPlayerJoin,
		Timestamp:  time.Date(2024, 1, 15, 12, 30, 45, 0, time.UTC),
		PlayerName: "TestUser",
	}

	tests := []struct {
		format    string
		wantErr   bool
		checkFunc func(string) bool
	}{
		{
			format:  "jsonl",
			wantErr: false,
			checkFunc: func(s string) bool {
				return strings.Contains(s, `"player_name":"TestUser"`)
			},
		},
		{
			format:  "pretty",
			wantErr: false,
			checkFunc: func(s string) bool {
				return strings.Contains(s, "+ TestUser joined")
			},
		},
		{
			format:  "unknown",
			wantErr: true,
			checkFunc: func(s string) bool {
				return true
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			var buf bytes.Buffer
			err := OutputEvent(tt.format, event, &buf)

			if (err != nil) != tt.wantErr {
				t.Errorf("OutputEvent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && !tt.checkFunc(buf.String()) {
				t.Errorf("OutputEvent() output check failed: %q", buf.String())
			}
		})
	}
}

// TestOutputEvent_Golden tests output formats using golden files.
// Run with -update-golden to update the golden files.
func TestOutputEvent_Golden(t *testing.T) {
	// Use fixed time in UTC for reproducibility
	fixedTime := time.Date(2024, 1, 15, 23, 59, 59, 0, time.UTC)

	tests := []struct {
		name   string
		format string
		event  vrclog.Event
	}{
		{
			name:   "pretty_player_join",
			format: "pretty",
			event: vrclog.Event{
				Type:       vrclog.EventPlayerJoin,
				Timestamp:  fixedTime,
				PlayerName: "TestUser",
			},
		},
		{
			name:   "pretty_player_left",
			format: "pretty",
			event: vrclog.Event{
				Type:       vrclog.EventPlayerLeft,
				Timestamp:  fixedTime,
				PlayerName: "TestUser",
			},
		},
		{
			name:   "pretty_world_join",
			format: "pretty",
			event: vrclog.Event{
				Type:      vrclog.EventWorldJoin,
				Timestamp: fixedTime,
				WorldName: "Test World",
			},
		},
		{
			name:   "jsonl_player_join",
			format: "jsonl",
			event: vrclog.Event{
				Type:       vrclog.EventPlayerJoin,
				Timestamp:  fixedTime,
				PlayerName: "TestUser",
			},
		},
	}

	// Support both flag and env var for updating golden files
	update := *updateGolden || os.Getenv("UPDATE_GOLDEN") != ""

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := OutputEvent(tt.format, tt.event, &buf); err != nil {
				t.Fatalf("OutputEvent() error = %v", err)
			}

			golden := filepath.Join("testdata", "golden", tt.name+".golden")

			if update {
				if err := os.MkdirAll(filepath.Dir(golden), 0755); err != nil {
					t.Fatalf("failed to create golden dir: %v", err)
				}
				if err := os.WriteFile(golden, buf.Bytes(), 0644); err != nil {
					t.Fatalf("failed to write golden file: %v", err)
				}
				t.Logf("updated golden file: %s", golden)
				return
			}

			expected, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("failed to read golden file %s: %v\nRun with -update-golden to create it", golden, err)
			}

			// Normalize line endings for cross-platform compatibility
			got := bytes.ReplaceAll(buf.Bytes(), []byte("\r\n"), []byte("\n"))
			want := bytes.ReplaceAll(expected, []byte("\r\n"), []byte("\n"))

			if !bytes.Equal(got, want) {
				t.Errorf("output mismatch for %s:\ngot:\n%s\nwant:\n%s", golden, got, want)
			}
		})
	}
}
