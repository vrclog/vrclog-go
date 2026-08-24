package vrclog

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAdapterEventValidateTag(t *testing.T) {
	tests := []struct {
		name    string
		tag     string
		wantErr bool
	}{
		{name: "simple", tag: "vrpoker.hole_cards.v1"},
		{name: "namespaced", tag: "org.example.vrpoker.table_state.v2"},
		{name: "hyphen and digits", tag: "org2.vr-poker.table_state2.v10"},
		{name: "maximum length", tag: "a." + strings.Repeat("b", maxAdapterTagBytes-5) + ".v1"},
		{name: "empty", tag: "", wantErr: true},
		{name: "uppercase", tag: "vrpoker.Hole_cards.v1", wantErr: true},
		{name: "missing version", tag: "vrpoker.hole_cards", wantErr: true},
		{name: "version zero", tag: "vrpoker.hole_cards.v0", wantErr: true},
		{name: "too long", tag: "a." + strings.Repeat("b", maxAdapterTagBytes-4) + ".v1", wantErr: true},
		{name: "invalid symbol", tag: "vrpoker.hole/cards.v1", wantErr: true},
		{name: "empty segment", tag: "vrpoker..hole_cards.v1", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := AdapterEvent{Tag: tt.tag, Data: json.RawMessage(`{}`)}
			err := event.validate()
			if tt.wantErr && err == nil {
				t.Error("validate() error = nil, want error")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("validate() error = %v, want nil", err)
			}
		})
	}
}

func TestAdapterEventValidateData(t *testing.T) {
	tests := []struct {
		name    string
		data    json.RawMessage
		wantErr bool
	}{
		{name: "object", data: json.RawMessage(`{"card1":"Jc","card2":"6d"}`)},
		{name: "empty object", data: json.RawMessage(`{}`)},
		{name: "missing", data: nil, wantErr: true},
		{name: "null", data: json.RawMessage(`null`), wantErr: true},
		{name: "array", data: json.RawMessage(`[]`), wantErr: true},
		{name: "number", data: json.RawMessage(`42`), wantErr: true},
		{name: "string", data: json.RawMessage(`"cards"`), wantErr: true},
		{name: "boolean", data: json.RawMessage(`true`), wantErr: true},
		{name: "invalid JSON", data: json.RawMessage(`{"card1":`), wantErr: true},
		{name: "bidi formatting", data: json.RawMessage("{\"card\":\"Jc\u202e\"}"), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := AdapterEvent{Tag: "vrpoker.hole_cards.v1", Data: tt.data}
			err := event.validate()
			if tt.wantErr && err == nil {
				t.Error("validate() error = nil, want error")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("validate() error = %v, want nil", err)
			}
		})
	}
}

func TestAdapterEventValidateDataSize(t *testing.T) {
	tests := []struct {
		name    string
		size    int
		wantErr bool
	}{
		{name: "64 KiB minus 1", size: maxAdapterDataBytes - 1},
		{name: "exactly 64 KiB", size: maxAdapterDataBytes},
		{name: "64 KiB plus 1", size: maxAdapterDataBytes + 1, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := json.RawMessage(`{"x":"` + strings.Repeat("a", tt.size-8) + `"}`)
			if len(data) != tt.size {
				t.Fatalf("test data size = %d, want %d", len(data), tt.size)
			}
			event := AdapterEvent{Tag: "example.payload.v1", Data: data}
			err := event.validate()
			if tt.wantErr && err == nil {
				t.Error("validate() error = nil, want error")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("validate() error = %v, want nil", err)
			}
		})
	}
}

func TestAdapterEventValidateDataDepth(t *testing.T) {
	tests := []struct {
		name    string
		depth   int
		wantErr bool
	}{
		{name: "depth 64", depth: maxAdapterDataDepth},
		{name: "depth 65", depth: maxAdapterDataDepth + 1, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := json.RawMessage(strings.Repeat(`{"x":`, tt.depth-1) + `{}` + strings.Repeat(`}`, tt.depth-1))
			event := AdapterEvent{Tag: "example.nested.v1", Data: data}
			err := event.validate()
			if tt.wantErr && err == nil {
				t.Error("validate() error = nil, want error")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("validate() error = %v, want nil", err)
			}
		})
	}
}

func TestAdapterEventKind(t *testing.T) {
	event := AdapterEvent{Tag: "vrpoker.hole_cards.v1", Data: json.RawMessage(`{}`)}
	if event.Kind() != EventKindAdapter {
		t.Errorf("Kind() = %q, want %q", event.Kind(), EventKindAdapter)
	}
}

// TestAdapterEventValidateDataEscapedBidi verifies that a JSON-escaped bidi
// override sequence (the six ASCII characters \, u, 2, 0, 2, e as they
// appear on the wire, not the raw UTF-8 bytes of U+202E) is rejected. A
// raw-byte scan of the JSON text would miss this, since none of those six
// characters are themselves control or bidi runes -- only the *decoded*
// string value contains the actual U+202E override character.
func TestAdapterEventValidateDataEscapedBidi(t *testing.T) {
	tests := []struct {
		name string
		data json.RawMessage
	}{
		{name: "escaped bidi in string value", data: json.RawMessage(`{"label":"safe\u202efake"}`)},
		{name: "escaped bidi in object key", data: json.RawMessage(`{"safe\u202efake":"value"}`)},
		{name: "escaped bidi inside nested array", data: json.RawMessage(`{"items":["ok","safe\u202efake"]}`)},
		{name: "escaped bidi inside nested object", data: json.RawMessage(`{"a":{"b":"safe\u202efake"}}`)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := AdapterEvent{Tag: "example.escaped.v1", Data: tt.data}
			if err := event.validate(); err == nil {
				t.Errorf("validate() error = nil, want error for %s", tt.data)
			}
		})
	}
}

func TestAdapterEventValidateDataTrailingGarbage(t *testing.T) {
	tests := []struct {
		name    string
		data    json.RawMessage
		wantErr bool
	}{
		{name: "trailing token", data: json.RawMessage(`{"a":1}extra`), wantErr: true},
		{name: "trailing JSON value", data: json.RawMessage(`{"a":1}{"b":2}`), wantErr: true},
		{name: "trailing whitespace only", data: json.RawMessage("{\"a\":1}   \n"), wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := AdapterEvent{Tag: "example.trailing.v1", Data: tt.data}
			err := event.validate()
			if tt.wantErr && err == nil {
				t.Error("validate() error = nil, want error")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("validate() error = %v, want nil", err)
			}
		})
	}
}
