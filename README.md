# vrclog-go

A Go library and CLI tool for parsing and monitoring VRChat log files.

[日本語版はこちら](README.ja.md)

## Features

- Parse VRChat log files into structured events
- Monitor log files in real-time (like `tail -f`)
- Output events as JSON Lines for easy processing with tools like `jq`
- Human-readable pretty output format
- Replay historical log data
- Cross-platform support (designed for Windows where VRChat runs)

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

### Basic Monitoring

```bash
# Monitor with auto-detected log directory
vrclog tail

# Specify log directory
vrclog tail --log-dir "C:\Users\me\AppData\LocalLow\VRChat\VRChat"

# Human-readable output
vrclog tail --format pretty
```

### Filtering Events

```bash
# Show only player join events
vrclog tail --types player_join

# Show player join and leave events
vrclog tail --types player_join,player_left
```

### Replay Historical Data

```bash
# Replay from the start of the log file
vrclog tail --replay-last 0

# Replay events since a specific time
vrclog tail --replay-since "2024-01-15T12:00:00Z"
```

### Processing with jq

```bash
# Filter specific player
vrclog tail | jq 'select(.player_name == "FriendName")'

# Count events by type
vrclog tail | jq -s 'group_by(.type) | map({type: .[0].type, count: length})'
```

## Library Usage

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

    // Start watching with default options (auto-detect log directory)
    events, errs, err := vrclog.Watch(ctx, vrclog.WatchOptions{})
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

### Parse Single Lines

```go
line := "2024.01.15 23:59:59 Log - [Behaviour] OnPlayerJoined TestUser"
event, err := vrclog.ParseLine(line)
if err != nil {
    log.Printf("parse error: %v", err)
} else if event != nil {
    fmt.Printf("Player joined: %s\n", event.PlayerName)
}
```

## Event Types

| Type | Description | Fields |
|------|-------------|--------|
| `world_join` | User joined a world | WorldName, WorldID, InstanceID |
| `player_join` | Player joined the instance | PlayerName, PlayerID |
| `player_left` | Player left the instance | PlayerName |

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

## License

MIT License

## Disclaimer

This is an unofficial tool and is not affiliated with VRChat Inc.
