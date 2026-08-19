package vrclog

import (
	"fmt"
	"unicode"
)

type AdapterID string

type RuleID string

type SourceID string

type RecordID string

type ObservationID string

type EventKind string

type Level string

const (
	LevelUnknown   Level = "unknown"
	LevelDebug     Level = "debug"
	LevelLog       Level = "log"
	LevelWarning   Level = "warning"
	LevelError     Level = "error"
	LevelException Level = "exception"
)

func validateAdapterID(id AdapterID) error {
	if id == "" {
		return fmt.Errorf("adapter ID must not be empty")
	}
	for _, r := range string(id) {
		if unicode.IsSpace(r) {
			return fmt.Errorf("adapter ID contains whitespace: %q", id)
		}
		if unicode.IsControl(r) {
			return fmt.Errorf("adapter ID contains control character: %q", id)
		}
		if !isAllowedAdapterIDRune(r) {
			return fmt.Errorf("adapter ID contains invalid character %q: %q", r, id)
		}
	}
	return nil
}

func isAllowedAdapterIDRune(r rune) bool {
	if r >= 'a' && r <= 'z' {
		return true
	}
	if r >= 'A' && r <= 'Z' {
		return true
	}
	if r >= '0' && r <= '9' {
		return true
	}
	switch r {
	case '.', '-', '_':
		return true
	}
	return false
}
