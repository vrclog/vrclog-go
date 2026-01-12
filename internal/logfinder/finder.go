// Package logfinder provides VRChat log directory and file detection.
package logfinder

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// EnvLogDir is the environment variable name for specifying log directory.
const EnvLogDir = "VRCLOG_LOGDIR"

// Sentinel errors.
var (
	ErrLogDirNotFound = errors.New("log directory not found")
	ErrNoLogFiles     = errors.New("no log files found")
)

// DefaultLogDirs returns candidate VRChat log directories in priority order.
// The directories are OS-specific (Windows only for VRChat PC).
func DefaultLogDirs() []string {
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		// Fallback: try to construct from USERPROFILE
		userProfile := os.Getenv("USERPROFILE")
		if userProfile != "" {
			localAppData = filepath.Join(userProfile, "AppData", "Local")
		}
	}

	if localAppData == "" {
		return nil
	}

	// LocalLow is one level up from Local
	localLow := filepath.Join(filepath.Dir(localAppData), "LocalLow")

	return []string{
		filepath.Join(localLow, "VRChat", "VRChat"),
		filepath.Join(localLow, "VRChat", "vrchat"),
	}
}

// FindLogDir returns the VRChat log directory.
//
// Priority:
//  1. explicit (if non-empty)
//  2. VRCLOG_LOGDIR environment variable
//  3. Auto-detect from DefaultLogDirs()
//
// Returns ErrLogDirNotFound if no valid directory is found.
// The returned path has symlinks resolved for consistency.
func FindLogDir(explicit string) (string, error) {
	// 1. Check explicit
	if explicit != "" {
		if resolved := resolveAndValidateLogDir(explicit); resolved != "" {
			return resolved, nil
		}
		return "", fmt.Errorf("%w: specified directory is invalid or contains no log files", ErrLogDirNotFound)
	}

	// 2. Check environment variable
	if envDir := os.Getenv(EnvLogDir); envDir != "" {
		if resolved := resolveAndValidateLogDir(envDir); resolved != "" {
			return resolved, nil
		}
		return "", fmt.Errorf("%w: %s environment variable points to invalid directory", ErrLogDirNotFound, EnvLogDir)
	}

	// 3. Auto-detect
	for _, dir := range DefaultLogDirs() {
		if resolved := resolveAndValidateLogDir(dir); resolved != "" {
			return resolved, nil
		}
	}

	return "", ErrLogDirNotFound
}

// logCandidate holds a log file path and its cached modification time.
// This avoids race conditions where files are deleted between stat and sort.
type logCandidate struct {
	path    string
	modTime int64
}

// FindLatestLogFile returns the path to the most recently modified
// output_log file in the given directory.
//
// Returns ErrNoLogFiles if no log files are found.
//
// Security: This function caches stat results to avoid TOCTOU race conditions
// where log files could be deleted between filtering and sorting.
func FindLatestLogFile(dir string) (string, error) {
	pattern := filepath.Join(dir, "output_log_*.txt")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", fmt.Errorf("globbing log files: %w", err)
	}

	if len(matches) == 0 {
		return "", ErrNoLogFiles
	}

	// Stat files once and cache results to avoid race conditions
	candidates := make([]logCandidate, 0, len(matches))
	for _, m := range matches {
		info, err := os.Stat(m)
		if err != nil {
			// Skip files that can't be stat'd (deleted, permission issues, etc.)
			continue
		}
		// Also skip non-regular files (directories, symlinks, etc.)
		if !info.Mode().IsRegular() {
			continue
		}
		candidates = append(candidates, logCandidate{
			path:    m,
			modTime: info.ModTime().UnixNano(),
		})
	}

	if len(candidates) == 0 {
		return "", ErrNoLogFiles
	}

	// Sort by cached modification time (newest first)
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].modTime > candidates[j].modTime
	})

	return candidates[0].path, nil
}

// resolveAndValidateLogDir resolves symlinks and validates the directory.
// Returns the resolved path if valid, empty string otherwise.
// This helps prevent symlink-based attacks and ensures path consistency.
func resolveAndValidateLogDir(dir string) string {
	// First check if path exists
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return ""
	}

	// Resolve symlinks (works with Windows Junctions in Go 1.20+)
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		// Symlink resolution failed - treat as invalid directory
		// to prevent potential security issues with broken/malicious symlinks
		return ""
	}

	// Check for log files in resolved path
	pattern := filepath.Join(resolved, "output_log_*.txt")
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return ""
	}

	return resolved
}
