# ADR 0004 - Event Type in Separate Package

## Status

Accepted

## Date

2026-01-14

## Context and Problem Statement

Go's import system prohibits circular dependencies. The architecture needs:
- `internal/parser` to parse log lines and return `Event` structs
- `pkg/vrclog` to provide public API wrapping `internal/parser`
- Both packages need to use the same `Event` type definition

Without careful design, this creates a circular import:
```
pkg/vrclog imports internal/parser
internal/parser imports pkg/vrclog  // for Event type
```

## Decision Drivers

- Avoid import cycles: Required by Go compiler
- Single source of truth: Event definition must be canonical
- API ergonomics: Users shouldn't need multiple imports
- Package cohesion: Event and event.Type should live together
- CLI usage: CLI needs event type names for validation

## Considered Options

1. **Event in pkg/vrclog** - Causes import cycle (internal can't import pkg)
2. **Event in internal/parser** - Users must import internal package (bad practice)
3. **Event in separate internal/event** - Hidden from users, requires re-export
4. **Event in separate pkg/vrclog/event** - Public, importable by all
5. **Event as interface** - Overly complex, no clear benefit

## Decision Outcome

Chose **Event in separate pkg/vrclog/event package** with convenience re-export in pkg/vrclog.

**Package Structure**:
```
pkg/vrclog/event/
  └── event.go        # Event struct, Type enum, TypeNames()

pkg/vrclog/
  ├── types.go        # Re-exports: type Event = event.Event
  └── ...

internal/parser/
  └── parser.go       # Imports pkg/vrclog/event
```

**Usage**:
```go
// Users can import just pkg/vrclog
import "github.com/owner/vrclog-go/pkg/vrclog"

evt := vrclog.Event{...}        // Uses re-exported type
typ := vrclog.EventWorldJoin    // Uses re-exported const

// Or explicit import when needed
import "github.com/owner/vrclog-go/pkg/vrclog/event"

names := event.TypeNames()     // CLI uses this
```

**Single Source of Truth**:
- `event.TypeNames()` in `pkg/vrclog/event/event.go` is canonical
- CLI's `eventtypes.go` delegates to it for validation and completion
- No duplicate type name lists

### Consequences

**Positive**:
- No import cycles: All packages can import `pkg/vrclog/event`
- Clean public API: Users need only `pkg/vrclog` import
- Type safety: Single Event definition, cannot mix incompatible types
- CLI integration: `event.TypeNames()` provides single source of truth
- Internal usage: `internal/parser` can create `event.Event` directly

**Negative**:
- Extra package: Adds one more package to navigate
- Re-export indirection: Users may be confused by `types.go` re-exports
- Import path: Some users may accidentally import `pkg/vrclog/event` directly (not harmful, just unnecessary)

## More Information

- [Go FAQ: Why can't I import my package?](https://go.dev/doc/faq#circular_imports)
- Related patterns in stdlib: `encoding/json` doesn't import user types, users import json
- Implementation: `pkg/vrclog/event/event.go`, `pkg/vrclog/types.go`
- CLI integration: `cmd/vrclog/eventtypes.go` uses `event.TypeNames()`
