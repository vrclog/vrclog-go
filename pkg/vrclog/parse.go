package vrclog

import (
	"bufio"
	"context"
	"errors"
	"iter"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/vrclog/vrclog-go/internal/logfinder"
)

// ParseLine parses a single VRChat log line into an Event.
//
// LEGACY API - This function has several limitations:
//   - Cannot be cancelled or timeout (uses context.Background internally)
//   - Returns only the first event if multiple events are parsed from one line
//   - For more control, use ParseFile with custom parser options
//
// Return values:
//   - (*Event, nil): Successfully parsed event
//   - (nil, nil): Line doesn't match any known event pattern (not an error)
//   - (nil, error): Line partially matches but is malformed
//
// Example:
//
//	line := "2024.01.15 23:59:59 Log - [Behaviour] OnPlayerJoined TestUser"
//	event, err := vrclog.ParseLine(line)
//	if err != nil {
//	    log.Printf("parse error: %v", err)
//	} else if event != nil {
//	    fmt.Printf("Player joined: %s\n", event.PlayerName)
//	}
//	// event == nil && err == nil means line is not a recognized event
func ParseLine(line string) (*Event, error) {
	// Maintain backward compatibility by using DefaultParser
	result, err := DefaultParser{}.ParseLine(context.Background(), line)
	if err != nil {
		return nil, err
	}
	if !result.Matched || len(result.Events) == 0 {
		return nil, nil
	}
	return &result.Events[0], nil
}

// ParseFile parses a VRChat log file and returns an iterator over events.
// The file is opened lazily on first iteration, so the returned iterator
// is cheap to create but must be consumed to release resources.
//
// The iterator yields (Event, error) pairs. When an error occurs:
//   - File open errors: yields (Event{}, error) once and stops
//   - Parse errors: skips the line by default, or stops if WithParseStopOnError is set
//   - Context cancellation: yields (Event{}, ctx.Err()) and stops
//
// Example:
//
//	for ev, err := range vrclog.ParseFile(ctx, "output_log.txt") {
//	    if err != nil {
//	        log.Printf("error: %v", err)
//	        break
//	    }
//	    fmt.Printf("event: %+v\n", ev)
//	}
func ParseFile(ctx context.Context, path string, opts ...ParseOption) iter.Seq2[Event, error] {
	// Validate path upfront
	if path == "" {
		return func(yield func(Event, error) bool) {
			yield(Event{}, errors.New("vrclog: path required"))
		}
	}

	cfg := applyParseOptions(opts)

	return func(yield func(Event, error) bool) {
		// Lazy file open
		file, err := os.Open(path)
		if err != nil {
			yield(Event{}, err)
			return
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		// Increase buffer size for long lines
		buf := make([]byte, 0, 64*1024)
		scanner.Buffer(buf, 512*1024)

		for scanner.Scan() {
			// Context cancellation check
			if err := ctx.Err(); err != nil {
				yield(Event{}, err)
				return
			}

			line := scanner.Text()
			result, err := cfg.parser.ParseLine(ctx, line)
			if err != nil {
				if cfg.stopOnError {
					yield(Event{}, &ParseError{Line: line, Err: err})
					return
				}
				// Skip malformed lines by default
				continue
			}
			if !result.Matched {
				continue // Not a recognized event
			}

			// Process all events from the result
			for _, ev := range result.Events {
				// Apply event type filter
				if cfg.filter != nil && !cfg.filter.Allows(EventType(ev.Type)) {
					continue
				}

				// Apply time range filter
				if !cfg.since.IsZero() && ev.Timestamp.Before(cfg.since) {
					continue
				}
				if !cfg.until.IsZero() && ev.Timestamp.After(cfg.until) {
					return // Past the time window, stop iteration
				}

				// Include raw line if requested
				if cfg.includeRawLine {
					ev.RawLine = line
				}

				if !yield(ev, nil) {
					return // Consumer requested stop (break)
				}
			}
		}

		// Check for scanner errors
		if err := scanner.Err(); err != nil {
			yield(Event{}, err)
		}
	}
}

// ParseFileAll is a convenience function that parses a log file and collects
// all events into a slice. Stops on first error and returns events collected so far.
//
// For large files, consider using ParseFile directly to avoid loading all events
// into memory at once.
//
// Example:
//
//	events, err := vrclog.ParseFileAll(ctx, "output_log.txt",
//	    vrclog.WithParseIncludeTypes(vrclog.EventPlayerJoin),
//	)
//	if err != nil {
//	    log.Printf("error: %v", err)
//	}
//	for _, ev := range events {
//	    fmt.Printf("player joined: %s\n", ev.PlayerName)
//	}
func ParseFileAll(ctx context.Context, path string, opts ...ParseOption) ([]Event, error) {
	seq := ParseFile(ctx, path, opts...)
	events := make([]Event, 0, 256)

	for ev, err := range seq {
		if err != nil {
			return events, err
		}
		events = append(events, ev)
	}
	return events, nil
}

// ParseDirOption configures ParseDir behavior.
type ParseDirOption func(*parseDirConfig)

// parseDirConfig holds internal configuration for directory parsing.
type parseDirConfig struct {
	parseConfig
	logDir string
	paths  []string // explicit file paths (optional)
}

// defaultParseDirConfig returns a parseDirConfig with sensible defaults.
func defaultParseDirConfig() *parseDirConfig {
	return &parseDirConfig{
		parseConfig: *defaultParseConfig(),
	}
}

// applyParseDirOptions applies functional options to a parseDirConfig.
func applyParseDirOptions(opts []ParseDirOption) *parseDirConfig {
	cfg := defaultParseDirConfig()
	for _, opt := range opts {
		if opt != nil {
			opt(cfg)
		}
	}
	return cfg
}

// WithDirLogDir sets the log directory to parse.
// If not set, auto-detects from default Windows locations.
func WithDirLogDir(dir string) ParseDirOption {
	return func(c *parseDirConfig) {
		c.logDir = dir
	}
}

// WithDirPaths sets explicit file paths to parse.
// If set, LogDir is ignored.
func WithDirPaths(paths ...string) ParseDirOption {
	return func(c *parseDirConfig) {
		c.paths = paths
	}
}

// WithDirIncludeTypes filters events to only include the specified types.
func WithDirIncludeTypes(types ...EventType) ParseDirOption {
	return func(c *parseDirConfig) {
		if c.filter == nil {
			c.filter = &compiledFilter{}
		}
		c.filter.include = make(map[EventType]struct{}, len(types))
		for _, t := range types {
			c.filter.include[t] = struct{}{}
		}
	}
}

// WithDirExcludeTypes filters out events of the specified types.
func WithDirExcludeTypes(types ...EventType) ParseDirOption {
	return func(c *parseDirConfig) {
		if c.filter == nil {
			c.filter = &compiledFilter{}
		}
		c.filter.exclude = make(map[EventType]struct{}, len(types))
		for _, t := range types {
			c.filter.exclude[t] = struct{}{}
		}
	}
}

// WithDirTimeRange filters events to only include those within the time range.
func WithDirTimeRange(since, until time.Time) ParseDirOption {
	return func(c *parseDirConfig) {
		c.since = since
		c.until = until
	}
}

// WithDirIncludeRawLine includes the original log line in Event.RawLine.
func WithDirIncludeRawLine(include bool) ParseDirOption {
	return func(c *parseDirConfig) {
		c.includeRawLine = include
	}
}

// WithDirStopOnError stops parsing on the first error instead of skipping.
func WithDirStopOnError(stop bool) ParseDirOption {
	return func(c *parseDirConfig) {
		c.stopOnError = stop
	}
}

// WithDirParser sets a custom parser for ParseDir.
// If p is nil, this option has no effect (the default parser remains active).
func WithDirParser(p Parser) ParseDirOption {
	return func(c *parseDirConfig) {
		if p != nil {
			c.parser = p
		}
	}
}

// ParseDir parses all VRChat log files in a directory, yielding events
// in chronological order (by file modification time, oldest first).
//
// The iterator yields (Event, error) pairs. When an error occurs:
//   - Directory access errors: yields (Event{}, error) once and stops
//   - File errors: skips to next file by default, or stops if WithDirStopOnError is set
//   - Parse errors: skips the line by default, or stops if WithDirStopOnError is set
//
// Example:
//
//	for ev, err := range vrclog.ParseDir(ctx,
//	    vrclog.WithDirIncludeTypes(vrclog.EventWorldJoin),
//	) {
//	    if err != nil {
//	        log.Printf("error: %v", err)
//	        break
//	    }
//	    fmt.Printf("world: %s\n", ev.WorldName)
//	}
func ParseDir(ctx context.Context, opts ...ParseDirOption) iter.Seq2[Event, error] {
	cfg := applyParseDirOptions(opts)

	return func(yield func(Event, error) bool) {
		var files []string
		var err error

		if len(cfg.paths) > 0 {
			// Use explicit paths
			files = cfg.paths
		} else {
			// Find log directory and list files
			logDir := cfg.logDir
			if logDir == "" {
				logDir, err = logfinder.FindLogDir("")
				if err != nil {
					yield(Event{}, err)
					return
				}
			}

			// List all log files
			files, err = listLogFiles(logDir)
			if err != nil {
				yield(Event{}, err)
				return
			}
		}

		if len(files) == 0 {
			yield(Event{}, ErrNoLogFiles)
			return
		}

		// Build ParseOptions from config
		// NOTE: This rebuilds the filter from maps to slices, which is somewhat inefficient.
		// A future optimization could pass parseConfig directly or use internal options.
		// However, the impact is minimal since filters are typically small.
		var parseOpts []ParseOption
		if cfg.filter != nil {
			include := make([]EventType, 0, len(cfg.filter.include))
			for t := range cfg.filter.include {
				include = append(include, t)
			}
			exclude := make([]EventType, 0, len(cfg.filter.exclude))
			for t := range cfg.filter.exclude {
				exclude = append(exclude, t)
			}
			parseOpts = append(parseOpts, WithParseFilter(include, exclude))
		}
		if !cfg.since.IsZero() || !cfg.until.IsZero() {
			parseOpts = append(parseOpts, WithParseTimeRange(cfg.since, cfg.until))
		}
		if cfg.includeRawLine {
			parseOpts = append(parseOpts, WithParseIncludeRawLine(true))
		}
		if cfg.stopOnError {
			parseOpts = append(parseOpts, WithParseStopOnError(true))
		}
		if cfg.parser != nil {
			parseOpts = append(parseOpts, WithParseParser(cfg.parser))
		}

		// Parse each file
		for _, file := range files {
			if ctx.Err() != nil {
				yield(Event{}, ctx.Err())
				return
			}

			for ev, err := range ParseFile(ctx, file, parseOpts...) {
				if err != nil {
					if cfg.stopOnError {
						yield(Event{}, err)
						return
					}
					// Skip to next file on error
					break
				}
				if !yield(ev, nil) {
					return // Consumer requested stop
				}
			}
		}
	}
}

// listLogFiles returns all VRChat log files in the directory,
// sorted by modification time (oldest first).
func listLogFiles(dir string) ([]string, error) {
	pattern := filepath.Join(dir, "output_log_*.txt")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	if len(matches) == 0 {
		return nil, nil
	}

	// Sort by modification time (oldest first)
	type fileInfo struct {
		path    string
		modTime int64
	}
	files := make([]fileInfo, 0, len(matches))
	for _, path := range matches {
		info, err := os.Stat(path)
		if err != nil {
			continue // Skip files we can't stat
		}
		files = append(files, fileInfo{path: path, modTime: info.ModTime().UnixNano()})
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime < files[j].modTime
	})

	result := make([]string, len(files))
	for i, f := range files {
		result[i] = f.path
	}
	return result, nil
}
