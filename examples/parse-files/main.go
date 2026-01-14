// Package main demonstrates batch processing of VRChat log files.
//
// This example shows how to parse existing log files using ParseFile, ParseFileAll,
// and ParseDir - useful for analyzing historical log data.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/vrclog/vrclog-go/pkg/vrclog"
)

// VRChat log format: "2024.01.15 23:59:59 Log        -  [Behaviour] OnPlayerJoined Name"
// IMPORTANT: "Log" followed by 10 spaces, "-", then 2 spaces
const testLogData = `2024.01.15 10:00:00 Log        -  [Behaviour] OnPlayerJoined Alice
2024.01.15 10:05:00 Log        -  [Behaviour] OnPlayerJoined Bob
2024.01.15 10:10:00 Log        -  [Behaviour] Entering Room: Test World
2024.01.15 10:15:00 Log        -  [Behaviour] OnPlayerLeft Alice
2024.01.15 10:20:00 Log        -  [Behaviour] OnPlayerJoined Charlie
`

func main() {
	ctx := context.Background()

	fmt.Println("Batch Processing Example")
	fmt.Println("========================")
	fmt.Println()

	// Create a temporary log file for demonstration
	tmpFile, err := createTempLogFile(testLogData)
	if err != nil {
		log.Fatal(err)
	}
	defer os.Remove(tmpFile)

	fmt.Printf("Created temporary log file: %s\n\n", tmpFile)

	// ========================================
	// Example 1: ParseFile - Iterator-based parsing
	// ========================================
	fmt.Println("Example 1: ParseFile (iterator-based)")
	fmt.Println("--------------------------------------")
	fmt.Println("Memory-efficient streaming, supports early termination")
	fmt.Println()

	eventCount := 0
	for ev, err := range vrclog.ParseFile(ctx, tmpFile) {
		if err != nil {
			log.Printf("Error: %v", err)
			break // Early termination on error
		}

		eventCount++
		fmt.Printf("[%d] %s - %s", eventCount, ev.Type, ev.Timestamp.Format("15:04:05"))
		if ev.PlayerName != "" {
			fmt.Printf(" | Player: %s", ev.PlayerName)
		}
		if ev.WorldName != "" {
			fmt.Printf(" | World: %s", ev.WorldName)
		}
		fmt.Println()

		// Example: stop after 3 events (demonstrates early termination)
		if eventCount == 3 {
			fmt.Println("(stopped after 3 events to demonstrate break)")
			break
		}
	}
	fmt.Printf("\nProcessed %d events\n\n", eventCount)

	// ========================================
	// Example 2: ParseFileAll - Collect all events
	// ========================================
	fmt.Println("Example 2: ParseFileAll (collect all)")
	fmt.Println("--------------------------------------")
	fmt.Println("Convenient function that returns all events as a slice")
	fmt.Println()

	events, err := vrclog.ParseFileAll(ctx, tmpFile)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Total events: %d\n", len(events))
	for i, ev := range events {
		fmt.Printf("[%d] %s - %s\n", i+1, ev.Type, ev.Timestamp.Format("15:04:05"))
	}
	fmt.Println()

	// ========================================
	// Example 3: ParseDir - Multiple files
	// ========================================
	fmt.Println("Example 3: ParseDir (multiple files)")
	fmt.Println("-------------------------------------")
	fmt.Println("Process multiple log files in chronological order")
	fmt.Println()

	// Create multiple temporary log files with different timestamps
	logFile1, logFile2, err := createMultipleLogFiles()
	if err != nil {
		log.Fatal(err)
	}
	defer os.Remove(logFile1)
	defer os.Remove(logFile2)

	fmt.Printf("File 1: %s\n", filepath.Base(logFile1))
	fmt.Printf("File 2: %s\n", filepath.Base(logFile2))
	fmt.Println()

	// ParseDir with explicit file paths
	// Note: Uses ParseDirOption (not ParseOption)
	eventCount = 0
	for ev, err := range vrclog.ParseDir(ctx,
		vrclog.WithDirPaths(logFile1, logFile2),
	) {
		if err != nil {
			log.Printf("Error: %v", err)
			break
		}

		eventCount++
		fmt.Printf("[%d] %s - %s", eventCount, ev.Type, ev.Timestamp.Format("15:04:05"))
		if ev.PlayerName != "" {
			fmt.Printf(" | %s", ev.PlayerName)
		}
		fmt.Println()
	}
	fmt.Printf("\nTotal events from both files: %d\n\n", eventCount)

	// ========================================
	// Key Differences
	// ========================================
	fmt.Println("Key Differences:")
	fmt.Println("----------------")
	fmt.Println("ParseFile      : iter.Seq2, memory-efficient, supports break")
	fmt.Println("ParseFileAll   : Returns []Event, convenient for small files")
	fmt.Println("ParseDir       : Multiple files, chronological order")
	fmt.Println("                 Uses ParseDirOption (WithDirPaths)")
}

// createTempLogFile creates a temporary log file with test data
func createTempLogFile(data string) (string, error) {
	tmpFile, err := os.CreateTemp("", "output_log_*.txt")
	if err != nil {
		return "", err
	}

	if _, err := tmpFile.WriteString(data); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return "", err
	}

	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpFile.Name())
		return "", err
	}

	return tmpFile.Name(), nil
}

// createMultipleLogFiles creates two log files with different timestamps
func createMultipleLogFiles() (string, string, error) {
	// File 1: Morning session
	data1 := `2024.01.15 10:00:00 Log        -  [Behaviour] OnPlayerJoined Alice
2024.01.15 10:05:00 Log        -  [Behaviour] OnPlayerJoined Bob
`

	// File 2: Afternoon session
	data2 := `2024.01.15 15:00:00 Log        -  [Behaviour] OnPlayerJoined Charlie
2024.01.15 15:05:00 Log        -  [Behaviour] OnPlayerLeft Bob
`

	file1, err := createTempLogFile(data1)
	if err != nil {
		return "", "", err
	}

	file2, err := createTempLogFile(data2)
	if err != nil {
		os.Remove(file1)
		return "", "", err
	}

	return file1, file2, nil
}
