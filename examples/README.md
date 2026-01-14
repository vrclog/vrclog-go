# Examples

This directory contains runnable examples demonstrating various features of vrclog-go.

## Prerequisites

- Go 1.25 or later
- For `watch-events`: VRChat installed and running on Windows (or access to VRChat log files)

## Running Examples

```bash
# Run from repository root
go run ./examples/custom-parser
go run ./examples/parser-chain
go run ./examples/parserfunc
go run ./examples/watch-events
go run ./examples/parse-files
go run ./examples/time-filter
go run ./examples/replay-options
go run ./examples/parser-interface
go run ./examples/error-handling
go run ./examples/event-filtering
go run ./examples/parser-chain-modes
go run ./examples/parser-decorator
go run ./examples/graceful-shutdown
```

---

## Examples Overview

### 1. custom-parser

**File**: `custom-parser/main.go`

**What it demonstrates**:
- Defining custom event patterns using YAML
- Using `RegexParser` to extract game-specific events
- Named capture groups to populate `Event.Data`

**Use case**: Extracting custom events from VRChat world logs (e.g., poker game events, quest progress, player stats).

**Key concepts**:
- `pattern.LoadBytes()` - Load patterns from YAML bytes
- `pattern.NewRegexParser()` - Create regex-based parser
- Named capture groups `(?P<name>...)` populate `Event.Data`

**Output example**:
```
Event: poker_hole_cards
  Timestamp: 12:00:00
  Data:
    card1: AceSpades
    card2: KingHearts
```

---

### 2. parser-chain

**File**: `parser-chain/main.go`

**What it demonstrates**:
- Combining `DefaultParser` with `RegexParser` using `ParserChain`
- Handling both built-in VRChat events and custom game events
- Three chain modes: `ChainAll`, `ChainFirst`, `ChainContinueOnError`

**Use case**: Processing logs that contain both standard VRChat events (player join/leave, world join) and custom world-specific events.

**Key concepts**:
- `vrclog.DefaultParser{}` - Handles built-in VRChat events
- `vrclog.ParserChain` - Combines multiple parsers
- `ChainAll` mode - Tries all parsers (each line can produce multiple events)

**Output example**:
```
[1] ✓ player_join - Player: Alice
[2] ✓ poker_hand - Cards: AceSpades, KingDiamonds
[3] ✓ player_left - Player: Bob
[4] ✓ poker_winner - Seat 2 won 150 chips
[5] ✓ world_join - World: Poker VIP
```

---

### 3. parserfunc

**File**: `parserfunc/main.go`

**What it demonstrates**:
- Creating custom parsers with `ParserFunc` (no YAML required)
- Constructing `event.Event` manually in Go code
- Using `Data` map for custom fields
- Proper handling of `Matched` return value
- Context cancellation support in custom parsers

**Use case**: When you need custom parsing logic in Go code rather than regex patterns. Useful for complex parsing that requires validation, lookups, or multi-step processing.

**Key concepts**:
- `vrclog.ParserFunc` - Adapter to convert functions to Parser interface
- Custom event types (any string, not limited to built-in constants)
- `event.Event` construction with required `Type` and `Timestamp` fields
- `Matched: false` for unrecognized lines (not an error)
- `ctx.Err()` checking for cancellation support

**Output example**:
```
[1] ✓ game_score
    Time: 12:00:00
    Data:
      player: Alice
      score: 50

[2] ✓ game_score
    Time: 12:00:15
    Data:
      player: Bob
      score: 75

[3] ✓ game_win
    Time: 12:00:30
    Data:
      player: Alice
```

---

### 4. watch-events

**File**: `watch-events/main.go`

**What it demonstrates**:
- Real-time log monitoring with `WatchWithOptions()`
- Event type filtering with `WithIncludeTypes()`
- Replaying recent events with `WithReplayLastN()`
- Graceful shutdown with context cancellation

**Use case**: Building real-time notifications, presence tracking, or live dashboards for VRChat activity.

**Requirements**:
- VRChat must be running and generating logs
- Windows OS (or manually specify log directory with `WithLogDir()`)

**Key concepts**:
- `vrclog.WatchWithOptions()` - Start watching with functional options
- `WithIncludeTypes()` - Filter specific event types
- `WithReplayLastN(N)` - Replay last N events from current log
- `WithPollInterval()` - Configure polling frequency
- Context cancellation for graceful shutdown

**Output example**:
```
[12:00:05] ✓ Alice joined
[12:00:23] ✗ Bob left
[12:01:15] ✓ Charlie joined
```

---

### 5. parse-files

**File**: `parse-files/main.go`

**What it demonstrates**:
- Batch processing with `ParseFile()` (iterator-based)
- `ParseFileAll()` for collecting all events into a slice
- `ParseDir()` for processing multiple files chronologically
- Early termination with `break`

**Use case**: Analyzing historical log files, batch processing existing logs without watching for new events.

**Key concepts**:
- `iter.Seq2[Event, error]` - Go 1.25+ iterators for memory-efficient streaming
- `ParseFile()` - Single file, supports early termination
- `ParseFileAll()` - Convenience function that collects all events
- `ParseDir()` - Multiple files in chronological order
- Uses `ParseDirOption` (not `ParseOption`) with `WithDirPaths()`

**Output example**:
```
[1] player_join - 10:00:00 | Player: Alice
[2] player_join - 10:05:00 | Player: Bob
[3] player_left - 10:15:00 | Player: Alice
(stopped after 3 events to demonstrate break)

Total events from both files: 4
```

---

### 6. time-filter

**File**: `time-filter/main.go`

**What it demonstrates**:
- Time-based filtering with `WithParseSince(t)`
- `WithParseUntil(t)` for events before a time
- `WithParseTimeRange(since, until)` for range filtering
- Combining filters with other parse options

**Use case**: Analyzing specific time periods, extracting events from a particular session or time window.

**Key concepts**:
- `WithParseSince(t)` - Events >= t (inclusive)
- `WithParseUntil(t)` - Events < t (exclusive)
  - Assumes monotonically increasing timestamps
  - Out-of-order timestamps may be skipped
- `WithParseTimeRange(s, u)` - Combines since and until
- Time filters use `ParseOption` (not `WatchOption`)

**Output example**:
```
Filter: 12:00 <= Events < 16:00

12:30:00 - Bob joined
15:00:00 - Charlie joined

Matched: 2 events
```

---

### 7. replay-options

**File**: `replay-options/main.go`

**What it demonstrates**:
- Replay configuration options for `Watcher`
- Five replay modes: `ReplayLastN`, `ReplayFromStart`, `ReplaySinceTime`, `ReplayNone`
- Memory protection with `WithMaxReplayBytes()` and `WithMaxReplayLineBytes()`
- Self-contained demo with mock log files

**Use case**: Configuring how much historical data to replay when starting a watcher, controlling memory usage during replay.

**Key concepts**:
- `WithReplayLastN(N)` - Replay last N non-empty lines
- `WithReplayFromStart()` - Replay from beginning of file
- `WithReplaySinceTime(t)` - Replay events >= timestamp t
- `ReplayNone` - No replay (default, tail -f behavior)
- Memory limits:
  - `WithMaxReplayBytes(max)` - Total bytes limit (default: 10MB)
  - `WithMaxReplayLineBytes(max)` - Per-line limit (default: 512KB)
- Replay options use `WatchOption` (not `ParseOption`)

**Output example**:
```
Example 1: WithReplayLastN(5)
✓ Watcher created
  Replayed 5 events: Event11, Event12, Event13, Event14, Event15

Example 2: WithReplayFromStart()
✓ Watcher created
  Replayed 15 events: Event01, Event02, ... (total: 15)

Example 5: Memory Limits
✓ Watcher created with memory limits:
  - WithReplayLastN(100)
  - WithMaxReplayBytes(1MB)
  - WithMaxReplayLineBytes(64KB)
```

---

### 8. parser-interface

**File**: `parser-interface/main.go`

**What it demonstrates**:
- Implementing `Parser` interface directly with a struct
- State management (match counting)
- Custom methods beyond the interface
- Compile-time interface verification with `var _ vrclog.Parser = (*Type)(nil)`
- Thread-safe parser with `sync.Mutex`

**Use case**: Complex parsers that need to maintain state, track statistics, or provide custom methods. Useful when `ParserFunc` is too limited.

**Key concepts**:
- `Parser` interface - `ParseLine(ctx, line) (ParseResult, error)`
- State management - struct fields for counters, caches, etc.
- Custom methods - `MatchCount()`, `Reset()`, `TotalLines()`
- Compile-time checks - `var _ vrclog.Parser = (*ScoreParser)(nil)`
- Thread safety - `sync.Mutex` for concurrent access
- Parser vs ParserFunc:
  - Parser (struct): Can maintain state, custom methods, thread-safe
  - ParserFunc (function): Simpler, stateless, inline definitions

**Output example**:
```
Example 1: Simple Parser with State Management
[1] game_score
    Time: 12:00:00
    Data:
      player: Alice
      score: 50

Parser matched 4 events

Example 2: Combining with DefaultParser
[1] game_score | Score: 50 | Player: Alice
[2] game_score | Score: 75 | Player: Bob
[3] game_win | Player: Alice
[4] player_join | Player: Charlie
[5] game_score | Score: 100 | Player: Charlie

Total events: 5
Custom parser matched: 4 events

Example 3: Thread-Safe Parser with Mutex
Initial stats: total=0, matched=0
Final stats: total=5, matched=3
```

---

### 9. error-handling

**File**: `error-handling/main.go`

**What it demonstrates**:
- Comprehensive error handling for all vrclog error types
- Using `errors.Is()` for sentinel errors (`ErrLogDirNotFound`, `ErrNoLogFiles`, `ErrReplayLimitExceeded`)
- Using `errors.As()` for typed errors (`ParseError`, `LineTooLongError`, `WatchError`)
- Error classification with type switches
- Operation-specific handling using `WatchError.Op`

**Use case**: Understanding and properly handling all error scenarios in production applications. Building robust error logging and recovery mechanisms.

**Key concepts**:
- Sentinel errors - `errors.Is(err, vrclog.ErrLogDirNotFound)`
- Typed errors - `errors.As(err, &parseErr)` to extract error details
- `ParseError` - Contains the problematic line and underlying error
- `LineTooLongError` - Line number, length, and max length information
- `WatchError.Op` - Identifies which operation failed (`WatchOpFindLatest`, `WatchOpReplay`, `WatchOpParse`, `WatchOpTail`, `WatchOpRotation`)
- `WithParseStopOnError(true)` - Controls error propagation vs skipping

**Output example**:
```
→ Testing ErrLogDirNotFound:
  ✓ Detected: Log directory not found

→ ParseError detected
  Line: "2024.01.15 10:01:00 Log        -  [Test] TRIGGER_ERROR bad line"
  Error: simulated parse error

→ LineTooLongError detected
  Line number: 1
  Length: 66594 bytes (max: 524288 bytes)

→ WatchError detected
  Operation: tail
  Path: /path/to/log
  Error: I/O failure
```

---

### 10. event-filtering

**File**: `event-filtering/main.go`

**What it demonstrates**:
- Event type filtering with `WithExcludeTypes()` and `WithFilter()`
- Filter precedence rules (exclude takes priority over include)
- Dynamic event type discovery with `event.TypeNames()`
- User input validation with `event.ParseType()`
- Building CLI-style filters from command-line arguments

**Use case**: Filtering events by type for focused analysis, building CLI tools with user-configurable filters.

**Key concepts**:
- `WithParseExcludeTypes(types...)` - Block specific event types
- `WithParseFilter(include, exclude)` - Combine include and exclude lists
- Filter precedence: **Exclude > Include** (if an event is in both lists, it's excluded)
- `event.TypeNames()` - Returns all valid event type strings for validation
- `event.ParseType(str)` - Validates and converts string to `event.Type`

**Output example**:
```
→ Exclude player_left events:
→ Collected 5 events:
  - player_join: 3
  - world_join: 2

→ Include: player_join + player_left
→ Exclude: player_left
→ Result: Only player_join (exclude wins)
→ Collected 3 events:
  - player_join: 3

→ All valid event types:
  1. player_join
  2. player_left
  3. world_join
```

---

### 11. parser-chain-modes

**File**: `parser-chain-modes/main.go`

**What it demonstrates**:
- `ParserChain` modes for different parsing strategies
- `ChainAll` - Execute all parsers and combine results
- `ChainFirst` - Stop at first match (priority/fallback pattern)
- `ChainContinueOnError` - Resilient parsing despite errors
- Practical use cases for each mode

**Use case**: Combining multiple parsers with different behaviors. Handling both built-in VRChat events and custom game events. Building fault-tolerant parsers.

**Key concepts**:
- `vrclog.ChainAll` mode - All parsers execute, results combined (one line can produce multiple events)
- `vrclog.ChainFirst` mode - Stop at first match (parser priority, fallback behavior)
- `vrclog.ChainContinueOnError` mode - Continue parsing even if one parser fails
- `WithParsers()` shorthand - Automatically uses `ChainAll` mode
- Strategy pattern - behavior changes based on mode selection

**Output example**:
```
→ ChainAll Mode:
  [Line 1] player_join - Alice
  [Line 2] game_win - Data: map[player:Alice]
  (Both DefaultParser and GameParser produced events)

→ ChainFirst Mode:
  [Line 2] game_win - Data: map[player:Alice]
  (GameParser had priority, DefaultParser not called)

→ ChainContinueOnError Mode:
  [Line 4] Errors occurred, but got events:
    Error 1: simulated parse error
    But still got event: player_join
  (ErrorParser failed, but DefaultParser succeeded)
```

---

### 12. parser-decorator

**File**: `parser-decorator/main.go`

**What it demonstrates**:
- Decorator pattern for extending `Parser` functionality
- `MetricsParser` - Wraps a parser to collect statistics
- `TransformingParser` - Wraps a parser to transform event data
- Composing multiple decorators for layered functionality

**Use case**: Adding metrics collection, logging, caching, data transformation, or validation to existing parsers without modifying them. Building reusable parser middleware.

**Key concepts**:
- Decorator pattern - Wraps existing parsers while implementing `Parser` interface
- `MetricsParser` - Tracks total lines, match count, error count
- `TransformingParser` - Applies a transformation function to all events
- Decorator composition - Stack multiple decorators (e.g., `Metrics(Transforming(Default))`)
- Compile-time interface check - `var _ vrclog.Parser = (*MetricsParser)(nil)`

**Output example**:
```
→ MetricsParser:
  Collected 7 events

→ Statistics:
  Total lines:   8
  Matched lines: 7
  Errors:        0
  Match rate:    87.5%

→ TransformingParser (normalize names):
  player_join - Alice  (was: alice)
  player_join - Bob    (was: BOB)

→ Composed decorators:
  Parser stack:
    1. DefaultParser    (parse VRChat events)
    2. Transforming     (normalize names)
    3. Metrics          (collect statistics)
```

---

### 13. graceful-shutdown

**File**: `graceful-shutdown/main.go`

**What it demonstrates**:
- Watcher lifecycle management and graceful shutdown
- Two-phase API: `NewWatcherWithOptions()` + `Watcher.Watch()`
- `Watcher.Close()` for synchronous shutdown
- `WithLogger()` for slog integration
- `sync.WaitGroup` for coordinating multiple goroutines
- Comparing context cancellation vs `Watcher.Close()`

**Use case**: Building long-running monitoring tools with clean shutdown behavior. Debugging watcher internals with structured logging. Coordinating multiple event processing goroutines.

**Key concepts**:
- Two-phase API:
  - `NewWatcherWithOptions(opts...)` - Create and validate (returns error on failure)
  - `watcher.Watch(ctx)` - Start watching (returns channels, non-blocking)
- `Watcher.Close()` - Synchronous shutdown (blocks until all resources released)
- `WithLogger(slog.Logger)` - Inject structured logger for debugging
- `WithWaitForLogs(true)` - Poll until log files appear (useful before VRChat starts)
- `sync.WaitGroup` - Track completion of processing goroutines
- Context cancellation vs Close() - Asynchronous signal vs synchronous cleanup

**Output example**:
```
→ Event processor goroutine started
→ Error handler goroutine started

→ Waiting for replay events to process...
  [Event 1] player_join - Alice
  [Event 2] player_join - Bob
  [Event 3] world_join - Test World

→ Calling Close()...
  Close() returned after 15ms
  → All goroutines have exited
  → Channels are closed

→ Event channel closed, processor exiting
→ Error channel closed, handler exiting
→ All goroutines finished
```

---

## Example Patterns

### Custom YAML Pattern File

Create a file `patterns.yaml`:

```yaml
version: 1
patterns:
  # Poker game events
  - id: poker_hole_cards
    event_type: poker_hole_cards
    regex: '\[Seat\]: Draw Local Hole Cards: (?P<card1>\w+), (?P<card2>\w+)'

  - id: poker_winner
    event_type: poker_winner
    regex: 'player (?P<seat_id>\d+) won (?P<amount>\d+)'

  # Quest events
  - id: quest_complete
    event_type: quest_complete
    regex: 'Quest "(?P<quest_name>[^"]+)" completed by (?P<player>\w+)'

  # Custom game score
  - id: player_score
    event_type: player_score
    regex: 'Player (?P<name>\w+) scored (?P<points>\d+) points'
```

Load in your code:

```go
parser, err := pattern.NewRegexParserFromFile("patterns.yaml")
if err != nil {
    log.Fatal(err)
}
```

---

## Tips

### Security Limits

Pattern files have security constraints to prevent DoS attacks:
- **File size**: Max 1MB (`pattern.MaxPatternFileSize`)
- **Pattern length**: Max 512 bytes per regex (`pattern.MaxPatternLength`)
- **File type**: Must be regular file (not FIFO, device, socket)

### Performance

- Use `ChainFirst` mode if order matters and you want to stop at first match
- Use `WithIncludeTypes()` for filtering instead of post-processing
- Increase `WithPollInterval()` if CPU usage is a concern

### Error Handling

```go
// Check for specific error types
var watchErr *vrclog.WatchError
if errors.As(err, &watchErr) {
    log.Printf("Watch error: op=%s, path=%s", watchErr.Op, watchErr.Path)
}

var parseErr *vrclog.ParseError
if errors.As(err, &parseErr) {
    log.Printf("Parse error: line=%q", parseErr.Line)
}

// Check for sentinel errors
if errors.Is(err, vrclog.ErrLogDirNotFound) {
    log.Println("VRChat log directory not found")
}
```

---

## More Information

- [Main README](../README.md) - Full library documentation
- [API Reference](https://pkg.go.dev/github.com/vrclog/vrclog-go) - pkg.go.dev documentation
- [CHANGELOG](../CHANGELOG.md) - Version history and features
