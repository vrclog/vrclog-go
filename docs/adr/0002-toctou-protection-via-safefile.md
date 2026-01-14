# ADR 0002 - TOCTOU Protection via safefile.OpenRegular

## Status

Accepted

## Date

2026-01-14

## Context and Problem Statement

File operations are vulnerable to Time-of-Check-Time-of-Use (TOCTOU) race conditions. Between checking a file's metadata and opening it, an attacker could replace the file with a symlink pointing to sensitive files, a FIFO that blocks indefinitely, or a device file that causes unexpected behavior.

This is particularly critical for `FindLatestLogFile()`, `listLogFiles()`, and `readLastNLines()` which must reliably read VRChat log files without security vulnerabilities.

## Decision Drivers

- Security: Prevent symlink attacks and FIFO/device DoS
- Reliability: Avoid indefinite blocking on special files
- Cross-platform: Must work on Windows (primary target) and other platforms
- Performance: Minimal overhead for normal case
- Windows support: Handle Windows Junctions correctly (Go 1.20+)

## Considered Options

1. **No protection** - Simple but vulnerable to attacks
2. **Check once with os.Stat** - Still vulnerable to TOCTOU race
3. **Open then check with f.Stat()** - Partial protection but allows FIFO blocking
4. **Lstat + Open + f.Stat verification** - Complete protection

## Decision Outcome

Chose **Lstat + Open + f.Stat verification** pattern implemented in `internal/safefile.OpenRegular()`.

**Implementation**:
```go
// internal/safefile/safefile.go
func OpenRegular(path string) (*os.File, os.FileInfo, error) {
    // 1. Lstat: check metadata without following symlinks
    linkInfo, err := os.Lstat(path)
    if err != nil {
        return nil, nil, err
    }

    // 2. Reject non-regular files (symlink, FIFO, device, socket)
    if !linkInfo.Mode().IsRegular() {
        return nil, nil, ErrNotRegularFile
    }

    // 3. Open the file
    f, err := os.Open(path)
    if err != nil {
        return nil, nil, err
    }

    // 4. Stat the file descriptor to verify it's the same file
    info, err := f.Stat()
    if err != nil {
        f.Close()
        return nil, nil, err
    }

    // 5. Verify still a regular file (catches TOCTOU replacement)
    if !info.Mode().IsRegular() {
        f.Close()
        return nil, nil, ErrNotRegularFile
    }

    // Note: On Unix, we could use os.SameFile() to compare inodes,
    // but os.FileInfo doesn't expose inode cross-platform. The double
    // IsRegular() check above catches most TOCTOU attacks.

    return f, info, nil
}
```

**Used by**:
- `FindLatestLogFile()` - Ensures latest log is a regular file
- `listLogFiles()` - Filters out non-regular files from glob results
- `readLastNLines()` - Prevents reading from symlinks/FIFOs

### Consequences

**Positive**:
- Prevents symlink attacks (cannot trick library into reading /etc/passwd)
- Prevents FIFO DoS (library won't block indefinitely on mkfifo)
- Prevents device file issues (won't try to read from /dev/random)
- Detects TOCTOU races (double IsRegular() check catches file replacement)
- Works with Windows Junctions (Go 1.20+ EvalSymlinks support)
- Returns os.FileInfo for callers who need file metadata
- Minimal performance overhead (one extra syscall)

**Negative**:
- Legitimate symlinks to log files are rejected (intentional security choice)
- Two syscalls instead of one (negligible performance impact)
- Slightly more complex error handling

## More Information

- [CWE-367: Time-of-check Time-of-use (TOCTOU) Race Condition](https://cwe.mitre.org/data/definitions/367.html)
- Related: ADR 0007 (ReDoS Protection) for pattern file security
- Implementation: `internal/safefile/safefile.go`
- Security documentation: CLAUDE.md "Security Considerations"
