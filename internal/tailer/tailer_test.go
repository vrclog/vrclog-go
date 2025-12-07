package tailer

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTailer_NewLines(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.log")

	// Create file
	f, err := os.Create(logFile)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	// Start tailer
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tailer, err := New(ctx, logFile, DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer tailer.Stop()

	// Write a line
	f.WriteString("line1\n")
	f.Sync()

	// Verify reception
	select {
	case line := <-tailer.Lines():
		if line != "line1" {
			t.Errorf("got %q, want %q", line, "line1")
		}
	case <-time.After(2 * time.Second):
		t.Error("timeout waiting for line")
	}
}

func TestTailer_MultipleLines(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.log")

	f, err := os.Create(logFile)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tailer, err := New(ctx, logFile, DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer tailer.Stop()

	// Give tailer a moment to start watching
	time.Sleep(100 * time.Millisecond)

	// Write multiple lines one by one with sync
	lines := []string{"line1", "line2", "line3"}
	for i, line := range lines {
		f.WriteString(line + "\n")
		f.Sync()

		// Verify each line is received in order
		select {
		case got := <-tailer.Lines():
			if got != line {
				t.Errorf("line %d: got %q, want %q", i, got, line)
			}
		case <-time.After(2 * time.Second):
			t.Errorf("timeout waiting for line %d: %q", i, line)
		}
	}
}

func TestTailer_FromStart(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.log")

	// Create file with existing content
	if err := os.WriteFile(logFile, []byte("existing1\nexisting2\n"), 0644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := DefaultConfig()
	cfg.FromStart = true

	tailer, err := New(ctx, logFile, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer tailer.Stop()

	// Should receive existing lines
	expected := []string{"existing1", "existing2"}
	for _, want := range expected {
		select {
		case got := <-tailer.Lines():
			if got != want {
				t.Errorf("got %q, want %q", got, want)
			}
		case <-time.After(2 * time.Second):
			t.Errorf("timeout waiting for line %q", want)
		}
	}
}

func TestTailer_Stop(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.log")

	f, err := os.Create(logFile)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tailer, err := New(ctx, logFile, DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}

	// Stop should close channels
	tailer.Stop()

	select {
	case _, ok := <-tailer.Lines():
		if ok {
			t.Error("expected Lines channel to be closed")
		}
	case <-time.After(time.Second):
		t.Error("timeout waiting for Lines channel to close")
	}
}

func TestTailer_StopMultipleTimes(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.log")

	f, err := os.Create(logFile)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tailer, err := New(ctx, logFile, DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}

	// Multiple Stop calls should be safe
	if err := tailer.Stop(); err != nil {
		t.Errorf("first Stop() error = %v", err)
	}
	if err := tailer.Stop(); err != nil {
		t.Errorf("second Stop() error = %v", err)
	}
}

func TestTailer_ContextCancel(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.log")

	f, err := os.Create(logFile)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	ctx, cancel := context.WithCancel(context.Background())

	tailer, err := New(ctx, logFile, DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer tailer.Stop()

	// Cancel context
	cancel()

	// Channels should close
	select {
	case _, ok := <-tailer.Lines():
		if ok {
			t.Error("expected Lines channel to be closed after context cancel")
		}
	case <-time.After(2 * time.Second):
		t.Error("timeout waiting for Lines channel to close")
	}
}

func TestTailer_FileNotExists(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, err := New(ctx, "/nonexistent/path/file.log", DefaultConfig())
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}
