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
)
