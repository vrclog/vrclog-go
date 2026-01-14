package parser

import (
	"testing"
	"time"

	"github.com/vrclog/vrclog-go/pkg/vrclog/event"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *event.Event
		wantErr bool
	}{
		// Player join events
		{
			name:  "player join without ID",
			input: "2024.01.15 23:59:59 Log        -  [Behaviour] OnPlayerJoined TestUser",
			want: &event.Event{
				Type:       event.PlayerJoin,
				Timestamp:  mustParseTime("2024.01.15 23:59:59"),
				PlayerName: "TestUser",
			},
		},
		{
			name:  "player join with ID",
			input: "2024.01.15 23:59:59 Log        -  [Behaviour] OnPlayerJoined TestUser (usr_12345678-1234-1234-1234-123456789abc)",
			want: &event.Event{
				Type:       event.PlayerJoin,
				Timestamp:  mustParseTime("2024.01.15 23:59:59"),
				PlayerName: "TestUser",
				PlayerID:   "usr_12345678-1234-1234-1234-123456789abc",
			},
		},
		{
			name:  "player join with spaces in name",
			input: "2024.01.15 23:59:59 Log        -  [Behaviour] OnPlayerJoined Test User Name",
			want: &event.Event{
				Type:       event.PlayerJoin,
				Timestamp:  mustParseTime("2024.01.15 23:59:59"),
				PlayerName: "Test User Name",
			},
		},
		{
			name:  "player join with japanese name",
			input: "2024.01.15 23:59:59 Log        -  [Behaviour] OnPlayerJoined テストユーザー",
			want: &event.Event{
				Type:       event.PlayerJoin,
				Timestamp:  mustParseTime("2024.01.15 23:59:59"),
				PlayerName: "テストユーザー",
			},
		},

		// Player left events
		{
			name:  "player left",
			input: "2024.01.15 23:59:59 Log        -  [Behaviour] OnPlayerLeft TestUser",
			want: &event.Event{
				Type:       event.PlayerLeft,
				Timestamp:  mustParseTime("2024.01.15 23:59:59"),
				PlayerName: "TestUser",
			},
		},
		{
			name:  "player left with special char name",
			input: "2024.01.15 23:59:59 Log        -  [Behaviour] OnPlayerLeft (Special) Name",
			want: &event.Event{
				Type:       event.PlayerLeft,
				Timestamp:  mustParseTime("2024.01.15 23:59:59"),
				PlayerName: "(Special) Name",
			},
		},

		// World join events
		{
			name:  "entering room",
			input: "2024.01.15 23:59:59 Log        -  [Behaviour] Entering Room: Test World",
			want: &event.Event{
				Type:      event.WorldJoin,
				Timestamp: mustParseTime("2024.01.15 23:59:59"),
				WorldName: "Test World",
			},
		},
		{
			name:  "entering room with special chars",
			input: "2024.01.15 23:59:59 Log        -  [Behaviour] Entering Room: Test [World] (v1.0)",
			want: &event.Event{
				Type:      event.WorldJoin,
				Timestamp: mustParseTime("2024.01.15 23:59:59"),
				WorldName: "Test [World] (v1.0)",
			},
		},
		{
			name:  "joining world with instance",
			input: "2024.01.15 23:59:59 Log        -  [Behaviour] Joining wrld_12345678-1234-1234-1234-123456789abc:12345~region(us)",
			want: &event.Event{
				Type:       event.WorldJoin,
				Timestamp:  mustParseTime("2024.01.15 23:59:59"),
				WorldID:    "wrld_12345678-1234-1234-1234-123456789abc",
				InstanceID: "12345~region(us)",
			},
		},

		// Unrecognized lines (should return nil, nil)
		{
			name:    "unrecognized line",
			input:   "2024.01.15 23:59:59 Log        -  [Network] Connected",
			want:    nil,
			wantErr: false,
		},
		{
			name:    "empty line",
			input:   "",
			want:    nil,
			wantErr: false,
		},

		// Exclusion patterns
		{
			name:    "exclusion: OnPlayerLeftRoom",
			input:   "2024.01.15 23:59:59 Log        -  [Behaviour] OnPlayerLeftRoom",
			want:    nil,
			wantErr: false,
		},
		{
			name:    "exclusion: Joining or Creating",
			input:   "2024.01.15 23:59:59 Log        -  [Behaviour] Joining or Creating Room",
			want:    nil,
			wantErr: false,
		},

		// Windows CRLF compatibility
		{
			name:  "CRLF line ending",
			input: "2024.01.15 23:59:59 Log        -  [Behaviour] OnPlayerJoined TestUser\r",
			want: &event.Event{
				Type:       event.PlayerJoin,
				Timestamp:  mustParseTime("2024.01.15 23:59:59"),
				PlayerName: "TestUser",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.input)

			// Check error
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Check result
			if !eventEqual(got, tt.want) {
				t.Errorf("Parse() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestParse_Parallel(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  *event.Event
	}{
		{
			name:  "player join",
			input: "2024.01.15 23:59:59 Log        -  [Behaviour] OnPlayerJoined TestUser",
			want: &event.Event{
				Type:       event.PlayerJoin,
				Timestamp:  mustParseTime("2024.01.15 23:59:59"),
				PlayerName: "TestUser",
			},
		},
		{
			name:  "player left",
			input: "2024.01.15 23:59:59 Log        -  [Behaviour] OnPlayerLeft TestUser",
			want: &event.Event{
				Type:       event.PlayerLeft,
				Timestamp:  mustParseTime("2024.01.15 23:59:59"),
				PlayerName: "TestUser",
			},
		},
	}

	for _, tt := range tests {
		tt := tt // capture range variable
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, _ := Parse(tt.input)
			if !eventEqual(got, tt.want) {
				t.Errorf("Parse() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func FuzzParse(f *testing.F) {
	// Seed corpus
	f.Add("2024.01.15 23:59:59 Log        -  [Behaviour] OnPlayerJoined TestUser")
	f.Add("2024.01.15 23:59:59 Log        -  [Behaviour] OnPlayerLeft TestUser")
	f.Add("2024.01.15 23:59:59 Log        -  [Behaviour] Entering Room: Test World")
	f.Add("")
	f.Add("invalid line")
	f.Add("2024.01.15 23:59:59 Log        -  [Network] Connected")

	f.Fuzz(func(t *testing.T, line string) {
		// Parse should never panic, regardless of input
		ev, err := Parse(line)

		// Check invariants:
		// 1. If event is returned, it should have a valid Type
		if ev != nil {
			if ev.Type == "" {
				t.Error("Parse returned event with empty Type")
			}
			// Timestamp should not be zero value for valid events
			if ev.Timestamp.IsZero() {
				t.Error("Parse returned event with zero Timestamp")
			}
		}

		// 2. If error is returned, event should be nil
		if err != nil && ev != nil {
			t.Error("Parse returned both event and error (should be mutually exclusive)")
		}

		// 3. Data map, if present, should not have empty keys
		if ev != nil && ev.Data != nil {
			for key := range ev.Data {
				if key == "" {
					t.Error("Event has Data with empty key")
				}
			}
		}
	})
}

// Helper functions

func mustParseTime(s string) time.Time {
	t, err := time.ParseInLocation("2006.01.02 15:04:05", s, time.Local)
	if err != nil {
		panic(err)
	}
	return t
}

func eventEqual(a, b *event.Event) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Type == b.Type &&
		a.Timestamp.Equal(b.Timestamp) &&
		a.PlayerName == b.PlayerName &&
		a.PlayerID == b.PlayerID &&
		a.WorldID == b.WorldID &&
		a.WorldName == b.WorldName &&
		a.InstanceID == b.InstanceID
}
