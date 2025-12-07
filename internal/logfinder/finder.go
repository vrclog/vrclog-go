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

// FindLatestLogFile returns the path to the most recently modified
// output_log file in the given directory.
//
// Returns ErrNoLogFiles if no log files are found.
func FindLatestLogFile(dir string) (string, error) {
	pattern := filepath.Join(dir, "output_log_*.txt")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", fmt.Errorf("globbing log files: %w", err)
	}

	if len(matches) == 0 {
		return "", ErrNoLogFiles
	}

	// Sort by modification time (newest first)
	sort.Slice(matches, func(i, j int) bool {
		infoI, errI := os.Stat(matches[i])
		infoJ, errJ := os.Stat(matches[j])
		if errI != nil || errJ != nil {
			return false
		}
		return infoI.ModTime().After(infoJ.ModTime())
	})

	return matches[0], nil
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
		// Fallback to original path if symlink resolution fails
		// (e.g., permission issues, broken links)
		resolved = dir
	}

	// Check for log files in resolved path
	pattern := filepath.Join(resolved, "output_log_*.txt")
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return ""
	}

	return resolved
}
