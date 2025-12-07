package vrclog

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/vrclog/vrclog-go/internal/logfinder"
	"github.com/vrclog/vrclog-go/internal/parser"
	"github.com/vrclog/vrclog-go/internal/tailer"
)

// ReplayMode specifies how to handle existing log lines.
type ReplayMode int

const (
	// ReplayNone only watches for new lines (default, tail -f behavior).
	ReplayNone ReplayMode = iota
	// ReplayFromStart reads from the beginning of the file.
	ReplayFromStart
	// ReplayLastN reads the last N lines before tailing.
	ReplayLastN
	// ReplaySinceTime reads lines since a specific timestamp.
	ReplaySinceTime
)

// ReplayConfig configures replay behavior.
// Only one mode can be active at a time (mutually exclusive).
type ReplayConfig struct {
	Mode  ReplayMode
	LastN int       // For ReplayLastN
	Since time.Time // For ReplaySinceTime
}

// WatchOptions configures log watching behavior.
// The zero value is valid and uses sensible defaults.
type WatchOptions struct {
	// LogDir specifies the VRChat log directory.
	// If empty, auto-detects from default Windows locations.
	// Can also be set via VRCLOG_LOGDIR environment variable.
	LogDir string

	// PollInterval is how often to check for new/rotated log files.
	// Default: 2 seconds.
	PollInterval time.Duration

	// IncludeRawLine includes the original log line in Event.RawLine.
	// Default: false.
	IncludeRawLine bool

	// Replay configures replay behavior for existing log lines.
	// Default: ReplayNone (only new lines).
	Replay ReplayConfig
}

// Validate checks for invalid option combinations.
func (o WatchOptions) Validate() error {
	// Currently no validation needed as ReplayConfig is designed
	// to be mutually exclusive by using a single Mode field
	return nil
}

// Watcher monitors VRChat log files.
type Watcher struct {
	opts   WatchOptions
	logDir string

	mu     sync.Mutex
	closed bool
}

// NewWatcher creates a watcher.
// Validates options and checks log directory existence.
// Does NOT start goroutines (cheap to call).
// Returns error for invalid options or missing log directory.
func NewWatcher(opts WatchOptions) (*Watcher, error) {
	if err := opts.Validate(); err != nil {
		return nil, fmt.Errorf("invalid options: %w", err)
	}

	// Find log directory
	logDir, err := logfinder.FindLogDir(opts.LogDir)
	if err != nil {
		return nil, err
	}

	return &Watcher{
		opts:   opts,
		logDir: logDir,
	}, nil
}

// Watch starts watching and returns channels.
// Starts internal goroutines here.
// When ctx is cancelled, channels are closed automatically.
// Both channels close on ctx.Done() or fatal error.
func (w *Watcher) Watch(ctx context.Context) (<-chan Event, <-chan error) {
	eventCh := make(chan Event)
	errCh := make(chan error)

	go w.run(ctx, eventCh, errCh)

	return eventCh, errCh
}

// Close stops the watcher and releases resources.
// Safe to call multiple times.
func (w *Watcher) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return nil
	}
	w.closed = true
	return nil
}

func (w *Watcher) run(ctx context.Context, eventCh chan<- Event, errCh chan<- error) {
	defer close(eventCh)
	defer close(errCh)

	// Find latest log file
	logFile, err := logfinder.FindLatestLogFile(w.logDir)
	if err != nil {
		sendError(errCh, err)
		return
	}

	// Configure tailer
	cfg := tailer.DefaultConfig()
	cfg.FromStart = w.opts.Replay.Mode == ReplayFromStart

	// Start tailer
	t, err := tailer.New(ctx, logFile, cfg)
	if err != nil {
		sendError(errCh, fmt.Errorf("starting tailer: %w", err))
		return
	}
	defer t.Stop()

	// Process lines
	for {
		select {
		case <-ctx.Done():
			return
		case line, ok := <-t.Lines():
			if !ok {
				return
			}
			w.processLine(ctx, line, eventCh, errCh)
		case err, ok := <-t.Errors():
			if !ok {
				return
			}
			sendError(errCh, err)
		}
	}
}

func (w *Watcher) processLine(ctx context.Context, line string, eventCh chan<- Event, errCh chan<- error) {
	ev, err := parser.Parse(line)
	if err != nil {
		sendError(errCh, fmt.Errorf("parse error: %w", err))
		return
	}
	if ev == nil {
		return // Not a recognized event
	}

	// Include raw line if requested
	if w.opts.IncludeRawLine {
		ev.RawLine = line
	}

	// Filter by replay time if needed
	if w.opts.Replay.Mode == ReplaySinceTime && ev.Timestamp.Before(w.opts.Replay.Since) {
		return
	}

	// Send event
	select {
	case eventCh <- *ev:
	case <-ctx.Done():
	}
}

// sendError sends an error non-blocking.
func sendError(errCh chan<- error, err error) {
	select {
	case errCh <- err:
	default:
		// Drop error if channel is full
	}
}

// Watch is a convenience function that creates a watcher and starts watching.
// Returns error immediately for initialization failures.
func Watch(ctx context.Context, opts WatchOptions) (<-chan Event, <-chan error, error) {
	w, err := NewWatcher(opts)
	if err != nil {
		return nil, nil, err
	}
	events, errs := w.Watch(ctx)
	return events, errs, nil
}
