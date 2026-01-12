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

func TestNewWatcherWithOptions_Basic(t *testing.T) {
	dir := t.TempDir()
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

func TestNewWatcherWithOptions_InvalidLogDir(t *testing.T) {
	_, err := vrclog.NewWatcherWithOptions(
		vrclog.WithLogDir("/nonexistent/path"),
	)

	if err == nil {
		t.Error("NewWatcherWithOptions() expected error for invalid log dir")
	}
	if !errors.Is(err, vrclog.ErrLogDirNotFound) {
		t.Errorf("NewWatcherWithOptions() error = %v, want %v", err, vrclog.ErrLogDirNotFound)
	}
}

func TestWatchWithOptions_Basic(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "output_log_test.txt")

	if err := os.WriteFile(logFile, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events, errs, err := vrclog.WatchWithOptions(ctx,
		vrclog.WithLogDir(dir),
	)
	if err != nil {
		t.Fatalf("WatchWithOptions() error = %v", err)
	}

	if events == nil {
		t.Error("WatchWithOptions() events channel is nil")
	}
	if errs == nil {
		t.Error("WatchWithOptions() errs channel is nil")
	}
}

func TestWatchWithOptions_WithIncludeTypes(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "output_log_test.txt")

	f, err := os.Create(logFile)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	events, errs, err := vrclog.WatchWithOptions(ctx,
		vrclog.WithLogDir(dir),
		vrclog.WithIncludeTypes(vrclog.EventPlayerJoin), // Only player join
	)
	if err != nil {
		t.Fatalf("WatchWithOptions() error = %v", err)
	}

	// Give watcher time to start
	time.Sleep(100 * time.Millisecond)

	// Write both player_join and player_left
	f.WriteString("2024.01.15 12:00:00 Log        -  [Behaviour] OnPlayerJoined User1\n")
	f.WriteString("2024.01.15 12:00:01 Log        -  [Behaviour] OnPlayerLeft User1\n")
	f.WriteString("2024.01.15 12:00:02 Log        -  [Behaviour] OnPlayerJoined User2\n")
	f.Sync()

	// Should only receive player_join events
	received := make([]vrclog.Event, 0)
	timeout := time.After(2 * time.Second)

loop:
	for {
		select {
		case event, ok := <-events:
			if !ok {
				break loop
			}
			received = append(received, event)
			if len(received) >= 2 {
				break loop
			}
		case err := <-errs:
			t.Fatalf("unexpected error: %v", err)
		case <-timeout:
			break loop
		}
	}

	// Verify we received only player_join events
	if len(received) != 2 {
		t.Errorf("got %d events, want 2", len(received))
	}
	for i, ev := range received {
		if ev.Type != vrclog.EventPlayerJoin {
			t.Errorf("event %d: got type %v, want %v", i, ev.Type, vrclog.EventPlayerJoin)
		}
	}
}

func TestWatchWithOptions_WithExcludeTypes(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "output_log_test.txt")

	f, err := os.Create(logFile)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	events, errs, err := vrclog.WatchWithOptions(ctx,
		vrclog.WithLogDir(dir),
		vrclog.WithExcludeTypes(vrclog.EventPlayerLeft), // Exclude player left
	)
	if err != nil {
		t.Fatalf("WatchWithOptions() error = %v", err)
	}

	// Give watcher time to start
	time.Sleep(100 * time.Millisecond)

	// Write both player_join and player_left
	f.WriteString("2024.01.15 12:00:00 Log        -  [Behaviour] OnPlayerJoined User1\n")
	f.WriteString("2024.01.15 12:00:01 Log        -  [Behaviour] OnPlayerLeft User1\n")
	f.WriteString("2024.01.15 12:00:02 Log        -  [Behaviour] OnPlayerJoined User2\n")
	f.Sync()

	// Should not receive player_left events
	received := make([]vrclog.Event, 0)
	timeout := time.After(2 * time.Second)

loop:
	for {
		select {
		case event, ok := <-events:
			if !ok {
				break loop
			}
			received = append(received, event)
			if len(received) >= 2 {
				break loop
			}
		case err := <-errs:
			t.Fatalf("unexpected error: %v", err)
		case <-timeout:
			break loop
		}
	}

	// Verify we didn't receive player_left events
	for i, ev := range received {
		if ev.Type == vrclog.EventPlayerLeft {
			t.Errorf("event %d: should not be player_left", i)
		}
	}
}

func TestWatchWithOptions_ReplayOptions(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "output_log_test.txt")

	// Write existing content
	content := `2024.01.15 12:00:00 Log        -  [Behaviour] OnPlayerJoined User1
2024.01.15 12:00:01 Log        -  [Behaviour] OnPlayerJoined User2
2024.01.15 12:00:02 Log        -  [Behaviour] OnPlayerJoined User3
`
	if err := os.WriteFile(logFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	events, errs, err := vrclog.WatchWithOptions(ctx,
		vrclog.WithLogDir(dir),
		vrclog.WithReplayLastN(2), // Last 2 lines
	)
	if err != nil {
		t.Fatalf("WatchWithOptions() error = %v", err)
	}

	// Should receive last 2 events (User2 and User3)
	expected := []string{"User2", "User3"}
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

func TestWatchWithOptions_ReplayFromStart(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "output_log_test.txt")

	content := "2024.01.15 12:00:00 Log        -  [Behaviour] OnPlayerJoined ExistingUser\n"
	if err := os.WriteFile(logFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	events, errs, err := vrclog.WatchWithOptions(ctx,
		vrclog.WithLogDir(dir),
		vrclog.WithReplayFromStart(),
	)
	if err != nil {
		t.Fatalf("WatchWithOptions() error = %v", err)
	}

	select {
	case event := <-events:
		if event.PlayerName != "ExistingUser" {
			t.Errorf("got player %q, want ExistingUser", event.PlayerName)
		}
	case err := <-errs:
		t.Fatalf("unexpected error: %v", err)
	case <-ctx.Done():
		t.Fatal("timeout waiting for event")
	}
}

func TestNewWatcherWithOptions_ZeroPollInterval(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "output_log_test.txt")

	if err := os.WriteFile(logFile, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := vrclog.NewWatcherWithOptions(
		vrclog.WithLogDir(dir),
		vrclog.WithPollInterval(0), // Zero poll interval should error
	)

	if err == nil {
		t.Error("NewWatcherWithOptions() expected error for zero poll interval")
	}
	if err != nil && err.Error() != "invalid options: poll interval must be positive, got 0s" {
		t.Errorf("NewWatcherWithOptions() error = %q, want error about positive poll interval", err)
	}
}

func TestNewWatcherWithOptions_NegativePollInterval(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "output_log_test.txt")

	if err := os.WriteFile(logFile, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := vrclog.NewWatcherWithOptions(
		vrclog.WithLogDir(dir),
		vrclog.WithPollInterval(-1*time.Second), // Negative poll interval should error
	)

	if err == nil {
		t.Error("NewWatcherWithOptions() expected error for negative poll interval")
	}
	if err != nil && err.Error() != "invalid options: poll interval must be positive, got -1s" {
		t.Errorf("NewWatcherWithOptions() error = %q, want error about positive poll interval", err)
	}
}
