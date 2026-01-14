# ADR 0007 - ReDoS Protection Limits

## Status

Accepted

## Date

2026-01-14

## Context and Problem Statement

`pattern.RegexParser` allows users to load custom regex patterns from YAML files. While Go's `regexp` package uses the RE2 engine (which guarantees linear time complexity and does not suffer from catastrophic backtracking), there are still security concerns:

1. **Slow regex operations**: Even with linear complexity, long or complex patterns can be slow on large inputs
2. **Memory exhaustion**: NFA construction for very long patterns can consume significant memory
3. **File-based attacks**: YAML files could be extremely large (GB size), contain FIFO/device files, or have hundreds of patterns
4. **Defense in depth**: Even though RE2 prevents catastrophic backtracking, limiting pattern complexity is good practice

Additionally, malicious YAML files could:
- Be extremely large (GB size) causing OOM
- Contain FIFO/device files instead of regular files
- Have hundreds of patterns causing slow loading

The library must protect against these attacks while allowing legitimate custom patterns.

## Decision Drivers

- Security: Prevent ReDoS and DoS attacks
- Usability: Don't overly restrict legitimate patterns
- Performance: Pattern loading should be fast
- Defense in depth: Multiple layers of protection
- Clear errors: Users should understand why patterns are rejected

## Considered Options

1. **No limits** - Simple but vulnerable
2. **Timeout-based protection** - Complex, platform-dependent
3. **Size limits only** - Partial protection
4. **Size + pattern length + file type checks** - Comprehensive

## Decision Outcome

Chose **size + pattern length + file type checks** with multiple protection layers:

**Protection Layer 1: File Security**
```go
// Reject non-regular files (FIFO, device, socket, symlink)
func Load(path string) (*PatternFile, error)
    // Opens file and verifies it's a regular file (no symlinks/FIFOs)
    // Uses os.Open() followed by f.Stat() and Mode().IsRegular() check
```

**Protection Layer 2: File Size Limit**
```go
const MaxPatternFileSize = 1 * 1024 * 1024  // 1MB

func LoadBytes(data []byte) (*PatternFile, error) {
    if len(data) > MaxPatternFileSize {
        return nil, fmt.Errorf("pattern file too large: %d bytes (max %d)", len(data), MaxPatternFileSize)
    }
}
```

**Protection Layer 3: Pattern Length Limit**
```go
const MaxPatternLength = 512  // bytes

func (pf *PatternFile) Validate() error {
    for _, p := range pf.Patterns {
        if len(p.Regex) > MaxPatternLength {
            return &PatternError{Pattern: p, Message: "pattern too long: ..."}
        }
    }
}
```

**Protection Layer 4: Validation Enforcement**
```go
func NewRegexParser(pf *PatternFile) (*RegexParser, error) {
    // ALWAYS validates, even if PatternFile constructed programmatically
    if err := pf.Validate(); err != nil {
        return nil, err
    }
    // Regex compilation happens here (Go's regex engine is safe but can be slow)
}
```

**Rationale for limits**:
- **1MB file**: Enough for hundreds of patterns, prevents memory exhaustion
- **512 byte regex**: Sufficient for complex patterns (e.g., `(?P<name>\w+) (?P<value>\d+\.\d+)`), limits NFA construction time and memory
- **Regular files only**: Prevents FIFO hang/device read issues

**Why 512 bytes is sufficient**:
- Most practical regex patterns are 50-200 bytes
- 512 bytes allows for complex patterns with multiple capture groups
- Named capture groups like `(?P<player_name>[A-Za-z0-9_]+)` are ~30-40 bytes each
- Limits worst-case NFA construction overhead

### Consequences

**Positive**:
- Performance protection: 512 byte limit constrains NFA construction time
- OOM prevention: 1MB file size prevents memory exhaustion
- DoS prevention: FIFO/device rejection prevents indefinite blocking
- Fast validation: Limits checked before regex compilation
- Fuzz tested: `FuzzRegexParser_ParseLine` and `FuzzLoadBytes` ensure panic-free operation
- Clear errors: Users get specific error messages via `PatternError` (pattern length) and formatted errors (file size)
- RE2 safety: Go's regex engine already prevents catastrophic backtracking

**Negative**:
- Legitimate complex patterns might exceed 512 bytes (rare)
- Large pattern files with hundreds of patterns might exceed 1MB (very rare)
- Users cannot use symlinks for pattern files (intentional security choice)

## More Information

- [OWASP: Regular Expression Denial of Service](https://owasp.org/www-community/attacks/Regular_expression_Denial_of_Service_-_ReDoS) (Note: Go's RE2 engine is immune to classical ReDoS)
- [RE2 Syntax](https://github.com/google/re2/wiki/Syntax) - Go uses RE2 which guarantees O(n) time complexity
- Related: ADR 0002 (TOCTOU Protection) for file security
- Implementation: `pkg/vrclog/pattern/pattern.go`, `pkg/vrclog/pattern/loader.go`
- Fuzz tests: `pkg/vrclog/pattern/fuzz_test.go`
- Security documentation: CLAUDE.md "Security Considerations"
- Go's regex engine: RE2-based, linear time, no catastrophic backtracking
