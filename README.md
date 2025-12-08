# vrclog-go

[![Go Reference](https://pkg.go.dev/badge/github.com/vrclog/vrclog-go.svg)](https://pkg.go.dev/github.com/vrclog/vrclog-go)

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

- Go 1.23+ (required for `iter.Seq2` iterator support)
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

## CLI Usage

### Commands

```bash
vrclog tail      # Monitor VRChat logs
vrclog version   # Print version information
vrclog --help    # Show help
```

### Global Flags

| Flag | Description |
|------|-------------|
| `--verbose`, `-v` | Enable verbose logging |

### Basic Monitoring

```bash
# Monitor with auto-detected log directory
vrclog tail

# Specify log directory
vrclog tail --log-dir "C:\Users\me\AppData\LocalLow\VRChat\VRChat"

# Human-readable output
vrclog tail --format pretty

# Include raw log lines in output
vrclog tail --raw
```

### Filtering Events

```bash
# Show only player join events
vrclog tail --types player_join

# Show only world join events
vrclog tail --types world_join

# Show player join and leave events
vrclog tail --types player_join,player_left

# Short form
vrclog tail -t player_join,player_left
```

### Replay Historical Data

```bash
# Replay from the start of the log file
vrclog tail --replay-last 0

# Replay last 100 lines
vrclog tail --replay-last 100

# Replay events since a specific time
vrclog tail --replay-since "2024-01-15T12:00:00Z"
```

Note: `--replay-last` and `--replay-since` cannot be used together.

### tail Command Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--log-dir` | `-d` | auto-detect | VRChat log directory |
| `--format` | `-f` | `jsonl` | Output format: `jsonl`, `pretty` |
| `--types` | `-t` | all | Event types to show (comma-separated) |
| `--raw` | | false | Include raw log lines in output |
| `--replay-last` | | -1 (disabled) | Replay last N lines (0 = from start) |
| `--replay-since` | | | Replay since timestamp (RFC3339) |

### Processing with jq

```bash
# Filter specific player
vrclog tail | jq 'select(.player_name == "FriendName")'

# Count events by type
vrclog tail | jq -s 'group_by(.type) | map({type: .[0].type, count: length})'

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
| `WithReplayLastN(n)` | Read last N lines before tailing |
| `WithReplaySinceTime(t)` | Read events since timestamp |
| `WithMaxReplayLines(n)` | Limit for ReplayLastN (default: 10000) |
| `WithLogger(logger)` | Set slog.Logger for debug output |

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
| `type` | `Type` | `string` | Event type (`world_join`, `player_join`, `player_left`) |
| `timestamp` | `Timestamp` | `string` | RFC3339 timestamp |
| `player_name` | `PlayerName` | `string` | Player display name (player events) |
| `player_id` | `PlayerID` | `string` | Player ID like `usr_xxx` (player_join only) |
| `world_name` | `WorldName` | `string` | World name (world_join only) |
| `world_id` | `WorldID` | `string` | World ID like `wrld_xxx` (world_join only) |
| `instance_id` | `InstanceID` | `string` | Full instance ID (world_join only) |
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
| `ParseError` | Malformed log line (wraps original error) |
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
│   └── event/         # Event type definitions
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
