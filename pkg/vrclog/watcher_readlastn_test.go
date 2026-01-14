package vrclog

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadLastNLines_Normal(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.txt")

	content := "line1\nline2\nline3\nline4\nline5\n"
	if err := os.WriteFile(logFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	lines, err := readLastNLines(context.Background(), logFile, 3, 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"line3", "line4", "line5"}
	if len(lines) != len(want) {
		t.Fatalf("got %d lines, want %d", len(lines), len(want))
	}
	for i, got := range lines {
		if got != want[i] {
			t.Errorf("line %d: got %q, want %q", i, got, want[i])
		}
	}
}

func TestReadLastNLines_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "empty.txt")

	if err := os.WriteFile(logFile, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	lines, err := readLastNLines(context.Background(), logFile, 10, 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(lines) != 0 {
		t.Errorf("got %d lines, want 0", len(lines))
	}
}

func TestReadLastNLines_FewerThanN(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.txt")

	content := "line1\nline2\n"
	if err := os.WriteFile(logFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	lines, err := readLastNLines(context.Background(), logFile, 10, 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"line1", "line2"}
	if len(lines) != len(want) {
		t.Fatalf("got %d lines, want %d", len(lines), len(want))
	}
	for i, got := range lines {
		if got != want[i] {
			t.Errorf("line %d: got %q, want %q", i, got, want[i])
		}
	}
}

func TestReadLastNLines_ExactlyN(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.txt")

	content := "line1\nline2\nline3\n"
	if err := os.WriteFile(logFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	lines, err := readLastNLines(context.Background(), logFile, 3, 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"line1", "line2", "line3"}
	if len(lines) != len(want) {
		t.Fatalf("got %d lines, want %d", len(lines), len(want))
	}
	for i, got := range lines {
		if got != want[i] {
			t.Errorf("line %d: got %q, want %q", i, got, want[i])
		}
	}
}

func TestReadLastNLines_NoTrailingNewline(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.txt")

	// No trailing newline on last line
	content := "line1\nline2\nline3"
	if err := os.WriteFile(logFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	lines, err := readLastNLines(context.Background(), logFile, 2, 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"line2", "line3"}
	if len(lines) != len(want) {
		t.Fatalf("got %d lines, want %d", len(lines), len(want))
	}
	for i, got := range lines {
		if got != want[i] {
			t.Errorf("line %d: got %q, want %q", i, got, want[i])
		}
	}
}

func TestReadLastNLines_EmptyLinesMixed(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.txt")

	// Empty lines should be skipped
	content := "line1\n\nline2\n\n\nline3\n"
	if err := os.WriteFile(logFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	lines, err := readLastNLines(context.Background(), logFile, 10, 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should only get non-empty lines
	want := []string{"line1", "line2", "line3"}
	if len(lines) != len(want) {
		t.Fatalf("got %d lines, want %d", len(lines), len(want))
	}
	for i, got := range lines {
		if got != want[i] {
			t.Errorf("line %d: got %q, want %q", i, got, want[i])
		}
	}
}

func TestReadLastNLines_CRLF(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.txt")

	// Windows-style CRLF line endings
	content := "line1\r\nline2\r\nline3\r\n"
	if err := os.WriteFile(logFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	lines, err := readLastNLines(context.Background(), logFile, 2, 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// \r should be stripped
	want := []string{"line2", "line3"}
	if len(lines) != len(want) {
		t.Fatalf("got %d lines, want %d", len(lines), len(want))
	}
	for i, got := range lines {
		if got != want[i] {
			t.Errorf("line %d: got %q, want %q", i, got, want[i])
		}
		// Verify no \r in result
		if strings.Contains(got, "\r") {
			t.Errorf("line %d: contains \\r", i)
		}
	}
}

func TestReadLastNLines_MaxBytesExceeded(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.txt")

	// Create file with ~100 bytes
	content := "line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10\n"
	if err := os.WriteFile(logFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Set maxBytes to 50 bytes (less than file size)
	_, err := readLastNLines(context.Background(), logFile, 10, 50, 0)
	if !errors.Is(err, ErrReplayLimitExceeded) {
		t.Errorf("expected ErrReplayLimitExceeded, got %v", err)
	}
}

func TestReadLastNLines_MaxLineBytesExceeded(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.txt")

	// Create file with one giant line
	giantLine := strings.Repeat("x", 1024) // 1KB line
	content := "line1\n" + giantLine + "\nline3\n"
	if err := os.WriteFile(logFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Set maxLineBytes to 512 bytes
	_, err := readLastNLines(context.Background(), logFile, 10, 0, 512)
	if !errors.Is(err, ErrReplayLimitExceeded) {
		t.Errorf("expected ErrReplayLimitExceeded, got %v", err)
	}
}

func TestReadLastNLines_GiantLineNoNewline(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.txt")

	// Single giant line with no newline
	giantLine := strings.Repeat("x", 10000)
	if err := os.WriteFile(logFile, []byte(giantLine), 0644); err != nil {
		t.Fatal(err)
	}

	// Set maxLineBytes to 5000 bytes
	_, err := readLastNLines(context.Background(), logFile, 1, 0, 5000)
	if !errors.Is(err, ErrReplayLimitExceeded) {
		t.Errorf("expected ErrReplayLimitExceeded, got %v", err)
	}
}

func TestReadLastNLines_MultipleChunks(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.txt")

	// Create file large enough to span multiple chunks (chunkSize = 4096)
	// Each line is ~50 bytes, so 100 lines = ~5000 bytes
	var lines []string
	for i := 1; i <= 100; i++ {
		lines = append(lines, strings.Repeat("x", 40))
	}
	content := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(logFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Request last 10 lines
	result, err := readLastNLines(context.Background(), logFile, 10, 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 10 {
		t.Errorf("got %d lines, want 10", len(result))
	}

	// All lines should be the same (40 x's)
	want := strings.Repeat("x", 40)
	for i, got := range result {
		if got != want {
			t.Errorf("line %d: got length %d, want %d", i, len(got), len(want))
		}
	}
}

func TestReadLastNLines_FileNotFound(t *testing.T) {
	_, err := readLastNLines(context.Background(), "/nonexistent/file.txt", 10, 0, 0)
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestReadLastNLines_VRChatLogFormat(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.txt")

	// Real VRChat log format
	content := `2024.01.15 12:00:00 Log        -  [Behaviour] OnPlayerJoined User1
2024.01.15 12:00:01 Log        -  [Behaviour] OnPlayerJoined User2
2024.01.15 12:00:02 Log        -  [Behaviour] OnPlayerJoined User3
2024.01.15 12:00:03 Log        -  [Behaviour] OnPlayerJoined User4
2024.01.15 12:00:04 Log        -  [Behaviour] OnPlayerJoined User5
`
	if err := os.WriteFile(logFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	lines, err := readLastNLines(context.Background(), logFile, 2, 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(lines))
	}

	// Verify we got the last 2 lines (User4 and User5)
	if !strings.Contains(lines[0], "User4") {
		t.Errorf("line 0 doesn't contain User4: %q", lines[0])
	}
	if !strings.Contains(lines[1], "User5") {
		t.Errorf("line 1 doesn't contain User5: %q", lines[1])
	}
}
