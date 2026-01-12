package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildParser_NoPatterns(t *testing.T) {
	parser, err := buildParser(nil)
	if err != nil {
		t.Fatalf("buildParser(nil) error = %v", err)
	}
	if parser != nil {
		t.Errorf("buildParser(nil) = %v, want nil", parser)
	}
}

func TestBuildParser_EmptySlice(t *testing.T) {
	parser, err := buildParser([]string{})
	if err != nil {
		t.Fatalf("buildParser([]) error = %v", err)
	}
	if parser != nil {
		t.Errorf("buildParser([]) = %v, want nil", parser)
	}
}

func TestBuildParser_ValidPattern(t *testing.T) {
	dir := t.TempDir()
	patternFile := filepath.Join(dir, "patterns.yaml")
	content := `version: 1
patterns:
  - id: test_event
    event_type: test
    regex: 'test: (?P<value>\w+)'
`
	if err := os.WriteFile(patternFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	parser, err := buildParser([]string{patternFile})
	if err != nil {
		t.Fatalf("buildParser() error = %v", err)
	}
	if parser == nil {
		t.Error("buildParser() = nil, want non-nil")
	}
}

func TestBuildParser_FileNotFound(t *testing.T) {
	_, err := buildParser([]string{"/nonexistent/patterns.yaml"})
	if err == nil {
		t.Fatal("buildParser() expected error for nonexistent file")
	}
	// Verify error message does NOT contain the path (security)
	errStr := err.Error()
	if strings.Contains(errStr, "/nonexistent") {
		t.Errorf("error message should not contain path: %s", errStr)
	}
	if strings.Contains(errStr, "patterns.yaml") {
		t.Errorf("error message should not contain filename: %s", errStr)
	}
}

func TestBuildParser_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	patternFile := filepath.Join(dir, "invalid.yaml")
	if err := os.WriteFile(patternFile, []byte("not: valid: yaml: content"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := buildParser([]string{patternFile})
	if err == nil {
		t.Fatal("buildParser() expected error for invalid YAML")
	}
}

func TestBuildParser_InvalidRegex(t *testing.T) {
	dir := t.TempDir()
	patternFile := filepath.Join(dir, "bad_regex.yaml")
	content := `version: 1
patterns:
  - id: bad
    event_type: test
    regex: '[invalid regex'
`
	if err := os.WriteFile(patternFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := buildParser([]string{patternFile})
	if err == nil {
		t.Fatal("buildParser() expected error for invalid regex")
	}
}

func TestBuildParser_MultiplePatterns(t *testing.T) {
	dir := t.TempDir()

	pattern1 := filepath.Join(dir, "p1.yaml")
	content1 := `version: 1
patterns:
  - id: event1
    event_type: type1
    regex: 'pattern1'
`
	if err := os.WriteFile(pattern1, []byte(content1), 0644); err != nil {
		t.Fatal(err)
	}

	pattern2 := filepath.Join(dir, "p2.yaml")
	content2 := `version: 1
patterns:
  - id: event2
    event_type: type2
    regex: 'pattern2'
`
	if err := os.WriteFile(pattern2, []byte(content2), 0644); err != nil {
		t.Fatal(err)
	}

	parser, err := buildParser([]string{pattern1, pattern2})
	if err != nil {
		t.Fatalf("buildParser() error = %v", err)
	}
	if parser == nil {
		t.Error("buildParser() = nil, want non-nil")
	}
}
