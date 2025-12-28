package event

import "testing"

func TestParseType(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Type
		wantOK  bool
	}{
		// Valid types - exact match
		{"world_join exact", "world_join", WorldJoin, true},
		{"player_join exact", "player_join", PlayerJoin, true},
		{"player_left exact", "player_left", PlayerLeft, true},

		// Case-insensitive
		{"uppercase WORLD_JOIN", "WORLD_JOIN", WorldJoin, true},
		{"mixed case World_Join", "World_Join", WorldJoin, true},
		{"uppercase PLAYER_JOIN", "PLAYER_JOIN", PlayerJoin, true},

		// Whitespace handling
		{"leading space", " world_join", WorldJoin, true},
		{"trailing space", "world_join ", WorldJoin, true},
		{"both spaces", " world_join ", WorldJoin, true},
		{"tab", "\tworld_join\t", WorldJoin, true},

		// Invalid types
		{"unknown type", "unknown", "", false},
		{"empty string", "", "", false},
		{"only spaces", "   ", "", false},
		{"internal space", "world join", "", false},
		{"typo", "world_jion", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ParseType(tt.input)
			if ok != tt.wantOK {
				t.Errorf("ParseType(%q) ok = %v, want %v", tt.input, ok, tt.wantOK)
			}
			if got != tt.want {
				t.Errorf("ParseType(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseType_RoundTrip(t *testing.T) {
	// All values from TypeNames() should parse successfully
	for _, name := range TypeNames() {
		t.Run(name, func(t *testing.T) {
			got, ok := ParseType(name)
			if !ok {
				t.Errorf("ParseType(%q) returned false, expected true", name)
			}
			if string(got) != name {
				t.Errorf("ParseType(%q) = %q, expected %q", name, got, name)
			}
		})
	}
}

func TestTypeNames_NoDuplicates(t *testing.T) {
	names := TypeNames()
	seen := make(map[string]bool)
	for _, name := range names {
		if seen[name] {
			t.Errorf("TypeNames() contains duplicate: %q", name)
		}
		seen[name] = true
	}
}

func TestTypeNames_Sorted(t *testing.T) {
	names := TypeNames()
	for i := 1; i < len(names); i++ {
		if names[i-1] > names[i] {
			t.Errorf("TypeNames() not sorted: %q > %q", names[i-1], names[i])
		}
	}
}
