package vrclog

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/vrclog/vrclog-go/internal/logfinder"
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

// watcherErrBuffer is the buffer size for the error channel.
// A small buffer prevents error loss during brief moments when the consumer
// is busy processing events, while keeping memory usage minimal.
const watcherErrBuffer = 16

// ReplayConfig configures replay behavior.
// Only one mode can be active at a time (mutually exclusive).
type ReplayConfig struct {
	Mode  ReplayMode
	LastN int       // For ReplayLastN
	Since time.Time // For ReplaySinceTime
}

// Watcher monitors VRChat log files.
type Watcher struct {
	cfg    watchConfig // internal configuration (immutable after creation)
	logDir string
	log    *slog.Logger

	mu       sync.Mutex
	closed   bool
	cancel   context.CancelFunc // cancel func to stop the goroutine
	doneCh   chan struct{}      // signals when goroutine has exited
	watching bool               // true if Watch() has been called
}

// discardLogger returns a logger that discards all output.
var discardLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

// Watch starts watching and returns channels.
// Starts internal goroutines here.
// When ctx is cancelled, channels are closed automatically.
// Both channels close on ctx.Done() or fatal error.
// Watch can only be called once per Watcher instance.
//
// Returns ErrWatcherClosed if the watcher has been closed.
// Returns ErrAlreadyWatching if Watch() has already been called.
func (w *Watcher) Watch(ctx context.Context) (<-chan Event, <-chan error, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return nil, nil, ErrWatcherClosed
	}
	if w.watching {
		return nil, nil, ErrAlreadyWatching
	}
	w.watching = true

	// Create cancellable context
	ctx, cancel := context.WithCancel(ctx)
	w.cancel = cancel
	w.doneCh = make(chan struct{})

	eventCh := make(chan Event)
	errCh := make(chan error, watcherErrBuffer)

	go w.run(ctx, eventCh, errCh)

	return eventCh, errCh, nil
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
		sendError(ctx, errCh, &WatchError{Op: WatchOpFindLatest, Err: err})
		return
	}
	w.log.Debug("found latest log file", "path", logFile)

	// Configure tailer
	cfg := tailer.DefaultConfig()
	// For ReplayFromStart and ReplaySinceTime, read from start
	// For ReplayLastN, we handle it specially below
	cfg.FromStart = w.cfg.replay.Mode == ReplayFromStart || w.cfg.replay.Mode == ReplaySinceTime

	// Handle ReplayLastN: read last N lines first, then tail from end
	if w.cfg.replay.Mode == ReplayLastN && w.cfg.replay.LastN > 0 {
		w.log.Debug("replaying last N lines", "n", w.cfg.replay.LastN, "path", logFile)
		if err := w.replayLastN(ctx, logFile, eventCh, errCh); err != nil {
			sendError(ctx, errCh, &WatchError{Op: WatchOpReplay, Path: logFile, Err: err})
		}
		cfg.FromStart = false // Continue from end after replay
	}

	// Start tailer
	t, err := tailer.New(ctx, logFile, cfg)
	if err != nil {
		sendError(ctx, errCh, &WatchError{Op: WatchOpTail, Path: logFile, Err: err})
		return
	}
	w.log.Debug("started tailing", "path", logFile, "from_start", cfg.FromStart)

	// Set poll interval for log rotation check (defaultWatchConfig guarantees valid interval)
	rotationTicker := time.NewTicker(w.cfg.pollInterval)
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
			sendError(ctx, errCh, err)
		case <-rotationTicker.C:
			// Check for new log file (log rotation)
			newFile, err := logfinder.FindLatestLogFile(w.logDir)
			if err != nil {
				sendError(ctx, errCh, &WatchError{Op: WatchOpRotation, Err: err})
				continue
			}
			if newFile != currentFile {
				// New log file found, switch to it
				w.log.Debug("log rotation detected", "from", currentFile, "to", newFile)
				_ = t.Stop()
				cfg := tailer.DefaultConfig()
				cfg.FromStart = true // Read new file from start
				newTailer, err := tailer.New(ctx, newFile, cfg)
				if err != nil {
					sendError(ctx, errCh, &WatchError{Op: WatchOpTail, Path: newFile, Err: err})
					continue
				}
				t = newTailer
				currentFile = newFile
			}
		}
	}
}

func (w *Watcher) processLine(ctx context.Context, line string, eventCh chan<- Event, errCh chan<- error) {
	result, err := w.cfg.parser.ParseLine(ctx, line)

	// Process events even if there's an error (e.g., ChainContinueOnError mode)
	// This ensures partial success from multi-parser chains is not lost
	hasEvents := len(result.Events) > 0

	if err != nil {
		// Send error but still process any events we got
		if hasEvents {
			// Process events first, then send the error
			// This allows ChainContinueOnError to emit events + errors
			for _, ev := range result.Events {
				// Apply same filters as below
				if w.cfg.replay.Mode == ReplaySinceTime && ev.Timestamp.Before(w.cfg.replay.Since) {
					continue
				}
				if w.cfg.filter != nil && !w.cfg.filter.Allows(EventType(ev.Type)) {
					continue
				}
				if w.cfg.includeRawLine {
					ev.RawLine = line
				}
				select {
				case eventCh <- ev:
				case <-ctx.Done():
					return
				}
			}
		}
		sendError(ctx, errCh, &ParseError{Line: line, Err: err})
		return
	}

	if !result.Matched {
		return // Not a recognized event
	}

	// Process all events from the result
	for _, ev := range result.Events {
		// Filter by replay time if needed (do this early before other processing)
		if w.cfg.replay.Mode == ReplaySinceTime && ev.Timestamp.Before(w.cfg.replay.Since) {
			continue
		}

		// Apply event type filter (do this before copying RawLine for efficiency)
		if w.cfg.filter != nil && !w.cfg.filter.Allows(EventType(ev.Type)) {
			continue
		}

		// Include raw line if requested
		if w.cfg.includeRawLine {
			ev.RawLine = line
		}

		// Send event
		select {
		case eventCh <- ev:
		case <-ctx.Done():
			return
		}
	}
}

// replayLastN reads and processes the last N lines from the log file.
func (w *Watcher) replayLastN(ctx context.Context, logFile string, eventCh chan<- Event, errCh chan<- error) error {
	lines, err := readLastNLines(logFile, w.cfg.replay.LastN)
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

// sendError sends an error to the error channel.
// With a buffered channel, errors are only dropped if the buffer is full.
// The context case ensures we don't block during shutdown.
func sendError(ctx context.Context, errCh chan<- error, err error) {
	if err == nil {
		return
	}
	select {
	case errCh <- err:
	case <-ctx.Done():
		// Don't block during shutdown
	default:
		// Drop error only if buffer is full (rare with buffer size 16)
	}
}

// WatchWithOptions creates a watcher using functional options and starts watching.
// This is the preferred way to create and start a watcher.
//
// Note: This function does not return the underlying Watcher, so callers cannot
// call Close() to perform synchronous shutdown. The watcher will stop when the
// context is cancelled. For more control over shutdown, use NewWatcherWithOptions
// and Watcher.Watch() directly.
//
// Example:
//
//	events, errs, err := vrclog.WatchWithOptions(ctx,
//	    vrclog.WithIncludeTypes(vrclog.EventPlayerJoin, vrclog.EventPlayerLeft),
//	    vrclog.WithLogger(logger),
//	)
func WatchWithOptions(ctx context.Context, opts ...WatchOption) (<-chan Event, <-chan error, error) {
	w, err := NewWatcherWithOptions(opts...)
	if err != nil {
		return nil, nil, err
	}
	return w.Watch(ctx)
}

// NewWatcherWithOptions creates a watcher using functional options.
// Validates options and checks log directory existence.
// Does NOT start goroutines (cheap to call).
// Returns error for invalid options or missing log directory.
//
// Example:
//
//	watcher, err := vrclog.NewWatcherWithOptions(
//	    vrclog.WithLogDir("/custom/path"),
//	    vrclog.WithIncludeTypes(vrclog.EventPlayerJoin),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	events, errs, err := watcher.Watch(ctx)
func NewWatcherWithOptions(opts ...WatchOption) (*Watcher, error) {
	cfg := applyWatchOptions(opts)

	// Validate configuration
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid options: %w", err)
	}

	// Find log directory
	logDir, err := logfinder.FindLogDir(cfg.logDir)
	if err != nil {
		return nil, fmt.Errorf("finding log directory: %w", err)
	}

	// Initialize logger (use discard logger if not provided)
	log := cfg.logger
	if log == nil {
		log = discardLogger
	}

	return &Watcher{
		cfg:    *cfg, // copy to ensure immutability
		logDir: logDir,
		log:    log,
	}, nil
}
