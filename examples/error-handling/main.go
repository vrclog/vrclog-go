// Package main demonstrates comprehensive error handling patterns for vrclog-go.
//
// This example shows how to properly handle all error types returned by the library:
// - Sentinel errors (ErrLogDirNotFound, ErrNoLogFiles, ErrReplayLimitExceeded)
// - ParseError with line information
// - LineTooLongError for oversized lines
// - WatchError with operation context
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vrclog/vrclog-go/pkg/vrclog"
)

func main() {
	fmt.Println("Error Handling Example")
	fmt.Println("======================")
	fmt.Println()

	// ========================================
	// 1. Sentinel Errors
	// ========================================
	fmt.Println("1. Sentinel Errors")
	fmt.Println("------------------")
	fmt.Println("Using errors.Is() to check for specific error conditions")
	fmt.Println()

	// ErrLogDirNotFound: Directory doesn't exist
	fmt.Println("→ Testing ErrLogDirNotFound:")
	_, err := vrclog.NewWatcherWithOptions(
		vrclog.WithLogDir("/nonexistent/directory/path"),
	)
	if errors.Is(err, vrclog.ErrLogDirNotFound) {
		fmt.Println("  ✓ Detected: Log directory not found")
	}
	fmt.Println()

	// ErrNoLogFiles: Directory exists but no log files
	emptyDir, err := os.MkdirTemp("", "vrclog_empty_*")
	if err != nil {
		log.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(emptyDir)

	fmt.Println("→ Testing ErrNoLogFiles:")
	_, err = vrclog.NewWatcherWithOptions(
		vrclog.WithLogDir(emptyDir),
	)
	if errors.Is(err, vrclog.ErrNoLogFiles) {
		fmt.Println("  ✓ Detected: No log files in directory")
	}
	fmt.Println()

	// ========================================
	// 2. ParseError
	// ========================================
	fmt.Println("2. ParseError")
	fmt.Println("-------------")
	fmt.Println("Using errors.As() to extract parse error details")
	fmt.Println()

	// Create a parser that intentionally fails on certain lines
	errorParser := vrclog.ParserFunc(func(ctx context.Context, line string) (vrclog.ParseResult, error) {
		if strings.Contains(line, "TRIGGER_ERROR") {
			return vrclog.ParseResult{}, fmt.Errorf("simulated parse error")
		}
		return vrclog.ParseResult{Matched: false}, nil
	})

	// Create temp file with problematic line
	tmpFile := createTempFile(`2024.01.15 10:00:00 Log        -  [Behaviour] OnPlayerJoined Alice
2024.01.15 10:01:00 Log        -  [Test] TRIGGER_ERROR bad line
2024.01.15 10:02:00 Log        -  [Behaviour] OnPlayerJoined Bob
`)
	defer os.Remove(tmpFile)

	fmt.Println("→ Parsing file with error on line 2:")
	ctx := context.Background()
	for ev, err := range vrclog.ParseFile(ctx, tmpFile,
		vrclog.WithParseParser(errorParser),
		vrclog.WithParseStopOnError(true), // Stop on first error
	) {
		if err != nil {
			var parseErr *vrclog.ParseError
			if errors.As(err, &parseErr) {
				fmt.Printf("  ✓ ParseError detected\n")
				fmt.Printf("    Line: \"%s\"\n", parseErr.Line)
				fmt.Printf("    Error: %v\n", parseErr.Err)
				break
			}
		}
		_ = ev // Process event normally
	}
	fmt.Println()

	fmt.Println("→ Comparing WithParseStopOnError behaviors:")
	fmt.Println("  With stopOnError=true:  Stops at first error")
	fmt.Println("  With stopOnError=false: Skips bad lines, continues parsing")
	fmt.Println()

	// ========================================
	// 3. LineTooLongError
	// ========================================
	fmt.Println("3. LineTooLongError")
	fmt.Println("-------------------")
	fmt.Println("Handling lines exceeding buffer limits (64KB)")
	fmt.Println()

	// Create file with very long line (65KB)
	longLine := strings.Repeat("x", 65*1024)
	longFile := createTempFile("2024.01.15 10:00:00 Log        -  " + longLine + "\n" +
		"2024.01.15 10:01:00 Log        -  [Behaviour] OnPlayerJoined Alice\n")
	defer os.Remove(longFile)

	fmt.Println("→ Parsing file with 65KB line:")
	for ev, err := range vrclog.ParseFile(ctx, longFile,
		vrclog.WithParseStopOnError(true),
	) {
		if err != nil {
			var longErr *vrclog.LineTooLongError
			if errors.As(err, &longErr) {
				fmt.Printf("  ✓ LineTooLongError detected\n")
				fmt.Printf("    Line number: %d\n", longErr.LineNumber)
				fmt.Printf("    Length: %d bytes (max: %d bytes)\n", longErr.Length, longErr.MaxLength)
				fmt.Println("    Note: This error is rare in normal VRChat logs")
				break
			}
		}
		_ = ev
	}
	fmt.Println()

	// ========================================
	// 4. WatchError with Operation Context
	// ========================================
	fmt.Println("4. WatchError")
	fmt.Println("-------------")
	fmt.Println("Using WatchError.Op to understand failure context")
	fmt.Println()

	// Create a watcher that will fail during replay
	tmpDir, err := os.MkdirTemp("", "vrclog_test_*")
	if err != nil {
		log.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a log file that will be deleted during watch
	logPath := filepath.Join(tmpDir, "output_log_test.txt")
	if err := os.WriteFile(logPath, []byte("2024.01.15 10:00:00 Log        -  [Behaviour] OnPlayerJoined Alice\n"), 0644); err != nil {
		log.Fatalf("Failed to create test file: %v", err)
	}

	watcher, err := vrclog.NewWatcherWithOptions(
		vrclog.WithLogDir(tmpDir),
		vrclog.WithReplayFromStart(),
	)
	if err != nil {
		log.Fatalf("Failed to create watcher: %v", err)
	}

	// Start watch with short timeout
	watchCtx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()

	events, errs, err := watcher.Watch(watchCtx)
	if err != nil {
		log.Fatalf("Failed to start watch: %v", err)
	}

	fmt.Println("→ Watching for errors:")
	fmt.Println("  Note: WatchError occurrence is timing-dependent in this demo")
	fmt.Println("  Real WatchErrors typically occur during I/O failures (file access, read errors)")
	done := false
	for !done {
		select {
		case _, ok := <-events:
			if !ok {
				done = true
			}
		case err, ok := <-errs:
			if !ok {
				done = true
				break
			}

			var watchErr *vrclog.WatchError
			if errors.As(err, &watchErr) {
				fmt.Printf("  ✓ WatchError detected\n")
				fmt.Printf("    Operation: %s\n", watchErr.Op)
				if watchErr.Path != "" {
					fmt.Printf("    Path: %s\n", watchErr.Path)
				}
				fmt.Printf("    Error: %v\n", watchErr.Err)

				// Demonstrate operation-specific handling
				switch watchErr.Op {
				case vrclog.WatchOpFindLatest:
					fmt.Println("    → Could retry with different directory")
				case vrclog.WatchOpReplay:
					fmt.Println("    → Could skip replay and tail from end")
				case vrclog.WatchOpParse:
					fmt.Println("    → Could use more lenient parser")
				case vrclog.WatchOpTail:
					fmt.Println("    → Could check if file was rotated")
				}
			}
		case <-time.After(300 * time.Millisecond):
			done = true
		}
	}

	watcher.Close()
	fmt.Println()

	// ========================================
	// 5. Error Classification with Type Switch
	// ========================================
	fmt.Println("5. Error Classification")
	fmt.Println("-----------------------")
	fmt.Println("Using type switch for unified error handling")
	fmt.Println()

	testErrors := []error{
		vrclog.ErrLogDirNotFound,
		&vrclog.ParseError{Line: "bad line", Err: fmt.Errorf("parse failed")},
		&vrclog.LineTooLongError{LineNumber: 5, Length: 100000, MaxLength: 65536},
		&vrclog.WatchError{Op: vrclog.WatchOpTail, Err: fmt.Errorf("tail failed")},
	}

	for i, err := range testErrors {
		fmt.Printf("→ Error %d: ", i+1)
		classifyError(err)
	}
	fmt.Println()

	// ========================================
	// Summary
	// ========================================
	fmt.Println("Summary:")
	fmt.Println("--------")
	fmt.Println("errors.Is()       : Check sentinel errors (ErrLogDirNotFound, etc.)")
	fmt.Println("errors.As()       : Extract typed errors (ParseError, WatchError, etc.)")
	fmt.Println("Type switch       : Unified error classification")
	fmt.Println("WatchError.Op     : Understand which operation failed")
	fmt.Println("WithStopOnError() : Control error propagation vs skipping")
}

// classifyError demonstrates error classification with type assertions
func classifyError(err error) {
	switch {
	case errors.Is(err, vrclog.ErrLogDirNotFound):
		fmt.Println("Sentinel error: Log directory not found")
	case errors.Is(err, vrclog.ErrNoLogFiles):
		fmt.Println("Sentinel error: No log files found")
	case errors.Is(err, vrclog.ErrReplayLimitExceeded):
		fmt.Println("Sentinel error: Replay memory limit exceeded")
	default:
		// Use type switch for complex error types
		var parseErr *vrclog.ParseError
		var longErr *vrclog.LineTooLongError
		var watchErr *vrclog.WatchError

		switch {
		case errors.As(err, &parseErr):
			fmt.Printf("ParseError: %v (line: %q)\n", parseErr.Err, parseErr.Line)
		case errors.As(err, &longErr):
			fmt.Printf("LineTooLongError: Line %d exceeds %d bytes\n", longErr.LineNumber, longErr.MaxLength)
		case errors.As(err, &watchErr):
			fmt.Printf("WatchError: %s failed (%v)\n", watchErr.Op, watchErr.Err)
		default:
			fmt.Printf("Unknown error: %v\n", err)
		}
	}
}

// createTempFile creates a temporary VRChat log file
func createTempFile(content string) string {
	tmpFile, err := os.CreateTemp("", "output_log_*.txt")
	if err != nil {
		log.Fatal(err)
	}

	if _, err := tmpFile.WriteString(content); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		log.Fatal(err)
	}

	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpFile.Name())
		log.Fatal(err)
	}

	return tmpFile.Name()
}
