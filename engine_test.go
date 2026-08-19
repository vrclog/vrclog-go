package vrclog

import (
	"errors"
	"testing"
	"time"
)

type mockAdapter struct {
	id     AdapterID
	decode func(Record) ([]Emission, error)
}

func (m *mockAdapter) ID() AdapterID                       { return m.id }
func (m *mockAdapter) Decode(r Record) ([]Emission, error) { return m.decode(r) }

func validRecord() Record {
	return Record{
		ID:       "rec-test",
		Time:     time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC),
		Level:    LevelLog,
		Message:  "test message",
		Raw:      "test message",
		SourceID: "src-test",
		Offset:   0,
		Line:     1,
	}
}

func validEmission() Emission {
	return Emission{
		Rule:  "test_rule",
		Event: PlayerJoined{Player: Player{DisplayName: "TestPlayer"}},
	}
}

func TestNewEngineZeroAdapters(t *testing.T) {
	_, err := NewEngine()
	if !errors.Is(err, ErrNoAdapters) {
		t.Errorf("got %v, want ErrNoAdapters", err)
	}
}

func TestNewEngineNilAdapter(t *testing.T) {
	_, err := NewEngine(nil)
	if !errors.Is(err, ErrNilAdapter) {
		t.Errorf("got %v, want ErrNilAdapter", err)
	}
}

func TestNewEngineEmptyAdapterID(t *testing.T) {
	a := &mockAdapter{id: "", decode: func(Record) ([]Emission, error) { return nil, nil }}
	_, err := NewEngine(a)
	if !errors.Is(err, ErrEmptyAdapterID) {
		t.Errorf("got %v, want ErrEmptyAdapterID", err)
	}
}

func TestNewEngineDuplicateAdapterID(t *testing.T) {
	a1 := &mockAdapter{id: "test.adapter", decode: func(Record) ([]Emission, error) { return nil, nil }}
	a2 := &mockAdapter{id: "test.adapter", decode: func(Record) ([]Emission, error) { return nil, nil }}
	_, err := NewEngine(a1, a2)
	if !errors.Is(err, ErrDuplicateAdapterID) {
		t.Errorf("got %v, want ErrDuplicateAdapterID", err)
	}
}

func TestNewEngineInvalidAdapterIDChars(t *testing.T) {
	a := &mockAdapter{id: "has space", decode: func(Record) ([]Emission, error) { return nil, nil }}
	_, err := NewEngine(a)
	if !errors.Is(err, ErrInvalidAdapterID) {
		t.Errorf("got %v, want ErrInvalidAdapterID", err)
	}
}

func TestProcessAllAdaptersRunInOrder(t *testing.T) {
	var order []string
	a1 := &mockAdapter{id: "first", decode: func(Record) ([]Emission, error) {
		order = append(order, "first")
		return nil, nil
	}}
	a2 := &mockAdapter{id: "second", decode: func(Record) ([]Emission, error) {
		order = append(order, "second")
		return nil, nil
	}}
	a3 := &mockAdapter{id: "third", decode: func(Record) ([]Emission, error) {
		order = append(order, "third")
		return nil, nil
	}}

	eng, err := NewEngine(a1, a2, a3)
	if err != nil {
		t.Fatal(err)
	}
	eng.Process(validRecord())

	if len(order) != 3 || order[0] != "first" || order[1] != "second" || order[2] != "third" {
		t.Errorf("adapter execution order = %v, want [first second third]", order)
	}
}

func TestProcessNilNilNoDiagnostic(t *testing.T) {
	a := &mockAdapter{id: "noop", decode: func(Record) ([]Emission, error) { return nil, nil }}
	eng, _ := NewEngine(a)
	result := eng.Process(validRecord())
	if len(result.Diagnostics) != 0 {
		t.Errorf("nil,nil should produce no diagnostics, got %d", len(result.Diagnostics))
	}
	if len(result.Observations) != 0 {
		t.Errorf("nil,nil should produce no observations, got %d", len(result.Observations))
	}
}

func TestProcessEmissionsAndErrorRejected(t *testing.T) {
	a := &mockAdapter{id: "bad", decode: func(Record) ([]Emission, error) {
		return []Emission{validEmission()}, errors.New("oops")
	}}
	eng, _ := NewEngine(a)
	result := eng.Process(validRecord())

	if len(result.Observations) != 0 {
		t.Errorf("emissions+error should discard all observations, got %d", len(result.Observations))
	}
	if len(result.Diagnostics) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(result.Diagnostics))
	}
	if result.Diagnostics[0].Code != DiagnosticInvalidAdapterResult {
		t.Errorf("code = %q, want %q", result.Diagnostics[0].Code, DiagnosticInvalidAdapterResult)
	}
}

func TestProcessAdapterErrorContinuesOthers(t *testing.T) {
	callCount := 0
	a1 := &mockAdapter{id: "failing", decode: func(Record) ([]Emission, error) {
		callCount++
		return nil, errors.New("fail")
	}}
	a2 := &mockAdapter{id: "succeeding", decode: func(Record) ([]Emission, error) {
		callCount++
		return []Emission{validEmission()}, nil
	}}
	eng, _ := NewEngine(a1, a2)
	result := eng.Process(validRecord())

	if callCount != 2 {
		t.Errorf("both adapters should run, callCount = %d", callCount)
	}
	if len(result.Observations) != 1 {
		t.Errorf("expected 1 observation, got %d", len(result.Observations))
	}
	hasDiag := false
	for _, d := range result.Diagnostics {
		if d.Code == DiagnosticAdapterError {
			hasDiag = true
		}
	}
	if !hasDiag {
		t.Error("expected DiagnosticAdapterError for failing adapter")
	}
}

func TestProcessEmptyRuleIDRejected(t *testing.T) {
	a := &mockAdapter{id: "empty.rule", decode: func(Record) ([]Emission, error) {
		return []Emission{{Rule: "", Event: PlayerJoined{Player: Player{DisplayName: "X"}}}}, nil
	}}
	eng, _ := NewEngine(a)
	result := eng.Process(validRecord())

	if len(result.Observations) != 0 {
		t.Errorf("empty RuleID should discard emission, got %d observations", len(result.Observations))
	}
	if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != DiagnosticInvalidRuleID {
		t.Errorf("expected DiagnosticInvalidRuleID, got %+v", result.Diagnostics)
	}
}

func TestProcessInvalidEventRejected(t *testing.T) {
	a := &mockAdapter{id: "invalid.event", decode: func(Record) ([]Emission, error) {
		return []Emission{{Rule: "r1", Event: PlayerJoined{Player: Player{DisplayName: ""}}}}, nil
	}}
	eng, _ := NewEngine(a)
	result := eng.Process(validRecord())

	if len(result.Observations) != 0 {
		t.Errorf("invalid event should be discarded, got %d observations", len(result.Observations))
	}
	if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != DiagnosticInvalidEvent {
		t.Errorf("expected DiagnosticInvalidEvent, got %+v", result.Diagnostics)
	}
}

func TestProcessZeroTimeEmissionsRejected(t *testing.T) {
	callCount := 0
	a := &mockAdapter{id: "zero.time", decode: func(Record) ([]Emission, error) {
		callCount++
		return []Emission{validEmission()}, nil
	}}
	eng, _ := NewEngine(a)

	rec := validRecord()
	rec.Time = time.Time{}

	result := eng.Process(rec)

	if callCount != 1 {
		t.Errorf("adapter should still be called, callCount = %d", callCount)
	}
	if len(result.Observations) != 0 {
		t.Errorf("zero-time emissions should be discarded, got %d observations", len(result.Observations))
	}
	if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != DiagnosticInvalidEvent {
		t.Errorf("expected DiagnosticInvalidEvent for zero time, got %+v", result.Diagnostics)
	}
}

func TestProcessRecordIssueSkipsAdapters(t *testing.T) {
	callCount := 0
	a := &mockAdapter{id: "should.not.run", decode: func(Record) ([]Emission, error) {
		callCount++
		return nil, nil
	}}
	eng, _ := NewEngine(a)

	rec := validRecord()
	rec.Issue = &RecordIssue{Code: "line_too_long", Message: "line exceeds max"}

	result := eng.Process(rec)

	if callCount != 0 {
		t.Errorf("adapter should NOT be called for record with issue, callCount = %d", callCount)
	}
	if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != DiagnosticRecordIssue {
		t.Errorf("expected DiagnosticRecordIssue, got %+v", result.Diagnostics)
	}
}

func TestProcessObservationIDsDeterministic(t *testing.T) {
	a := &mockAdapter{id: "det", decode: func(Record) ([]Emission, error) {
		return []Emission{validEmission()}, nil
	}}
	eng, _ := NewEngine(a)
	rec := validRecord()

	r1 := eng.Process(rec)
	r2 := eng.Process(rec)

	if len(r1.Observations) != 1 || len(r2.Observations) != 1 {
		t.Fatal("expected 1 observation each")
	}
	if r1.Observations[0].ID != r2.Observations[0].ID {
		t.Errorf("Observation IDs should be deterministic: %q vs %q",
			r1.Observations[0].ID, r2.Observations[0].ID)
	}
}

func TestProcessEmissionIndexDifferentiatesIDs(t *testing.T) {
	a := &mockAdapter{id: "multi", decode: func(Record) ([]Emission, error) {
		return []Emission{
			{Rule: "r1", Event: PlayerJoined{Player: Player{DisplayName: "A"}}},
			{Rule: "r1", Event: PlayerJoined{Player: Player{DisplayName: "B"}}},
		}, nil
	}}
	eng, _ := NewEngine(a)
	result := eng.Process(validRecord())

	if len(result.Observations) != 2 {
		t.Fatalf("expected 2 observations, got %d", len(result.Observations))
	}
	if result.Observations[0].ID == result.Observations[1].ID {
		t.Errorf("different emission indices should produce different IDs: both %q", result.Observations[0].ID)
	}
}

func TestProcessNoDeduplification(t *testing.T) {
	em := validEmission()
	a1 := &mockAdapter{id: "dup.a", decode: func(Record) ([]Emission, error) {
		return []Emission{em}, nil
	}}
	a2 := &mockAdapter{id: "dup.b", decode: func(Record) ([]Emission, error) {
		return []Emission{em}, nil
	}}
	eng, _ := NewEngine(a1, a2)
	result := eng.Process(validRecord())

	if len(result.Observations) != 2 {
		t.Errorf("both equivalent emissions should produce observations, got %d", len(result.Observations))
	}
	if result.Observations[0].ID == result.Observations[1].ID {
		t.Errorf("observations from different adapters should have different IDs")
	}
}

func TestProcessNilEventInEmission(t *testing.T) {
	a := &mockAdapter{id: "nil.event", decode: func(Record) ([]Emission, error) {
		return []Emission{{Rule: "r1", Event: nil}}, nil
	}}
	eng, _ := NewEngine(a)
	result := eng.Process(validRecord())

	if len(result.Observations) != 0 {
		t.Errorf("nil event should be rejected, got %d observations", len(result.Observations))
	}
	if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != DiagnosticInvalidEvent {
		t.Errorf("expected DiagnosticInvalidEvent for nil event, got %+v", result.Diagnostics)
	}
}

func TestProcessObservationFields(t *testing.T) {
	a := &mockAdapter{id: "check.fields", decode: func(Record) ([]Emission, error) {
		return []Emission{validEmission()}, nil
	}}
	eng, _ := NewEngine(a)
	rec := validRecord()
	result := eng.Process(rec)

	if len(result.Observations) != 1 {
		t.Fatalf("expected 1 observation, got %d", len(result.Observations))
	}
	obs := result.Observations[0]
	if obs.Time != rec.Time {
		t.Errorf("Time = %v, want %v", obs.Time, rec.Time)
	}
	if obs.AdapterID != "check.fields" {
		t.Errorf("AdapterID = %q, want %q", obs.AdapterID, "check.fields")
	}
	if obs.RuleID != "test_rule" {
		t.Errorf("RuleID = %q, want %q", obs.RuleID, "test_rule")
	}
	if obs.Record.ID != rec.ID {
		t.Errorf("Record.ID = %q, want %q", obs.Record.ID, rec.ID)
	}
	if obs.Record.SourceID != rec.SourceID {
		t.Errorf("Record.SourceID = %q, want %q", obs.Record.SourceID, rec.SourceID)
	}
	if obs.Record.Offset != rec.Offset {
		t.Errorf("Record.Offset = %d, want %d", obs.Record.Offset, rec.Offset)
	}
	if obs.Record.Line != rec.Line {
		t.Errorf("Record.Line = %d, want %d", obs.Record.Line, rec.Line)
	}
}
