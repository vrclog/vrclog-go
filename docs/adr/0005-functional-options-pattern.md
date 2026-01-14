# ADR 0005 - Functional Options Pattern

## Status

Accepted

## Date

2026-01-14

## Context and Problem Statement

The library has many optional configurations:
- Log directory path
- Poll interval for file watching
- Replay behavior (last N lines, from start, etc.)
- Event filtering
- Custom parsers
- Memory limits
- Time range filtering

Traditional approaches have drawbacks:
- Many constructor parameters: `NewWatcher(logDir, pollInterval, replayMode, replayCount, ...)`
- Config struct: Requires nil-checking, defaults unclear, hard to evolve
- Builder pattern: Verbose, non-idiomatic Go

## Decision Drivers

- API ergonomics: Should be easy and natural to use
- Backward compatibility: Easy to add new options without breaking changes
- Self-documenting: Option names should be clear
- Default values: Sensible defaults, explicit overrides
- Idiomatic Go: Follow patterns used by grpc-go, zap, etc.
- Type safety: Compiler catches invalid options

## Considered Options

1. **Many constructor parameters** - `NewWatcher(logDir, poll, replay, count, ...)`
2. **Config struct** - `NewWatcher(&Config{LogDir: "..."})`
3. **Builder pattern** - `NewWatcherBuilder().LogDir("...").Build()`
4. **Functional options** - `NewWatcher(WithLogDir("..."), WithPollInterval(...))`
5. **Variadic config** - `NewWatcher(configs ...WatcherConfig)`

## Decision Outcome

Chose **functional options pattern** popularized by Dave Cheney and used extensively in grpc-go and uber-go/zap.

**Implementation**:
```go
// Option function type
type WatchOption func(*watchConfig)

// Constructor with variadic options
func NewWatcherWithOptions(opts ...WatchOption) (*Watcher, error)

// Option constructors
func WithLogDir(dir string) WatchOption
func WithPollInterval(d time.Duration) WatchOption
func WithReplayLastN(n int) WatchOption
func WithParser(p Parser) WatchOption
func WithExcludeTypes(types ...EventType) WatchOption
```

**Usage**:
```go
// Minimal - uses all defaults
watcher, err := vrclog.NewWatcherWithOptions()

// Custom configuration
watcher, err := vrclog.NewWatcherWithOptions(
    vrclog.WithLogDir("/path/to/logs"),
    vrclog.WithPollInterval(5*time.Second),
    vrclog.WithReplayLastN(100),
    vrclog.WithExcludeTypes(vrclog.EventPlayerLeft),
)

// Easy to add new options without breaking existing code
watcher, err := vrclog.NewWatcherWithOptions(
    vrclog.WithLogDir("/custom/path"),
    vrclog.WithWaitForLogs(true),  // New option, doesn't break old code
)
```

**Error Handling**:
Options configure internal state; validation happens in constructor:
```go
watcher, err := vrclog.NewWatcherWithOptions(
    vrclog.WithPollInterval(0),  // Invalid
)
// err: "poll interval must be positive, got 0s"
// Error detected by watchConfig.validate() called in NewWatcherWithOptions
```

### Consequences

**Positive**:
- Easy to add new options: Just add new `WithXxx()` function
- Self-documenting: `WithReplayLastN(100)` is clearer than `100` as parameter
- Sensible defaults: Options are opt-in, defaults always work
- Backward compatible: New options don't break existing code
- Composable: Can create option slices and merge them
- Type-safe: Wrong option type caught at compile time
- Validation: Each option can validate its input
- IDE-friendly: Autocomplete shows available `WithXxx()` options

**Negative**:
- More verbose than config struct for many options
- Option order doesn't matter (could be confusing)
- Internal config struct still needed (not exposed to users)
- More functions to document (each `WithXxx()` needs godoc)

## More Information

- [Functional Options in Go - Dave Cheney](https://dave.cheney.net/2014/10/17/functional-options-for-friendly-apis)
- Used by: grpc-go (DialOption), uber-go/zap (Option), golang.org/x/net/context
- Implementation: `pkg/vrclog/options.go`
- Related: ADR 0006 (Two-Phase Watcher API) uses options in first phase
