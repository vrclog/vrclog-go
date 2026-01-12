# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

vrclog-go is a Go library and CLI for parsing and monitoring VRChat PC log files. It extracts structured events (player join/leave, world join) from VRChat's `output_log_*.txt` files on Windows.

## API Policy

vrclog-go follows [Semantic Versioning](https://semver.org/). While the module is in the **v0.x.x** series:

- APIs may be added, changed, or removed without a deprecation period
- No backward-compatibility guarantees are provided before v1.0.0
- Deprecated code is removed immediately rather than maintained

**Guidance for Claude**: When modifying exported APIs, prefer clean removals over backward-compatibility shims. Don't add legacy wrappers, re-exports, or compatibility layers. It's fine to delete deprecated code without maintaining old interfaces while we're pre-1.0.

Reference: [Go Module Version Numbering](https://go.dev/doc/modules/version-numbers)

## Common Commands

```bash
# Build
make build                    # Build CLI binary
make build-windows            # Cross-compile for Windows

# Test
go test ./...                 # Run all tests
go test -v ./internal/parser  # Run specific package tests
go test -run TestName ./...   # Run single test by name
go test -race ./...           # With race detector
make test-cover               # Generate coverage report

# Lint (requires golangci-lint v2)
make lint                     # Run golangci-lint
make fmt-check                # Check formatting (used in CI)

# Format
go fmt ./...

# Other
make tidy                     # go mod tidy
make vet                      # go vet
make clean                    # Remove build artifacts
make release-snapshot         # Test goreleaser locally
```

## Architecture

### Package Structure

```
pkg/vrclog/           # Public API - users import this
├── event/            # Event type (separate to avoid import cycles)
│   └── event.go      # Event struct, Type enum, TypeNames()
├── pattern/          # Custom pattern matching (Phase 1b)
│   ├── pattern.go    # PatternFile, Pattern types
│   ├── loader.go     # YAML pattern file loading
│   ├── regex_parser.go # RegexParser implementation
│   └── errors.go     # ValidationError, PatternError
├── parser.go         # Parser interface, ParserChain, ParseResult
├── parser_default.go # DefaultParser (wraps internal/parser)
├── watcher.go        # NewWatcherWithOptions(), WatchWithOptions(), Watcher type
├── options.go        # Functional options (WithLogDir, WithParser, WithParsers, etc.)
├── parse.go          # ParseFile(), ParseDir() - uses Parser interface
├── types.go          # Re-exports event types for convenience
└── errors.go         # Sentinel errors (ErrLogDirNotFound, ErrNoLogFiles)

internal/             # Implementation details
├── parser/           # Log line parsing with regex patterns (built-in events)
├── tailer/           # File tailing wrapper around nxadm/tail
└── logfinder/        # Log directory/file detection

cmd/vrclog/           # CLI entry point
├── main.go           # Root command, version command
├── tail.go           # tail subcommand (real-time monitoring)
├── parse.go          # parse subcommand (batch parsing)
├── completion.go     # Shell completion subcommand (bash/zsh/fish/powershell)
├── format.go         # Shared output formatting
└── eventtypes.go     # Shared event type validation (uses event.TypeNames())
```

### Key Design Patterns

**Import Cycle Avoidance**: `Event` type lives in `pkg/vrclog/event/` so `internal/parser` can import it, then `pkg/vrclog/types.go` re-exports it for convenience.

**Functional Options Pattern**: The API uses functional options (like grpc-go, zap):
```go
watcher, err := vrclog.NewWatcherWithOptions(
    vrclog.WithLogDir("/path/to/logs"),
    vrclog.WithPollInterval(5*time.Second),
    vrclog.WithReplayLastN(100),
)
```

**Two-Phase Watcher API**:
- `NewWatcherWithOptions(opts...)` - validates options, finds log directory (returns error on failure)
- `watcher.Watch(ctx)` - starts goroutines, returns event/error channels

**Parser Interface** (Phase 1a):
- `Parser` interface allows pluggable log line parsing
- `DefaultParser` wraps the built-in `internal/parser` for standard VRChat events
- `ParserChain` combines multiple parsers with modes: `ChainAll`, `ChainFirst`, `ChainContinueOnError`
- All parsers accept `context.Context` for cancellation/timeout support
- `ParseResult` contains `Events []event.Event` and `Matched bool`
- Custom parsers can be set via `WithParser()` or `WithParsers()` options

**Custom Pattern Matching** (Phase 1b):
- `pattern.RegexParser` allows users to define custom events via YAML patterns
- YAML files define patterns with `id`, `event_type`, and `regex` fields
- Named capture groups `(?P<name>...)` populate `Event.Data` map
- Pattern files support ReDoS protection (512 byte limit per pattern, 1MB file size limit)
- Correctly handles mixed unnamed `(\d+)` and named `(?P<name>\w+)` capture groups
- Example:
  ```yaml
  version: 1
  patterns:
    - id: custom_event
      event_type: my_event
      regex: 'Player (?P<name>\w+) score: (?P<score>\d+)'
  ```

**Legacy ParseLine Convention** (internal/parser only):
- `(*Event, nil)` - successfully parsed
- `(nil, nil)` - not a recognized event (skip, not an error)
- `(nil, error)` - malformed line
- **Note**: This is the old convention used by `internal/parser`. The new `Parser` interface uses `ParseResult` instead.

**Event Type Single Source of Truth**: `event.TypeNames()` in `pkg/vrclog/event/event.go` is the canonical list of event type names. CLI's `eventtypes.go` delegates to it for validation and completion.

### Event Types

Built-in event types (from `internal/parser`):
- `world_join` - User joined a world (from `[Behaviour] Entering Room:` or `Joining wrld_xxx`)
- `player_join` - Player joined instance (from `[Behaviour] OnPlayerJoined`)
- `player_left` - Player left instance (from `[Behaviour] OnPlayerLeft`)

Custom event types can be defined via `pattern.RegexParser` YAML files.

## VRChat Log Format

Log files located at: `%LOCALAPPDATA%Low\VRChat\VRChat\output_log_YYYY-MM-DD_HH-MM-SS.txt`

Timestamp format: `2006.01.02 15:04:05` (Go layout)

Example lines:
```
2024.01.15 23:59:59 Log        -  [Behaviour] OnPlayerJoined TestUser
2024.01.15 23:59:59 Log        -  [Behaviour] OnPlayerLeft TestUser
2024.01.15 23:59:59 Log        -  [Behaviour] Entering Room: World Name
```

## Linting

This project uses golangci-lint v2 with configuration in `.golangci.yml`. The config:
- Uses standard default linters (errcheck, govet, ineffassign, staticcheck, unused)
- Excludes errcheck for test files
- Excludes errcheck for common defer patterns (Close, Sync)

## Security Considerations

- **Read-only tool**: This library only reads log files, never writes
- **No external command execution**: No `os/exec` usage
- **Symlink resolution**: `FindLogDir()` uses `filepath.EvalSymlinks()` to prevent symlink attacks (works with Windows Junctions in Go 1.20+). As of recent updates, symlink resolution failures are treated as invalid directories (no fallback) to prevent security issues with broken/malicious symlinks
- **UTF-8 sanitization**: `internal/parser.Parse()` sanitizes invalid UTF-8 sequences using `strings.ToValidUTF8()` to prevent issues in JSON output
- **Error message sanitization**: Pattern loader sanitizes paths from `os.PathError` to prevent information leakage
- **ReplayLastN limit**: Default maximum of 10000 lines (`DefaultMaxReplayLastN`) to prevent memory exhaustion; configurable via `WithMaxReplayLines()`
- **Poll interval validation**: `WithPollInterval(0)` returns an error - poll intervals must be positive to prevent panics in `time.NewTicker()`
- **FIFO/Device DoS protection**: `pattern.Load()` rejects non-regular files (FIFO, device, socket) to prevent hang/OOM attacks. Uses `os.Open()` + `f.Stat()` + `io.LimitReader` to avoid TOCTOU races
- **Pattern file size limits**: 1MB max file size, 512 byte max regex pattern length (ReDoS protection)
- **Race condition protection**: `FindLatestLogFile()` caches stat results to prevent nil-deref panics when log files are deleted during sorting

## Testing Notes

- macOS uses `/var` as a symlink to `/private/var`, so tests comparing paths must use `filepath.EvalSymlinks()` for expected values
- Use `t.TempDir()` for temporary test directories (auto-cleanup)
- Use `time.Local` consistently: both `time.ParseInLocation(..., time.Local)` and `time.Date(..., time.Local)` to avoid timezone-dependent test failures
- Golden file tests in `cmd/vrclog/format_test.go`: update with `go test ./cmd/vrclog -run TestOutputEvent_Golden -update-golden`
- When testing capture groups in `pattern.RegexParser`, test both named-only patterns and mixed unnamed/named patterns

## Implementation Notes

**Pattern Package**:
- `pattern.RegexParser` uses `SubexpNames()` directly to maintain 1:1 index correspondence with `FindStringSubmatch()` results
- Correctly handles patterns with mixed unnamed `(\d+)` and named `(?P<name>\w+)` capture groups
- `Event.Data` is `nil` when no named capture groups exist (not an empty map)
- Validation happens in `Validate()` for schema checks and `NewRegexParser()` for regex compilation
- **Validation enforcement**: `NewRegexParser()` always calls `pf.Validate()` to enforce security constraints (max pattern length, required fields, etc.) even when `PatternFile` is constructed programmatically
- **Error unwrapping**: `PatternError` implements `Unwrap()` to expose underlying regex compile errors for `errors.Is()` and `errors.As()` support

**Parser Interface**:
- All `Parser` implementations must accept `context.Context` for cancellation support
- `ParseResult.Matched` can be `true` even when `Events` is empty (e.g., filter that matches but outputs nothing)
- `ParserChain` checks `ctx.Err()` between parser invocations to respect cancellation
- `nil` parsers in `ParserChain.Parsers` are skipped (not an error)
- `WithParser(nil)`, `WithParseParser(nil)`, `WithDirParser(nil)` have no effect - the default parser remains active. This behavior is documented in function comments
- **ChainContinueOnError behavior**: When a parser errors but produces events, both `ParseFile` and `Watcher.processLine` now emit the events before reporting the error. This ensures partial success from multi-parser chains is not lost

**API Limitations**:
- `WatchWithOptions()` does not return the underlying Watcher, so callers cannot call `Close()` for synchronous shutdown. Use `NewWatcherWithOptions()` + `Watcher.Watch()` for more control
- `WithParseUntil()` assumes timestamps are monotonically increasing. Out-of-order timestamps may cause events to be skipped
