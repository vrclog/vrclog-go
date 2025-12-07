package vrclog

import (
	"context"
	"fmt"
	"os"
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

// DefaultMaxReplayLastN is the default maximum lines for ReplayLastN mode.
// This limits memory usage to roughly tens of MB for typical VRChat logs.
const DefaultMaxReplayLastN = 10000

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

	// MaxReplayLines is the maximum lines to replay in ReplayLastN mode.
	// 0 uses default (10000). Set to -1 for unlimited (not recommended).
	MaxReplayLines int
}

// Validate checks for invalid option combinations.
func (o WatchOptions) Validate() error {
	// Validate ReplayLastN
	if o.Replay.Mode == ReplayLastN && o.Replay.LastN < 0 {
		return fmt.Errorf("replay LastN must be non-negative, got %d", o.Replay.LastN)
	}

	// Validate ReplayLastN against maximum limit
	if o.Replay.Mode == ReplayLastN {
		maxLines := o.MaxReplayLines
		if maxLines == 0 {
			maxLines = DefaultMaxReplayLastN
		}
		if maxLines > 0 && o.Replay.LastN > maxLines {
			return fmt.Errorf("replay LastN (%d) exceeds maximum of %d", o.Replay.LastN, maxLines)
		}
	}

	// Validate ReplaySinceTime
	if o.Replay.Mode == ReplaySinceTime && o.Replay.Since.IsZero() {
		return fmt.Errorf("replay Since must be set when mode is ReplaySinceTime")
	}

	// Validate PollInterval
	if o.PollInterval < 0 {
		return fmt.Errorf("poll interval must be non-negative, got %v", o.PollInterval)
	}

	return nil
}

// Watcher monitors VRChat log files.
type Watcher struct {
	opts   WatchOptions
	logDir string

	mu       sync.Mutex
	closed   bool
	cancel   context.CancelFunc // cancel func to stop the goroutine
	doneCh   chan struct{}      // signals when goroutine has exited
	watching bool               // true if Watch() has been called
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
// Watch can only be called once per Watcher instance.
func (w *Watcher) Watch(ctx context.Context) (<-chan Event, <-chan error) {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		// Return closed channels if already closed
		eventCh := make(chan Event)
		errCh := make(chan error)
		close(eventCh)
		close(errCh)
		return eventCh, errCh
	}
	if w.watching {
		w.mu.Unlock()
		// Return closed channels if already watching
		eventCh := make(chan Event)
		errCh := make(chan error)
		close(eventCh)
		close(errCh)
		return eventCh, errCh
	}
	w.watching = true

	// Create cancellable context
	ctx, cancel := context.WithCancel(ctx)
	w.cancel = cancel
	w.doneCh = make(chan struct{})
	w.mu.Unlock()

	eventCh := make(chan Event)
	errCh := make(chan error)

	go w.run(ctx, eventCh, errCh)

	return eventCh, errCh
}

// Close stops the watcher and releases resources.
// Safe to call multiple times.
// Blocks until the goroutine has exited.
func (w *Watcher) Close() error {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return nil
	}
	w.closed = true

	// Cancel the context to stop the goroutine
	if w.cancel != nil {
		w.cancel()
	}
	doneCh := w.doneCh
	w.mu.Unlock()

	// Wait for goroutine to exit if Watch was called
	if doneCh != nil {
		<-doneCh
	}
	return nil
}

func (w *Watcher) run(ctx context.Context, eventCh chan<- Event, errCh chan<- error) {
	defer close(w.doneCh) // Signal that goroutine has exited
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
	// For ReplayFromStart and ReplaySinceTime, read from start
	// For ReplayLastN, we handle it specially below
	cfg.FromStart = w.opts.Replay.Mode == ReplayFromStart || w.opts.Replay.Mode == ReplaySinceTime

	// Handle ReplayLastN: read last N lines first, then tail from end
	if w.opts.Replay.Mode == ReplayLastN && w.opts.Replay.LastN > 0 {
		if err := w.replayLastN(ctx, logFile, eventCh, errCh); err != nil {
			sendError(errCh, fmt.Errorf("replaying last N lines: %w", err))
		}
		cfg.FromStart = false // Continue from end after replay
	}

	// Start tailer
	t, err := tailer.New(ctx, logFile, cfg)
	if err != nil {
		sendError(errCh, fmt.Errorf("starting tailer: %w", err))
		return
	}

	// Set poll interval for log rotation check
	pollInterval := w.opts.PollInterval
	if pollInterval <= 0 {
		pollInterval = 2 * time.Second // Default
	}
	rotationTicker := time.NewTicker(pollInterval)
	defer rotationTicker.Stop()
	defer func() { _ = t.Stop() }()

	currentFile := logFile

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
		case <-rotationTicker.C:
			// Check for new log file (log rotation)
			newFile, err := logfinder.FindLatestLogFile(w.logDir)
			if err != nil {
				sendError(errCh, fmt.Errorf("checking for new log file: %w", err))
				continue
			}
			if newFile != currentFile {
				// New log file found, switch to it
				_ = t.Stop()
				cfg := tailer.DefaultConfig()
				cfg.FromStart = true // Read new file from start
				newTailer, err := tailer.New(ctx, newFile, cfg)
				if err != nil {
					sendError(errCh, fmt.Errorf("switching to new log file: %w", err))
					continue
				}
				t = newTailer
				currentFile = newFile
			}
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

// replayLastN reads and processes the last N lines from the log file.
func (w *Watcher) replayLastN(ctx context.Context, logFile string, eventCh chan<- Event, errCh chan<- error) error {
	lines, err := readLastNLines(logFile, w.opts.Replay.LastN)
	if err != nil {
		return err
	}

	for _, line := range lines {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			w.processLine(ctx, line, eventCh, errCh)
		}
	}
	return nil
}

// readLastNLines reads the last N lines from a file.
// Returns lines in order (oldest first).
func readLastNLines(filepath string, n int) ([]string, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Get file size
	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}
	fileSize := stat.Size()

	if fileSize == 0 {
		return nil, nil
	}

	// Read from end in chunks
	const chunkSize = 4096
	var lines []string
	var buffer []byte
	offset := fileSize

	for len(lines) < n && offset > 0 {
		// Calculate read position
		readSize := int64(chunkSize)
		if offset < readSize {
			readSize = offset
		}
		offset -= readSize

		// Read chunk
		chunk := make([]byte, readSize)
		_, err := file.ReadAt(chunk, offset)
		if err != nil {
			return nil, err
		}

		// Prepend to buffer
		buffer = append(chunk, buffer...)

		// Extract complete lines from buffer
		lines = extractLines(buffer, n)
	}

	// If we have the entire file in buffer, extract all lines
	if offset == 0 && len(lines) < n {
		lines = extractLines(buffer, n)
	}

	return lines, nil
}

// extractLines extracts up to n lines from buffer, keeping only the last n.
// Returns lines in order (oldest first).
func extractLines(buffer []byte, n int) []string {
	var lines []string
	start := 0

	for i := 0; i < len(buffer); i++ {
		if buffer[i] == '\n' {
			line := string(buffer[start:i])
			// Remove trailing \r for CRLF
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}
			if line != "" {
				lines = append(lines, line)
			}
			start = i + 1
		}
	}

	// Handle last line without newline
	if start < len(buffer) {
		line := string(buffer[start:])
		if len(line) > 0 && line[len(line)-1] == '\r' {
			line = line[:len(line)-1]
		}
		if line != "" {
			lines = append(lines, line)
		}
	}

	// Keep only last n lines
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}

	return lines
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
