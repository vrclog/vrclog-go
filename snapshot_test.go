package vrclog

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestCaptureLogSnapshot_EmptyDirectory(t *testing.T) {
	dir := t.TempDir()

	snap, err := CaptureLogSnapshot(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(snap.sizes) != 0 {
		t.Errorf("expected empty snapshot, got %d entries", len(snap.sizes))
	}
}

func TestLogSnapshot_ContainsPriorRecord(t *testing.T) {
	dir := t.TempDir()
	content := logLine("2024.01.01 00:00:01", "line one") +
		logLine("2024.01.01 00:00:02", "line two")
	path := writeLogFile(t, dir, "output_log_2024-01-01_00-00-00.txt", content)

	snap, err := CaptureLogSnapshot(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var records []Record
	for rec, err := range ReadFile(context.Background(), ReadFileConfig{Path: path}) {
		if err != nil {
			t.Fatal(err)
		}
		records = append(records, rec)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}

	for i, rec := range records {
		if !snap.Contains(rec) {
			t.Errorf("record %d: expected Contains == true for pre-capture record", i)
		}
	}
}

func TestLogSnapshot_DoesNotContainPostCaptureRecord(t *testing.T) {
	dir := t.TempDir()
	path := writeLogFile(t, dir, "output_log_2024-01-01_00-00-00.txt",
		logLine("2024.01.01 00:00:01", "line one"))

	snap, err := CaptureLogSnapshot(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	appendToFile(t, path, logLine("2024.01.01 00:00:02", "line two"))

	var records []Record
	for rec, err := range ReadFile(context.Background(), ReadFileConfig{Path: path}) {
		if err != nil {
			t.Fatal(err)
		}
		records = append(records, rec)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}

	if !snap.Contains(records[0]) {
		t.Error("expected Contains == true for pre-capture record")
	}
	if snap.Contains(records[1]) {
		t.Error("expected Contains == false for post-capture record")
	}
}

func TestLogSnapshot_DoesNotContainNewFile(t *testing.T) {
	dir := t.TempDir()
	writeLogFile(t, dir, "output_log_2024-01-01_00-00-00.txt",
		logLine("2024.01.01 00:00:01", "old file"))

	snap, err := CaptureLogSnapshot(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	newPath := writeLogFile(t, dir, "output_log_2024-01-02_00-00-00.txt",
		logLine("2024.01.02 00:00:01", "new file"))

	var records []Record
	for rec, err := range ReadFile(context.Background(), ReadFileConfig{Path: newPath}) {
		if err != nil {
			t.Fatal(err)
		}
		records = append(records, rec)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}

	if snap.Contains(records[0]) {
		t.Error("expected Contains == false for record from a file created after capture")
	}
}

func TestLogSnapshot_PartialLineCompletedAfterCapture(t *testing.T) {
	dir := t.TempDir()
	fullLine := logLine("2024.01.01 00:00:01", "half then whole")
	half := fullLine[:len(fullLine)/2]
	path := writeLogFile(t, dir, "output_log_2024-01-01_00-00-00.txt", half)

	snap, err := CaptureLogSnapshot(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	appendToFile(t, path, fullLine[len(fullLine)/2:])

	var records []Record
	for rec, err := range ReadFile(context.Background(), ReadFileConfig{Path: path}) {
		if err != nil {
			t.Fatal(err)
		}
		records = append(records, rec)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}

	// The record's NextOffset now exceeds the size captured while the
	// line was still incomplete, so it must not be reported as
	// pre-existing.
	if snap.Contains(records[0]) {
		t.Error("expected Contains == false for a record whose completing bytes arrived after capture")
	}
}

func TestLogSnapshot_SourceReplacedDifferentSourceID(t *testing.T) {
	dir := t.TempDir()
	writeLogFile(t, dir, "output_log_2024-01-01_00-00-00.txt",
		logLine("2024.01.01 00:00:01", "line one"))

	snap, err := CaptureLogSnapshot(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// A record from a source that was never part of the snapshot (e.g.
	// a file at a different path with a different SourceID) must not be
	// reported as pre-existing, regardless of offset.
	fake := Record{
		SourceID:   SourceID("nonexistent-source-id"),
		Offset:     0,
		NextOffset: 1,
	}
	if snap.Contains(fake) {
		t.Error("expected Contains == false for unknown SourceID")
	}
}

func TestLogSnapshot_SamePathReplacementWithinSize(t *testing.T) {
	// Documents the accepted API limitation: SourceID is derived from
	// the normalized path only, so replacing a file's content at the
	// same path (same or larger size) is indistinguishable from an
	// untouched file. VRChat's timestamped log filenames make this a
	// non-issue in practice.
	dir := t.TempDir()
	path := writeLogFile(t, dir, "output_log_2024-01-01_00-00-00.txt",
		logLine("2024.01.01 00:00:01", "original"))

	snap, err := CaptureLogSnapshot(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Replace file content at the same path with equal-length content.
	original := logLine("2024.01.01 00:00:01", "original")
	replaced := logLine("2024.01.01 00:00:01", "replaced")
	if len(original) != len(replaced) {
		t.Fatalf("test fixture requires equal-length replacement content")
	}
	if err := os.WriteFile(path, []byte(replaced), 0644); err != nil {
		t.Fatal(err)
	}

	var records []Record
	for rec, err := range ReadFile(context.Background(), ReadFileConfig{Path: path}) {
		if err != nil {
			t.Fatal(err)
		}
		records = append(records, rec)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}

	// Accepted false positive: same path, same size envelope.
	if !snap.Contains(records[0]) {
		t.Error("expected Contains == true (accepted false positive for same-path replacement within captured size)")
	}
}

func TestLogSnapshot_ReusesDirectoryNormalization(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "logs")
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatal(err)
	}
	writeLogFile(t, dir, "output_log_2024-01-01_00-00-00.txt",
		logLine("2024.01.01 00:00:01", "line"))

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWd)
	if err := os.Chdir(parent); err != nil {
		t.Fatal(err)
	}

	snap, err := CaptureLogSnapshot("logs")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(snap.sizes) != 1 {
		t.Fatalf("expected 1 source in snapshot, got %d", len(snap.sizes))
	}
}

func TestLogSnapshot_StrictDiscoveryPropagatesErrors(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission bits behave differently on windows")
	}
	if os.Getuid() == 0 {
		t.Skip("skipping permission test when running as root")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "output_log_2024-01-01_00-00-00.txt")
	if err := os.WriteFile(path, []byte(logLine("2024.01.01 00:00:01", "x")), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0000); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(path, 0644)

	_, err := CaptureLogSnapshot(dir)
	if err == nil {
		t.Fatal("expected error for unreadable candidate file")
	}
}
