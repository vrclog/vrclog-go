# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

vrclog-go is a Go library and CLI for parsing and monitoring VRChat PC log files. It extracts structured events (player join/leave, world join) from VRChat's `output_log_*.txt` files on Windows.

**Requirements**: Go 1.25+ (uses `iter.Seq2` iterators introduced in Go 1.23)

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
go build ./examples/...       # Build all examples

# Test
go test ./...                 # Run all tests
go test -v ./internal/parser  # Run specific package tests
go test -run TestName ./...   # Run single test by name
go test -race ./...           # With race detector
make test-cover               # Generate coverage report

# Lint (requires golangci-lint v2)
make lint                     # Run golangci-lint (note: may report unused code in examples/)
golangci-lint run ./pkg/...   # Lint only production code
make fmt-check                # Check formatting (used in CI)

# Format
go fmt ./...

# Examples
go run ./examples/<name>      # Run a specific example (see examples/README.md for list)

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
├── logfinder/        # Log directory/file detection
└── safefile/         # Security-hardened file operations (TOCTOU protection)

cmd/vrclog/           # CLI entry point
├── main.go           # Root command, version command
├── tail.go           # tail subcommand (real-time monitoring)
├── parse.go          # parse subcommand (batch parsing)
├── completion.go     # Shell completion subcommand (bash/zsh/fish/powershell)
├── format.go         # Shared output formatting
└── eventtypes.go     # Shared event type validation (uses event.TypeNames())

examples/             # 13 runnable examples (see examples/README.md)
├── custom-parser/    # YAML-based custom event parsing
├── parser-chain/     # Combining multiple parsers
├── parserfunc/       # ParserFunc adapter pattern
├── parser-interface/ # Implementing Parser interface
├── parser-chain-modes/ # ChainAll/ChainFirst/ChainContinueOnError
├── parser-decorator/ # Decorator pattern (MetricsParser, TransformingParser)
├── watch-events/     # Real-time monitoring
├── parse-files/      # Batch file parsing with iterators
├── time-filter/      # Time-based filtering
├── replay-options/   # Replay configuration modes
├── error-handling/   # Comprehensive error handling patterns
├── event-filtering/  # Event type filtering
└── graceful-shutdown/ # Watcher lifecycle and shutdown
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

**Iterator-based Parsing** (Go 1.23+ `iter.Seq2`):
- `ParseFile(ctx, path, opts...)` returns `iter.Seq2[Event, error]` for memory-efficient streaming
- `ParseDir(ctx, opts...)` yields events from multiple files in chronological order
- `ParseFileAll(ctx, path, opts...)` convenience function that collects all events into a slice
- Iterators support early termination via `break` and proper cleanup via `defer`

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
- Pattern files support ReDoS protection (512 byte limit per pattern, 1MB file size limit, 1000 pattern max count)
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

## Documentation

When adding new examples to `examples/`:
1. Add the example directory with a `main.go` containing package documentation
2. Update `examples/README.md`:
   - Add to "Running Examples" command list
   - Add a numbered section (### N. example-name) with:
     - **File**, **What it demonstrates**, **Use case**, **Key concepts**, **Output example**
3. Update `CHANGELOG.md` Unreleased section to list the new example

The project has comprehensive GoDoc coverage. All exported types, functions, and methods should have documentation comments.

**Documentation Review Guidelines**:
When updating documentation (ADRs, READMEs), verify:
- Event type constants use correct names: `EventWorldJoin`, `EventPlayerJoin`, `EventPlayerLeft` (NOT `TypeWorldJoin`)
- Function signatures match implementation exactly (including return types)
- Error types are accurate (e.g., `PatternError` struct, not `ErrPatternTooLong` sentinel)
- Code examples are compilable when copied directly from docs
- README.md and README.ja.md remain consistent
- All dates in ADR files use YYYY-MM-DD format

## Linting

This project uses golangci-lint v2 with configuration in `.golangci.yml`. The config:
- Uses standard default linters (errcheck, govet, ineffassign, staticcheck, unused)
- Excludes errcheck for test files
- Excludes errcheck for common defer patterns (Close, Sync)

**Note**: `make lint` may report unused code in `examples/` directory (e.g., unused helper functions). This is acceptable for example code. When fixing lint issues, focus on production code (`pkg/`, `internal/`, `cmd/`).

## Security Considerations

- **Read-only tool**: This library only reads log files, never writes
- **No external command execution**: No `os/exec` usage
- **Symlink resolution**: `FindLogDir()` uses `filepath.EvalSymlinks()` to prevent symlink attacks (works with Windows Junctions in Go 1.20+). As of recent updates, symlink resolution failures are treated as invalid directories (no fallback) to prevent security issues with broken/malicious symlinks
- **TOCTOU protection**: `internal/safefile.OpenRegular()` mitigates time-of-check-time-of-use race conditions by using `os.Lstat()` followed by `os.Open()` and `f.Stat()` to verify the file wasn't replaced with a symlink/FIFO/device between checks. Used by `FindLatestLogFile()`, `listLogFiles()`, and `readLastNLines()`
- **UTF-8 sanitization**: `internal/parser.Parse()` sanitizes invalid UTF-8 sequences using `strings.ToValidUTF8()` to prevent issues in JSON output
- **Error message sanitization**: Pattern loader sanitizes paths from `os.PathError` to prevent information leakage
- **ReplayLastN limit**: Default maximum of 10000 lines (`DefaultMaxReplayLastN`) to prevent memory exhaustion; configurable via `WithMaxReplayLines()`
- **Poll interval validation**: `WithPollInterval(0)` returns an error - poll intervals must be positive to prevent panics in `time.NewTicker()`
- **Negative value validation**: `watchConfig.validate()` rejects negative values for `maxReplayBytes` and `maxReplayLineBytes` (0 means unlimited)
- **FIFO/Device DoS protection**: `pattern.Load()` and `safefile.OpenRegular()` reject non-regular files (FIFO, device, socket, symlink) to prevent hang/OOM attacks
- **Pattern file size limits**: 1MB max file size, 512 byte max regex pattern length, 1000 pattern max count (ReDoS and CPU exhaustion protection)
- **Directory accessibility checks**: `FindLatestLogFile()` and `listLogFiles()` verify directory accessibility before calling `filepath.Glob()` to detect permission errors that Glob might hide
- **Context cancellation**: `ParseFile()` and `ParseDir()` properly detect and propagate context cancellation without wrapping it in `ParseError`. `readLastNLines()` checks context between chunk reads for long-running replays

## Testing Notes

- macOS uses `/var` as a symlink to `/private/var`, so tests comparing paths must use `filepath.EvalSymlinks()` for expected values
- Use `t.TempDir()` for temporary test directories (auto-cleanup)
- Use `time.Local` consistently: both `time.ParseInLocation(..., time.Local)` and `time.Date(..., time.Local)` to avoid timezone-dependent test failures
- Golden file tests in `cmd/vrclog/format_test.go`: update with `go test ./cmd/vrclog -run TestOutputEvent_Golden -update-golden`
- When testing capture groups in `pattern.RegexParser`, test both named-only patterns and mixed unnamed/named patterns
- **Error comparison**: Use `errors.Is()` instead of `==` when comparing sentinel errors for future-proofing against error wrapping
- **Log rotation tests**: See `watcher_rotation_test.go` for examples of testing file rotation scenarios with `WithPollInterval()` set to short durations (100ms) for faster test execution

## Implementation Notes

**Pattern Package**:
- `pattern.RegexParser` caches `SubexpNames()` indices during construction in `compiledPattern.groupIndex` to avoid repeated allocations on every match
- Correctly handles patterns with mixed unnamed `(\d+)` and named `(?P<name>\w+)` capture groups
- `Event.Data` is `nil` when no named capture groups exist (not an empty map)
- Validation happens in `Validate()` for schema checks and `NewRegexParser()` for regex compilation
- **Validation enforcement**: `NewRegexParser()` always calls `pf.Validate()` to enforce security constraints (max pattern length, max pattern count, required fields, etc.) even when `PatternFile` is constructed programmatically
- **Pattern count limit**: `MaxPatternCount = 1000` prevents CPU exhaustion attacks via files with thousands of patterns
- **Error unwrapping**: `PatternError` implements `Unwrap()` to expose underlying regex compile errors for `errors.Is()` and `errors.As()` support

**Parser Interface**:
- All `Parser` implementations must accept `context.Context` for cancellation support
- `ParseResult.Matched` can be `true` even when `Events` is empty (e.g., filter that matches but outputs nothing)
- `ParserChain` checks `ctx.Err()` between parser invocations to respect cancellation
- `nil` parsers in `ParserChain.Parsers` are skipped (not an error)
- `WithParser(nil)`, `WithParseParser(nil)`, `WithDirParser(nil)` have no effect - the default parser remains active. This behavior is documented in function comments
- **ChainContinueOnError behavior**: When a parser errors but produces events, both `ParseFile` and `Watcher.processLine` now emit the events before reporting the error. This ensures partial success from multi-parser chains is not lost

**ParseFile Long Line Handling**:
- Uses `bufio.Reader` instead of `bufio.Scanner` to properly handle lines exceeding 512KB
- Long lines are read and discarded (not parsed) to allow parsing to continue on subsequent lines
- With `WithParseStopOnError(true)`: returns `LineTooLongError` and stops parsing
- Default behavior: skips long lines silently and continues parsing
- This prevents the old `bufio.Scanner` behavior where a single long line would stop all parsing

**API Limitations**:
- `WatchWithOptions()` does not return the underlying Watcher, so callers cannot call `Close()` for synchronous shutdown. Use `NewWatcherWithOptions()` + `Watcher.Watch()` for more control
- `WithParseUntil()` assumes timestamps are monotonically increasing. Out-of-order timestamps may cause events to be skipped

**ReplayLastN Memory Protection**:
- `readLastNLines()` uses backward chunk scanning (4KB chunks) with carry buffer to prevent partial line corruption
- Memory limits enforced via `WithMaxReplayBytes(max int)` (default: 10MB) and `WithMaxReplayLineBytes(max int)` (default: 512KB)
- Returns `ErrReplayLimitExceeded` if limits are exceeded during replay
- `maxReplayLineBytes` only checks lines that will actually be returned (not old lines outside the requested N)
- `maxReplayBytes` correctly counts only newly read bytes (not the carry buffer which was already counted)
- Handles partial reads from `ReadAt` when `io.EOF` is returned with valid data
- Accepts `context.Context` and checks cancellation between chunk reads
- O(bytes read) complexity instead of naive O(n²) approach

**Watcher Log Directory Handling**:
- `FindLogDir()` only validates directory existence, not log file presence
- `FindLatestLogFile()` returns `ErrNoLogFiles` if directory exists but has no log files
- `WithWaitForLogs(bool)` option allows waiting for log files to appear (useful for starting watcher before VRChat launches)
- When `waitForLogs=true`: polls at `pollInterval` until logs appear or context cancels
- When `waitForLogs=false` (default): returns `ErrNoLogFiles` immediately for backward compatibility

**Watcher Log Rotation**:
- Watcher detects log rotation by checking for newer log files at `pollInterval` intervals
- When rotation is detected, the watcher creates a new tailer for the newer file BEFORE stopping the old tailer
- This ensures continuity: if the new tailer fails to open, the watcher continues monitoring the old file
- Log rotation is tested in `watcher_rotation_test.go` with scenarios including rotation failures and multiple rotations

**Watcher Error Channel**:
- Error channel has a buffer size of 16 to prevent blocking the watcher goroutine
- If errors are produced faster than consumed, additional errors are silently dropped (documented in API)
- This is a deliberate design trade-off to prevent deadlock
- Consumers should process errors promptly to avoid drops
