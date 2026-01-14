package vrclog

import (
	"fmt"
	"log/slog"
	"time"
)

// WatchOption configures Watch behavior using the functional options pattern.
type WatchOption func(*watchConfig)

// watchConfig holds internal configuration for the watcher.
type watchConfig struct {
	logDir             string
	pollInterval       time.Duration
	includeRawLine     bool
	replay             ReplayConfig
	maxReplayLines     int
	maxReplayBytes     int  // Maximum total bytes for replay (0 = unlimited)
	maxReplayLineBytes int  // Maximum bytes per line for replay (0 = unlimited)
	waitForLogs        bool // Wait for log files to appear if directory exists but is empty
	logger             *slog.Logger
	filter             *compiledFilter
	parser             Parser // NEW: Parser for log lines
}

// defaultWatchConfig returns a watchConfig with sensible defaults.
func defaultWatchConfig() *watchConfig {
	return &watchConfig{
		pollInterval:       2 * time.Second,
		maxReplayLines:     DefaultMaxReplayLastN,
		maxReplayBytes:     10 * 1024 * 1024, // 10MB default
		maxReplayLineBytes: 512 * 1024,       // 512KB default
		parser:             DefaultParser{},  // NEW: Default parser
	}
}

// applyWatchOptions applies functional options to a watchConfig.
func applyWatchOptions(opts []WatchOption) *watchConfig {
	cfg := defaultWatchConfig()
	for _, opt := range opts {
		if opt != nil {
			opt(cfg)
		}
	}
	return cfg
}

// validate checks for invalid option combinations.
func (c *watchConfig) validate() error {
	// Validate ReplayLastN
	if c.replay.Mode == ReplayLastN && c.replay.LastN < 0 {
		return fmt.Errorf("replay LastN must be non-negative, got %d", c.replay.LastN)
	}

	// Validate ReplayLastN against maximum limit
	if c.replay.Mode == ReplayLastN {
		maxLines := c.maxReplayLines
		if maxLines == 0 {
			maxLines = DefaultMaxReplayLastN
		}
		if maxLines > 0 && c.replay.LastN > maxLines {
			return fmt.Errorf("replay LastN (%d) exceeds maximum of %d", c.replay.LastN, maxLines)
		}
	}

	// Validate ReplaySinceTime
	if c.replay.Mode == ReplaySinceTime && c.replay.Since.IsZero() {
		return fmt.Errorf("replay Since must be set when mode is ReplaySinceTime")
	}

	// Validate PollInterval
	if c.pollInterval <= 0 {
		return fmt.Errorf("poll interval must be positive, got %v", c.pollInterval)
	}

	// Validate maxReplayBytes
	if c.maxReplayBytes < 0 {
		return fmt.Errorf("maxReplayBytes must be non-negative, got %d", c.maxReplayBytes)
	}

	// Validate maxReplayLineBytes
	if c.maxReplayLineBytes < 0 {
		return fmt.Errorf("maxReplayLineBytes must be non-negative, got %d", c.maxReplayLineBytes)
	}

	return nil
}

// WithLogDir sets the VRChat log directory.
// If not set, auto-detects from default Windows locations.
// Can also be set via VRCLOG_LOGDIR environment variable.
func WithLogDir(dir string) WatchOption {
	return func(c *watchConfig) {
		c.logDir = dir
	}
}

// WithPollInterval sets how often to check for new/rotated log files.
// Default: 2 seconds.
func WithPollInterval(interval time.Duration) WatchOption {
	return func(c *watchConfig) {
		c.pollInterval = interval
	}
}

// WithWaitForLogs configures whether to wait for log files to appear.
// When true, if the log directory exists but has no log files yet,
// the watcher will poll at pollInterval until logs appear (useful for
// starting the watcher before VRChat launches).
// When false (default), ErrNoLogFiles is returned immediately if no logs exist.
func WithWaitForLogs(wait bool) WatchOption {
	return func(c *watchConfig) {
		c.waitForLogs = wait
	}
}

// WithIncludeRawLine includes the original log line in Event.RawLine.
// Default: false.
func WithIncludeRawLine(include bool) WatchOption {
	return func(c *watchConfig) {
		c.includeRawLine = include
	}
}

// WithReplay configures replay behavior for existing log lines.
// Default: ReplayNone (only new lines).
func WithReplay(config ReplayConfig) WatchOption {
	return func(c *watchConfig) {
		c.replay = config
	}
}

// WithReplayFromStart reads from the beginning of the log file.
func WithReplayFromStart() WatchOption {
	return func(c *watchConfig) {
		c.replay = ReplayConfig{Mode: ReplayFromStart}
	}
}

// WithReplayLastN reads the last N non-empty lines before tailing.
// Empty lines are skipped and not counted towards N.
func WithReplayLastN(n int) WatchOption {
	return func(c *watchConfig) {
		c.replay = ReplayConfig{Mode: ReplayLastN, LastN: n}
	}
}

// WithReplaySinceTime reads lines since a specific timestamp.
func WithReplaySinceTime(since time.Time) WatchOption {
	return func(c *watchConfig) {
		c.replay = ReplayConfig{Mode: ReplaySinceTime, Since: since}
	}
}

// WithMaxReplayLines sets the maximum lines for ReplayLastN mode.
// 0 uses default (10000). Set to -1 for unlimited (not recommended).
func WithMaxReplayLines(max int) WatchOption {
	return func(c *watchConfig) {
		c.maxReplayLines = max
	}
}

// WithMaxReplayBytes sets the maximum total bytes to read during replay.
// Default is 10MB (10485760 bytes). Set to 0 for unlimited (not recommended).
// If the limit is exceeded during ReplayLastN, ErrReplayLimitExceeded is returned.
func WithMaxReplayBytes(max int) WatchOption {
	return func(c *watchConfig) {
		c.maxReplayBytes = max
	}
}

// WithMaxReplayLineBytes sets the maximum bytes per line during replay.
// Default is 512KB (524288 bytes). Set to 0 for unlimited (not recommended).
// If a single line exceeds this limit, ErrReplayLimitExceeded is returned.
func WithMaxReplayLineBytes(max int) WatchOption {
	return func(c *watchConfig) {
		c.maxReplayLineBytes = max
	}
}

// WithLogger sets a custom logger for debug output.
// If logger is nil, logging is disabled (default behavior).
func WithLogger(logger *slog.Logger) WatchOption {
	return func(c *watchConfig) {
		c.logger = logger
	}
}

// WithParser sets a custom parser for log line parsing.
// If p is nil, this option has no effect (the default parser remains active).
// Use this to customize how log lines are interpreted.
func WithParser(p Parser) WatchOption {
	return func(c *watchConfig) {
		if p != nil {
			c.parser = p
		}
	}
}

// WithParsers combines multiple parsers using ChainAll mode.
// At least one parser is required.
func WithParsers(parsers ...Parser) WatchOption {
	return func(c *watchConfig) {
		if len(parsers) > 0 {
			c.parser = &ParserChain{
				Mode:    ChainAll,
				Parsers: parsers,
			}
		}
	}
}

// WithIncludeTypes filters events to only include the specified types.
// If called multiple times, only the last call takes effect.
func WithIncludeTypes(types ...EventType) WatchOption {
	return func(c *watchConfig) {
		if c.filter == nil {
			c.filter = &compiledFilter{}
		}
		c.filter.include = make(map[EventType]struct{}, len(types))
		for _, t := range types {
			c.filter.include[t] = struct{}{}
		}
	}
}

// WithExcludeTypes filters out events of the specified types.
// Exclude takes precedence over include.
// If called multiple times, only the last call takes effect.
func WithExcludeTypes(types ...EventType) WatchOption {
	return func(c *watchConfig) {
		if c.filter == nil {
			c.filter = &compiledFilter{}
		}
		c.filter.exclude = make(map[EventType]struct{}, len(types))
		for _, t := range types {
			c.filter.exclude[t] = struct{}{}
		}
	}
}

// WithFilter sets both include and exclude type filters.
// Exclude takes precedence over include.
func WithFilter(include, exclude []EventType) WatchOption {
	return func(c *watchConfig) {
		c.filter = newCompiledFilter(include, exclude)
	}
}

// ParseOption configures ParseFile/ParseDir behavior.
type ParseOption func(*parseConfig)

// parseConfig holds internal configuration for parsing.
type parseConfig struct {
	filter         *compiledFilter
	includeRawLine bool
	since          time.Time
	until          time.Time
	stopOnError    bool
	parser         Parser // NEW: Parser for log lines
}

// defaultParseConfig returns a parseConfig with sensible defaults.
func defaultParseConfig() *parseConfig {
	return &parseConfig{
		parser: DefaultParser{}, // NEW: Default parser
	}
}

// applyParseOptions applies functional options to a parseConfig.
func applyParseOptions(opts []ParseOption) *parseConfig {
	cfg := defaultParseConfig()
	for _, opt := range opts {
		if opt != nil {
			opt(cfg)
		}
	}
	return cfg
}

// WithParseIncludeTypes filters events to only include the specified types.
func WithParseIncludeTypes(types ...EventType) ParseOption {
	return func(c *parseConfig) {
		if c.filter == nil {
			c.filter = &compiledFilter{}
		}
		c.filter.include = make(map[EventType]struct{}, len(types))
		for _, t := range types {
			c.filter.include[t] = struct{}{}
		}
	}
}

// WithParseExcludeTypes filters out events of the specified types.
func WithParseExcludeTypes(types ...EventType) ParseOption {
	return func(c *parseConfig) {
		if c.filter == nil {
			c.filter = &compiledFilter{}
		}
		c.filter.exclude = make(map[EventType]struct{}, len(types))
		for _, t := range types {
			c.filter.exclude[t] = struct{}{}
		}
	}
}

// WithParseFilter sets both include and exclude type filters for parsing.
func WithParseFilter(include, exclude []EventType) ParseOption {
	return func(c *parseConfig) {
		c.filter = newCompiledFilter(include, exclude)
	}
}

// WithParseIncludeRawLine includes the original log line in Event.RawLine.
func WithParseIncludeRawLine(include bool) ParseOption {
	return func(c *parseConfig) {
		c.includeRawLine = include
	}
}

// WithParseTimeRange filters events to only include those within the time range.
// since is inclusive, until is exclusive.
// Zero values are ignored (no filtering for that boundary).
func WithParseTimeRange(since, until time.Time) ParseOption {
	return func(c *parseConfig) {
		c.since = since
		c.until = until
	}
}

// WithParseSince filters events to only include those at or after the given time.
func WithParseSince(since time.Time) ParseOption {
	return func(c *parseConfig) {
		c.since = since
	}
}

// WithParseUntil filters events to only include those before the given time.
//
// Note: This optimization assumes timestamps in log files are monotonically
// increasing. If timestamps are out of order (e.g., due to timezone changes),
// some events may be skipped. For guaranteed completeness, omit this option.
func WithParseUntil(until time.Time) ParseOption {
	return func(c *parseConfig) {
		c.until = until
	}
}

// WithParseParser sets a custom parser for ParseFile/ParseDir.
// If p is nil, this option has no effect (the default parser remains active).
func WithParseParser(p Parser) ParseOption {
	return func(c *parseConfig) {
		if p != nil {
			c.parser = p
		}
	}
}

// WithParseStopOnError stops parsing on the first error instead of skipping.
// Default: false (skip malformed lines and continue).
func WithParseStopOnError(stop bool) ParseOption {
	return func(c *parseConfig) {
		c.stopOnError = stop
	}
}
