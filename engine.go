package vrclog

import "fmt"

// Engine processes Records through registered Adapters to produce Observations.
// Process is NOT safe for concurrent calls from multiple goroutines;
// callers must serialize calls (e.g. via a single range-loop over iter.Seq2).
type Engine struct {
	adapters []Adapter
}

type Result struct {
	Observations []Observation
	Diagnostics  []Diagnostic
}

func NewEngine(adapters ...Adapter) (*Engine, error) {
	if len(adapters) == 0 {
		return nil, ErrNoAdapters
	}
	seen := make(map[AdapterID]struct{}, len(adapters))
	copied := make([]Adapter, len(adapters))
	for i, a := range adapters {
		if a == nil {
			return nil, ErrNilAdapter
		}
		id := a.ID()
		if id == "" {
			return nil, ErrEmptyAdapterID
		}
		if err := validateAdapterID(id); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidAdapterID, err)
		}
		if _, dup := seen[id]; dup {
			return nil, fmt.Errorf("%w: %s", ErrDuplicateAdapterID, id)
		}
		seen[id] = struct{}{}
		copied[i] = a
	}
	return &Engine{adapters: copied}, nil
}

// Process is NOT safe for concurrent calls from multiple goroutines;
// callers must serialize calls (e.g. via a single range-loop over iter.Seq2).
func (e *Engine) Process(record Record) Result {
	var result Result
	ref := RecordRef{
		ID:       record.ID,
		SourceID: record.SourceID,
		Offset:   record.Offset,
		Line:     record.Line,
	}

	if record.Issue != nil {
		result.Diagnostics = append(result.Diagnostics, Diagnostic{
			Code:    DiagnosticRecordIssue,
			Message: record.Issue.Message,
			Record:  ref,
		})
		return result
	}

	for _, adapter := range e.adapters {
		emissions, err := adapter.Decode(record)

		if len(emissions) > 0 && err != nil {
			result.Diagnostics = append(result.Diagnostics, Diagnostic{
				Code:      DiagnosticInvalidAdapterResult,
				Message:   "adapter returned both emissions and error",
				AdapterID: adapter.ID(),
				Record:    ref,
				Err:       err,
			})
			continue
		}

		if err != nil {
			result.Diagnostics = append(result.Diagnostics, Diagnostic{
				Code:      DiagnosticAdapterError,
				Message:   err.Error(),
				AdapterID: adapter.ID(),
				Record:    ref,
				Err:       err,
			})
			continue
		}

		ruleCount := make(map[RuleID]int, len(emissions))
		for _, em := range emissions {
			if em.Rule != "" {
				ruleCount[em.Rule]++
			}
		}
		diagnosedDuplicate := make(map[RuleID]bool, len(ruleCount))

		for _, em := range emissions {
			if em.Rule == "" {
				result.Diagnostics = append(result.Diagnostics, Diagnostic{
					Code:      DiagnosticInvalidRuleID,
					Message:   "empty rule ID",
					AdapterID: adapter.ID(),
					Record:    ref,
				})
				continue
			}

			if ruleCount[em.Rule] > 1 {
				if !diagnosedDuplicate[em.Rule] {
					diagnosedDuplicate[em.Rule] = true
					result.Diagnostics = append(result.Diagnostics, Diagnostic{
						Code:      DiagnosticDuplicateRuleID,
						Message:   "duplicate rule ID in single decode result",
						AdapterID: adapter.ID(),
						RuleID:    em.Rule,
						Record:    ref,
					})
				}
				continue
			}

			if em.Event == nil {
				result.Diagnostics = append(result.Diagnostics, Diagnostic{
					Code:      DiagnosticInvalidEvent,
					Message:   "nil event",
					AdapterID: adapter.ID(),
					RuleID:    em.Rule,
					Record:    ref,
				})
				continue
			}

			if vErr := em.Event.validate(); vErr != nil {
				result.Diagnostics = append(result.Diagnostics, Diagnostic{
					Code:      DiagnosticInvalidEvent,
					Message:   vErr.Error(),
					AdapterID: adapter.ID(),
					RuleID:    em.Rule,
					Record:    ref,
				})
				continue
			}

			if record.Time.IsZero() {
				result.Diagnostics = append(result.Diagnostics, Diagnostic{
					Code:      DiagnosticInvalidEvent,
					Message:   "record has zero time",
					AdapterID: adapter.ID(),
					RuleID:    em.Rule,
					Record:    ref,
				})
				continue
			}

			obs := Observation{
				ID:        generateObservationID(record.ID, adapter.ID(), em.Rule),
				Time:      record.Time,
				AdapterID: adapter.ID(),
				RuleID:    em.Rule,
				Record:    ref,
				Event:     em.Event,
			}
			result.Observations = append(result.Observations, obs)
		}
	}
	return result
}
