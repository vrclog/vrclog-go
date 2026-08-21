package vrclog

import (
	"encoding/json"
	"testing"
	"time"
)

func TestEncodeObservationJSONShape(t *testing.T) {
	obs := Observation{
		ID:        "obs-id-123",
		Time:      time.Date(2026, 8, 18, 13, 0, 0, 0, time.FixedZone("JST", 9*3600)),
		AdapterID: "vrchat.core",
		RuleID:    "player_joined",
		Record: RecordRef{
			ID:       "rec-id-456",
			SourceID: "src-id-789",
			Offset:   1234,
			Line:     52,
		},
		Event: PlayerJoined{Player: Player{DisplayName: "Alice"}},
	}

	data, err := EncodeObservationJSON(obs)
	if err != nil {
		t.Fatalf("EncodeObservationJSON: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("JSON unmarshal: %v", err)
	}

	requiredKeys := []string{"id", "time", "adapter_id", "rule_id", "record", "type", "payload"}
	for _, key := range requiredKeys {
		if _, ok := raw[key]; !ok {
			t.Errorf("missing key %q in observation JSON", key)
		}
	}

	forbiddenKeys := []string{"path", "raw", "next_offset"}
	for _, key := range forbiddenKeys {
		if _, ok := raw[key]; ok {
			t.Errorf("forbidden key %q present in observation JSON", key)
		}
	}

	var record map[string]json.RawMessage
	if err := json.Unmarshal(raw["record"], &record); err != nil {
		t.Fatalf("record unmarshal: %v", err)
	}
	recordRequired := []string{"id", "source_id", "offset", "line"}
	for _, key := range recordRequired {
		if _, ok := record[key]; !ok {
			t.Errorf("missing key %q in record JSON", key)
		}
	}
	recordForbidden := []string{"path", "raw", "next_offset"}
	for _, key := range recordForbidden {
		if _, ok := record[key]; ok {
			t.Errorf("forbidden key %q present in record JSON", key)
		}
	}

	var typStr string
	if err := json.Unmarshal(raw["type"], &typStr); err != nil {
		t.Fatalf("type unmarshal: %v", err)
	}
	if typStr != string(EventKindPlayerJoined) {
		t.Errorf("type = %q, want %q", typStr, EventKindPlayerJoined)
	}
}

func TestObservationJSONRoundTrip(t *testing.T) {
	obs := Observation{
		ID:        "obs-rt",
		Time:      time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC),
		AdapterID: "test.adapter",
		RuleID:    "test_rule",
		Record: RecordRef{
			ID:       "rec-rt",
			SourceID: "src-rt",
			Offset:   500,
			Line:     10,
		},
		Event: WorldJoiningObserved{World: World{ID: "wrld_abc", InstanceID: "999~region(us)"}},
	}

	data, err := EncodeObservationJSON(obs)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	decoded, err := DecodeObservationJSON(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if decoded.ID != obs.ID {
		t.Errorf("ID = %q, want %q", decoded.ID, obs.ID)
	}
	if !decoded.Time.Equal(obs.Time) {
		t.Errorf("Time = %v, want %v", decoded.Time, obs.Time)
	}
	if decoded.AdapterID != obs.AdapterID {
		t.Errorf("AdapterID = %q, want %q", decoded.AdapterID, obs.AdapterID)
	}
	if decoded.RuleID != obs.RuleID {
		t.Errorf("RuleID = %q, want %q", decoded.RuleID, obs.RuleID)
	}
	if decoded.Record != obs.Record {
		t.Errorf("Record = %+v, want %+v", decoded.Record, obs.Record)
	}
	if decoded.Event.Kind() != obs.Event.Kind() {
		t.Errorf("Event.Kind() = %q, want %q", decoded.Event.Kind(), obs.Event.Kind())
	}
	wj, ok := decoded.Event.(WorldJoiningObserved)
	if !ok {
		t.Fatalf("Event type = %T, want WorldJoiningObserved", decoded.Event)
	}
	if wj.World.ID != "wrld_abc" || wj.World.InstanceID != "999~region(us)" {
		t.Errorf("World = %+v", wj.World)
	}
}

func TestGenerateObservationIDDeterministic(t *testing.T) {
	id1 := generateObservationID("rec1", "adapter1", "rule1")
	id2 := generateObservationID("rec1", "adapter1", "rule1")
	if id1 != id2 {
		t.Errorf("same inputs should produce same ID: %q vs %q", id1, id2)
	}
}

func TestGenerateObservationIDDifferentAdapter(t *testing.T) {
	id1 := generateObservationID("rec1", "adapter1", "rule1")
	id2 := generateObservationID("rec1", "adapter2", "rule1")
	if id1 == id2 {
		t.Errorf("different adapter_id should produce different ID: both %q", id1)
	}
}

func TestGenerateObservationIDDifferentRule(t *testing.T) {
	id1 := generateObservationID("rec1", "adapter1", "rule1")
	id2 := generateObservationID("rec1", "adapter1", "rule2")
	if id1 == id2 {
		t.Errorf("different rule_id should produce different ID: both %q", id1)
	}
}

func TestGenerateObservationIDDifferentRecord(t *testing.T) {
	id1 := generateObservationID("rec1", "adapter1", "rule1")
	id2 := generateObservationID("rec2", "adapter1", "rule1")
	if id1 == id2 {
		t.Errorf("different record_id should produce different ID: both %q", id1)
	}
}

func TestObservationID_OrderIndependent(t *testing.T) {
	a := &mockAdapter{id: "order.test", decode: func(Record) ([]Emission, error) {
		return []Emission{
			{Rule: "r1", Event: PlayerJoined{Player: Player{DisplayName: "A"}}},
			{Rule: "r2", Event: PlayerLeft{Player: Player{DisplayName: "B"}}},
		}, nil
	}}
	b := &mockAdapter{id: "order.test", decode: func(Record) ([]Emission, error) {
		return []Emission{
			{Rule: "r2", Event: PlayerLeft{Player: Player{DisplayName: "B"}}},
			{Rule: "r1", Event: PlayerJoined{Player: Player{DisplayName: "A"}}},
		}, nil
	}}

	engA, _ := NewEngine(a)
	engB, _ := NewEngine(b)
	rec := validRecord()

	resultA := engA.Process(rec)
	resultB := engB.Process(rec)

	idsA := map[RuleID]ObservationID{}
	for _, obs := range resultA.Observations {
		idsA[obs.RuleID] = obs.ID
	}
	idsB := map[RuleID]ObservationID{}
	for _, obs := range resultB.Observations {
		idsB[obs.RuleID] = obs.ID
	}

	if idsA["r1"] != idsB["r1"] {
		t.Errorf("r1 ID should be order-independent: %q vs %q", idsA["r1"], idsB["r1"])
	}
	if idsA["r2"] != idsB["r2"] {
		t.Errorf("r2 ID should be order-independent: %q vs %q", idsA["r2"], idsB["r2"])
	}
}

func TestObservationID_PayloadChangeDoesNotAffectID(t *testing.T) {
	a := &mockAdapter{id: "payload.test", decode: func(Record) ([]Emission, error) {
		return []Emission{{Rule: "r1", Event: PlayerJoined{Player: Player{DisplayName: "A"}}}}, nil
	}}
	b := &mockAdapter{id: "payload.test", decode: func(Record) ([]Emission, error) {
		return []Emission{{Rule: "r1", Event: PlayerJoined{Player: Player{DisplayName: "DifferentName"}}}}, nil
	}}

	engA, _ := NewEngine(a)
	engB, _ := NewEngine(b)
	rec := validRecord()

	resultA := engA.Process(rec)
	resultB := engB.Process(rec)

	if len(resultA.Observations) != 1 || len(resultB.Observations) != 1 {
		t.Fatal("expected 1 observation each")
	}
	if resultA.Observations[0].ID != resultB.Observations[0].ID {
		t.Errorf("payload change should not affect ID: %q vs %q",
			resultA.Observations[0].ID, resultB.Observations[0].ID)
	}
}

func TestDecodeObservationJSONInvalidJSON(t *testing.T) {
	_, err := DecodeObservationJSON([]byte("not json"))
	if err == nil {
		t.Error("DecodeObservationJSON with invalid JSON should fail")
	}
}

func TestDecodeObservationJSONInvalidEvent(t *testing.T) {
	bad := `{"id":"x","time":"2026-01-01T00:00:00Z","adapter_id":"a","rule_id":"r","record":{"id":"r","source_id":"s","offset":0,"line":1},"type":"nonexistent.kind","payload":{}}`
	_, err := DecodeObservationJSON([]byte(bad))
	if err == nil {
		t.Error("DecodeObservationJSON with unknown event kind should fail")
	}
}
