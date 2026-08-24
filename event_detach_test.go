package vrclog

import (
	"encoding/json"
	"testing"
)

func TestDetachEventForObservationTypedNilRejected(t *testing.T) {
	cases := []struct {
		name  string
		event Event
	}{
		{"PlayerJoined", (*PlayerJoined)(nil)},
		{"PlayerLeft", (*PlayerLeft)(nil)},
		{"WorldEnteringObserved", (*WorldEnteringObserved)(nil)},
		{"WorldJoiningObserved", (*WorldJoiningObserved)(nil)},
		{"ResourceURLObserved", (*ResourceURLObserved)(nil)},
		{"ResourceResolved", (*ResourceResolved)(nil)},
		{"MediaErrorObserved", (*MediaErrorObserved)(nil)},
		{"AdapterEvent", (*AdapterEvent)(nil)},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			_, err := detachEventForObservation(tt.event)
			if err == nil {
				t.Fatal("expected error for typed-nil pointer, got nil")
			}
		})
	}
}

func TestDetachEventForObservationNormalizesPointers(t *testing.T) {
	target := &MediaTarget{Component: "vrchat", Backend: MediaBackendAVPro}
	resource := &RemoteResource{URL: "https://example.com/a.mp4", Kind: ResourceKindVideo, Role: ResourceRoleSource}

	cases := []struct {
		name  string
		event Event
	}{
		{"PlayerJoined", &PlayerJoined{Player: Player{DisplayName: "alice"}}},
		{"PlayerLeft", &PlayerLeft{Player: Player{DisplayName: "alice"}}},
		{"WorldEnteringObserved", &WorldEnteringObserved{World: World{Name: "Test World"}}},
		{"WorldJoiningObserved", &WorldJoiningObserved{World: World{ID: "wrld_1", InstanceID: "1"}}},
		{"ResourceURLObserved", &ResourceURLObserved{
			Resource: RemoteResource{URL: "https://example.com/a.mp4", Kind: ResourceKindVideo, Role: ResourceRoleSource},
			Target:   target,
		}},
		{"ResourceResolved", &ResourceResolved{
			Input:  RemoteResource{URL: "https://example.com/a.mp4", Kind: ResourceKindVideo, Role: ResourceRoleSource},
			Output: RemoteResource{URL: "https://example.com/b.mp4", Kind: ResourceKindVideo, Role: ResourceRoleResolved},
			Target: target,
		}},
		{"MediaErrorObserved", &MediaErrorObserved{
			Stage: MediaStageResolve, Message: "boom", Resource: resource, Target: target,
		}},
		{"AdapterEvent", &AdapterEvent{Tag: "vrpoker.hole_cards.v1", Data: json.RawMessage(`{"card1":"Jc"}`)}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			normalized, err := detachEventForObservation(tt.event)
			if err != nil {
				t.Fatalf("detachEventForObservation() error = %v", err)
			}
			if err := normalized.validate(); err != nil {
				t.Fatalf("normalized event failed validate(): %v", err)
			}
			if normalized.Kind() != tt.event.Kind() {
				t.Errorf("Kind() = %q, want %q", normalized.Kind(), tt.event.Kind())
			}
		})
	}
}

func TestDetachEventForObservationClonesAdapterEventData(t *testing.T) {
	data := json.RawMessage(`{"card1":"Jc","card2":"6d"}`)
	want := append(json.RawMessage(nil), data...)

	normalized, err := detachEventForObservation(AdapterEvent{Tag: "vrpoker.hole_cards.v1", Data: data})
	if err != nil {
		t.Fatalf("detachEventForObservation() error = %v", err)
	}
	data[2] = 'X'

	got, ok := normalized.(AdapterEvent)
	if !ok {
		t.Fatalf("normalized type = %T, want AdapterEvent", normalized)
	}
	if string(got.Data) != string(want) {
		t.Errorf("Data mutated after detach: got %s, want %s", got.Data, want)
	}
}

func TestDetachEventForObservationDeepCopiesOptionalPointerFields(t *testing.T) {
	target := &MediaTarget{Component: "vrchat", Backend: MediaBackendAVPro}
	original := ResourceURLObserved{
		Resource: RemoteResource{URL: "https://example.com/a.mp4", Kind: ResourceKindVideo, Role: ResourceRoleSource},
		Target:   target,
	}

	normalized, err := detachEventForObservation(original)
	if err != nil {
		t.Fatalf("detachEventForObservation() error = %v", err)
	}
	got, ok := normalized.(ResourceURLObserved)
	if !ok {
		t.Fatalf("normalized type = %T, want ResourceURLObserved", normalized)
	}
	if got.Target == target {
		t.Error("Target was not deep-copied: points to the same MediaTarget as the original")
	}
	if got.Target == nil || *got.Target != *target {
		t.Errorf("Target = %+v, want a copy equal to %+v", got.Target, target)
	}

	target.Component = "mutated"
	if got.Target.Component == "mutated" {
		t.Error("mutating the original Target affected the detached observation's Target")
	}
}

func TestDetachEventForObservationUnknownType(t *testing.T) {
	_, err := detachEventForObservation(nil)
	if err == nil {
		t.Fatal("expected error for nil Event, got nil")
	}
}
