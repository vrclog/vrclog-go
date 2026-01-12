package vrclog_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/vrclog/vrclog-go/pkg/vrclog"
)

func TestParseFile_Basic(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "output_log_test.txt")

	content := `2024.01.15 12:00:00 Log        -  [Behaviour] OnPlayerJoined User1
2024.01.15 12:00:01 Log        -  [Behaviour] OnPlayerJoined User2
2024.01.15 12:00:02 Log        -  [Behaviour] OnPlayerLeft User1
`
	if err := os.WriteFile(logFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	var events []vrclog.Event

	for ev, err := range vrclog.ParseFile(ctx, logFile) {
		if err != nil {
			t.Fatalf("ParseFile error: %v", err)
		}
		events = append(events, ev)
	}

	if len(events) != 3 {
		t.Errorf("got %d events, want 3", len(events))
	}

	// Verify event order
	expected := []struct {
		playerName string
		eventType  vrclog.EventType
	}{
		{"User1", vrclog.EventPlayerJoin},
		{"User2", vrclog.EventPlayerJoin},
		{"User1", vrclog.EventPlayerLeft},
	}

	for i, want := range expected {
		if i >= len(events) {
			break
		}
		if events[i].PlayerName != want.playerName {
			t.Errorf("event %d: got player %q, want %q", i, events[i].PlayerName, want.playerName)
		}
		if events[i].Type != want.eventType {
			t.Errorf("event %d: got type %v, want %v", i, events[i].Type, want.eventType)
		}
	}
}

func TestParseFile_EmptyPath(t *testing.T) {
	ctx := context.Background()
	var errCount int

	for _, err := range vrclog.ParseFile(ctx, "") {
		if err != nil {
			errCount++
			break
		}
	}

	if errCount != 1 {
		t.Error("ParseFile with empty path should yield an error")
	}
}

func TestParseFile_FileNotFound(t *testing.T) {
	ctx := context.Background()
	var errCount int

	for _, err := range vrclog.ParseFile(ctx, "/nonexistent/file.txt") {
		if err != nil {
			errCount++
			break
		}
	}

	if errCount != 1 {
		t.Error("ParseFile with nonexistent file should yield an error")
	}
}

func TestParseFile_WithIncludeTypes(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "output_log_test.txt")

	content := `2024.01.15 12:00:00 Log        -  [Behaviour] OnPlayerJoined User1
2024.01.15 12:00:01 Log        -  [Behaviour] OnPlayerLeft User1
2024.01.15 12:00:02 Log        -  [Behaviour] OnPlayerJoined User2
`
	if err := os.WriteFile(logFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	var events []vrclog.Event

	for ev, err := range vrclog.ParseFile(ctx, logFile,
		vrclog.WithParseIncludeTypes(vrclog.EventPlayerJoin),
	) {
		if err != nil {
			t.Fatalf("ParseFile error: %v", err)
		}
		events = append(events, ev)
	}

	if len(events) != 2 {
		t.Errorf("got %d events, want 2", len(events))
	}

	for i, ev := range events {
		if ev.Type != vrclog.EventPlayerJoin {
			t.Errorf("event %d: got type %v, want %v", i, ev.Type, vrclog.EventPlayerJoin)
		}
	}
}

func TestParseFile_WithExcludeTypes(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "output_log_test.txt")

	content := `2024.01.15 12:00:00 Log        -  [Behaviour] OnPlayerJoined User1
2024.01.15 12:00:01 Log        -  [Behaviour] OnPlayerLeft User1
2024.01.15 12:00:02 Log        -  [Behaviour] OnPlayerJoined User2
`
	if err := os.WriteFile(logFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	var events []vrclog.Event

	for ev, err := range vrclog.ParseFile(ctx, logFile,
		vrclog.WithParseExcludeTypes(vrclog.EventPlayerLeft),
	) {
		if err != nil {
			t.Fatalf("ParseFile error: %v", err)
		}
		events = append(events, ev)
	}

	if len(events) != 2 {
		t.Errorf("got %d events, want 2", len(events))
	}

	for i, ev := range events {
		if ev.Type == vrclog.EventPlayerLeft {
			t.Errorf("event %d should not be player_left", i)
		}
	}
}

func TestParseFile_WithTimeRange(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "output_log_test.txt")

	content := `2024.01.15 12:00:00 Log        -  [Behaviour] OnPlayerJoined EarlyUser
2024.01.15 14:00:00 Log        -  [Behaviour] OnPlayerJoined MiddleUser
2024.01.15 16:00:00 Log        -  [Behaviour] OnPlayerJoined LateUser
`
	if err := os.WriteFile(logFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	since, _ := time.ParseInLocation("2006.01.02 15:04:05", "2024.01.15 13:00:00", time.Local)
	until, _ := time.ParseInLocation("2006.01.02 15:04:05", "2024.01.15 15:00:00", time.Local)

	ctx := context.Background()
	var events []vrclog.Event

	for ev, err := range vrclog.ParseFile(ctx, logFile,
		vrclog.WithParseTimeRange(since, until),
	) {
		if err != nil {
			t.Fatalf("ParseFile error: %v", err)
		}
		events = append(events, ev)
	}

	if len(events) != 1 {
		t.Errorf("got %d events, want 1", len(events))
	}

	if len(events) > 0 && events[0].PlayerName != "MiddleUser" {
		t.Errorf("got player %q, want MiddleUser", events[0].PlayerName)
	}
}

func TestParseFile_WithIncludeRawLine(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "output_log_test.txt")

	rawLine := "2024.01.15 12:00:00 Log        -  [Behaviour] OnPlayerJoined User1"
	if err := os.WriteFile(logFile, []byte(rawLine+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	var events []vrclog.Event

	for ev, err := range vrclog.ParseFile(ctx, logFile,
		vrclog.WithParseIncludeRawLine(true),
	) {
		if err != nil {
			t.Fatalf("ParseFile error: %v", err)
		}
		events = append(events, ev)
	}

	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}

	if events[0].RawLine != rawLine {
		t.Errorf("got RawLine %q, want %q", events[0].RawLine, rawLine)
	}
}

func TestParseFile_ContextCancellation(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "output_log_test.txt")

	// Write many lines
	var content string
	for i := 0; i < 1000; i++ {
		content += "2024.01.15 12:00:00 Log        -  [Behaviour] OnPlayerJoined User\n"
	}
	if err := os.WriteFile(logFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	var errCount int
	for _, err := range vrclog.ParseFile(ctx, logFile) {
		if err != nil {
			errCount++
			break
		}
	}

	// Should get context cancellation error
	if errCount == 0 {
		t.Error("ParseFile should yield context cancellation error")
	}
}

func TestParseFileAll_Basic(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "output_log_test.txt")

	content := `2024.01.15 12:00:00 Log        -  [Behaviour] OnPlayerJoined User1
2024.01.15 12:00:01 Log        -  [Behaviour] OnPlayerJoined User2
`
	if err := os.WriteFile(logFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	events, err := vrclog.ParseFileAll(ctx, logFile)
	if err != nil {
		t.Fatalf("ParseFileAll error: %v", err)
	}

	if len(events) != 2 {
		t.Errorf("got %d events, want 2", len(events))
	}
}

func TestParseFileAll_FileNotFound(t *testing.T) {
	ctx := context.Background()
	events, err := vrclog.ParseFileAll(ctx, "/nonexistent/file.txt")

	if err == nil {
		t.Error("ParseFileAll should return error for nonexistent file")
	}

	// Should return empty slice (no events collected before error)
	if len(events) != 0 {
		t.Errorf("got %d events, want 0", len(events))
	}
}

func TestParseDir_Basic(t *testing.T) {
	dir := t.TempDir()

	// Create two log files with different modification times
	logFile1 := filepath.Join(dir, "output_log_2024-01-15_12-00-00.txt")
	logFile2 := filepath.Join(dir, "output_log_2024-01-15_13-00-00.txt")

	content1 := "2024.01.15 12:00:00 Log        -  [Behaviour] OnPlayerJoined User1\n"
	content2 := "2024.01.15 13:00:00 Log        -  [Behaviour] OnPlayerJoined User2\n"

	if err := os.WriteFile(logFile1, []byte(content1), 0644); err != nil {
		t.Fatal(err)
	}
	// Set modification time for proper ordering
	time.Sleep(10 * time.Millisecond)
	if err := os.WriteFile(logFile2, []byte(content2), 0644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	var events []vrclog.Event

	for ev, err := range vrclog.ParseDir(ctx, vrclog.WithDirLogDir(dir)) {
		if err != nil {
			t.Fatalf("ParseDir error: %v", err)
		}
		events = append(events, ev)
	}

	if len(events) != 2 {
		t.Errorf("got %d events, want 2", len(events))
	}

	// Should be in chronological order (by file modification time)
	if len(events) >= 2 {
		if events[0].PlayerName != "User1" {
			t.Errorf("first event: got player %q, want User1", events[0].PlayerName)
		}
		if events[1].PlayerName != "User2" {
			t.Errorf("second event: got player %q, want User2", events[1].PlayerName)
		}
	}
}

func TestParseDir_WithPaths(t *testing.T) {
	dir := t.TempDir()

	logFile1 := filepath.Join(dir, "custom_log1.txt")
	logFile2 := filepath.Join(dir, "custom_log2.txt")

	content1 := "2024.01.15 12:00:00 Log        -  [Behaviour] OnPlayerJoined User1\n"
	content2 := "2024.01.15 13:00:00 Log        -  [Behaviour] OnPlayerJoined User2\n"

	if err := os.WriteFile(logFile1, []byte(content1), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(logFile2, []byte(content2), 0644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	var events []vrclog.Event

	for ev, err := range vrclog.ParseDir(ctx, vrclog.WithDirPaths(logFile1, logFile2)) {
		if err != nil {
			t.Fatalf("ParseDir error: %v", err)
		}
		events = append(events, ev)
	}

	if len(events) != 2 {
		t.Errorf("got %d events, want 2", len(events))
	}
}

func TestParseDir_EmptyDir(t *testing.T) {
	dir := t.TempDir()

	ctx := context.Background()
	var errCount int

	for _, err := range vrclog.ParseDir(ctx, vrclog.WithDirLogDir(dir)) {
		if err != nil {
			errCount++
			break
		}
	}

	if errCount != 1 {
		t.Error("ParseDir with empty dir should yield ErrNoLogFiles")
	}
}

func TestParseDir_WithIncludeTypes(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "output_log_test.txt")

	content := `2024.01.15 12:00:00 Log        -  [Behaviour] OnPlayerJoined User1
2024.01.15 12:00:01 Log        -  [Behaviour] OnPlayerLeft User1
`
	if err := os.WriteFile(logFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	var events []vrclog.Event

	for ev, err := range vrclog.ParseDir(ctx,
		vrclog.WithDirLogDir(dir),
		vrclog.WithDirIncludeTypes(vrclog.EventPlayerJoin),
	) {
		if err != nil {
			t.Fatalf("ParseDir error: %v", err)
		}
		events = append(events, ev)
	}

	if len(events) != 1 {
		t.Errorf("got %d events, want 1", len(events))
	}

	if len(events) > 0 && events[0].Type != vrclog.EventPlayerJoin {
		t.Errorf("got type %v, want %v", events[0].Type, vrclog.EventPlayerJoin)
	}
}

func TestParseFile_WithCustomParser(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "output_log_test.txt")

	content := "2024.01.15 12:00:00 Log        -  [Behaviour] OnPlayerJoined User1\n"
	if err := os.WriteFile(logFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Custom parser that adds a prefix to player names
	customParser := vrclog.ParserFunc(func(ctx context.Context, line string) (vrclog.ParseResult, error) {
		// Use default parser first
		result, err := vrclog.DefaultParser{}.ParseLine(ctx, line)
		if err != nil || !result.Matched {
			return result, err
		}
		// Modify events
		for i := range result.Events {
			result.Events[i].PlayerName = "Custom-" + result.Events[i].PlayerName
		}
		return result, nil
	})

	ctx := context.Background()
	var events []vrclog.Event

	for ev, err := range vrclog.ParseFile(ctx, logFile, vrclog.WithParseParser(customParser)) {
		if err != nil {
			t.Fatalf("ParseFile error: %v", err)
		}
		events = append(events, ev)
	}

	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}

	if events[0].PlayerName != "Custom-User1" {
		t.Errorf("got PlayerName %q, want Custom-User1", events[0].PlayerName)
	}
}

func TestParseDir_WithCustomParser(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "output_log_test.txt")

	content := "2024.01.15 12:00:00 Log        -  [Behaviour] OnPlayerJoined User1\n"
	if err := os.WriteFile(logFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Custom parser that adds a suffix to player names
	customParser := vrclog.ParserFunc(func(ctx context.Context, line string) (vrclog.ParseResult, error) {
		result, err := vrclog.DefaultParser{}.ParseLine(ctx, line)
		if err != nil || !result.Matched {
			return result, err
		}
		for i := range result.Events {
			result.Events[i].PlayerName = result.Events[i].PlayerName + "-Modified"
		}
		return result, nil
	})

	ctx := context.Background()
	var events []vrclog.Event

	for ev, err := range vrclog.ParseDir(ctx,
		vrclog.WithDirLogDir(dir),
		vrclog.WithDirParser(customParser),
	) {
		if err != nil {
			t.Fatalf("ParseDir error: %v", err)
		}
		events = append(events, ev)
	}

	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}

	if events[0].PlayerName != "User1-Modified" {
		t.Errorf("got PlayerName %q, want User1-Modified", events[0].PlayerName)
	}
}
