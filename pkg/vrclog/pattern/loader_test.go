package pattern_test

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vrclog/vrclog-go/pkg/vrclog/pattern"
)

func TestLoad_Valid(t *testing.T) {
	pf, err := pattern.Load("testdata/valid.yaml")
	require.NoError(t, err)
	assert.Equal(t, 1, pf.Version)
	assert.Len(t, pf.Patterns, 2)
	assert.Equal(t, "poker_hole_cards", pf.Patterns[0].ID)
	assert.Equal(t, "poker_hole_cards", pf.Patterns[0].EventType)
	assert.Equal(t, "poker_winner", pf.Patterns[1].ID)
}

func TestLoad_InvalidRegex(t *testing.T) {
	// Load should succeed because validation doesn't compile regex
	pf, err := pattern.Load("testdata/invalid_regex.yaml")
	require.NoError(t, err)
	assert.NotNil(t, pf)
	// NewRegexParser would fail on this file (tested in regex_parser_test.go)
}

func TestLoad_MissingFields(t *testing.T) {
	_, err := pattern.Load("testdata/missing_fields.yaml")
	require.Error(t, err)
	var patErr *pattern.PatternError
	require.True(t, errors.As(err, &patErr))
	assert.Contains(t, err.Error(), "event_type")
}

func TestLoad_UnsupportedVersion(t *testing.T) {
	_, err := pattern.Load("testdata/unsupported_version.yaml")
	require.Error(t, err)
	var valErr *pattern.ValidationError
	require.True(t, errors.As(err, &valErr))
	assert.Contains(t, err.Error(), "unsupported version")
}

func TestLoad_DuplicateID(t *testing.T) {
	_, err := pattern.Load("testdata/duplicate_id.yaml")
	require.Error(t, err)
	var patErr *pattern.PatternError
	require.True(t, errors.As(err, &patErr))
	assert.Contains(t, err.Error(), "duplicate id")
}

func TestLoad_PatternTooLong(t *testing.T) {
	_, err := pattern.Load("testdata/pattern_too_long.yaml")
	require.Error(t, err)
	var patErr *pattern.PatternError
	require.True(t, errors.As(err, &patErr))
	assert.Contains(t, err.Error(), "pattern too long")
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := pattern.Load("testdata/nonexistent.yaml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to open pattern file")
}

func TestLoad_PathSanitization(t *testing.T) {
	// Test that error messages don't leak file paths
	_, err := pattern.Load("/nonexistent/secret/path.yaml")
	require.Error(t, err)
	errStr := err.Error()
	// Verify path components are NOT in the error message
	assert.NotContains(t, errStr, "/nonexistent", "error message should not contain path")
	assert.NotContains(t, errStr, "secret", "error message should not contain path component")
	assert.NotContains(t, errStr, "path.yaml", "error message should not contain filename")
	// Verify error message still contains useful information
	assert.Contains(t, errStr, "failed to open pattern file", "error message should contain operation description")
}

func TestLoad_EmptyFile(t *testing.T) {
	// Create a temporary empty file is tricky, so we test via LoadBytes
	_, err := pattern.LoadBytes([]byte{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

func TestLoadBytes_Valid(t *testing.T) {
	data := []byte(`version: 1
patterns:
  - id: test
    event_type: test_event
    regex: 'test_pattern'
`)
	pf, err := pattern.LoadBytes(data)
	require.NoError(t, err)
	assert.Equal(t, 1, pf.Version)
	assert.Len(t, pf.Patterns, 1)
	assert.Equal(t, "test", pf.Patterns[0].ID)
}

func TestLoadBytes_InvalidYAML(t *testing.T) {
	data := []byte(`version: 1
patterns:
  - id: test
    event_type: test
    regex: [invalid yaml structure`)
	_, err := pattern.LoadBytes(data)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse YAML")
}

func TestLoadBytes_TooLarge(t *testing.T) {
	// Create data larger than MaxPatternFileSize
	data := make([]byte, pattern.MaxPatternFileSize+1)
	_, err := pattern.LoadBytes(data)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "too large")
}

func TestValidate_NoPatterns(t *testing.T) {
	pf := &pattern.PatternFile{
		Version:  1,
		Patterns: []pattern.Pattern{},
	}
	err := pf.Validate()
	require.Error(t, err)
	var valErr *pattern.ValidationError
	require.True(t, errors.As(err, &valErr))
	assert.Contains(t, err.Error(), "at least one pattern")
}

func TestValidate_MissingID(t *testing.T) {
	pf := &pattern.PatternFile{
		Version: 1,
		Patterns: []pattern.Pattern{
			{ID: "", EventType: "test", Regex: "test"},
		},
	}
	err := pf.Validate()
	require.Error(t, err)
	var patErr *pattern.PatternError
	require.True(t, errors.As(err, &patErr))
	assert.Contains(t, err.Error(), "id is required")
}

func TestValidate_MissingEventType(t *testing.T) {
	pf := &pattern.PatternFile{
		Version: 1,
		Patterns: []pattern.Pattern{
			{ID: "test", EventType: "", Regex: "test"},
		},
	}
	err := pf.Validate()
	require.Error(t, err)
	var patErr *pattern.PatternError
	require.True(t, errors.As(err, &patErr))
	assert.Contains(t, err.Error(), "event_type is required")
}

func TestValidate_MissingRegex(t *testing.T) {
	pf := &pattern.PatternFile{
		Version: 1,
		Patterns: []pattern.Pattern{
			{ID: "test", EventType: "test", Regex: ""},
		},
	}
	err := pf.Validate()
	require.Error(t, err)
	var patErr *pattern.PatternError
	require.True(t, errors.As(err, &patErr))
	assert.Contains(t, err.Error(), "regex is required")
}

func TestValidate_DuplicateIDInMiddle(t *testing.T) {
	pf := &pattern.PatternFile{
		Version: 1,
		Patterns: []pattern.Pattern{
			{ID: "first", EventType: "test1", Regex: "test1"},
			{ID: "second", EventType: "test2", Regex: "test2"},
			{ID: "first", EventType: "test3", Regex: "test3"},
		},
	}
	err := pf.Validate()
	require.Error(t, err)
	var patErr *pattern.PatternError
	require.True(t, errors.As(err, &patErr))
	assert.Equal(t, 2, patErr.Index) // Third pattern (index 2)
	assert.Contains(t, err.Error(), "duplicate id")
	assert.Contains(t, err.Error(), "pattern[0]") // References first occurrence
}

func TestValidate_PatternLengthExactlyMax(t *testing.T) {
	// Pattern exactly at MaxPatternLength should be allowed
	regex := strings.Repeat("a", pattern.MaxPatternLength)
	pf := &pattern.PatternFile{
		Version: 1,
		Patterns: []pattern.Pattern{
			{ID: "max_length", EventType: "test", Regex: regex},
		},
	}
	err := pf.Validate()
	assert.NoError(t, err)
}

func TestValidate_PatternLengthOverMax(t *testing.T) {
	// Pattern over MaxPatternLength should fail
	regex := strings.Repeat("a", pattern.MaxPatternLength+1)
	pf := &pattern.PatternFile{
		Version: 1,
		Patterns: []pattern.Pattern{
			{ID: "too_long", EventType: "test", Regex: regex},
		},
	}
	err := pf.Validate()
	require.Error(t, err)
	var patErr *pattern.PatternError
	require.True(t, errors.As(err, &patErr))
	assert.Contains(t, err.Error(), "pattern too long")
}
func TestLoad_NonRegularFile_Directory(t *testing.T) {
	// Attempt to load a directory should fail
	_, err := pattern.Load("testdata")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "regular file")
}

func TestLoad_NonRegularFile_Symlink(t *testing.T) {
	// Create a temporary symlink to a valid file
	// Use absolute path to avoid path resolution issues
	absTarget, err := os.Getwd()
	require.NoError(t, err)
	absTarget = absTarget + "/testdata/valid.yaml"

	link := t.TempDir() + "/symlink.yaml"
	err = os.Symlink(absTarget, link)
	require.NoError(t, err)

	// Symlinks to regular files should be accepted (os.Open follows them)
	// The security check is about FIFO/device files, not symlinks
	pf, loadErr := pattern.Load(link)
	require.NoError(t, loadErr)
	assert.NotNil(t, pf)
	assert.Equal(t, 1, pf.Version)
}

func TestLoad_NonExistent(t *testing.T) {
	_, err := pattern.Load("testdata/nonexistent.yaml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "open")
}
