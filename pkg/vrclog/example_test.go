package vrclog_test

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/vrclog/vrclog-go/pkg/vrclog"
)

// ExampleWatch demonstrates basic usage of the Watch convenience function.
func ExampleWatch() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start watching with default options (auto-detect log directory)
	events, errs, err := vrclog.Watch(ctx, vrclog.WatchOptions{})
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

// ExampleNewWatcher demonstrates advanced usage with explicit Watcher control.
func ExampleNewWatcher() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create watcher with custom options
	watcher, err := vrclog.NewWatcher(vrclog.WatchOptions{
		LogDir:         "", // auto-detect
		PollInterval:   5 * time.Second,
		IncludeRawLine: true,
		Replay: vrclog.ReplayConfig{
			Mode:  vrclog.ReplayLastN,
			LastN: 100,
		},
	})
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
