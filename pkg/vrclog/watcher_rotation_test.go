package vrclog_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/vrclog/vrclog-go/pkg/vrclog"
)

// TestWatcher_LogRotation tests that the watcher detects log rotation
// and switches to the new log file.
func TestWatcher_LogRotation(t *testing.T) {
	dir := t.TempDir()

	// Create initial log file with older timestamp
	oldLogFile := filepath.Join(dir, "output_log_2024-01-15_10-00-00.txt")
	f1, err := os.Create(oldLogFile)
	if err != nil {
		t.Fatal(err)
	}
	defer f1.Close()

	// Create watcher with short poll interval for faster test
	watcher, err := vrclog.NewWatcherWithOptions(
		vrclog.WithLogDir(dir),
		vrclog.WithPollInterval(100*time.Millisecond), // Check for rotation every 100ms
	)
	if err != nil {
		t.Fatal(err)
	}
	defer watcher.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	events, errs, err := watcher.Watch(ctx)
	if err != nil {
		t.Fatalf("Watch() error = %v", err)
	}

	// Give watcher time to start
	time.Sleep(200 * time.Millisecond)

	// Write initial event to old file after watcher starts
	f1.WriteString("2024.01.15 10:00:01 Log        -  [Behaviour] OnPlayerJoined OldUser\n")
	f1.Sync()

	// Verify initial event
	select {
	case event := <-events:
		if event.Type != vrclog.EventPlayerJoin {
			t.Errorf("initial event: got type %v, want %v", event.Type, vrclog.EventPlayerJoin)
		}
		if event.PlayerName != "OldUser" {
			t.Errorf("initial event: got player %q, want %q", event.PlayerName, "OldUser")
		}
	case err := <-errs:
		t.Fatalf("unexpected error before rotation: %v", err)
	case <-ctx.Done():
		t.Fatal("timeout waiting for initial event")
	}

	// Now simulate log rotation by creating a newer log file
	newLogFile := filepath.Join(dir, "output_log_2024-01-15_12-00-00.txt")
	f2, err := os.Create(newLogFile)
	if err != nil {
		t.Fatal(err)
	}
	defer f2.Close()

	// Write event to new file
	f2.WriteString("2024.01.15 12:00:01 Log        -  [Behaviour] OnPlayerJoined NewUser\n")
	f2.Sync()

	// Close old file (simulates VRChat closing the old log)
	f1.Close()

	// Wait for rotation detection (pollInterval is 100ms, give it time)
	time.Sleep(300 * time.Millisecond)

	// Verify event from new file is received
	select {
	case event := <-events:
		if event.Type != vrclog.EventPlayerJoin {
			t.Errorf("rotated event: got type %v, want %v", event.Type, vrclog.EventPlayerJoin)
		}
		if event.PlayerName != "NewUser" {
			t.Errorf("rotated event: got player %q, want %q", event.PlayerName, "NewUser")
		}
	case err := <-errs:
		t.Fatalf("unexpected error after rotation: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for event from rotated log file")
	}

	// Write another event to new file to confirm watcher is using it
	f2.WriteString("2024.01.15 12:00:02 Log        -  [Behaviour] OnPlayerLeft NewUser\n")
	f2.Sync()

	select {
	case event := <-events:
		if event.Type != vrclog.EventPlayerLeft {
			t.Errorf("second event: got type %v, want %v", event.Type, vrclog.EventPlayerLeft)
		}
		if event.PlayerName != "NewUser" {
			t.Errorf("second event: got player %q, want %q", event.PlayerName, "NewUser")
		}
	case err := <-errs:
		t.Fatalf("unexpected error for second event: %v", err)
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for second event from rotated log file")
	}
}

// TestWatcher_LogRotationWithReplay tests that replay works correctly
// when starting on an older log file.
func TestWatcher_LogRotationWithReplay(t *testing.T) {
	dir := t.TempDir()

	// Create initial log file with existing content
	oldLogFile := filepath.Join(dir, "output_log_2024-01-15_10-00-00.txt")
	f1, err := os.Create(oldLogFile)
	if err != nil {
		t.Fatal(err)
	}

	// Write multiple events to old file
	f1.WriteString("2024.01.15 10:00:01 Log        -  [Behaviour] OnPlayerJoined User1\n")
	f1.WriteString("2024.01.15 10:00:02 Log        -  [Behaviour] OnPlayerJoined User2\n")
	f1.Sync()
	f1.Close()

	// Create watcher with replay
	watcher, err := vrclog.NewWatcherWithOptions(
		vrclog.WithLogDir(dir),
		vrclog.WithPollInterval(100*time.Millisecond),
		vrclog.WithReplayFromStart(), // Replay all events
	)
	if err != nil {
		t.Fatal(err)
	}
	defer watcher.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	events, errs, err := watcher.Watch(ctx)
	if err != nil {
		t.Fatalf("Watch() error = %v", err)
	}

	// Verify replayed events
	for i := 1; i <= 2; i++ {
		select {
		case event := <-events:
			if event.Type != vrclog.EventPlayerJoin {
				t.Errorf("replayed event %d: got type %v, want %v", i, event.Type, vrclog.EventPlayerJoin)
			}
		case err := <-errs:
			t.Fatalf("unexpected error during replay: %v", err)
		case <-time.After(1 * time.Second):
			t.Fatalf("timeout waiting for replayed event %d", i)
		}
	}

	// Create newer log file
	newLogFile := filepath.Join(dir, "output_log_2024-01-15_12-00-00.txt")
	f2, err := os.Create(newLogFile)
	if err != nil {
		t.Fatal(err)
	}
	defer f2.Close()

	// Write event to new file
	f2.WriteString("2024.01.15 12:00:01 Log        -  [Behaviour] OnPlayerJoined User3\n")
	f2.Sync()

	// Wait for rotation detection
	time.Sleep(300 * time.Millisecond)

	// Verify event from new file
	select {
	case event := <-events:
		if event.Type != vrclog.EventPlayerJoin {
			t.Errorf("new file event: got type %v, want %v", event.Type, vrclog.EventPlayerJoin)
		}
		if event.PlayerName != "User3" {
			t.Errorf("new file event: got player %q, want %q", event.PlayerName, "User3")
		}
	case err := <-errs:
		t.Fatalf("unexpected error after rotation: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for event from new file")
	}
}

// TestWatcher_LogRotationContinuesOnError tests that if creating a new
// tailer fails during rotation, the watcher continues with the old file.
func TestWatcher_LogRotationContinuesOnError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod does not restrict file access on Windows")
	}

	dir := t.TempDir()

	// Create initial log file
	oldLogFile := filepath.Join(dir, "output_log_2024-01-15_10-00-00.txt")
	f1, err := os.Create(oldLogFile)
	if err != nil {
		t.Fatal(err)
	}
	defer f1.Close()

	watcher, err := vrclog.NewWatcherWithOptions(
		vrclog.WithLogDir(dir),
		vrclog.WithPollInterval(100*time.Millisecond),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer watcher.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	events, errs, err := watcher.Watch(ctx)
	if err != nil {
		t.Fatalf("Watch() error = %v", err)
	}

	// Wait for watcher to start
	time.Sleep(200 * time.Millisecond)

	// Write initial event after watcher starts
	f1.WriteString("2024.01.15 10:00:01 Log        -  [Behaviour] OnPlayerJoined User1\n")
	f1.Sync()
	select {
	case event := <-events:
		if event.PlayerName != "User1" {
			t.Errorf("initial event: got player %q, want %q", event.PlayerName, "User1")
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for initial event")
	}

	// Create a newer log file but make it unreadable to simulate rotation error
	newLogFile := filepath.Join(dir, "output_log_2024-01-15_12-00-00.txt")
	f2, err := os.Create(newLogFile)
	if err != nil {
		t.Fatal(err)
	}
	f2.Close()

	// Make the new file unreadable (Unix only)
	if err := os.Chmod(newLogFile, 0000); err != nil {
		t.Skip("chmod not supported on this platform")
	}
	defer os.Chmod(newLogFile, 0644) // Cleanup

	// Wait for rotation attempt (should fail and log error)
	time.Sleep(300 * time.Millisecond)

	// Should receive an error about failing to open new file
	select {
	case err := <-errs:
		// Good - we expect an error when trying to open the unreadable file
		t.Logf("expected error received: %v", err)
	case <-time.After(500 * time.Millisecond):
		// It's also OK if no error is received immediately
		t.Log("no error received yet (might be buffered or not yet detected)")
	}

	// Watcher should still be working with old file
	// Write another event to old file
	f1.WriteString("2024.01.15 10:00:02 Log        -  [Behaviour] OnPlayerJoined User2\n")
	f1.Sync()

	// Verify we still receive events from old file
	select {
	case event := <-events:
		if event.PlayerName != "User2" {
			t.Errorf("continued event: got player %q, want %q", event.PlayerName, "User2")
		}
		t.Log("successfully received event from old file after rotation error")
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for event from old file after rotation error - watcher may have stopped")
	}
}

// TestWatcher_MultipleRotations tests handling of multiple log rotations
func TestWatcher_MultipleRotations(t *testing.T) {
	dir := t.TempDir()

	// Create initial log file
	logFile1 := filepath.Join(dir, "output_log_2024-01-15_10-00-00.txt")
	f1, err := os.Create(logFile1)
	if err != nil {
		t.Fatal(err)
	}

	watcher, err := vrclog.NewWatcherWithOptions(
		vrclog.WithLogDir(dir),
		vrclog.WithPollInterval(100*time.Millisecond),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer watcher.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	events, errs, err := watcher.Watch(ctx)
	if err != nil {
		t.Fatalf("Watch() error = %v", err)
	}

	// Helper to receive next event
	receiveEvent := func(expectedPlayer string) {
		t.Helper()
		select {
		case event := <-events:
			if event.Type != vrclog.EventPlayerJoin {
				t.Errorf("got type %v, want %v", event.Type, vrclog.EventPlayerJoin)
			}
			if event.PlayerName != expectedPlayer {
				t.Errorf("got player %q, want %q", event.PlayerName, expectedPlayer)
			}
		case err := <-errs:
			t.Fatalf("unexpected error: %v", err)
		case <-time.After(2 * time.Second):
			t.Fatalf("timeout waiting for event with player %s", expectedPlayer)
		}
	}

	time.Sleep(200 * time.Millisecond)

	// Write first event after watcher starts
	f1.WriteString("2024.01.15 10:00:01 Log        -  [Behaviour] OnPlayerJoined User1\n")
	f1.Sync()

	receiveEvent("User1")

	// First rotation
	logFile2 := filepath.Join(dir, "output_log_2024-01-15_11-00-00.txt")
	f2, err := os.Create(logFile2)
	if err != nil {
		t.Fatal(err)
	}
	f2.WriteString("2024.01.15 11:00:01 Log        -  [Behaviour] OnPlayerJoined User2\n")
	f2.Sync()
	f1.Close()

	time.Sleep(300 * time.Millisecond)
	receiveEvent("User2")

	// Second rotation
	logFile3 := filepath.Join(dir, "output_log_2024-01-15_12-00-00.txt")
	f3, err := os.Create(logFile3)
	if err != nil {
		t.Fatal(err)
	}
	defer f3.Close()
	f3.WriteString("2024.01.15 12:00:01 Log        -  [Behaviour] OnPlayerJoined User3\n")
	f3.Sync()
	f2.Close()

	time.Sleep(300 * time.Millisecond)
	receiveEvent("User3")

	// Third rotation
	logFile4 := filepath.Join(dir, "output_log_2024-01-15_13-00-00.txt")
	f4, err := os.Create(logFile4)
	if err != nil {
		t.Fatal(err)
	}
	defer f4.Close()
	f4.WriteString("2024.01.15 13:00:01 Log        -  [Behaviour] OnPlayerJoined User4\n")
	f4.Sync()
	f3.Close()

	time.Sleep(300 * time.Millisecond)
	receiveEvent("User4")

	t.Log("successfully handled 3 log rotations")
}
