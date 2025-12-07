// Package tailer provides file tailing functionality for VRChat log files.
package tailer

import (
	"context"
	"fmt"
	"sync"

	"github.com/nxadm/tail"
)

// tailerErrBuffer is the buffer size for the error channel.
// A small buffer prevents error loss during brief moments when the consumer
// is busy processing lines.
const tailerErrBuffer = 16

// Tailer wraps nxadm/tail for VRChat log file tailing.
type Tailer struct {
	t      *tail.Tail
	ctx    context.Context
	cancel context.CancelFunc
	lines  chan string
	errors chan error
	doneCh chan struct{}

	mu      sync.Mutex
	stopped bool
}

// Config holds configuration for tailing.
type Config struct {
	// Follow continues reading as the file grows (tail -f).
	Follow bool

	// ReOpen reopens the file when it's truncated or recreated (tail -F).
	ReOpen bool

	// Poll uses polling instead of inotify (more compatible but less efficient).
	Poll bool

	// MustExist requires the file to exist before starting (false = wait for creation).
	MustExist bool

	// FromStart reads from the beginning of the file instead of the end.
	FromStart bool
}

// DefaultConfig returns the default configuration for VRChat logs.
func DefaultConfig() Config {
	return Config{
		Follow:    true,
		ReOpen:    true,
		Poll:      false, // Use inotify/ReadDirectoryChangesW when available
		MustExist: true,
		FromStart: false, // Start from end (tail -f behavior)
	}
}

// New creates a new Tailer for the specified file.
// The provided context controls the tailer's lifecycle.
func New(ctx context.Context, filepath string, cfg Config) (*Tailer, error) {
	// Determine seek location
	location := &tail.SeekInfo{Offset: 0, Whence: 2} // End of file
	if cfg.FromStart {
		location = &tail.SeekInfo{Offset: 0, Whence: 0} // Start of file
	}

	t, err := tail.TailFile(filepath, tail.Config{
		Follow:    cfg.Follow,
		ReOpen:    cfg.ReOpen,
		Poll:      cfg.Poll,
		MustExist: cfg.MustExist,
		Location:  location,
	})
	if err != nil {
		return nil, fmt.Errorf("opening tail: %w", err)
	}

	ctx, cancel := context.WithCancel(ctx)

	tailer := &Tailer{
		t:      t,
		ctx:    ctx,
		cancel: cancel,
		lines:  make(chan string),
		errors: make(chan error, tailerErrBuffer),
		doneCh: make(chan struct{}),
	}

	go tailer.run()

	return tailer, nil
}

// Lines returns a channel that receives log lines.
func (t *Tailer) Lines() <-chan string {
	return t.lines
}

// Errors returns a channel that receives errors from tailing.
// Errors are sent non-blocking; if the channel is not read, errors are dropped.
func (t *Tailer) Errors() <-chan error {
	return t.errors
}

// Stop stops tailing and closes all channels.
// Safe to call multiple times.
func (t *Tailer) Stop() error {
	t.mu.Lock()
	if t.stopped {
		t.mu.Unlock()
		return nil
	}
	t.stopped = true
	t.mu.Unlock()

	t.cancel()
	<-t.doneCh // Wait for run() to finish
	return t.t.Stop()
}

func (t *Tailer) run() {
	defer close(t.doneCh)
	defer close(t.lines)
	defer close(t.errors)

	for {
		select {
		case <-t.ctx.Done():
			return
		case line, ok := <-t.t.Lines:
			if !ok {
				return
			}
			if line.Err != nil {
				// Send error with context awareness
				// With buffered channel, errors are only dropped if buffer is full
				select {
				case t.errors <- fmt.Errorf("tail: %w", line.Err):
				case <-t.ctx.Done():
					return
				default:
					// Drop error only if buffer is full (rare with buffer size 16)
				}
				continue
			}
			select {
			case t.lines <- line.Text:
			case <-t.ctx.Done():
				return
			}
		}
	}
}
