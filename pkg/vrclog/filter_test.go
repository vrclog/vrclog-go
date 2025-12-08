package vrclog

import "testing"

func TestCompiledFilter_Allows(t *testing.T) {
	tests := []struct {
		name    string
		include []EventType
		exclude []EventType
		event   EventType
		want    bool
	}{
		{
			name:  "nil filter allows all",
			event: EventPlayerJoin,
			want:  true,
		},
		{
			name:    "include only specified types",
			include: []EventType{EventPlayerJoin},
			event:   EventPlayerJoin,
			want:    true,
		},
		{
			name:    "include rejects non-specified types",
			include: []EventType{EventPlayerJoin},
			event:   EventPlayerLeft,
			want:    false,
		},
		{
			name:    "exclude specified types",
			exclude: []EventType{EventPlayerLeft},
			event:   EventPlayerLeft,
			want:    false,
		},
		{
			name:    "exclude allows non-specified types",
			exclude: []EventType{EventPlayerLeft},
			event:   EventPlayerJoin,
			want:    true,
		},
		{
			name:    "exclude takes precedence over include",
			include: []EventType{EventPlayerJoin, EventPlayerLeft},
			exclude: []EventType{EventPlayerLeft},
			event:   EventPlayerLeft,
			want:    false,
		},
		{
			name:    "include and exclude - allowed type",
			include: []EventType{EventPlayerJoin, EventPlayerLeft},
			exclude: []EventType{EventPlayerLeft},
			event:   EventPlayerJoin,
			want:    true,
		},
		{
			name:    "empty include allows all",
			include: []EventType{},
			event:   EventPlayerJoin,
			want:    true,
		},
		{
			name:    "empty exclude allows all",
			exclude: []EventType{},
			event:   EventPlayerJoin,
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newCompiledFilter(tt.include, tt.exclude)
			got := f.Allows(tt.event)
			if got != tt.want {
				t.Errorf("Allows(%v) = %v, want %v", tt.event, got, tt.want)
			}
		})
	}
}

func TestNewCompiledFilter_NilForEmpty(t *testing.T) {
	f := newCompiledFilter(nil, nil)
	if f != nil {
		t.Error("newCompiledFilter(nil, nil) should return nil")
	}

	f = newCompiledFilter([]EventType{}, []EventType{})
	if f != nil {
		t.Error("newCompiledFilter([], []) should return nil")
	}
}
