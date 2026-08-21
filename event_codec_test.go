package vrclog

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestEncodeDecodeRoundTrip(t *testing.T) {
	events := []Event{
		PlayerJoined{Player: Player{ID: "usr_123", DisplayName: "Alice"}},
		PlayerLeft{Player: Player{DisplayName: "Bob"}},
		WorldEnteringObserved{World: World{Name: "Cool World"}},
		WorldJoiningObserved{World: World{ID: "wrld_abc", InstanceID: "123~private(usr_xyz)~region(jp)"}},
		ResourceURLObserved{
			Resource: RemoteResource{URL: "https://example.com/video", Kind: ResourceKindVideo, Role: ResourceRoleResolverInput},
			Target:   &MediaTarget{Component: "vrchat", Backend: MediaBackendUnknown},
		},
		ResourceResolved{
			Input:  RemoteResource{URL: "https://youtube.com/watch?v=abc", Kind: ResourceKindVideo, Role: ResourceRoleResolverInput},
			Output: RemoteResource{URL: "https://cdn.example.com/video.mp4", Kind: ResourceKindVideo, Role: ResourceRoleResolved},
		},
		MediaErrorObserved{Stage: MediaStageResolve, Message: "resolution failed", Target: &MediaTarget{Component: "vrchat", Backend: MediaBackendUnknown}},
	}

	for _, ev := range events {
		kind, payload, err := EncodeEvent(ev)
		if err != nil {
			t.Fatalf("EncodeEvent(%T) error: %v", ev, err)
		}
		if kind != ev.Kind() {
			t.Fatalf("EncodeEvent(%T) kind = %q, want %q", ev, kind, ev.Kind())
		}

		decoded, err := DecodeEvent(kind, payload)
		if err != nil {
			t.Fatalf("DecodeEvent(%q) error: %v", kind, err)
		}
		if decoded.Kind() != ev.Kind() {
			t.Fatalf("decoded.Kind() = %q, want %q", decoded.Kind(), ev.Kind())
		}

		reKind, rePayload, err := EncodeEvent(decoded)
		if err != nil {
			t.Fatalf("re-EncodeEvent error: %v", err)
		}
		if reKind != kind {
			t.Fatalf("re-encoded kind = %q, want %q", reKind, kind)
		}
		if string(rePayload) != string(payload) {
			t.Fatalf("re-encoded payload differs:\n  got:  %s\n  want: %s", rePayload, payload)
		}
	}
}

func TestDecodeEventUnknownKind(t *testing.T) {
	_, err := DecodeEvent("nonexistent.kind", []byte("{}"))
	if !errors.Is(err, ErrUnknownEventKind) {
		t.Errorf("DecodeEvent unknown kind: got %v, want ErrUnknownEventKind", err)
	}
}

func TestDecodeEventKindTypeMismatch(t *testing.T) {
	playerPayload := []byte(`{"player":{"display_name":"Alice"}}`)
	_, err := DecodeEvent(EventKindWorldEnteringObserved, playerPayload)
	if err == nil {
		t.Error("DecodeEvent with mismatched kind/payload should fail")
	}
}

func TestDecodeEventToleratesUnknownFields(t *testing.T) {
	payload := []byte(`{"player":{"display_name":"Alice","unknown_field":"value"},"extra":true}`)
	ev, err := DecodeEvent(EventKindPlayerJoined, payload)
	if err != nil {
		t.Fatalf("DecodeEvent with unknown fields: %v", err)
	}
	pj, ok := ev.(PlayerJoined)
	if !ok {
		t.Fatalf("expected PlayerJoined, got %T", ev)
	}
	if pj.Player.DisplayName != "Alice" {
		t.Errorf("DisplayName = %q, want %q", pj.Player.DisplayName, "Alice")
	}
}

func TestDecodeEventMissingRequiredFields(t *testing.T) {
	payload := []byte(`{"player":{"display_name":""}}`)
	_, err := DecodeEvent(EventKindPlayerJoined, payload)
	if err == nil {
		t.Error("DecodeEvent with missing required DisplayName should fail validation")
	}
}

func TestEncodeEventNil(t *testing.T) {
	_, _, err := EncodeEvent(nil)
	if err == nil {
		t.Error("EncodeEvent(nil) should return error")
	}
}

func TestEncodeEventInvalidEventRejected(t *testing.T) {
	ev := PlayerJoined{Player: Player{DisplayName: ""}}
	_, _, err := EncodeEvent(ev)
	if err == nil {
		t.Error("EncodeEvent with an invalid event (empty DisplayName) should fail validation")
	}
}

func TestDecodeEventInvalidJSON(t *testing.T) {
	_, err := DecodeEvent(EventKindPlayerJoined, []byte("not json"))
	if err == nil {
		t.Error("DecodeEvent with invalid JSON should fail")
	}
}

func TestEncodeEventDeterministic(t *testing.T) {
	ev := PlayerJoined{Player: Player{ID: "usr_1", DisplayName: "Test"}}
	_, p1, err := EncodeEvent(ev)
	if err != nil {
		t.Fatal(err)
	}
	_, p2, err := EncodeEvent(ev)
	if err != nil {
		t.Fatal(err)
	}
	if string(p1) != string(p2) {
		t.Errorf("EncodeEvent not deterministic:\n  %s\n  %s", p1, p2)
	}
}

func TestDecodeWorldEnteringMissingName(t *testing.T) {
	payload := []byte(`{"world":{}}`)
	_, err := DecodeEvent(EventKindWorldEnteringObserved, payload)
	if err == nil {
		t.Error("WorldEnteringObserved with missing Name should fail")
	}
}

func TestDecodeMediaErrorMissingCodeAndMessage(t *testing.T) {
	payload := []byte(`{"stage":"resolve"}`)
	_, err := DecodeEvent(EventKindMediaErrorObserved, payload)
	if err == nil {
		t.Error("MediaErrorObserved with no code and no message should fail")
	}
}

func TestDecodeResourceURLEmptyURL(t *testing.T) {
	payload := []byte(`{"resource":{"url":"","kind":"video","role":"source"}}`)
	_, err := DecodeEvent(EventKindResourceURLObserved, payload)
	if err == nil {
		t.Error("ResourceURLObserved with empty URL should fail")
	}
}

func TestEncodeDecodeAllEventKinds(t *testing.T) {
	kinds := map[EventKind]bool{
		EventKindPlayerJoined:          false,
		EventKindPlayerLeft:            false,
		EventKindWorldEnteringObserved: false,
		EventKindWorldJoiningObserved:  false,
		EventKindResourceURLObserved:   false,
		EventKindResourceResolved:      false,
		EventKindMediaErrorObserved:    false,
	}

	events := []Event{
		PlayerJoined{Player: Player{DisplayName: "A"}},
		PlayerLeft{Player: Player{DisplayName: "B"}},
		WorldEnteringObserved{World: World{Name: "W"}},
		WorldJoiningObserved{World: World{ID: "wrld_1", InstanceID: "i1"}},
		ResourceURLObserved{Resource: RemoteResource{URL: "https://x", Kind: ResourceKindVideo, Role: ResourceRoleSource}},
		ResourceResolved{
			Input:  RemoteResource{URL: "https://a", Kind: ResourceKindVideo, Role: ResourceRoleResolverInput},
			Output: RemoteResource{URL: "https://b", Kind: ResourceKindVideo, Role: ResourceRoleResolved},
		},
		MediaErrorObserved{Stage: MediaStageLoad, Code: "E1"},
	}

	for _, ev := range events {
		kind, payload, err := EncodeEvent(ev)
		if err != nil {
			t.Fatalf("EncodeEvent(%T): %v", ev, err)
		}
		kinds[kind] = true

		var raw map[string]json.RawMessage
		if err := json.Unmarshal(payload, &raw); err != nil {
			t.Fatalf("payload is not valid JSON object for %T: %v", ev, err)
		}
	}

	for k, covered := range kinds {
		if !covered {
			t.Errorf("EventKind %q was not covered", k)
		}
	}
}
