package vrclog

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWatcher_WaitForLogs_Immediate(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "output_log_test.txt")

	// Create log file with an event
	content := `2024.01.15 12:00:00 Log        -  [Behaviour] OnPlayerJoined TestUser
`
	if err := os.WriteFile(logFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// WithWaitForLogs should work even if logs already exist
	watcher, err := NewWatcherWithOptions(
		WithLogDir(dir),
		WithWaitForLogs(true),
		WithReplayFromStart(), // Read existing content
	)
	if err != nil {
		t.Fatal(err)
	}
	defer watcher.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	events, errs, err := watcher.Watch(ctx)
	if err != nil {
		t.Fatalf("Watch() error = %v", err)
	}

	// Should receive the event immediately
	select {
	case event := <-events:
		if event.PlayerName != "TestUser" {
			t.Errorf("got player %q, want %q", event.PlayerName, "TestUser")
		}
	case err := <-errs:
		t.Fatalf("unexpected error: %v", err)
	case <-ctx.Done():
		t.Fatal("timeout waiting for event")
	}
}

func TestWatcher_WaitForLogs_False(t *testing.T) {
	dir := t.TempDir()

	// Start watcher with empty directory and waitForLogs=false
	watcher, err := NewWatcherWithOptions(
		WithLogDir(dir),
		WithWaitForLogs(false), // Explicitly false
	)
	if err != nil {
		t.Fatal(err)
	}
	defer watcher.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	events, errs, err := watcher.Watch(ctx)
	if err != nil {
		t.Fatalf("Watch() error = %v", err)
	}

	// Should get ErrNoLogFiles immediately
	select {
	case event := <-events:
		t.Errorf("unexpected event: %+v", event)
	case err := <-errs:
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		// Should be a WatchError wrapping ErrNoLogFiles
		var watchErr *WatchError
		if !errors.As(err, &watchErr) {
			t.Errorf("expected WatchError, got %T: %v", err, err)
		}
		if !errors.Is(err, ErrNoLogFiles) {
			t.Errorf("expected ErrNoLogFiles, got: %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected immediate error, got timeout")
	}
}

func TestWatcher_WaitForLogs_Default(t *testing.T) {
	dir := t.TempDir()

	// Start watcher with empty directory and default (no WithWaitForLogs)
	watcher, err := NewWatcherWithOptions(
		WithLogDir(dir),
		// Default waitForLogs should be false
	)
	if err != nil {
		t.Fatal(err)
	}
	defer watcher.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	events, errs, err := watcher.Watch(ctx)
	if err != nil {
		t.Fatalf("Watch() error = %v", err)
	}

	// Should get ErrNoLogFiles immediately (default is false)
	select {
	case event := <-events:
		t.Errorf("unexpected event: %+v", event)
	case err := <-errs:
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, ErrNoLogFiles) {
			t.Errorf("expected ErrNoLogFiles, got: %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected immediate error, got timeout")
	}
}

func TestWatcher_WaitForLogs_ContextCancel(t *testing.T) {
	dir := t.TempDir()

	watcher, err := NewWatcherWithOptions(
		WithLogDir(dir),
		WithWaitForLogs(true),
		WithPollInterval(100*time.Millisecond),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer watcher.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	events, errs, err := watcher.Watch(ctx)
	if err != nil {
		t.Fatalf("Watch() error = %v", err)
	}

	// Wait for context to be cancelled (no log file created)
	select {
	case event := <-events:
		t.Errorf("unexpected event: %+v", event)
	case err := <-errs:
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		// Should get context.DeadlineExceeded
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("expected context.DeadlineExceeded, got: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("expected context cancellation error")
	}
}
