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
		{
			name: "custom_event_with_data",
			event: vrclog.Event{
				Type:      "poker_hole_cards",
				Timestamp: time.Date(2024, 1, 15, 12, 30, 45, 0, time.UTC),
				Data:      map[string]string{"card1": "Ah", "card2": "Kh"},
			},
			contains: "* poker_hole_cards: card1=Ah card2=Kh",
		},
		{
			name: "custom_event_without_data",
			event: vrclog.Event{
				Type:      "custom_notification",
				Timestamp: time.Date(2024, 1, 15, 12, 30, 45, 0, time.UTC),
			},
			contains: "* custom_notification",
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
		{
			name:   "pretty_custom_event_with_data",
			format: "pretty",
			event: vrclog.Event{
				Type:      "poker_hole_cards",
				Timestamp: fixedTime,
				Data:      map[string]string{"card1": "Ah", "card2": "Kh"},
			},
		},
		{
			name:   "pretty_custom_event_without_data",
			format: "pretty",
			event: vrclog.Event{
				Type:      "custom_notification",
				Timestamp: fixedTime,
			},
		},
		{
			name:   "jsonl_custom_event_with_data",
			format: "jsonl",
			event: vrclog.Event{
				Type:      "poker_hole_cards",
				Timestamp: fixedTime,
				Data:      map[string]string{"card1": "Ah", "card2": "Kh"},
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

func TestQuoteIfNeeded(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"simple", "hello", "hello"},
		{"empty", "", `""`},
		{"with_space", "hello world", `"hello world"`},
		{"with_equals", "a=b", `"a=b"`},
		{"with_quote", `say "hi"`, `"say \"hi\""`},
		{"with_backslash", `path\to`, `"path\\to"`},
		{"with_newline", "line1\nline2", `"line1\nline2"`},
		{"with_tab", "col1\tcol2", `"col1\tcol2"`},
		{"with_carriage_return", "a\rb", `"a\rb"`},
		{"with_null", "a\x00b", `"a\x00b"`},
		{"with_del", "a\x7fb", `"a\x7fb"`},
		{"unicode", "テスト", "テスト"},
		{"unicode_with_space", "日本語 テスト", `"日本語 テスト"`},
		{"emoji", "🎮", "🎮"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := quoteIfNeeded(tt.input)
			if got != tt.want {
				t.Errorf("quoteIfNeeded(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFormatData(t *testing.T) {
	tests := []struct {
		name  string
		input map[string]string
		want  string
	}{
		{"nil", nil, ""},
		{"empty", map[string]string{}, ""},
		{"single", map[string]string{"key": "value"}, "key=value"},
		{"multiple_sorted", map[string]string{"b": "2", "a": "1", "c": "3"}, "a=1 b=2 c=3"},
		{"with_spaces", map[string]string{"msg": "hello world"}, `msg="hello world"`},
		{"mixed", map[string]string{"name": "Bob", "msg": "hi there"}, `msg="hi there" name=Bob`},
		{"key_with_space", map[string]string{"key name": "value"}, `"key name"=value`},
		{"key_with_equals", map[string]string{"key=name": "value"}, `"key=name"=value`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatData(tt.input)
			if got != tt.want {
				t.Errorf("formatData(%v) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
