package logfinder

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFindLatestLogFile(t *testing.T) {
	// Create temp directory
	dir := t.TempDir()

	// Create test log files with different modification times
	files := []string{
		"output_log_2024-01-01_00-00-00.txt",
		"output_log_2024-01-02_00-00-00.txt",
		"output_log_2024-01-03_00-00-00.txt",
	}

	for i, name := range files {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}
		// Set modification time (oldest first)
		modTime := time.Now().Add(time.Duration(i) * time.Hour)
		if err := os.Chtimes(path, modTime, modTime); err != nil {
			t.Fatal(err)
		}
	}

	// Test
	got, err := FindLatestLogFile(dir)
	if err != nil {
		t.Fatalf("FindLatestLogFile() error = %v", err)
	}

	// Should return the most recently modified file (last one)
	want := files[len(files)-1]
	if filepath.Base(got) != want {
		t.Errorf("FindLatestLogFile() = %v, want %v", filepath.Base(got), want)
	}
}

func TestFindLatestLogFile_NoFiles(t *testing.T) {
	dir := t.TempDir()

	_, err := FindLatestLogFile(dir)
	if err == nil {
		t.Error("FindLatestLogFile() expected error for empty directory")
	}
	if !errors.Is(err, ErrNoLogFiles) {
		t.Errorf("FindLatestLogFile() error = %v, want %v", err, ErrNoLogFiles)
	}
}

func TestFindLogDir_EnvVar(t *testing.T) {
	// Create temp directory with log file
	dir := t.TempDir()
	logFile := filepath.Join(dir, "output_log_test.txt")
	if err := os.WriteFile(logFile, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	// Set environment variable
	oldVal := os.Getenv(EnvLogDir)
	os.Setenv(EnvLogDir, dir)
	defer os.Setenv(EnvLogDir, oldVal)

	// Test
	got, err := FindLogDir("")
	if err != nil {
		t.Fatalf("FindLogDir() error = %v", err)
	}

	// Resolve symlinks in expected path for comparison (e.g., /var -> /private/var on macOS)
	want, _ := filepath.EvalSymlinks(dir)
	if want == "" {
		want = dir
	}
	if got != want {
		t.Errorf("FindLogDir() = %v, want %v", got, want)
	}
}

func TestFindLogDir_Explicit(t *testing.T) {
	// Create temp directory with log file
	dir := t.TempDir()
	logFile := filepath.Join(dir, "output_log_test.txt")
	if err := os.WriteFile(logFile, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	// Explicit should take priority over env
	oldVal := os.Getenv(EnvLogDir)
	os.Setenv(EnvLogDir, "/some/other/path")
	defer os.Setenv(EnvLogDir, oldVal)

	got, err := FindLogDir(dir)
	if err != nil {
		t.Fatalf("FindLogDir() error = %v", err)
	}

	// Resolve symlinks in expected path for comparison (e.g., /var -> /private/var on macOS)
	want, _ := filepath.EvalSymlinks(dir)
	if want == "" {
		want = dir
	}
	if got != want {
		t.Errorf("FindLogDir() = %v, want %v", got, want)
	}
}

func TestFindLogDir_ExplicitInvalid(t *testing.T) {
	_, err := FindLogDir("/nonexistent/path")
	if err == nil {
		t.Error("FindLogDir() expected error for invalid explicit path")
	}
	if !errors.Is(err, ErrLogDirNotFound) {
		t.Errorf("FindLogDir() error = %v, want %v", err, ErrLogDirNotFound)
	}
}

func TestFindLogDir_EnvVarInvalid(t *testing.T) {
	// Set environment variable to invalid path
	oldVal := os.Getenv(EnvLogDir)
	os.Setenv(EnvLogDir, "/nonexistent/path")
	defer os.Setenv(EnvLogDir, oldVal)

	_, err := FindLogDir("")
	if err == nil {
		t.Error("FindLogDir() expected error for invalid env var path")
	}
	if !errors.Is(err, ErrLogDirNotFound) {
		t.Errorf("FindLogDir() error = %v, want %v", err, ErrLogDirNotFound)
	}
}

func TestResolveAndValidateLogDir(t *testing.T) {
	// Create temp directory with log file
	dir := t.TempDir()
	logFile := filepath.Join(dir, "output_log_test.txt")
	if err := os.WriteFile(logFile, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	resolved := resolveAndValidateLogDir(dir)
	if resolved == "" {
		t.Error("resolveAndValidateLogDir() = empty, want non-empty for valid dir")
	}
}

func TestResolveAndValidateLogDir_EmptyDir(t *testing.T) {
	dir := t.TempDir()

	// Directory exists but has no log files - should still return resolved path
	resolved := resolveAndValidateLogDir(dir)
	if resolved == "" {
		t.Error("resolveAndValidateLogDir() = empty, want non-empty for existing dir even without log files")
	}
}

func TestResolveAndValidateLogDir_NotExists(t *testing.T) {
	resolved := resolveAndValidateLogDir("/nonexistent/path")
	if resolved != "" {
		t.Error("resolveAndValidateLogDir() = non-empty, want empty for nonexistent path")
	}
}
