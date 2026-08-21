package vrclog

import "errors"

var (
	ErrUnknownEventKind    = errors.New("unknown event kind")
	ErrEventKindMismatch   = errors.New("event kind does not match type")
	ErrCursorSourceMissing = errors.New("cursor source file not found")
	ErrNoLogDirectory      = errors.New("no log directory available")
	ErrNoAdapters          = errors.New("at least one adapter is required")
	ErrNilAdapter          = errors.New("adapter must not be nil")
	ErrEmptyAdapterID      = errors.New("adapter ID must not be empty")
	ErrDuplicateAdapterID  = errors.New("duplicate adapter ID")
	ErrInvalidAdapterID    = errors.New("invalid adapter ID")
	ErrInvalidOffset       = errors.New("invalid offset")

	// ErrSourceTruncated is returned when an actively-followed log
	// file's size drops below the offset Follow has already committed.
	// This can happen if the file is truncated or replaced out from
	// under Follow. Follow treats this as fatal: it does not rewind the
	// cursor or silently restart from the beginning of the file.
	ErrSourceTruncated = errors.New("log source truncated")
)
