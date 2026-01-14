# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- CLI `parse` command for batch/offline log parsing
- `--include-types`, `--exclude-types` flags for both `tail` and `parse` commands
- Time range filtering with `--since` and `--until` flags in `parse` command
- `ParseFile()`, `ParseDir()` library functions for offline parsing
- **Parser interface** for pluggable log parsing (`pkg/vrclog/parser.go`)
  - `Parser` interface with `ParseLine(ctx, line) (ParseResult, error)`
  - `ParserFunc` adapter for functional parsers
  - `ParserChain` for combining parsers with modes: `ChainAll`, `ChainFirst`, `ChainContinueOnError`
  - `DefaultParser` type wrapping built-in VRChat event parsing
  - `WithParser()`, `WithParsers()` watch/parse options
- **Custom pattern matching** via YAML files (`pkg/vrclog/pattern/`)
  - `PatternFile` and `Pattern` types for YAML schema
  - `RegexParser` implementing `Parser` interface
  - Named capture groups `(?P<name>...)` populate `Event.Data` map
  - YAML pattern file format with `version`, `id`, `event_type`, `regex` fields
  - `Load()`, `LoadBytes()` for pattern file loading
  - `NewRegexParser()`, `NewRegexParserFromFile()` constructors
  - ReDoS protection: 512 byte max pattern length, 1MB max file size
- `Event.Data` field for custom key-value data from parsers
- **Runnable examples** demonstrating library features (`examples/`)
  - `custom-parser/` - Custom event extraction using `RegexParser` with YAML patterns
  - `parser-chain/` - Combining `DefaultParser` with `RegexParser` using `ParserChain`
  - `watch-events/` - Real-time log monitoring with `WatchWithOptions()`
  - `parserfunc/` - Creating custom parsers with `ParserFunc` adapter
  - `parse-files/` - Batch processing with `ParseFile()` and `ParseDir()` iterators
  - `time-filter/` - Time-based filtering with `WithParseSince()` and `WithParseUntil()`
  - `replay-options/` - Replay configuration modes (`ReplayLastN`, `ReplayFromStart`, etc.)
  - `parser-interface/` - Implementing `Parser` interface with state management
  - `error-handling/` - Comprehensive error handling with `errors.Is()` and `errors.As()`
  - `event-filtering/` - Event type filtering with `WithExcludeTypes()` and `WithFilter()`
  - `parser-chain-modes/` - `ParserChain` modes: `ChainAll`, `ChainFirst`, `ChainContinueOnError`
  - `parser-decorator/` - Decorator pattern for extending parsers (`MetricsParser`, `TransformingParser`)
  - `graceful-shutdown/` - Watcher lifecycle management with `Watcher.Close()` and `sync.WaitGroup`

### Changed

- `tail --types` replaced with `--include-types` (breaking change)
- Event type filtering is now case-insensitive and trims whitespace

### Security

- Pattern file FIFO/device DoS protection (rejects non-regular files)
- Pattern file size limits (1MB max, prevents OOM)
- Regex pattern length limits (512 bytes, ReDoS mitigation)
- Race condition fix in `FindLatestLogFile()` (nil-deref prevention)
- Symlink resolution failure handling (no fallback to prevent attacks)
- `ChainContinueOnError` now preserves events from partially successful parsers
- `NewRegexParser()` enforces validation even when `PatternFile` is constructed programmatically
- Fuzz tests for robustness (`FuzzRegexParser_ParseLine`, `FuzzLoadBytes`) to ensure panic-free operation with arbitrary input

## [0.1.0] - Initial Release

### Added

- Initial implementation of VRChat log parser and watcher
- `vrclog.Watch()` function for real-time log monitoring
- `vrclog.NewWatcher()` for advanced watcher configuration
- `vrclog.ParseLine()` for parsing individual log lines
- Event types: `world_join`, `player_join`, `player_left`
- Replay functionality with `ReplayConfig` options
- CLI tool with `tail` command
- JSON Lines and pretty output formats
- Event type filtering
- Log directory auto-detection
- Log file rotation handling

### Documentation

- README.md with usage examples
- README.ja.md (Japanese translation)
- Package documentation in doc.go
