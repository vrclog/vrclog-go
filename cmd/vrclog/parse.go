package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/vrclog/vrclog-go/pkg/vrclog"
)

var (
	// parse flags
	parseLogDir       string
	parseIncludeTypes []string
	parseExcludeTypes []string
	parseSince        string
	parseUntil        string
	parseFormat       string
	parseRaw          bool
	parseStopOnError  bool
	parsePatterns     []string
)

var parseCmd = &cobra.Command{
	Use:   "parse [files...]",
	Short: "Parse VRChat log files (batch mode)",
	Long: `Parse all VRChat log files in a directory and output events.

Unlike 'tail', this command processes historical files without real-time
following. It reads all matching log files in chronological order.

Examples:
  # Parse all logs in auto-detected directory
  vrclog parse

  # Specify log directory
  vrclog parse --log-dir "C:\Users\me\AppData\LocalLow\VRChat\VRChat"

  # Filter by time range (useful for multi-day queries)
  vrclog parse --since "2024-01-15T12:00:00Z" --until "2024-01-16T00:00:00Z"

  # Filter by event type
  vrclog parse --include-types player_join,player_left

  # Human-readable output
  vrclog parse --format pretty

  # Parse specific files
  vrclog parse output_log_2024-01-15.txt output_log_2024-01-16.txt

  # Pipe to jq for filtering
  vrclog parse | jq 'select(.type == "world_join")'`,
	RunE: runParse,
}

func init() {
	parseCmd.Flags().StringVarP(&parseLogDir, "log-dir", "d", "",
		"VRChat log directory (auto-detected if not specified)")
	parseCmd.Flags().StringSliceVar(&parseIncludeTypes, "include-types", nil,
		"Event types to include (comma-separated: world_join,player_join,player_left)")
	parseCmd.Flags().StringSliceVar(&parseExcludeTypes, "exclude-types", nil,
		"Event types to exclude (comma-separated)")
	parseCmd.Flags().StringVar(&parseSince, "since", "",
		"Only events at/after timestamp (RFC3339 format, e.g., 2024-01-15T12:00:00Z)")
	parseCmd.Flags().StringVar(&parseUntil, "until", "",
		"Only events before timestamp (RFC3339 format)")
	parseCmd.Flags().StringVarP(&parseFormat, "format", "f", "jsonl",
		"Output format: jsonl, pretty")
	parseCmd.Flags().BoolVar(&parseRaw, "raw", false,
		"Include raw log lines in output")
	parseCmd.Flags().BoolVar(&parseStopOnError, "stop-on-error", false,
		"Stop on first error instead of skipping")

	// Pattern file flag
	parseCmd.Flags().StringSliceVar(&parsePatterns, "patterns", nil,
		"YAML pattern file(s) for custom event detection (can be specified multiple times)")

	// Enable filename completion for --patterns
	_ = parseCmd.MarkFlagFilename("patterns", "yaml", "yml")

	// Register completion for event type flags
	registerEventTypeCompletion(parseCmd, "include-types")
	registerEventTypeCompletion(parseCmd, "exclude-types")
}

func runParse(cmd *cobra.Command, args []string) error {
	// Validate format
	if !ValidFormats[parseFormat] {
		return fmt.Errorf("invalid format %q: must be one of: jsonl, pretty", parseFormat)
	}

	// Normalize and validate event types
	includes, err := NormalizeEventTypes(parseIncludeTypes)
	if err != nil {
		return err
	}
	excludes, err := NormalizeEventTypes(parseExcludeTypes)
	if err != nil {
		return err
	}
	if err := RejectOverlap(includes, excludes); err != nil {
		return err
	}

	// Parse time range
	sinceTime, untilTime, err := parseTimeRange(parseSince, parseUntil)
	if err != nil {
		return err
	}

	// Build parser from pattern files
	parser, err := buildParser(parsePatterns)
	if err != nil {
		return err
	}

	// Setup context with signal handling
	ctx, stop := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Build parse options
	var opts []vrclog.ParseDirOption

	// Add custom parser if specified
	if parser != nil {
		opts = append(opts, vrclog.WithDirParser(parser))
	}

	if parseLogDir != "" {
		opts = append(opts, vrclog.WithDirLogDir(parseLogDir))
	}

	// Use positional args as explicit file paths
	if len(args) > 0 {
		opts = append(opts, vrclog.WithDirPaths(args...))
	}

	if len(includes) > 0 {
		opts = append(opts, vrclog.WithDirIncludeTypes(includes...))
	}
	if len(excludes) > 0 {
		opts = append(opts, vrclog.WithDirExcludeTypes(excludes...))
	}

	if !sinceTime.IsZero() || !untilTime.IsZero() {
		opts = append(opts, vrclog.WithDirTimeRange(sinceTime, untilTime))
	}

	if parseRaw {
		opts = append(opts, vrclog.WithDirIncludeRawLine(true))
	}
	if parseStopOnError {
		opts = append(opts, vrclog.WithDirStopOnError(true))
	}

	// Parse all files
	for ev, err := range vrclog.ParseDir(ctx, opts...) {
		if err != nil {
			// Ctrl+C: exit silently
			if errors.Is(err, context.Canceled) && ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("parse error: %w", err)
		}

		if err := OutputEvent(parseFormat, ev, os.Stdout); err != nil {
			return fmt.Errorf("output error: %w", err)
		}
	}

	return nil
}

// parseTimeRange parses since and until strings into time.Time values.
func parseTimeRange(since, until string) (time.Time, time.Time, error) {
	var sinceTime, untilTime time.Time
	var err error

	if since != "" {
		sinceTime, err = time.Parse(time.RFC3339, since)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid --since format: %w (expected RFC3339, e.g., 2024-01-15T12:00:00Z)", err)
		}
	}

	if until != "" {
		untilTime, err = time.Parse(time.RFC3339, until)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid --until format: %w (expected RFC3339, e.g., 2024-01-15T12:00:00Z)", err)
		}
	}

	// Validate that since is before until
	if !sinceTime.IsZero() && !untilTime.IsZero() && sinceTime.After(untilTime) {
		return time.Time{}, time.Time{}, fmt.Errorf("--since must be before --until")
	}

	return sinceTime, untilTime, nil
}
