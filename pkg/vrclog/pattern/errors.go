package pattern

import "fmt"

// ValidationError represents a schema-level validation error.
// These errors occur when a pattern file violates structural requirements
// (e.g., missing required fields, invalid version number).
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error: %s: %s", e.Field, e.Message)
}

// PatternError represents an error specific to an individual pattern.
// These errors occur when a single pattern has issues (e.g., invalid regex,
// duplicate ID, missing fields).
type PatternError struct {
	Index   int    // 0-based index of the pattern in the file
	ID      string // Pattern ID (may be empty if ID field is missing)
	Field   string
	Message string
	Cause   error // Underlying error (e.g., regex compile error)
}

func (e *PatternError) Error() string {
	if e.ID != "" {
		return fmt.Sprintf("pattern %q: %s: %s", e.ID, e.Field, e.Message)
	}
	return fmt.Sprintf("pattern[%d]: %s: %s", e.Index, e.Field, e.Message)
}

// Unwrap returns the underlying cause of the error.
// This enables errors.Is() and errors.As() to work with PatternError.
func (e *PatternError) Unwrap() error {
	return e.Cause
}
