package vrclog_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/vrclog/vrclog-go/pkg/vrclog"
)

func TestParseLine(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantType vrclog.EventType
		wantNil  bool
	}{
		{
			name:     "player join",
			input:    "2024.01.15 23:59:59 Log        -  [Behaviour] OnPlayerJoined TestUser",
			wantType: vrclog.EventPlayerJoin,
		},
		{
			name:    "unrecognized line returns nil",
			input:   "some random text",
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := vrclog.ParseLine(tt.input)
			if err != nil {
				t.Fatalf("ParseLine() error = %v", err)
			}

			if tt.wantNil {
				if got != nil {
					t.Errorf("ParseLine() = %+v, want nil", got)
				}
				return
			}

			if got == nil {
				t.Fatal("ParseLine() = nil, want non-nil")
				return
			}
			if got.Type != tt.wantType {
				t.Errorf("ParseLine().Type = %v, want %v", got.Type, tt.wantType)
			}
		})
	}
}

func TestNewWatcherWithOptions_ValidLogDir(t *testing.T) {
	dir := t.TempDir()
	// Create a log file
	logFile := filepath.Join(dir, "output_log_test.txt")
	if err := os.WriteFile(logFile, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	watcher, err := vrclog.NewWatcherWithOptions(
		vrclog.WithLogDir(dir),
	)
	if err != nil {
		t.Fatalf("NewWatcherWithOptions() error = %v", err)
	}
	defer watcher.Close()
}

func TestWatcher_ReceivesEvents(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "output_log_test.txt")

	f, err := os.Create(logFile)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	watcher, err := vrclog.NewWatcherWithOptions(
		vrclog.WithLogDir(dir),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer watcher.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	events, errs, err := watcher.Watch(ctx)
	if err != nil {
		t.Fatalf("Watch() error = %v", err)
	}

	// Give watcher time to start
	time.Sleep(100 * time.Millisecond)

	// Write a log line
	f.WriteString("2024.01.15 23:59:59 Log        -  [Behaviour] OnPlayerJoined TestUser\n")
	f.Sync()

	// Verify event reception
	select {
	case event := <-events:
		if event.Type != vrclog.EventPlayerJoin {
			t.Errorf("got type %v, want %v", event.Type, vrclog.EventPlayerJoin)
		}
		if event.PlayerName != "TestUser" {
			t.Errorf("got player %q, want %q", event.PlayerName, "TestUser")
		}
	case err := <-errs:
		t.Fatalf("unexpected error: %v", err)
	case <-ctx.Done():
		t.Fatal("timeout waiting for event")
	}
}

func TestWatcher_ContextCancel(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "output_log_test.txt")

	if err := os.WriteFile(logFile, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	watcher, err := vrclog.NewWatcherWithOptions(
		vrclog.WithLogDir(dir),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer watcher.Close()

	ctx, cancel := context.WithCancel(context.Background())
	events, _, err := watcher.Watch(ctx)
	if err != nil {
		t.Fatalf("Watch() error = %v", err)
	}

	// Cancel context
	cancel()

	// Channels should close
	select {
	case _, ok := <-events:
		if ok {
			t.Error("expected events channel to be closed")
		}
	case <-time.After(2 * time.Second):
		t.Error("timeout waiting for events channel to close")
	}
}

func TestWatcher_Close(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "output_log_test.txt")

	if err := os.WriteFile(logFile, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	watcher, err := vrclog.NewWatcherWithOptions(
		vrclog.WithLogDir(dir),
	)
	if err != nil {
		t.Fatal(err)
	}

	// Close() should be safe to call multiple times
	if err := watcher.Close(); err != nil {
		t.Errorf("first Close() error = %v", err)
	}
	if err := watcher.Close(); err != nil {
		t.Errorf("second Close() error = %v", err)
	}
}

func TestWatcher_CloseStopsGoroutine(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "output_log_test.txt")

	if err := os.WriteFile(logFile, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	watcher, err := vrclog.NewWatcherWithOptions(
		vrclog.WithLogDir(dir),
	)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	events, _, err := watcher.Watch(ctx)
	if err != nil {
		t.Fatalf("Watch() error = %v", err)
	}

	// Give goroutine time to start
	time.Sleep(50 * time.Millisecond)

	// Close should stop the goroutine and close channels
	done := make(chan struct{})
	go func() {
		watcher.Close()
		close(done)
	}()

	// Verify Close() returns (doesn't hang)
	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("Close() timed out")
	}

	// Verify channels are closed
	select {
	case _, ok := <-events:
		if ok {
			t.Error("expected events channel to be closed")
		}
	case <-time.After(time.Second):
		t.Error("timeout waiting for events channel to close")
	}
}

func TestWatcher_WatchAfterClose(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "output_log_test.txt")

	if err := os.WriteFile(logFile, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	watcher, err := vrclog.NewWatcherWithOptions(
		vrclog.WithLogDir(dir),
	)
	if err != nil {
		t.Fatal(err)
	}

	// Close before watching
	watcher.Close()

	// Watch after close should return ErrWatcherClosed
	ctx := context.Background()
	events, errs, err := watcher.Watch(ctx)

	if err == nil {
		t.Fatal("Watch() after Close() should return error")
	}
	if !errors.Is(err, vrclog.ErrWatcherClosed) {
		t.Errorf("Watch() error = %v, want %v", err, vrclog.ErrWatcherClosed)
	}
	if events != nil {
		t.Error("expected events channel to be nil")
	}
	if errs != nil {
		t.Error("expected errs channel to be nil")
	}
}

func TestWatcher_WatchCalledTwice(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "output_log_test.txt")

	if err := os.WriteFile(logFile, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	watcher, err := vrclog.NewWatcherWithOptions(
		vrclog.WithLogDir(dir),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer watcher.Close()

	ctx := context.Background()
	events1, _, err := watcher.Watch(ctx)
	if err != nil {
		t.Fatalf("first Watch() error = %v", err)
	}

	// Second Watch should return ErrAlreadyWatching
	events2, errs2, err := watcher.Watch(ctx)
	if err == nil {
		t.Fatal("second Watch() should return error")
	}
	if !errors.Is(err, vrclog.ErrAlreadyWatching) {
		t.Errorf("second Watch() error = %v, want %v", err, vrclog.ErrAlreadyWatching)
	}
	if events2 != nil {
		t.Error("expected second events channel to be nil")
	}
	if errs2 != nil {
		t.Error("expected second errs channel to be nil")
	}

	// First events channel should still be open
	// (until Close() or context cancel)
	select {
	case <-events1:
		t.Error("first events channel should not have received anything yet")
	case <-time.After(100 * time.Millisecond):
		// Expected - no events yet
	}
}

func TestNewWatcherWithOptions_Validation(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "output_log_test.txt")
	if err := os.WriteFile(logFile, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		opts    []vrclog.WatchOption
		wantErr bool
	}{
		{
			name:    "default options are valid",
			opts:    []vrclog.WatchOption{vrclog.WithLogDir(dir)},
			wantErr: false,
		},
		{
			name: "ReplayLastN is valid",
			opts: []vrclog.WatchOption{
				vrclog.WithLogDir(dir),
				vrclog.WithReplayLastN(100),
			},
			wantErr: false,
		},
		{
			name: "ReplaySinceTime is valid",
			opts: []vrclog.WatchOption{
				vrclog.WithLogDir(dir),
				vrclog.WithReplaySinceTime(time.Now().Add(-time.Hour)),
			},
			wantErr: false,
		},
		{
			name: "negative LastN is invalid",
			opts: []vrclog.WatchOption{
				vrclog.WithLogDir(dir),
				vrclog.WithReplayLastN(-1),
			},
			wantErr: true,
		},
		{
			name: "ReplaySinceTime with zero Since is invalid",
			opts: []vrclog.WatchOption{
				vrclog.WithLogDir(dir),
				vrclog.WithReplay(vrclog.ReplayConfig{
					Mode: vrclog.ReplaySinceTime,
					// Since is zero
				}),
			},
			wantErr: true,
		},
		{
			name: "negative PollInterval is invalid",
			opts: []vrclog.WatchOption{
				vrclog.WithLogDir(dir),
				vrclog.WithPollInterval(-time.Second),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			watcher, err := vrclog.NewWatcherWithOptions(tt.opts...)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewWatcherWithOptions() error = %v, wantErr %v", err, tt.wantErr)
			}
			if watcher != nil {
				watcher.Close()
			}
		})
	}
}

func TestWatcher_IncludeRawLine(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "output_log_test.txt")

	f, err := os.Create(logFile)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	watcher, err := vrclog.NewWatcherWithOptions(
		vrclog.WithLogDir(dir),
		vrclog.WithIncludeRawLine(true),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer watcher.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	events, errs, err := watcher.Watch(ctx)
	if err != nil {
		t.Fatalf("Watch() error = %v", err)
	}

	// Give watcher time to start
	time.Sleep(100 * time.Millisecond)

	rawLine := "2024.01.15 23:59:59 Log        -  [Behaviour] OnPlayerJoined TestUser"
	f.WriteString(rawLine + "\n")
	f.Sync()

	select {
	case event := <-events:
		if event.RawLine != rawLine {
			t.Errorf("got RawLine %q, want %q", event.RawLine, rawLine)
		}
	case err := <-errs:
		t.Fatalf("unexpected error: %v", err)
	case <-ctx.Done():
		t.Fatal("timeout waiting for event")
	}
}

func TestWatcher_ReplayFromStart(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "output_log_test.txt")

	// Write existing content
	content := "2024.01.15 12:00:00 Log        -  [Behaviour] OnPlayerJoined ExistingUser\n"
	if err := os.WriteFile(logFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	watcher, err := vrclog.NewWatcherWithOptions(
		vrclog.WithLogDir(dir),
		vrclog.WithReplayFromStart(),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer watcher.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	events, errs, err := watcher.Watch(ctx)
	if err != nil {
		t.Fatalf("Watch() error = %v", err)
	}

	// Should receive existing event
	select {
	case event := <-events:
		if event.PlayerName != "ExistingUser" {
			t.Errorf("got player %q, want %q", event.PlayerName, "ExistingUser")
		}
	case err := <-errs:
		t.Fatalf("unexpected error: %v", err)
	case <-ctx.Done():
		t.Fatal("timeout waiting for existing event")
	}
}

func TestWatcher_ReplayLastN(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "output_log_test.txt")

	// Write 5 events
	content := `2024.01.15 12:00:00 Log        -  [Behaviour] OnPlayerJoined User1
2024.01.15 12:00:01 Log        -  [Behaviour] OnPlayerJoined User2
2024.01.15 12:00:02 Log        -  [Behaviour] OnPlayerJoined User3
2024.01.15 12:00:03 Log        -  [Behaviour] OnPlayerJoined User4
2024.01.15 12:00:04 Log        -  [Behaviour] OnPlayerJoined User5
`
	if err := os.WriteFile(logFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	watcher, err := vrclog.NewWatcherWithOptions(
		vrclog.WithLogDir(dir),
		vrclog.WithReplayLastN(2), // Only last 2 events
	)
	if err != nil {
		t.Fatal(err)
	}
	defer watcher.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	events, errs, err := watcher.Watch(ctx)
	if err != nil {
		t.Fatalf("Watch() error = %v", err)
	}

	// Should receive last 2 events (User4 and User5)
	expected := []string{"User4", "User5"}
	for i, want := range expected {
		select {
		case event := <-events:
			if event.PlayerName != want {
				t.Errorf("event %d: got player %q, want %q", i, event.PlayerName, want)
			}
		case err := <-errs:
			t.Fatalf("unexpected error: %v", err)
		case <-ctx.Done():
			t.Fatalf("timeout waiting for event %d", i)
		}
	}
}

func TestWatcher_ReplaySinceTime(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "output_log_test.txt")

	// Write events with different timestamps
	content := `2024.01.15 12:00:00 Log        -  [Behaviour] OnPlayerJoined OldUser1
2024.01.15 12:00:01 Log        -  [Behaviour] OnPlayerJoined OldUser2
2024.01.15 14:00:00 Log        -  [Behaviour] OnPlayerJoined NewUser1
2024.01.15 14:00:01 Log        -  [Behaviour] OnPlayerJoined NewUser2
`
	if err := os.WriteFile(logFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Replay since 13:00 (should get NewUser1 and NewUser2)
	since, _ := time.ParseInLocation("2006.01.02 15:04:05", "2024.01.15 13:00:00", time.Local)

	watcher, err := vrclog.NewWatcherWithOptions(
		vrclog.WithLogDir(dir),
		vrclog.WithReplaySinceTime(since),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer watcher.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	events, errs, err := watcher.Watch(ctx)
	if err != nil {
		t.Fatalf("Watch() error = %v", err)
	}

	// Should receive only events after 13:00 (NewUser1, NewUser2)
	expected := []string{"NewUser1", "NewUser2"}
	for i, want := range expected {
		select {
		case event := <-events:
			if event.PlayerName != want {
				t.Errorf("event %d: got player %q, want %q", i, event.PlayerName, want)
			}
		case err := <-errs:
			t.Fatalf("unexpected error: %v", err)
		case <-ctx.Done():
			t.Fatalf("timeout waiting for event %d", i)
		}
	}
}
