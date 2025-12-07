# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

vrclog-go is a Go library and CLI for parsing and monitoring VRChat PC log files. It extracts structured events (player join/leave, world join) from VRChat's `output_log_*.txt` files on Windows.

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
├── watcher.go        # Watch(), NewWatcher(), Watcher type
├── parse.go          # ParseLine() - delegates to internal/parser
├── types.go          # Re-exports event types for convenience
└── errors.go         # Sentinel errors (ErrLogDirNotFound, ErrNoLogFiles)

internal/             # Implementation details
├── parser/           # Log line parsing with regex patterns
├── tailer/           # File tailing wrapper around nxadm/tail
└── logfinder/        # Log directory/file detection

cmd/vrclog/           # CLI entry point
├── main.go           # Root command, version command
└── tail.go           # tail subcommand implementation
```

### Key Design Patterns

**Import Cycle Avoidance**: `Event` type lives in `pkg/vrclog/event/` so `internal/parser` can import it, then `pkg/vrclog/types.go` re-exports it for convenience.

**Two-Phase Watcher API**:
- `NewWatcher(opts)` - validates options, finds log directory (returns error on failure)
- `watcher.Watch(ctx)` - starts goroutines, returns event/error channels

**ParseLine Return Convention**:
- `(*Event, nil)` - successfully parsed
- `(nil, nil)` - not a recognized event (skip, not an error)
- `(nil, error)` - malformed line

### Event Types

- `world_join` - User joined a world (from `[Behaviour] Entering Room:` or `Joining wrld_xxx`)
- `player_join` - Player joined instance (from `[Behaviour] OnPlayerJoined`)
- `player_left` - Player left instance (from `[Behaviour] OnPlayerLeft`)

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
- **Symlink resolution**: `FindLogDir()` uses `filepath.EvalSymlinks()` to prevent symlink attacks (works with Windows Junctions in Go 1.20+)
- **Error message sanitization**: User paths are not included in error messages to prevent information leakage
- **ReplayLastN limit**: Default maximum of 10000 lines (`DefaultMaxReplayLastN`) to prevent memory exhaustion; configurable via `WatchOptions.MaxReplayLines`

## Testing Notes

- macOS uses `/var` as a symlink to `/private/var`, so tests comparing paths must use `filepath.EvalSymlinks()` for expected values
- Use `t.TempDir()` for temporary test directories (auto-cleanup)
