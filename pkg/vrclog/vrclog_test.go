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
			}
			if got.Type != tt.wantType {
				t.Errorf("ParseLine().Type = %v, want %v", got.Type, tt.wantType)
			}
		})
	}
}

func TestNewWatcher_InvalidLogDir(t *testing.T) {
	_, err := vrclog.NewWatcher(vrclog.WatchOptions{
		LogDir: "/nonexistent/path",
	})
	if err == nil {
		t.Error("NewWatcher() expected error for invalid log dir")
	}
	if !errors.Is(err, vrclog.ErrLogDirNotFound) {
		t.Errorf("NewWatcher() error = %v, want %v", err, vrclog.ErrLogDirNotFound)
	}
}

func TestNewWatcher_ValidLogDir(t *testing.T) {
	dir := t.TempDir()
	// Create a log file
	logFile := filepath.Join(dir, "output_log_test.txt")
	if err := os.WriteFile(logFile, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	watcher, err := vrclog.NewWatcher(vrclog.WatchOptions{
		LogDir: dir,
	})
	if err != nil {
		t.Fatalf("NewWatcher() error = %v", err)
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

	watcher, err := vrclog.NewWatcher(vrclog.WatchOptions{
		LogDir: dir,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer watcher.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	events, errs := watcher.Watch(ctx)

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

	watcher, err := vrclog.NewWatcher(vrclog.WatchOptions{
		LogDir: dir,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer watcher.Close()

	ctx, cancel := context.WithCancel(context.Background())
	events, _ := watcher.Watch(ctx)

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

	watcher, err := vrclog.NewWatcher(vrclog.WatchOptions{
		LogDir: dir,
	})
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

func TestWatchOptions_Validate(t *testing.T) {
	tests := []struct {
		name    string
		opts    vrclog.WatchOptions
		wantErr bool
	}{
		{
			name:    "zero value is valid",
			opts:    vrclog.WatchOptions{},
			wantErr: false,
		},
		{
			name: "ReplayLastN is valid",
			opts: vrclog.WatchOptions{
				Replay: vrclog.ReplayConfig{
					Mode:  vrclog.ReplayLastN,
					LastN: 100,
				},
			},
			wantErr: false,
		},
		{
			name: "ReplaySinceTime is valid",
			opts: vrclog.WatchOptions{
				Replay: vrclog.ReplayConfig{
					Mode:  vrclog.ReplaySinceTime,
					Since: time.Now().Add(-time.Hour),
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.opts.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestWatch_ConvenienceFunction(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "output_log_test.txt")

	if err := os.WriteFile(logFile, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events, errs, err := vrclog.Watch(ctx, vrclog.WatchOptions{
		LogDir: dir,
	})
	if err != nil {
		t.Fatalf("Watch() error = %v", err)
	}

	// Channels should be returned
	if events == nil {
		t.Error("Watch() events channel is nil")
	}
	if errs == nil {
		t.Error("Watch() errs channel is nil")
	}

	cancel() // Cleanup
}

func TestWatch_ConvenienceFunction_InvalidLogDir(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, _, err := vrclog.Watch(ctx, vrclog.WatchOptions{
		LogDir: "/nonexistent/path",
	})
	if err == nil {
		t.Error("Watch() expected error for invalid log dir")
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

	watcher, err := vrclog.NewWatcher(vrclog.WatchOptions{
		LogDir:         dir,
		IncludeRawLine: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer watcher.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	events, errs := watcher.Watch(ctx)

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

	watcher, err := vrclog.NewWatcher(vrclog.WatchOptions{
		LogDir: dir,
		Replay: vrclog.ReplayConfig{
			Mode: vrclog.ReplayFromStart,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer watcher.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	events, errs := watcher.Watch(ctx)

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
