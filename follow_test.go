package vrclog

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

const testPollInterval = MinPollInterval

func logLine(ts string, msg string) string {
	return fmt.Sprintf("%s Log        -  %s\n", ts, msg)
}

func writeLogFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return p
}

func appendToFile(t *testing.T, path, content string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
}

func collectRecords(t *testing.T, ctx context.Context, cfg FollowConfig, maxRecords int) ([]Record, []error) {
	t.Helper()
	var records []Record
	var errs []error
	for rec, err := range Follow(ctx, cfg) {
		if err != nil {
			errs = append(errs, err)
			break
		}
		records = append(records, rec)
		if len(records) >= maxRecords {
			break
		}
	}
	return records, errs
}

func TestFollow_ExistingFileFromOffset0(t *testing.T) {
	dir := t.TempDir()
	content := logLine("2024.01.01 00:00:01", "line one") +
		logLine("2024.01.01 00:00:02", "line two") +
		logLine("2024.01.01 00:00:03", "line three")
	writeLogFile(t, dir, "output_log_2024-01-01_00-00-00.txt", content)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	records, errs := collectRecords(t, ctx, FollowConfig{
		Directory:    dir,
		PollInterval: testPollInterval,
	}, 3)

	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(records) != 3 {
		t.Fatalf("expected 3 records, got %d", len(records))
	}
	if records[0].Message != "line one" {
		t.Errorf("records[0].Message = %q, want %q", records[0].Message, "line one")
	}
	if records[0].Offset != 0 {
		t.Errorf("records[0].Offset = %d, want 0 (should start from beginning)", records[0].Offset)
	}
	if records[2].Message != "line three" {
		t.Errorf("records[2].Message = %q, want %q", records[2].Message, "line three")
	}
}

func TestFollow_NoFilesWaitForCreation(t *testing.T) {
	dir := t.TempDir()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	done := make(chan struct{})
	var records []Record
	var errs []error

	go func() {
		defer close(done)
		for rec, err := range Follow(ctx, FollowConfig{
			Directory:    dir,
			PollInterval: testPollInterval,
		}) {
			if err != nil {
				errs = append(errs, err)
				return
			}
			records = append(records, rec)
			if len(records) >= 2 {
				return
			}
		}
	}()

	time.Sleep(100 * time.Millisecond)

	content := logLine("2024.01.01 00:00:01", "appeared one") +
		logLine("2024.01.01 00:00:02", "appeared two")
	writeLogFile(t, dir, "output_log_2024-01-01_00-00-00.txt", content)

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for records")
	}

	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
	if records[0].Offset != 0 {
		t.Errorf("should start from offset 0, got %d", records[0].Offset)
	}
	if records[0].Message != "appeared one" {
		t.Errorf("records[0].Message = %q, want %q", records[0].Message, "appeared one")
	}
}

func TestFollow_CursorResume(t *testing.T) {
	dir := t.TempDir()
	line1 := logLine("2024.01.01 00:00:01", "first")
	line2 := logLine("2024.01.01 00:00:02", "second")
	path := writeLogFile(t, dir, "output_log_2024-01-01_00-00-00.txt", line1+line2)

	ctx := context.Background()
	var cursor Cursor
	count := 0
	for rec, err := range ReadFile(ctx, ReadFileConfig{Path: path}) {
		if err != nil {
			t.Fatal(err)
		}
		count++
		if count == 1 {
			cursor = rec.Cursor()
		}
	}

	appendToFile(t, path, logLine("2024.01.01 00:00:03", "third"))

	ctxFollow, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	records, errs := collectRecords(t, ctxFollow, FollowConfig{
		Directory:    dir,
		Cursor:       &cursor,
		PollInterval: testPollInterval,
	}, 2)

	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(records) < 2 {
		t.Fatalf("expected at least 2 records, got %d", len(records))
	}
	if records[0].Message != "second" {
		t.Errorf("records[0].Message = %q, want %q (resume after first)", records[0].Message, "second")
	}
	if records[1].Message != "third" {
		t.Errorf("records[1].Message = %q, want %q", records[1].Message, "third")
	}
}

func TestFollow_CursorMissing(t *testing.T) {
	dir := t.TempDir()

	cursor := Cursor{
		SourceID: "nonexistent",
		Path:     filepath.Join(dir, "output_log_gone.txt"),
		Offset:   0,
		Line:     1,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var records []Record
	var errs []error
	for rec, err := range Follow(ctx, FollowConfig{
		Directory:    dir,
		Cursor:       &cursor,
		PollInterval: testPollInterval,
	}) {
		if err != nil {
			errs = append(errs, err)
		} else {
			records = append(records, rec)
		}
	}

	if len(records) != 0 {
		t.Errorf("expected 0 records, got %d", len(records))
	}
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if !errors.Is(errs[0], ErrCursorSourceMissing) {
		t.Errorf("expected ErrCursorSourceMissing, got %v", errs[0])
	}
}

func TestFollow_CursorOffsetBeyondFileSize(t *testing.T) {
	dir := t.TempDir()
	path := writeLogFile(t, dir, "output_log_2024-01-01_00-00-00.txt",
		logLine("2024.01.01 00:00:01", "line"))

	var rec Record
	for r, err := range ReadFile(context.Background(), ReadFileConfig{Path: path}) {
		if err != nil {
			t.Fatal(err)
		}
		rec = r
		break
	}

	cursor := rec.Cursor()
	cursor.Offset += 100

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	records, errs := collectRecords(t, ctx, FollowConfig{
		Directory:    dir,
		Cursor:       &cursor,
		PollInterval: testPollInterval,
	}, 1)

	if len(records) != 0 {
		t.Fatalf("expected 0 records, got %d", len(records))
	}
	if len(errs) != 1 || !errors.Is(errs[0], ErrInvalidOffset) {
		t.Fatalf("expected ErrInvalidOffset, got %v", errs)
	}
}

func TestFollow_AppendNewLines(t *testing.T) {
	dir := t.TempDir()
	path := writeLogFile(t, dir, "output_log_2024-01-01_00-00-00.txt",
		logLine("2024.01.01 00:00:01", "initial"))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	done := make(chan struct{})
	var records []Record

	go func() {
		defer close(done)
		for rec, err := range Follow(ctx, FollowConfig{
			Directory:    dir,
			PollInterval: testPollInterval,
		}) {
			if err != nil {
				return
			}
			records = append(records, rec)
			if len(records) >= 3 {
				return
			}
		}
	}()

	time.Sleep(100 * time.Millisecond)
	appendToFile(t, path, logLine("2024.01.01 00:00:02", "appended one"))
	time.Sleep(50 * time.Millisecond)
	appendToFile(t, path, logLine("2024.01.01 00:00:03", "appended two"))

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out")
	}

	if len(records) != 3 {
		t.Fatalf("expected 3 records, got %d", len(records))
	}
	if records[0].Message != "initial" {
		t.Errorf("records[0].Message = %q, want %q", records[0].Message, "initial")
	}
	if records[1].Message != "appended one" {
		t.Errorf("records[1].Message = %q, want %q", records[1].Message, "appended one")
	}
	if records[2].Message != "appended two" {
		t.Errorf("records[2].Message = %q, want %q", records[2].Message, "appended two")
	}
}

func TestFollow_SingleRotation(t *testing.T) {
	dir := t.TempDir()
	writeLogFile(t, dir, "output_log_2024-01-01_00-00-00.txt",
		logLine("2024.01.01 00:00:01", "old file line"))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	done := make(chan struct{})
	var records []Record

	go func() {
		defer close(done)
		for rec, err := range Follow(ctx, FollowConfig{
			Directory:    dir,
			PollInterval: testPollInterval,
		}) {
			if err != nil {
				return
			}
			records = append(records, rec)
			if len(records) >= 2 {
				return
			}
		}
	}()

	time.Sleep(200 * time.Millisecond)
	writeLogFile(t, dir, "output_log_2024-01-02_00-00-00.txt",
		logLine("2024.01.02 00:00:01", "new file line"))

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("timed out")
	}

	if len(records) < 2 {
		t.Fatalf("expected at least 2 records, got %d", len(records))
	}
	if records[0].Message != "old file line" {
		t.Errorf("records[0].Message = %q, want %q", records[0].Message, "old file line")
	}
	if records[1].Message != "new file line" {
		t.Errorf("records[1].Message = %q, want %q", records[1].Message, "new file line")
	}

	if records[0].Path == records[1].Path {
		t.Error("records should be from different files")
	}
}

func TestFollow_MultipleRotations(t *testing.T) {
	dir := t.TempDir()
	writeLogFile(t, dir, "output_log_2024-01-01_00-00-00.txt",
		logLine("2024.01.01 00:00:01", "file1"))

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	done := make(chan struct{})
	var records []Record

	go func() {
		defer close(done)
		for rec, err := range Follow(ctx, FollowConfig{
			Directory:    dir,
			PollInterval: testPollInterval,
		}) {
			if err != nil {
				return
			}
			records = append(records, rec)
			if len(records) >= 3 {
				return
			}
		}
	}()

	time.Sleep(200 * time.Millisecond)
	writeLogFile(t, dir, "output_log_2024-01-02_00-00-00.txt",
		logLine("2024.01.02 00:00:01", "file2"))

	time.Sleep(200 * time.Millisecond)
	writeLogFile(t, dir, "output_log_2024-01-03_00-00-00.txt",
		logLine("2024.01.03 00:00:01", "file3"))

	select {
	case <-done:
	case <-time.After(15 * time.Second):
		t.Fatal("timed out")
	}

	if len(records) != 3 {
		t.Fatalf("expected 3 records, got %d", len(records))
	}

	expected := []string{"file1", "file2", "file3"}
	for i, want := range expected {
		if records[i].Message != want {
			t.Errorf("records[%d].Message = %q, want %q", i, records[i].Message, want)
		}
	}
}

func TestFollow_ContextCancellation(t *testing.T) {
	dir := t.TempDir()
	writeLogFile(t, dir, "output_log_2024-01-01_00-00-00.txt",
		logLine("2024.01.01 00:00:01", "line"))

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	start := time.Now()
	for _, err := range Follow(ctx, FollowConfig{
		Directory:    dir,
		PollInterval: testPollInterval,
	}) {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	elapsed := time.Since(start)

	if elapsed > 3*time.Second {
		t.Errorf("Follow took %v after cancellation, expected prompt stop", elapsed)
	}
}

func TestFollow_NegativePollInterval(t *testing.T) {
	dir := t.TempDir()

	ctx := context.Background()
	var gotErr error
	for _, err := range Follow(ctx, FollowConfig{
		Directory:    dir,
		PollInterval: -1 * time.Second,
	}) {
		if err != nil {
			gotErr = err
			break
		}
	}

	if gotErr == nil {
		t.Fatal("expected error for negative poll interval")
	}
}

func TestFollow_PollIntervalBelowMinimum(t *testing.T) {
	dir := t.TempDir()

	ctx := context.Background()
	var gotErr error
	for _, err := range Follow(ctx, FollowConfig{
		Directory:    dir,
		PollInterval: MinPollInterval - time.Nanosecond,
	}) {
		if err != nil {
			gotErr = err
			break
		}
	}

	if gotErr == nil {
		t.Fatal("expected error for poll interval below minimum")
	}
}

func TestFollow_ZeroPollInterval(t *testing.T) {
	dir := t.TempDir()
	writeLogFile(t, dir, "output_log_2024-01-01_00-00-00.txt",
		logLine("2024.01.01 00:00:01", "line"))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	records, errs := collectRecords(t, ctx, FollowConfig{
		Directory:    dir,
		PollInterval: 0,
	}, 1)

	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
}

func TestFollow_GoroutineLeak(t *testing.T) {
	dir := t.TempDir()
	writeLogFile(t, dir, "output_log_2024-01-01_00-00-00.txt",
		logLine("2024.01.01 00:00:01", "line"))

	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	before := runtime.NumGoroutine()

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	for _, err := range Follow(ctx, FollowConfig{
		Directory:    dir,
		PollInterval: testPollInterval,
	}) {
		if err != nil {
			break
		}
	}

	time.Sleep(200 * time.Millisecond)
	runtime.GC()
	time.Sleep(50 * time.Millisecond)

	after := runtime.NumGoroutine()

	if after > before+2 {
		t.Errorf("possible goroutine leak: before=%d, after=%d", before, after)
	}
}

func TestFollow_CursorResumeNoDuplicates(t *testing.T) {
	dir := t.TempDir()
	line1 := logLine("2024.01.01 00:00:01", "alpha")
	line2 := logLine("2024.01.01 00:00:02", "beta")
	line3 := logLine("2024.01.01 00:00:03", "gamma")
	path := writeLogFile(t, dir, "output_log_2024-01-01_00-00-00.txt", line1+line2+line3)

	ctx := context.Background()
	var allRecords []Record
	for rec, err := range ReadFile(ctx, ReadFileConfig{Path: path}) {
		if err != nil {
			t.Fatal(err)
		}
		allRecords = append(allRecords, rec)
	}

	if len(allRecords) != 3 {
		t.Fatalf("expected 3 records from ReadFile, got %d", len(allRecords))
	}

	cursor := allRecords[1].Cursor()

	appendToFile(t, path, logLine("2024.01.01 00:00:04", "delta"))

	ctxFollow, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	records, errs := collectRecords(t, ctxFollow, FollowConfig{
		Directory:    dir,
		Cursor:       &cursor,
		PollInterval: testPollInterval,
	}, 2)

	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
	if records[0].Message != "gamma" {
		t.Errorf("records[0].Message = %q, want %q", records[0].Message, "gamma")
	}
	if records[1].Message != "delta" {
		t.Errorf("records[1].Message = %q, want %q", records[1].Message, "delta")
	}
}
