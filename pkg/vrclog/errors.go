package vrclog

import "github.com/vrclog/vrclog-go/internal/logfinder"

// Sentinel errors returned by this package.
var (
	// ErrLogDirNotFound is returned when the VRChat log directory
	// cannot be found or accessed.
	ErrLogDirNotFound = logfinder.ErrLogDirNotFound

	// ErrNoLogFiles is returned when no log files are found
	// in the specified directory.
	ErrNoLogFiles = logfinder.ErrNoLogFiles
)
