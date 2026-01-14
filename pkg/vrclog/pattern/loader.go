package pattern

import (
	"errors"
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

// sanitizePathError removes the path from os.PathError to prevent information leakage.
// This ensures error messages don't expose file system paths to users.
func sanitizePathError(err error) error {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		// Return just the operation and underlying error, without the path
		return fmt.Errorf("%s: %w", pathErr.Op, pathErr.Err)
	}
	return err
}

const (
	// MaxPatternFileSize is the maximum allowed size for a pattern file (1MB).
	// This limit prevents denial-of-service attacks via extremely large files.
	MaxPatternFileSize = 1 * 1024 * 1024 // 1 MB

	// MaxPatternLength is the maximum allowed length for a regex pattern (512 bytes).
	// This limit helps mitigate ReDoS (Regular Expression Denial of Service) attacks
	// by preventing excessively complex patterns.
	MaxPatternLength = 512

	// MaxPatternCount is the maximum number of patterns allowed in a pattern file.
	// This limit prevents CPU exhaustion attacks via files with thousands of patterns.
	MaxPatternCount = 1000

	// SupportedVersion is the currently supported pattern file format version.
	SupportedVersion = 1
)

// Load reads and parses a pattern file from the given path.
// Returns an error if the file cannot be read, is too large, or fails validation.
//
// Security: This function protects against FIFO/device file DoS attacks by:
//   - Opening the file and stat-ing the file descriptor (avoiding TOCTOU)
//   - Rejecting non-regular files (FIFO, device, socket, etc.)
//   - Using io.LimitReader to enforce size limits during read
//
// Example:
//
//	pf, err := pattern.Load("patterns.yaml")
//	if err != nil {
//	    log.Fatalf("failed to load pattern file: %v", err)
//	}
func Load(path string) (*PatternFile, error) {
	// Open file first (don't use os.ReadFile which doesn't check file type)
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open pattern file: %w", sanitizePathError(err))
	}
	defer f.Close()

	// Stat the file descriptor (not the path) to avoid TOCTOU
	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat pattern file: %w", sanitizePathError(err))
	}

	// Reject non-regular files (FIFO, device, socket, etc.) to prevent DoS
	if !info.Mode().IsRegular() {
		return nil, errors.New("pattern file must be a regular file (not FIFO, device, or special file)")
	}

	// Check size constraints
	if info.Size() == 0 {
		return nil, errors.New("pattern file is empty")
	}
	if info.Size() > MaxPatternFileSize {
		return nil, fmt.Errorf("pattern file too large: %d bytes (max %d)", info.Size(), MaxPatternFileSize)
	}

	// Read with size limit to prevent unbounded reads
	// Read MaxPatternFileSize+1 to detect if file grows beyond limit
	data, err := io.ReadAll(io.LimitReader(f, MaxPatternFileSize+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read pattern file: %w", sanitizePathError(err))
	}

	// Double-check size (protects against file growing between Stat and Read)
	if len(data) > MaxPatternFileSize {
		return nil, fmt.Errorf("pattern file too large: %d bytes (max %d)", len(data), MaxPatternFileSize)
	}

	return LoadBytes(data)
}

// LoadBytes parses a pattern file from a byte slice.
// Returns an error if the data cannot be parsed or fails validation.
//
// Example:
//
//	data := []byte("version: 1\npatterns:\n  - id: test\n    ...")
//	pf, err := pattern.LoadBytes(data)
func LoadBytes(data []byte) (*PatternFile, error) {
	if len(data) == 0 {
		return nil, errors.New("pattern file is empty")
	}
	if len(data) > MaxPatternFileSize {
		return nil, fmt.Errorf("pattern file too large: %d bytes (max %d)", len(data), MaxPatternFileSize)
	}

	var pf PatternFile
	if err := yaml.Unmarshal(data, &pf); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	if err := pf.Validate(); err != nil {
		return nil, err
	}

	return &pf, nil
}

// Validate performs schema-level validation on the pattern file.
// It checks for:
//   - Supported version number
//   - At least one pattern
//   - Required fields (id, event_type, regex)
//   - Unique pattern IDs
//   - Pattern length limits (ReDoS protection)
//
// Note: This function does NOT compile regular expressions. Regex compilation
// and validation happens in NewRegexParser() to avoid duplicating work.
func (pf *PatternFile) Validate() error {
	// Check version
	if pf.Version != SupportedVersion {
		return &ValidationError{
			Field:   "version",
			Message: fmt.Sprintf("unsupported version %d (only version %d is supported)", pf.Version, SupportedVersion),
		}
	}

	// Check for at least one pattern
	if len(pf.Patterns) == 0 {
		return &ValidationError{
			Field:   "patterns",
			Message: "at least one pattern is required",
		}
	}

	// Check for maximum pattern count
	if len(pf.Patterns) > MaxPatternCount {
		return &ValidationError{
			Field:   "patterns",
			Message: fmt.Sprintf("too many patterns (%d), maximum allowed is %d", len(pf.Patterns), MaxPatternCount),
		}
	}

	// Track IDs for uniqueness check
	seenIDs := make(map[string]int, len(pf.Patterns))

	// Validate each pattern
	for i, p := range pf.Patterns {
		// Check required fields
		if p.ID == "" {
			return &PatternError{
				Index:   i,
				Field:   "id",
				Message: "id is required",
			}
		}
		if p.EventType == "" {
			return &PatternError{
				Index:   i,
				ID:      p.ID,
				Field:   "event_type",
				Message: "event_type is required",
			}
		}
		if p.Regex == "" {
			return &PatternError{
				Index:   i,
				ID:      p.ID,
				Field:   "regex",
				Message: "regex is required",
			}
		}

		// Check ID uniqueness
		if prevIndex, exists := seenIDs[p.ID]; exists {
			return &PatternError{
				Index:   i,
				ID:      p.ID,
				Field:   "id",
				Message: fmt.Sprintf("duplicate id (previously defined at pattern[%d])", prevIndex),
			}
		}
		seenIDs[p.ID] = i

		// Check pattern length for ReDoS protection
		if len(p.Regex) > MaxPatternLength {
			return &PatternError{
				Index:   i,
				ID:      p.ID,
				Field:   "regex",
				Message: fmt.Sprintf("pattern too long: %d bytes (max %d)", len(p.Regex), MaxPatternLength),
			}
		}
	}

	return nil
}
