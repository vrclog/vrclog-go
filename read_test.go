package vrclog

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func writeLog(t *testing.T, dir string, lines ...string) string {
	t.Helper()
	path := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestReadFile_FullFile(t *testing.T) {
	dir := t.TempDir()
	path := writeLog(t, dir,
		"2026.08.18 12:00:00 Log        -  [Behaviour] Entering Room: Test World",
		"2026.08.18 12:00:01 Log        -  [Behaviour] OnPlayerJoined TestUser",
	)

	var records []Record
	for rec, err := range ReadFile(context.Background(), ReadFileConfig{Path: path}) {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		records = append(records, rec)
	}

	if len(records) != 2 {
		t.Fatalf("got %d records, want 2", len(records))
	}

	if records[0].Line != 1 {
		t.Errorf("first record line = %d, want 1", records[0].Line)
	}
	if records[0].Offset != 0 {
		t.Errorf("first record offset = %d, want 0", records[0].Offset)
	}
	if records[0].Level != LevelLog {
		t.Errorf("first record level = %q, want %q", records[0].Level, LevelLog)
	}
	if records[0].ID == "" {
		t.Error("first record ID is empty")
	}
	if records[0].SourceID == "" {
		t.Error("first record SourceID is empty")
	}
	if records[0].Time.IsZero() {
		t.Error("first record time is zero")
	}

	if records[1].Line != 2 {
		t.Errorf("second record line = %d, want 2", records[1].Line)
	}
	if records[1].Offset != records[0].NextOffset {
		t.Errorf("second record offset = %d, want %d", records[1].Offset, records[0].NextOffset)
	}
}

func TestReadFile_ResumeFromOffset(t *testing.T) {
	dir := t.TempDir()
	path := writeLog(t, dir,
		"2026.08.18 12:00:00 Log        -  line one",
		"2026.08.18 12:00:01 Log        -  line two",
		"2026.08.18 12:00:02 Log        -  line three",
	)

	// Read first record to get cursor
	var firstRec Record
	for rec, err := range ReadFile(context.Background(), ReadFileConfig{Path: path}) {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		firstRec = rec
		break
	}

	cursor := firstRec.Cursor()

	// Resume from cursor
	var records []Record
	for rec, err := range ReadFile(context.Background(), ReadFileConfig{
		Path:   cursor.Path,
		Offset: cursor.Offset,
		Line:   cursor.Line,
	}) {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		records = append(records, rec)
	}

	if len(records) != 2 {
		t.Fatalf("got %d records, want 2", len(records))
	}

	if records[0].Line != 2 {
		t.Errorf("resumed first record line = %d, want 2", records[0].Line)
	}
	if !strings.Contains(records[0].Message, "line two") {
		t.Errorf("resumed first record message = %q, expected to contain 'line two'", records[0].Message)
	}
}

func TestReadFile_NegativeOffset(t *testing.T) {
	dir := t.TempDir()
	path := writeLog(t, dir, "line1")

	var gotErr error
	for _, err := range ReadFile(context.Background(), ReadFileConfig{Path: path, Offset: -1}) {
		gotErr = err
		break
	}

	if gotErr == nil {
		t.Error("expected error for negative offset")
	}
}

func TestReadFile_OffsetBeyondFileSize(t *testing.T) {
	dir := t.TempDir()
	path := writeLog(t, dir, "line1")

	var gotErr error
	for _, err := range ReadFile(context.Background(), ReadFileConfig{Path: path, Offset: 100, Line: 2}) {
		gotErr = err
		break
	}

	if !errors.Is(gotErr, ErrInvalidOffset) {
		t.Fatalf("expected ErrInvalidOffset, got %v", gotErr)
	}
}

func TestReadFile_ContextCancellation(t *testing.T) {
	dir := t.TempDir()
	var lines []string
	for i := 0; i < 1000; i++ {
		lines = append(lines, "2026.08.18 12:00:00 Log        -  line content here")
	}
	path := writeLog(t, dir, lines...)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	count := 0
	for _, err := range ReadFile(ctx, ReadFileConfig{Path: path}) {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		count++
		if count >= 5 {
			cancel()
		}
	}

	if count >= 1000 {
		t.Errorf("expected early stop due to cancellation, got %d records", count)
	}
}

func TestReadFile_Break(t *testing.T) {
	dir := t.TempDir()
	path := writeLog(t, dir,
		"2026.08.18 12:00:00 Log        -  line one",
		"2026.08.18 12:00:01 Log        -  line two",
		"2026.08.18 12:00:02 Log        -  line three",
	)

	count := 0
	for _, err := range ReadFile(context.Background(), ReadFileConfig{Path: path}) {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		count++
		if count == 1 {
			break
		}
	}

	if count != 1 {
		t.Errorf("count = %d, want 1", count)
	}

	// Verify file is closed: we can read it again without issues
	count2 := 0
	for _, err := range ReadFile(context.Background(), ReadFileConfig{Path: path}) {
		if err != nil {
			t.Fatalf("re-read error: %v", err)
		}
		count2++
	}
	if count2 != 3 {
		t.Errorf("re-read count = %d, want 3", count2)
	}
}

func TestReadFile_OversizedLineIsNotFatalError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "big.txt")

	bigLine := strings.Repeat("X", maxLineSize+100)
	content := bigLine + "\n" + "2026.08.18 12:00:00 Log        -  normal line\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	var records []Record
	for rec, err := range ReadFile(context.Background(), ReadFileConfig{Path: path}) {
		if err != nil {
			t.Fatalf("oversized line should not be a fatal error: %v", err)
		}
		records = append(records, rec)
	}

	if len(records) != 2 {
		t.Fatalf("got %d records, want 2", len(records))
	}

	if records[0].Issue == nil {
		t.Error("first record should have an issue")
	} else if records[0].Issue.Code != "line_too_long" {
		t.Errorf("issue code = %q, want %q", records[0].Issue.Code, "line_too_long")
	}

	if records[1].Issue != nil {
		t.Error("second record should not have an issue")
	}
	if !strings.Contains(records[1].Message, "normal line") {
		t.Errorf("second record message = %q", records[1].Message)
	}
}

func TestReadFile_NonExistentPath(t *testing.T) {
	var gotErr error
	count := 0
	for _, err := range ReadFile(context.Background(), ReadFileConfig{Path: "/nonexistent/file.txt"}) {
		if err != nil {
			gotErr = err
		}
		count++
	}

	if gotErr == nil {
		t.Error("expected error for nonexistent path")
	}
	if count != 1 {
		t.Errorf("expected exactly 1 yield, got %d", count)
	}
}

func TestReadFile_RejectsSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink test requires Unix")
	}

	dir := t.TempDir()
	target := writeLog(t, dir, "line1")
	link := filepath.Join(dir, "link.txt")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	var gotErr error
	for _, err := range ReadFile(context.Background(), ReadFileConfig{Path: link}) {
		if err != nil {
			gotErr = err
		}
		break
	}

	if gotErr == nil {
		t.Error("expected error for symlink")
	}
}

func TestReadFile_EmptyPath(t *testing.T) {
	var gotErr error
	for _, err := range ReadFile(context.Background(), ReadFileConfig{Path: ""}) {
		if err != nil {
			gotErr = err
		}
		break
	}

	if gotErr == nil {
		t.Error("expected error for empty path")
	}
}

func TestReadFile_OffsetWithoutLine(t *testing.T) {
	dir := t.TempDir()
	path := writeLog(t, dir, "line1")

	var gotErr error
	for _, err := range ReadFile(context.Background(), ReadFileConfig{Path: path, Offset: 10, Line: 0}) {
		if err != nil {
			gotErr = err
		}
		break
	}

	if gotErr == nil {
		t.Error("expected error for offset > 0 with line == 0")
	}
}

func TestReadFile_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.txt")
	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	count := 0
	for _, err := range ReadFile(context.Background(), ReadFileConfig{Path: path}) {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		count++
	}

	if count != 0 {
		t.Errorf("expected 0 records for empty file, got %d", count)
	}
}

func TestReadFile_UnterminatedFinalLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "unterminated.txt")
	content := "2026.08.18 12:00:00 Log        -  [Behaviour] Entering Room: Test World\n" +
		"2026.08.18 12:00:01 Log        -  [Behaviour] OnPlayerJoined TestUser"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	var records []Record
	for rec, err := range ReadFile(context.Background(), ReadFileConfig{Path: path}) {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		records = append(records, rec)
	}

	if len(records) != 2 {
		t.Fatalf("got %d records, want 2 (ReadFile must flush the unterminated final line)", len(records))
	}
	if records[1].Message != "[Behaviour] OnPlayerJoined TestUser" {
		t.Errorf("records[1].Message = %q, want %q", records[1].Message, "[Behaviour] OnPlayerJoined TestUser")
	}
}

func TestReadFile_RelativePathYieldsAbsolutePath(t *testing.T) {
	dir := t.TempDir()
	path := writeLog(t, dir, "line1")

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWd)

	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	var rec Record
	for r, err := range ReadFile(context.Background(), ReadFileConfig{Path: filepath.Base(path)}) {
		if err != nil {
			t.Fatal(err)
		}
		rec = r
		break
	}

	if rec.Path == "" {
		t.Fatal("expected a record")
	}
	if !filepath.IsAbs(rec.Path) {
		t.Fatalf("Record.Path = %q, want absolute path", rec.Path)
	}
	want, err := filepath.Abs(filepath.Base(path))
	if err != nil {
		t.Fatal(err)
	}
	want = filepath.Clean(want)
	if rec.Path != want {
		t.Fatalf("Record.Path = %q, want %q", rec.Path, want)
	}
}

func TestReadFile_RecordIDDeterminism(t *testing.T) {
	dir := t.TempDir()
	path := writeLog(t, dir,
		"2026.08.18 12:00:00 Log        -  [Behaviour] test line",
	)

	var id1, id2 RecordID
	for rec, err := range ReadFile(context.Background(), ReadFileConfig{Path: path}) {
		if err != nil {
			t.Fatal(err)
		}
		id1 = rec.ID
	}
	for rec, err := range ReadFile(context.Background(), ReadFileConfig{Path: path}) {
		if err != nil {
			t.Fatal(err)
		}
		id2 = rec.ID
	}

	if id1 != id2 {
		t.Errorf("RecordID not deterministic across reads: %q != %q", id1, id2)
	}
}

func TestReadFile_HeaderParsing(t *testing.T) {
	dir := t.TempDir()
	path := writeLog(t, dir,
		"2026.08.18 12:00:00 Log        -  [Behaviour] Entering Room: Test World",
		"not a header line",
	)

	loc := time.FixedZone("TEST", 0)
	_ = loc

	var records []Record
	for rec, err := range ReadFile(context.Background(), ReadFileConfig{Path: path}) {
		if err != nil {
			t.Fatal(err)
		}
		records = append(records, rec)
	}

	if len(records) != 2 {
		t.Fatalf("got %d records, want 2", len(records))
	}

	// First record: valid header
	if records[0].Level != LevelLog {
		t.Errorf("record 1 level = %q, want %q", records[0].Level, LevelLog)
	}
	if records[0].Time.IsZero() {
		t.Error("record 1 time is zero")
	}
	if !strings.Contains(records[0].Message, "[Behaviour] Entering Room") {
		t.Errorf("record 1 message = %q", records[0].Message)
	}

	// Second record: no valid header
	if records[1].Level != LevelUnknown {
		t.Errorf("record 2 level = %q, want %q", records[1].Level, LevelUnknown)
	}
	if !records[1].Time.IsZero() {
		t.Error("record 2 time should be zero")
	}
	if records[1].Message != "not a header line" {
		t.Errorf("record 2 message = %q, want %q", records[1].Message, "not a header line")
	}
}

func TestReadFile_UTF8Sanitization(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "invalid_utf8.txt")
	content := []byte{0x80, 0x81, 0x82, '\n', 'o', 'k', '\n'}
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatal(err)
	}

	var records []Record
	for rec, err := range ReadFile(context.Background(), ReadFileConfig{Path: path}) {
		if err != nil {
			t.Fatal(err)
		}
		records = append(records, rec)
	}

	if len(records) != 2 {
		t.Fatalf("got %d records, want 2", len(records))
	}

	// First record should have sanitized invalid UTF-8
	if strings.Contains(records[0].Raw, string([]byte{0x80})) {
		t.Error("Raw should have sanitized invalid UTF-8 bytes")
	}
	if !strings.Contains(records[0].Raw, "�") {
		t.Error("Raw should contain replacement character")
	}

	// Second record is fine
	if records[1].Raw != "ok" {
		t.Errorf("record 2 raw = %q, want %q", records[1].Raw, "ok")
	}
}
