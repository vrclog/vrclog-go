# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
