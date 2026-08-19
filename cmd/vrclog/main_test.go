package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestRunReadWithFixture(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runRead([]string{"../../testdata/logs/vrchat_full.txt"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr: %s", code, stderr.String())
	}

	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) == 0 {
		t.Fatal("expected at least one JSON line on stdout")
	}

	for i, line := range lines {
		var obj map[string]any
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			t.Errorf("line %d is not valid JSON: %v\nline: %s", i+1, err, line)
		}
	}
}

func TestRunReadNonexistentFile(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runRead([]string{"/nonexistent/path/file.txt"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "/nonexistent/path/file.txt") {
		t.Errorf("stderr should mention the path; got: %s", stderr.String())
	}
}

func TestRunReadNoArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runRead([]string{}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
}

func TestRunReadMultipleFilesWithOneBad(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runRead([]string{
		"../../testdata/logs/vrchat_full.txt",
		"/nonexistent/file.txt",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if stdout.Len() == 0 {
		t.Error("expected some output from the valid file")
	}
}

func TestRunVersion(t *testing.T) {
	out := runVersion()
	if !strings.Contains(out, "vrclog version") {
		t.Errorf("version output should contain 'vrclog version'; got: %s", out)
	}
}

func TestRunFollowCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var stdout, stderr bytes.Buffer
	code := runFollow(ctx, []string{"--dir", t.TempDir()}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0 on cancelled context, got %d; stderr: %s", code, stderr.String())
	}
}

func TestRunFollowFlagParsing(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runFollow(context.Background(), []string{"--invalid-flag"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected exit code 2 for invalid flag, got %d", code)
	}
}
