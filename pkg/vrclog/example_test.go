package vrclog_test

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/vrclog/vrclog-go/pkg/vrclog"
	"github.com/vrclog/vrclog-go/pkg/vrclog/event"
)

// ExampleWatchWithOptions demonstrates basic usage of the WatchWithOptions convenience function.
func ExampleWatchWithOptions() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start watching with functional options (auto-detect log directory)
	events, errs, err := vrclog.WatchWithOptions(ctx,
		vrclog.WithIncludeTypes(vrclog.EventPlayerJoin, vrclog.EventPlayerLeft),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Process events
	for {
		select {
		case event, ok := <-events:
			if !ok {
				return
			}
			switch event.Type {
			case vrclog.EventPlayerJoin:
				fmt.Printf("%s joined\n", event.PlayerName)
			case vrclog.EventPlayerLeft:
				fmt.Printf("%s left\n", event.PlayerName)
			case vrclog.EventWorldJoin:
				fmt.Printf("Joined world: %s\n", event.WorldName)
			}
		case err, ok := <-errs:
			if !ok {
				return
			}
			log.Printf("error: %v", err)
		case <-ctx.Done():
			return
		}
	}
}

// ExampleNewWatcherWithOptions demonstrates advanced usage with explicit Watcher control.
func ExampleNewWatcherWithOptions() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create watcher with functional options
	watcher, err := vrclog.NewWatcherWithOptions(
		// LogDir auto-detected if not specified
		vrclog.WithPollInterval(5*time.Second),
		vrclog.WithIncludeRawLine(true),
		vrclog.WithReplayLastN(100),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	// Start watching
	events, errs, err := watcher.Watch(ctx)
	if err != nil {
		log.Fatal(err)
	}

	for {
		select {
		case event, ok := <-events:
			if !ok {
				return
			}
			fmt.Printf("[%s] %s: %s\n",
				event.Timestamp.Format("15:04:05"),
				event.Type,
				event.PlayerName)
		case err, ok := <-errs:
			if !ok {
				return
			}
			log.Printf("error: %v", err)
		case <-ctx.Done():
			return
		}
	}
}

// ExampleParseLine demonstrates parsing a single log line.
func ExampleParseLine() {
	line := "2024.01.15 23:59:59 Log        -  [Behaviour] OnPlayerJoined TestUser"

	event, err := vrclog.ParseLine(line)
	if err != nil {
		log.Printf("parse error: %v", err)
		return
	}

	if event == nil {
		// Line doesn't match any known event pattern
		fmt.Println("Not a recognized event")
		return
	}

	fmt.Printf("Type: %s\n", event.Type)
	fmt.Printf("Player: %s\n", event.PlayerName)
	// Output:
	// Type: player_join
	// Player: TestUser
}

// ExampleParseLine_worldJoin demonstrates parsing a world join event.
func ExampleParseLine_worldJoin() {
	line := "2024.01.15 23:59:59 Log        -  [Behaviour] Entering Room: Test World"

	event, err := vrclog.ParseLine(line)
	if err != nil {
		log.Printf("parse error: %v", err)
		return
	}

	if event != nil {
		fmt.Printf("Type: %s\n", event.Type)
		fmt.Printf("World: %s\n", event.WorldName)
	}
	// Output:
	// Type: world_join
	// World: Test World
}

// Example_errorsIs demonstrates how to check for sentinel errors using errors.Is.
// This is useful for checking specific error conditions regardless of wrapping.
func Example_errorsIs() {
	// Simulate a wrapped error (e.g., from NewWatcherWithOptions)
	err := fmt.Errorf("failed to initialize watcher: %w", vrclog.ErrLogDirNotFound)

	// Use errors.Is to check for specific sentinel errors
	if errors.Is(err, vrclog.ErrLogDirNotFound) {
		fmt.Println("VRChat log directory not found")
	}
	// Output: VRChat log directory not found
}

// Example_errorsAs_ParseError demonstrates how to extract ParseError details.
// ParseError is returned when a log line matches an event pattern but has invalid data.
func Example_errorsAs_ParseError() {
	// Simulate a parse error
	originalErr := fmt.Errorf("invalid timestamp")
	err := fmt.Errorf("processing failed: %w", &vrclog.ParseError{
		Line: "malformed log line here",
		Err:  originalErr,
	})

	// Use errors.As to extract the ParseError
	var parseErr *vrclog.ParseError
	if errors.As(err, &parseErr) {
		fmt.Printf("Failed to parse line: %q\n", parseErr.Line)
		fmt.Printf("Cause: %v\n", parseErr.Err)
	}
	// Output:
	// Failed to parse line: "malformed log line here"
	// Cause: invalid timestamp
}

// Example_errorsAs_WatchError demonstrates how to extract WatchError details.
// WatchError is returned for errors during watch operations (tail, parse, rotation).
func Example_errorsAs_WatchError() {
	// Simulate a watch error
	err := fmt.Errorf("watcher failed: %w", &vrclog.WatchError{
		Op:   vrclog.WatchOpTail,
		Path: "/path/to/log.txt",
		Err:  fmt.Errorf("file not accessible"),
	})

	// Use errors.As to extract the WatchError
	var watchErr *vrclog.WatchError
	if errors.As(err, &watchErr) {
		fmt.Printf("Operation: %s\n", watchErr.Op)
		fmt.Printf("Path: %s\n", watchErr.Path)
		fmt.Printf("Error: %v\n", watchErr.Err)
	}
	// Output:
	// Operation: tail
	// Path: /path/to/log.txt
	// Error: file not accessible
}

// ExampleParserChain demonstrates combining multiple parsers.
func ExampleParserChain() {
	ctx := context.Background()

	// Create a custom parser using ParserFunc
	customParser := vrclog.ParserFunc(func(ctx context.Context, line string) (vrclog.ParseResult, error) {
		// Match lines containing "[Custom]"
		if strings.Contains(line, "[Custom]") {
			return vrclog.ParseResult{
				Events:  []event.Event{{Type: "custom_event"}},
				Matched: true,
			}, nil
		}
		return vrclog.ParseResult{Matched: false}, nil
	})

	// Combine with default parser using ChainAll mode
	chain := &vrclog.ParserChain{
		Mode: vrclog.ChainAll,
		Parsers: []vrclog.Parser{
			vrclog.DefaultParser{},
			customParser,
		},
	}

	// Parse a VRChat log line
	result, err := chain.ParseLine(ctx, "2024.01.15 23:59:59 Log - [Behaviour] OnPlayerJoined TestUser")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Matched: %v, Events: %d\n", result.Matched, len(result.Events))
	// Output: Matched: true, Events: 1
}

// ExampleParserFunc demonstrates creating a parser from a function.
func ExampleParserFunc() {
	ctx := context.Background()

	// Create a simple parser that matches lines starting with "CUSTOM:"
	parser := vrclog.ParserFunc(func(ctx context.Context, line string) (vrclog.ParseResult, error) {
		if strings.HasPrefix(line, "CUSTOM:") {
			return vrclog.ParseResult{
				Events: []event.Event{{
					Type: "custom",
					Data: map[string]string{"content": strings.TrimPrefix(line, "CUSTOM:")},
				}},
				Matched: true,
			}, nil
		}
		return vrclog.ParseResult{Matched: false}, nil
	})

	result, _ := parser.ParseLine(ctx, "CUSTOM:hello world")
	if result.Matched {
		fmt.Printf("Content: %s\n", result.Events[0].Data["content"])
	}
	// Output: Content: hello world
}

// Example_parseResult demonstrates the ParseResult structure.
func Example_parseResult() {
	// ParseResult has two fields:
	// - Events: slice of parsed events
	// - Matched: whether the parser recognized the input

	// Result with events
	result := vrclog.ParseResult{
		Events:  []event.Event{{Type: "test"}},
		Matched: true,
	}

	// Matched can be true even with empty Events
	// (e.g., a filter that matches but outputs nothing)
	filterResult := vrclog.ParseResult{
		Events:  nil,
		Matched: true,
	}

	fmt.Printf("result.Matched=%v, events=%d\n", result.Matched, len(result.Events))
	fmt.Printf("filterResult.Matched=%v, events=%d\n", filterResult.Matched, len(filterResult.Events))
	// Output:
	// result.Matched=true, events=1
	// filterResult.Matched=true, events=0
}
