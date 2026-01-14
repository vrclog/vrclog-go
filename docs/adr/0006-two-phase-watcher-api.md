# ADR 0006 - Two-Phase Watcher API

## Status

Accepted

## Date

2026-01-14

## Context and Problem Statement

Real-time log watching involves:
1. Configuration and validation (can fail)
2. Starting goroutines and returning channels (no error return)

Initial design (`WatchWithOptions()`) combined both phases:
```go
func WatchWithOptions(ctx, opts...) (<-chan Event, <-chan error)
// Problem: Can't return error if log directory doesn't exist!
```

This created issues:
- Invalid configuration couldn't be reported as an error
- Users had to check error channel for initialization failures
- No way to call `Close()` for synchronous shutdown
- Testing was difficult (no way to verify configuration)

## Decision Drivers

- Error handling: Initialization errors must be reported synchronously
- Resource management: Users need `Close()` for graceful shutdown
- Testability: Configuration should be verifiable before watching starts
- Backward compatibility: Keep simple `WatchWithOptions()` for basic use
- API clarity: Separate concerns (config validation vs. event streaming)

## Considered Options

1. **Single-phase with error channel** - Current problematic design
2. **Constructor returns error** - `NewWatcher() (*Watcher, error)` but then `Watch()` can't return error
3. **Two separate functions** - `NewWatcherWithOptions()` + `watcher.Watch()`
4. **Builder with Start()** - `NewWatcherBuilder().Build().Start(ctx)`

## Decision Outcome

Chose **two separate functions** for advanced usage, kept simple function for basic usage:

**Phase 1: Construction and validation**
```go
// Returns *Watcher or error
func NewWatcherWithOptions(opts ...WatchOption) (*Watcher, error)
// - Validates all options
// - Finds log directory (not log file - that happens in Watch())
// - Returns ErrLogDirNotFound if directory not found
```

**Phase 2: Start watching**
```go
// Returns event/error channels plus error for initialization failures
func (w *Watcher) Watch(ctx context.Context) (<-chan Event, <-chan error, error)
// - Starts goroutines
// - Returns error if log file cannot be found initially
// - Respects context cancellation
```

**Graceful shutdown**:
```go
func (w *Watcher) Close() error
// - Stops goroutines
// - Waits for cleanup
// - Synchronous, blocks until done
```

**Simple API (for basic use)**:
```go
// All-in-one function (cannot call Close)
func WatchWithOptions(ctx context.Context, opts ...WatchOption) (<-chan Event, <-chan error, error)
```

**Usage comparison**:
```go
// Simple: Combined initialization and start
events, errs, err := vrclog.WatchWithOptions(ctx,
    vrclog.WithReplayLastN(100),
)
if err != nil {
    return fmt.Errorf("watch failed: %w", err)
}

// Advanced: Proper error handling + graceful shutdown
watcher, err := vrclog.NewWatcherWithOptions(
    vrclog.WithReplayLastN(100),
)
if err != nil {
    return fmt.Errorf("watcher init: %w", err)
}
defer watcher.Close()  // Graceful shutdown

events, errs, err := watcher.Watch(ctx)
if err != nil {
    return fmt.Errorf("watch start failed: %w", err)
}
```

### Consequences

**Positive**:
- Clear error handling: Init errors returned as `error`, runtime errors on channel
- Graceful shutdown: `Close()` blocks until all goroutines finish
- Testable: Can verify configuration without starting goroutines
- Resource control: Users control when goroutines start/stop
- Separation of concerns: Configuration vs. execution
- Still convenient: Simple `WatchWithOptions()` for basic use

**Negative**:
- Two APIs for watching: Users must choose between simple/advanced
- `WatchWithOptions()` cannot be gracefully shut down (context cancellation only)
- More API surface: Both `NewWatcherWithOptions()` and `WatchWithOptions()` exist
- Documentation overhead: Must explain when to use each approach

## More Information

- Related: ADR 0005 (Functional Options Pattern) used in Phase 1
- Related: ADR 0001 (iter.Seq2) uses similar two-phase pattern (open, iterate)
- Implementation: `pkg/vrclog/watcher.go`
- Examples: `examples/graceful-shutdown/` demonstrates `Close()` usage
- Limitation: `WatchWithOptions()` doesn't expose `Watcher` reference
