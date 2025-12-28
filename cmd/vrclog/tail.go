package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/vrclog/vrclog-go/pkg/vrclog"
)

var (
	// tail flags
	logDir           string
	format           string
	tailIncludeTypes []string
	tailExcludeTypes []string
	includeRaw       bool
	replayLast       int
	replaySince      string
)

var tailCmd = &cobra.Command{
	Use:   "tail",
	Short: "Monitor VRChat logs and output events",
	Long: `Monitor VRChat log files in real-time and output parsed events.

Events are output as JSON Lines by default (one JSON object per line),
which makes it easy to process with tools like jq.

Examples:
  # Monitor with default settings (auto-detect log directory)
  vrclog tail

  # Specify log directory
  vrclog tail --log-dir "C:\Users\me\AppData\LocalLow\VRChat\VRChat"

  # Output only player join/leave events
  vrclog tail --include-types player_join,player_left

  # Exclude world_join events
  vrclog tail --exclude-types world_join

  # Human-readable output
  vrclog tail --format pretty

  # Replay from start of log file
  vrclog tail --replay-last 0  # 0 means from start

  # Pipe to jq for filtering
  vrclog tail | jq 'select(.type == "player_join")'`,
	RunE: runTail,
}

func init() {
	tailCmd.Flags().StringVarP(&logDir, "log-dir", "d", "",
		"VRChat log directory (auto-detected if not specified)")
	tailCmd.Flags().StringVarP(&format, "format", "f", "jsonl",
		"Output format: jsonl, pretty")
	tailCmd.Flags().StringSliceVar(&tailIncludeTypes, "include-types", nil,
		"Event types to include (comma-separated: world_join,player_join,player_left)")
	tailCmd.Flags().StringSliceVar(&tailExcludeTypes, "exclude-types", nil,
		"Event types to exclude (comma-separated)")
	tailCmd.Flags().BoolVar(&includeRaw, "raw", false,
		"Include raw log lines in output")

	// Replay options
	tailCmd.Flags().IntVar(&replayLast, "replay-last", -1,
		"Replay last N lines before tailing (-1 = disabled, 0 = from start)")
	tailCmd.Flags().StringVar(&replaySince, "replay-since", "",
		"Replay events since timestamp (RFC3339 format, e.g., 2024-01-15T12:00:00Z)")

	// Register completion for event type flags
	registerEventTypeCompletion(tailCmd, "include-types")
	registerEventTypeCompletion(tailCmd, "exclude-types")
}

func runTail(cmd *cobra.Command, args []string) error {
	// Validate format
	if !ValidFormats[format] {
		return fmt.Errorf("invalid format %q: must be one of: jsonl, pretty", format)
	}

	// Normalize and validate event types
	includes, err := NormalizeEventTypes(tailIncludeTypes)
	if err != nil {
		return err
	}
	excludes, err := NormalizeEventTypes(tailExcludeTypes)
	if err != nil {
		return err
	}
	if err := RejectOverlap(includes, excludes); err != nil {
		return err
	}

	// Validate replay options are not both specified
	if replayLast >= 0 && replaySince != "" {
		return fmt.Errorf("--replay-last and --replay-since cannot be used together")
	}

	// Setup context with signal handling
	ctx, stop := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Build watch options using functional options pattern
	var watchOpts []vrclog.WatchOption

	if logDir != "" {
		watchOpts = append(watchOpts, vrclog.WithLogDir(logDir))
	}

	if includeRaw {
		watchOpts = append(watchOpts, vrclog.WithIncludeRawLine(true))
	}

	// Handle replay options
	if replayLast >= 0 {
		if replayLast == 0 {
			watchOpts = append(watchOpts, vrclog.WithReplayFromStart())
		} else {
			watchOpts = append(watchOpts, vrclog.WithReplayLastN(replayLast))
		}
	} else if replaySince != "" {
		t, err := time.Parse(time.RFC3339, replaySince)
		if err != nil {
			return fmt.Errorf("invalid --replay-since format: %w", err)
		}
		watchOpts = append(watchOpts, vrclog.WithReplaySinceTime(t))
	}

	// Setup logger based on verbose flag
	if verbose {
		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}))
		watchOpts = append(watchOpts, vrclog.WithLogger(logger))
	}

	// Use library-level filtering (more efficient than CLI-side filtering)
	if len(includes) > 0 {
		watchOpts = append(watchOpts, vrclog.WithIncludeTypes(includes...))
	}
	if len(excludes) > 0 {
		watchOpts = append(watchOpts, vrclog.WithExcludeTypes(excludes...))
	}

	// Create watcher with functional options
	watcher, err := vrclog.NewWatcherWithOptions(watchOpts...)
	if err != nil {
		return err
	}
	defer watcher.Close()

	// Start watching
	events, errs, err := watcher.Watch(ctx)
	if err != nil {
		return err
	}

	// Output loop
	for {
		select {
		case event, ok := <-events:
			if !ok {
				return nil // Channel closed
			}

			// Output event (filtering is now done at library level)
			if err := OutputEvent(format, event, os.Stdout); err != nil {
				return fmt.Errorf("output error: %w", err)
			}

		case err, ok := <-errs:
			if !ok {
				return nil // Channel closed
			}
			// Always output errors to stderr
			fmt.Fprintf(os.Stderr, "warning: %v\n", err)

		case <-ctx.Done():
			return nil
		}
	}
}
