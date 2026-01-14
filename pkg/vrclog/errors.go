package vrclog

import (
	"errors"
	"fmt"

	"github.com/vrclog/vrclog-go/internal/logfinder"
)

// Sentinel errors returned by this package.
var (
	// ErrLogDirNotFound is returned when the VRChat log directory
	// cannot be found or accessed.
	ErrLogDirNotFound = logfinder.ErrLogDirNotFound

	// ErrNoLogFiles is returned when no log files are found
	// in the specified directory.
	ErrNoLogFiles = logfinder.ErrNoLogFiles

	// ErrWatcherClosed is returned when Watch() is called on a closed Watcher.
	ErrWatcherClosed = errors.New("watcher closed")

	// ErrAlreadyWatching is returned when Watch() is called on a Watcher
	// that is already watching.
	ErrAlreadyWatching = errors.New("watch already in progress")

	// ErrReplayLimitExceeded is returned when replay exceeds memory limits
	// configured via WithMaxReplayBytes or WithMaxReplayLineBytes.
	ErrReplayLimitExceeded = errors.New("replay memory limit exceeded")
)

// ParseError represents an error that occurred while parsing a log line.
// Use errors.As to check for this error type and access the original line.
type ParseError struct {
	Line string // The original log line that failed to parse
	Err  error  // The underlying error
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("parse error: %v", e.Err)
}

func (e *ParseError) Unwrap() error {
	return e.Err
}

// LineTooLongError indicates a log line exceeded the maximum length limit.
// The line is skipped by default, but if WithParseStopOnError(true) is set,
// parsing stops and this error is returned.
type LineTooLongError struct {
	LineNumber int // Line number in file (1-indexed)
	Length     int // Actual line length in bytes
	MaxLength  int // Maximum allowed length
}

func (e *LineTooLongError) Error() string {
	return fmt.Sprintf("line %d too long (%d bytes, max %d)", e.LineNumber, e.Length, e.MaxLength)
}

// WatchOp represents an operation that can fail during watching.
type WatchOp string

const (
	// WatchOpFindLatest is the operation of finding the latest log file.
	WatchOpFindLatest WatchOp = "find_latest"
	// WatchOpTail is the operation of tailing a log file.
	WatchOpTail WatchOp = "tail"
	// WatchOpParse is the operation of parsing a log line.
	WatchOpParse WatchOp = "parse"
	// WatchOpReplay is the operation of replaying log lines.
	WatchOpReplay WatchOp = "replay"
	// WatchOpRotation is the operation of checking for log rotation.
	WatchOpRotation WatchOp = "rotation"
)

// WatchError represents an error that occurred during watch operations.
// Use errors.As to check for this error type and determine the operation.
type WatchError struct {
	Op   WatchOp // The operation that failed
	Path string  // The file path involved (if any)
	Err  error   // The underlying error
}

func (e *WatchError) Error() string {
	if e.Path == "" {
		return fmt.Sprintf("%s: %v", e.Op, e.Err)
	}
	return fmt.Sprintf("%s %s: %v", e.Op, e.Path, e.Err)
}

func (e *WatchError) Unwrap() error {
	return e.Err
}
