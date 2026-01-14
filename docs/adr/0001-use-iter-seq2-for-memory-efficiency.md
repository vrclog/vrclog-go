# ADR 0001 - Use iter.Seq2 for Memory-Efficient Parsing

## Status

Accepted

## Date

2026-01-14

## Context and Problem Statement

VRChat log files can be very large (hundreds of MB), and parsing them into memory all at once would cause memory exhaustion. Users need to process log files efficiently without loading the entire file into memory, especially when searching for specific events or processing files on resource-constrained systems.

Traditional approaches like reading all lines into a slice (`[]Event`) would consume memory proportional to file size, making the library impractical for large log files.

## Decision Drivers

- Memory efficiency: Must handle multi-GB log files without exhaustion
- Go 1.23+ feature availability: `iter.Seq2` provides native iterator support
- Early termination: Users should be able to stop iteration without processing entire file
- Idiomatic Go: Should feel natural to Go developers familiar with range loops
- Backward compatibility alternative: Need convenience function for users who want slice output

## Considered Options

1. **Return `[]Event` slices** - Simple but memory-inefficient
2. **Callback function pattern** - Memory-efficient but less idiomatic
3. **Channel-based streaming** - Goroutine overhead and complexity
4. **iter.Seq2 iterators** - Go 1.23+ native iterator support

## Decision Outcome

Chose **iter.Seq2 iterators** for the primary API (`ParseFile`, `ParseDir`), with a convenience function `ParseFileAll` for slice-based usage.

**Implementation**:
```go
// Memory-efficient streaming
func ParseFile(ctx context.Context, path string, opts ...ParseOption) iter.Seq2[Event, error]

// Convenience function for small files
func ParseFileAll(ctx context.Context, path string, opts ...ParseOption) ([]Event, error)
```

**Usage**:
```go
// Stream events with minimal memory
for evt, err := range vrclog.ParseFile(ctx, path) {
    if err != nil {
        return err
    }
    // Process event
    if evt.Type == vrclog.EventWorldJoin {
        break // Early termination supported
    }
}
```

### Consequences

**Positive**:
- O(1) memory usage regardless of file size
- Early termination via `break` statement
- Proper cleanup via `defer` in iterator implementation
- Native Go 1.23+ range loop support
- Events can be processed as they're parsed (streaming)
- `ParseDir` can merge multiple files without loading all into memory

**Negative**:
- Requires Go 1.23+ (iter.Seq2 not available in earlier versions)
- Learning curve for developers unfamiliar with iterators
- Cannot easily get total count without iterating through all events
- Error handling is per-event rather than single return value

## More Information

- [Go 1.23 Release Notes - Iterators](https://go.dev/doc/go1.23)
- Related: ADR 0006 (Two-Phase Watcher API) also uses streaming pattern
- Implementation: `pkg/vrclog/parse.go`
- Examples: `examples/parse-files/`
