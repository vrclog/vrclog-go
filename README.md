# vrclog-go

[![Go Reference](https://pkg.go.dev/badge/github.com/vrclog/vrclog-go.svg)](https://pkg.go.dev/github.com/vrclog/vrclog-go)
[![CI](https://github.com/vrclog/vrclog-go/actions/workflows/ci.yml/badge.svg)](https://github.com/vrclog/vrclog-go/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/vrclog/vrclog-go)](https://goreportcard.com/report/github.com/vrclog/vrclog-go)
[![codecov](https://codecov.io/gh/vrclog/vrclog-go/branch/main/graph/badge.svg)](https://codecov.io/gh/vrclog/vrclog-go)

A Go library and CLI tool for parsing and monitoring VRChat log files.

[日本語版はこちら](README.ja.md)

## API Stability

> **Note**: This library is pre-1.0 (`v0.x.x`). The API may change between minor versions without deprecation warnings. Pin to a specific version if you need stability.

## Features

- Parse VRChat log files into structured events
- Monitor log files in real-time (like `tail -f`)
- Output events as JSON Lines for easy processing with tools like `jq`
- Human-readable pretty output format
- Replay historical log data
- Designed for Windows where VRChat runs

## Requirements

- Go 1.25+ (this project's minimum version; `iter.Seq2` was introduced in Go 1.23)
- Windows (for actual VRChat log monitoring)

## Installation

```bash
go install github.com/vrclog/vrclog-go/cmd/vrclog@latest
```

Or build from source:

```bash
git clone https://github.com/vrclog/vrclog-go.git
cd vrclog-go
go build -o vrclog ./cmd/vrclog/
```

## Quick Start

Get started in 30 seconds:

```bash
# 1. Install
go install github.com/vrclog/vrclog-go/cmd/vrclog@latest

# 2. Monitor VRChat events (requires VRChat to be running)
vrclog tail
```

For library usage and advanced features, see [Library Usage](#library-usage) below.

## CLI Usage

### Commands

```bash
vrclog tail       # Monitor VRChat logs (real-time)
vrclog parse      # Parse VRChat logs (batch/offline)
vrclog completion # Generate shell completion scripts
vrclog version    # Print version information
vrclog --help     # Show help
```

### Shell Completion

Enable tab completion for your shell:

```bash
# Bash
vrclog completion bash > /etc/bash_completion.d/vrclog

# Zsh
vrclog completion zsh > "${fpath[1]}/_vrclog"

# Fish
vrclog completion fish > ~/.config/fish/completions/vrclog.fish

# PowerShell
vrclog completion powershell | Out-String | Invoke-Expression
```

### Streaming vs Batch

| Feature | `tail` | `parse` |
|---------|--------|---------|
| Mode | Real-time monitoring | Batch processing |
| File handling | Latest file + rotation | All matching files |
| Use case | Live monitoring | Historical analysis |
| Event delivery | Channel-based | Iterator-based |

### Global Flags

| Flag | Description |
|------|-------------|
| `--verbose`, `-v` | Enable verbose logging |

### Common Options

These options work with both `tail` and `parse`:

| Flag | Short | Description |
|------|-------|-------------|
| `--log-dir` | `-d` | VRChat log directory (auto-detect if not set) |
| `--format` | `-f` | Output format: `jsonl` (default), `pretty` |
| `--include-types` | | Event types to include (comma-separated) |
| `--exclude-types` | | Event types to exclude (comma-separated) |
| `--raw` | | Include raw log lines in output |

### tail Command

Monitor logs in real-time:

```bash
# Monitor with auto-detected log directory
vrclog tail

# Specify log directory
vrclog tail --log-dir "C:\Users\me\AppData\LocalLow\VRChat\VRChat"

# Human-readable output
vrclog tail --format pretty

# Show only player events
vrclog tail --include-types player_join,player_left

# Exclude world join events
vrclog tail --exclude-types world_join

# Replay from start of log file
vrclog tail --replay-last 0

# Replay last 100 lines
vrclog tail --replay-last 100

# Replay events since a specific time
vrclog tail --replay-since "2024-01-15T12:00:00Z"
```

#### tail-specific Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--replay-last` | -1 (disabled) | Replay last N non-empty lines (0 = from start) |
| `--replay-since` | | Replay since timestamp (RFC3339) |

Note: `--replay-last` and `--replay-since` cannot be used together.

### parse Command

Parse historical logs (batch mode):

```bash
# Parse all logs in auto-detected directory
vrclog parse

# Specify log directory
vrclog parse --log-dir "C:\Users\me\AppData\LocalLow\VRChat\VRChat"

# Filter by time range (multi-day queries)
vrclog parse --since "2024-01-15T00:00:00Z" --until "2024-01-16T00:00:00Z"

# Filter by event type
vrclog parse --include-types world_join --format pretty

# Parse specific files
vrclog parse output_log_2024-01-15.txt output_log_2024-01-16.txt
```

#### parse-specific Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--since` | | Only events at/after timestamp (RFC3339) |
| `--until` | | Only events before timestamp (RFC3339) |
| `--stop-on-error` | false | Stop on first error instead of skipping |
| `[files...]` | | Specific file paths to parse |

### Processing with jq

Both `tail` and `parse` output JSON Lines format:

```bash
# Filter specific player
vrclog tail | jq 'select(.player_name == "FriendName")'

# Count events by type
vrclog parse | jq -s 'group_by(.type) | map({type: .[0].type, count: length})'

# Extract player names from join events
vrclog tail | jq 'select(.type == "player_join") | .player_name'
```

## Library Usage

### Quick Start (Real-time Watching)

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/vrclog/vrclog-go/pkg/vrclog"
)

func main() {
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // Start watching with functional options (recommended)
    events, errs, err := vrclog.WatchWithOptions(ctx,
        vrclog.WithIncludeTypes(vrclog.EventPlayerJoin, vrclog.EventPlayerLeft),
        vrclog.WithReplayLastN(100),
    )
    if err != nil {
        log.Fatal(err)
    }

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
        }
    }
}
```

### Watch Options (Functional Options Pattern)

| Option | Description |
|--------|-------------|
| `WithLogDir(dir)` | Set VRChat log directory (auto-detect if not set) |
| `WithPollInterval(d)` | Log rotation check interval (default: 2s) |
| `WithIncludeRawLine(bool)` | Include raw log line in events |
| `WithIncludeTypes(types...)` | Filter to only these event types |
| `WithExcludeTypes(types...)` | Filter out these event types |
| `WithReplayFromStart()` | Read from file start |
| `WithReplayLastN(n)` | Read last N non-empty lines before tailing |
| `WithReplaySinceTime(since)` | Read events since timestamp |
| `WithMaxReplayLines(n)` | Limit for ReplayLastN (default: 10000) |
| `WithParser(p)` | Use custom parser (replaces default) |
| `WithParsers(parsers...)` | Combine multiple parsers (ChainAll mode) |
| `WithLogger(logger)` | Set slog.Logger for debug output |
| `WithWaitForLogs(wait)` | Wait for log files to appear if directory exists but is empty |
| `WithMaxReplayBytes(max)` | Max bytes to read during replay (default: 10MB) |
| `WithMaxReplayLineBytes(max)` | Max bytes per line during replay (default: 512KB) |

### Advanced Usage with Watcher

For more control over the watcher lifecycle:

```go
// Create watcher with functional options
watcher, err := vrclog.NewWatcherWithOptions(
    vrclog.WithLogDir("/custom/path"),
    vrclog.WithIncludeTypes(vrclog.EventPlayerJoin),
    vrclog.WithReplayLastN(100),
)
if err != nil {
    log.Fatal(err)
}
defer watcher.Close()

// Start watching
events, errs, err := watcher.Watch(ctx)
// ... process events
```

### Offline Parsing (iter.Seq2)

Parse log files without starting a watcher. Uses Go 1.23+ iterators for memory-efficient streaming:

```go
// Parse a single file
for ev, err := range vrclog.ParseFile(ctx, "output_log.txt",
    vrclog.WithParseIncludeTypes(vrclog.EventPlayerJoin),
) {
    if err != nil {
        log.Printf("error: %v", err)
        break
    }
    fmt.Printf("Player joined: %s\n", ev.PlayerName)
}

// Collect all events into a slice
events, err := vrclog.ParseFileAll(ctx, "output_log.txt")

// Parse all log files in a directory (chronological order)
for ev, err := range vrclog.ParseDir(ctx,
    vrclog.WithDirLogDir("/path/to/logs"),
    vrclog.WithDirIncludeTypes(vrclog.EventWorldJoin),
) {
    if err != nil {
        break
    }
    fmt.Printf("World: %s\n", ev.WorldName)
}
```

### Parse Options

| Option | Description |
|--------|-------------|
| `WithParseIncludeTypes(types...)` | Filter to only these event types |
| `WithParseExcludeTypes(types...)` | Filter out these event types |
| `WithParseTimeRange(since, until)` | Filter by time range |
| `WithParseSince(t)` | Filter events at or after time |
| `WithParseUntil(t)` | Filter events before time |
| `WithParseIncludeRawLine(bool)` | Include raw log line |
| `WithParseStopOnError(bool)` | Stop on first error (default: skip) |
| `WithParseParser(p)` | Use custom parser (replaces default) |

### ParseDir Options

| Option | Description |
|--------|-------------|
| `WithDirLogDir(dir)` | Log directory (auto-detect if not set) |
| `WithDirPaths(paths...)` | Explicit file paths to parse |
| `WithDirIncludeTypes(types...)` | Filter to only these event types |
| `WithDirExcludeTypes(types...)` | Filter out these event types |
| `WithDirTimeRange(since, until)` | Filter by time range |
| `WithDirIncludeRawLine(bool)` | Include raw log line |
| `WithDirStopOnError(bool)` | Stop on first error |
| `WithDirParser(p)` | Use custom parser (replaces default) |

### Parse Single Lines

```go
line := "2024.01.15 23:59:59 Log - [Behaviour] OnPlayerJoined TestUser"
event, err := vrclog.ParseLine(line)
if err != nil {
    log.Printf("parse error: %v", err)
} else if event != nil {
    fmt.Printf("Player joined: %s\n", event.PlayerName)
}
// event == nil && err == nil means line is not a recognized event
```

## Custom Parsers

### Parser Interface

vrclog-go supports custom parsers for handling non-standard log formats or extracting additional data.

#### Parser Interface Definition

```go
type Parser interface {
    ParseLine(ctx context.Context, line string) (ParseResult, error)
}

type ParseResult struct {
    Events  []event.Event
    Matched bool
}
```

#### Using Custom Parsers

```go
// With Watch
events, errs, err := vrclog.WatchWithOptions(ctx,
    vrclog.WithParser(myCustomParser),
)

// With ParseFile
for ev, err := range vrclog.ParseFile(ctx, "log.txt",
    vrclog.WithParseParser(myCustomParser),
) {
    // ...
}

// With ParseDir
for ev, err := range vrclog.ParseDir(ctx,
    vrclog.WithDirParser(myCustomParser),
) {
    // ...
}
```

#### ParserChain

Combine multiple parsers for complex parsing scenarios:

```go
chain := &vrclog.ParserChain{
    Mode: vrclog.ChainAll, // ChainFirst, ChainContinueOnError
    Parsers: []vrclog.Parser{
        vrclog.DefaultParser{},  // Built-in events
        customParser,            // Custom events
    },
}

events, errs, err := vrclog.WatchWithOptions(ctx,
    vrclog.WithParser(chain),
)
```

| Mode | Behavior |
|------|----------|
| `ChainAll` | Execute all parsers, combine results |
| `ChainFirst` | Stop at first matching parser |
| `ChainContinueOnError` | Skip erroring parsers, continue |

#### ParserFunc Adapter

Convert functions to parsers:

```go
myParser := vrclog.ParserFunc(func(ctx context.Context, line string) (vrclog.ParseResult, error) {
    // Parse logic here
    return vrclog.ParseResult{Events: events, Matched: true}, nil
})
```

### YAML Pattern Files (RegexParser)

Define custom events without writing Go code using YAML pattern files.

#### Pattern File Format

```yaml
version: 1
patterns:
  - id: poker_hole_cards
    event_type: poker_hole_cards
    regex: '\[Seat\]: Draw Local Hole Cards: (?P<card1>\w+), (?P<card2>\w+)'
  - id: poker_winner
    event_type: poker_winner
    regex: '\[PotManager\]: .* player (?P<seat_id>\d+) won (?P<amount>\d+)'
```

| Field | Required | Description |
|-------|----------|-------------|
| `version` | Yes | Schema version (currently `1`) |
| `id` | Yes | Unique pattern identifier |
| `event_type` | Yes | Value for `Event.Type` field |
| `regex` | Yes | Regular expression (max 512 bytes) |

Named capture groups `(?P<name>...)` are extracted into `Event.Data`.

#### Using RegexParser

```go
import "github.com/vrclog/vrclog-go/pkg/vrclog/pattern"

// Load from file
parser, err := pattern.NewRegexParserFromFile("patterns.yaml")
if err != nil {
    log.Fatal(err)
}

// Use with Watch
events, errs, err := vrclog.WatchWithOptions(ctx,
    vrclog.WithParser(parser),
)

// Or combine with default parser
chain := &vrclog.ParserChain{
    Mode: vrclog.ChainAll,
    Parsers: []vrclog.Parser{
        vrclog.DefaultParser{},
        parser,
    },
}
```

#### Output Example

Input log line:
```
2024.01.15 23:59:59 Debug - [Seat]: Draw Local Hole Cards: Jc, 6d
```

Output event:
```json
{
  "type": "poker_hole_cards",
  "timestamp": "2024-01-15T23:59:59+09:00",
  "data": {
    "card1": "Jc",
    "card2": "6d"
  }
}
```

#### Security Limits

| Limit | Value | Purpose |
|-------|-------|---------|
| Max file size | 1 MB | Prevent OOM attacks |
| Max pattern length | 512 bytes | ReDoS mitigation |
| File type | Regular files only | Prevent FIFO/device DoS |

## Event Types

| Type | Description | Fields |
|------|-------------|--------|
| `world_join` | User joined a world | WorldName, WorldID, InstanceID |
| `player_join` | Player joined the instance | PlayerName, PlayerID |
| `player_left` | Player left the instance | PlayerName |

### Event JSON Schema

All events have these common fields:

| JSON Field | Go Field | Type | Description |
|------------|----------|------|-------------|
| `type` | `Type` | `string` | Event type (`world_join`, `player_join`, `player_left`, or custom) |
| `timestamp` | `Timestamp` | `string` | RFC3339 timestamp |
| `player_name` | `PlayerName` | `string` | Player display name (player events) |
| `player_id` | `PlayerID` | `string` | Player ID like `usr_xxx` (player_join only) |
| `world_name` | `WorldName` | `string` | World name (world_join only) |
| `world_id` | `WorldID` | `string` | World ID like `wrld_xxx` (world_join only) |
| `instance_id` | `InstanceID` | `string` | Full instance ID (world_join only) |
| `data` | `Data` | `map[string]string` | Custom key-value data (custom parsers only) |
| `raw_line` | `RawLine` | `string` | Original log line (if IncludeRawLine enabled) |

## Runtime Behavior

### Channel Lifecycle

- Both `events` and `errs` channels close when:
  - Context is cancelled (`ctx.Done()`)
  - A fatal error occurs (e.g., log directory deleted)
  - `watcher.Close()` is called
- Always check the `ok` value when receiving from channels

### Log Rotation

- The watcher polls for new log files at `PollInterval` (default: 2 seconds)
- When VRChat creates a new log file, the watcher automatically switches to it
- New log files are read from the beginning
- The watcher does not return to old log files

### Error Handling

Errors are sent to the error channel and can be inspected with `errors.Is()`:

```go
import "errors"

case err := <-errs:
    if errors.Is(err, vrclog.ErrLogDirNotFound) {
        // Log directory was deleted
    }
    var parseErr *vrclog.ParseError
    if errors.As(err, &parseErr) {
        // Malformed log line
        fmt.Printf("bad line: %s\n", parseErr.Line)
    }
```

| Error | Description |
|-------|-------------|
| `ErrLogDirNotFound` | Log directory not found |
| `ErrNoLogFiles` | No log files in directory |
| `ErrWatcherClosed` | Watch called after Close |
| `ErrAlreadyWatching` | Watch called twice |
| `ErrReplayLimitExceeded` | Replay exceeded memory limits (maxReplayBytes or maxReplayLineBytes) |
| `ParseError` | Malformed log line (wraps original error) |
| `LineTooLongError` | Log line exceeds maximum length (512KB) |
| `WatchError` | Watch operation error (includes operation type) |

## Output Format

### JSON Lines (default)

```json
{"type":"player_join","timestamp":"2024-01-15T23:59:59+09:00","player_name":"TestUser"}
{"type":"player_left","timestamp":"2024-01-16T00:00:05+09:00","player_name":"TestUser"}
```

### Pretty

```
[23:59:59] + TestUser joined
[00:00:05] - TestUser left
[00:01:00] > Joined world: Test World
```

## Environment Variables

| Variable | Description |
|----------|-------------|
| `VRCLOG_LOGDIR` | Override default log directory |

## Project Structure

```
vrclog-go/
├── cmd/vrclog/        # CLI application
├── pkg/vrclog/        # Public API
│   ├── event/         # Event type definitions
│   └── pattern/       # Custom pattern matching (YAML)
└── internal/          # Internal packages
    ├── parser/        # Log line parser
    ├── tailer/        # File tailing
    └── logfinder/     # Log directory detection
```

## Testing

```bash
# Run all tests
go test ./...

# Run with verbose output
go test -v ./...

# Run with race detector
go test -race ./...

# Run with coverage
go test -cover ./...
```

## FAQ

### Q: Can I use this on non-Windows platforms?
A: No. VRChat PC client only runs on Windows, so this library is Windows-specific. However, you can copy log files to other platforms for offline parsing.

### Q: Where are the log files located?
A: `%LOCALAPPDATA%Low\VRChat\VRChat\output_log_*.txt` on Windows.

### Q: Real-time monitoring doesn't work. What should I check?
A: Ensure VRChat is running. Use the `WithWaitForLogs(true)` option to wait for log files to be created.

### Q: What is `iter.Seq2`?
A: It's an iterator type introduced in Go 1.23, enabling memory-efficient streaming. See [Go's official documentation](https://go.dev/doc/go1.23) for details.

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Ensure code is formatted (`go fmt ./...`)
4. Run tests (`go test ./...`)
5. Commit your changes
6. Push to the branch
7. Open a Pull Request

## License

MIT License

## Disclaimer

This is an unofficial tool and is not affiliated with VRChat Inc.
