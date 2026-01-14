# ADR 0003 - ParserChain Three Modes

## Status

Accepted

## Date

2026-01-14

## Context and Problem Statement

When combining multiple parsers (e.g., DefaultParser for built-in events + RegexParser for custom events), users need different behaviors depending on their use case:

1. Some want ALL parsers to run and aggregate results
2. Some want the FIRST successful parser to win (short-circuit)
3. Some want to continue even if a parser fails (best-effort)

A single "combine parsers" function cannot satisfy all these needs without additional configuration.

## Decision Drivers

- Flexibility: Support diverse combining strategies
- Performance: Allow short-circuiting when appropriate
- Error handling: Enable best-effort parsing for fault tolerance
- API clarity: Make the mode explicit and self-documenting
- Common patterns: Cover typical multi-parser scenarios

## Considered Options

1. **Single mode with flags** - `ParserChain(parsers, continueOnError, stopOnFirst)`
2. **Separate constructors** - `NewChainAll()`, `NewChainFirst()`, `NewChainContinueOnError()`
3. **Enum mode parameter** - `NewParserChain(mode ChainMode, parsers)`
4. **Builder pattern** - `NewChainBuilder().All().WithParsers(...).Build()`

## Decision Outcome

Chose **explicit mode constants** with struct initialization:

```go
// ChainMode specifies how ParserChain executes parsers
type ChainMode int

const (
    ChainAll ChainMode = iota  // Run all parsers, aggregate all events
    ChainFirst                  // Stop at first successful parser
    ChainContinueOnError        // Run all parsers, ignore individual errors
)

// ParserChain combines multiple parsers
type ParserChain struct {
    Mode    ChainMode
    Parsers []Parser
}
```

**Behavior**:

| Mode | Stops on error? | Stops on match? | Returns all events? |
|------|----------------|-----------------|---------------------|
| ChainAll | Yes | No | Yes (aggregated) |
| ChainFirst | Yes | Yes (first match) | No (first only) |
| ChainContinueOnError | No | No | Yes (best-effort) |

**Examples**:

```go
// Use case 1: Built-in events + custom events
parser := &vrclog.ParserChain{
    Mode: vrclog.ChainAll,
    Parsers: []vrclog.Parser{
        vrclog.DefaultParser{},
        customParser,
    },
}

// Use case 2: Try parsers in priority order
parser := &vrclog.ParserChain{
    Mode: vrclog.ChainFirst,
    Parsers: []vrclog.Parser{
        highPriorityParser,
        fallbackParser,
    },
}

// Use case 3: Best-effort parsing (never fail)
parser := &vrclog.ParserChain{
    Mode: vrclog.ChainContinueOnError,
    Parsers: []vrclog.Parser{
        experimentalParser,
        vrclog.DefaultParser{},
    },
}
```

### Consequences

**Positive**:
- Self-documenting: mode constants make intent clear
- Type-safe: ChainMode enum prevents invalid values
- Composable: ParserChain itself implements Parser, allowing nesting
- Idiomatic Go: follows struct initialization pattern
- Flexible: users can create parser chains directly without helper functions
- Context cancellation: all modes check `ctx.Err()` between parsers

**Negative**:
- Slightly more verbose than constructor functions (must specify Mode and Parsers)
- Cannot dynamically change mode at runtime (must construct new chain)
- Mode field can be set to invalid values (mitigated by defaulting to ChainAll)

## More Information

- Related: ADR 0005 (Functional Options Pattern) for configuration
- Implementation: `pkg/vrclog/parser.go`
- Examples: `examples/parser-chain/`, `examples/parser-chain-modes/`
- Special behavior: ChainContinueOnError emits partial events before error (as of recent update)
