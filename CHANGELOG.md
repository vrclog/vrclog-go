# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `AdapterEvent`, the sole size- and shape-constrained extension envelope for
  structured external-Adapter data that cannot map to the other seven
  canonical Event types.

### Changed (Breaking) — Data integrity hardening

Follow-up hardening pass fixing several data-integrity gaps in the
architecture above. No backward compatibility shims were added.

- **`ObservationID` formula changed**: no longer includes the emission
  index (`SHA-256(record_id + NUL + adapter_id + NUL + rule_id)`). IDs
  computed under the old formula will not match. An adapter returning
  the same non-empty rule ID more than once for a single record now has
  **all** of those emissions rejected (new `duplicate_rule_id`
  diagnostic) instead of getting distinct IDs.
- **`Follow` no longer emits unterminated log lines**: a partially
  written line (no trailing newline yet) is held back and only emitted
  once completed. `ReadFile` is unaffected and still flushes an
  unterminated final line.
- **Filesystem errors during `Follow` are now propagated** instead of
  being silently treated as "no new data" or "no newer files". New
  sentinel error `ErrSourceTruncated` is returned when an actively
  followed file's size drops below the already-committed offset.
- **Cursor resume error classification narrowed**: `ErrCursorSourceMissing`
  is now returned only when the cursor's file does not exist or its
  `SourceID` no longer matches; other failures (permission denied,
  non-regular file, I/O errors) are wrapped and returned as-is.
- **`MediaTarget.Backend` is now a required field** (JSON `omitempty`
  removed): validation rejects an empty or undefined `Backend`, and the
  built-in VRChat adapter now sets `Backend: MediaBackendUnknown` on
  every video-resolve rule that previously left it unset.
- **Event validation is stricter and now recurses into nested values**:
  `RemoteResource.URL` must be a well-formed absolute `http`/`https` URL
  under 16 KiB with no userinfo, control, whitespace, or Unicode bidi
  formatting characters; `MediaTarget` and any nested `RemoteResource`/
  `MediaTarget` on `ResourceURLObserved`, `ResourceResolved`, and
  `MediaErrorObserved` are now validated as part of `Event.validate()`.
- **Built-in VRChat adapter prefix matching fully anchored**: recognition
  now requires the known prefix (`[Behaviour]`, `[Video Playback]`,
  `[AVProVideo]`) at the very start of the message and all regexes are
  anchored, so a log line that merely contains one of these tags
  mid-message (e.g. echoed by a third-party mod) is no longer
  misinterpreted as a genuine VRChat client event.
- **Built-in adapter error text sanitized** before being placed into a
  canonical `MediaErrorObserved.Message`: trimmed, embedded `http(s)`
  URLs redacted to `<url>` (URLs may carry signed access tokens),
  control characters and Unicode bidi formatting characters normalized
  to spaces, truncated to 4096 UTF-8-safe bytes.

### Added — Data integrity hardening

- `CaptureLogSnapshot(directory) (LogSnapshot, error)` and
  `LogSnapshot.Contains(Record) bool`: lets a downstream consumer (e.g.
  a companion process) distinguish log bytes that existed before a
  point in time from bytes appended afterward, without relying on
  timestamps. Uses path-and-size semantics — see README for the
  documented same-path-replacement limitation.
- `ErrSourceTruncated` sentinel error.
- `DiagnosticDuplicateRuleID` diagnostic code.
- `internal/logfile.ListLogFilesStrict`: like `ListLogFiles`, but
  propagates open/stat failures on matched candidate files instead of
  silently skipping them (used internally by `Follow` and
  `CaptureLogSnapshot`).

### Changed (Breaking)

Complete architecture replacement. No backward compatibility with the 0.1.x API.

- **New pipeline**: `output_log_*.txt -> Read/Follow -> Record -> Engine (Adapters) -> Event -> Observation`
- **Flat package layout**: single `package vrclog` at module root (was `pkg/vrclog/`)
- **Zero external dependencies**: standard library only (removed nxadm/tail, cobra, testify, yaml.v3, wazero)
- **Sealed canonical Events**: 7 concrete types (`PlayerJoined`, `PlayerLeft`, `WorldEnteringObserved`, `WorldJoiningObserved`, `ResourceURLObserved`, `ResourceResolved`, `MediaErrorObserved`) with sealed interface
- **Deterministic IDs**: `SourceID`, `RecordID`, `ObservationID` all derived from SHA-256 hashes
- **Cursor-based positioning**: file offset and line number, not timestamps
- **iter.Seq2 iterators**: `ReadFile` and `Follow` return `iter.Seq2[Record, error]`
- **New CLI**: `vrclog read|follow|version` (plain `flag` package, no Cobra)

### Removed

- `Parser`, `ParserFunc`, `ParserChain`, `DefaultParser`
- YAML pattern matching (`pattern/` package)
- WebAssembly plugin system (`internal/wasm/`)
- Channel-based `Watcher` / `Watch()` / `WatchWithOptions()`
- Replay modes (`ReplayConfig`, `ReplayLastN`, etc.)
- Event type filtering (`--include-types`, `--exclude-types`)
- Time range filtering (`--since`, `--until`)
- `ParseFile()`, `ParseDir()`
- `Event.Data map[string]string` (replaced by typed Event structs)
- Cobra CLI framework
- All external dependencies

### Added

- `ReadFile(ctx, ReadFileConfig) iter.Seq2[Record, error]` for batch file reading
- `Follow(ctx, FollowConfig) iter.Seq2[Record, error]` for live log tailing with rotation handling
- `Engine` with `Process(Record) Result` for adapter-based event extraction
- `Adapter` interface (`ID() AdapterID`, `Decode(Record) ([]Emission, error)`)
- `NewVRChatAdapter()` built-in adapter for VRChat client log lines
- `Observation` with full provenance (adapter ID, rule ID, record reference)
- `EncodeObservationJSON` / `DecodeObservationJSON` for JSON serialization
- `EncodeEvent` / `DecodeEvent` for polymorphic event codec
- `Diagnostic` type for non-fatal processing issues
- `DefaultLogDirectory()` for platform-specific VRChat log path detection
- `examples/` directory with minimal usage examples

## [0.1.0] - Initial Release

### Added

- Initial implementation of VRChat log parser and watcher
- `vrclog.Watch()` function for real-time log monitoring
- `vrclog.ParseLine()` for parsing individual log lines
- Event types: `world_join`, `player_join`, `player_left`
- CLI tool with `tail` command
- JSON Lines and pretty output formats
