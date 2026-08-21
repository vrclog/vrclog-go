# vrclog-go

A Go library that reads VRChat's local `output_log_*.txt` files, converts each
physical line to a `Record`, runs registered `Adapter`s via an `Engine` to
extract canonical `Event`s, and produces `Observation`s with provenance.

```text
VRChat output_log_*.txt
        |
        v
  Read / Follow          -- file I/O, line framing, header decode
        |
        v
      Record              -- timestamped line with offset, hash-based ID
        |
        v
      Engine              -- runs all registered Adapters
        |
        v
   canonical Event        -- 7 sealed types (player, world, resource, media)
        |
        v
    Observation           -- event + provenance (adapter, rule, record ref)
```

`Observation.ID` is a deterministic identifier derived only from the
record, adapter, and rule that produced it:

```text
observation_id = SHA-256(record_id + NUL + adapter_id + NUL + rule_id)
```

It does not depend on emission order, event payload, or timestamp, so the
same logical observation always gets the same ID regardless of how many
other emissions an adapter returned for the same record. An adapter that
returns the same non-empty rule ID more than once for a single record has
all of those emissions rejected (`duplicate_rule_id` diagnostic) rather
than silently picking one.

## Install

```bash
go get github.com/vrclog/vrclog-go
```

```go
import vrclog "github.com/vrclog/vrclog-go"
```

## Built-in Adapter

`NewVRChatAdapter()` handles log lines emitted by the VRChat client itself:

- Player join/leave (`[Behaviour] OnPlayerJoined`, `OnPlayerLeft`)
- World entering/joining (`[Behaviour] Entering Room`, `Joining wrld_...`)
- Video URL resolve attempts and results (`[Video Playback]`)
- AVPro video opening and errors (`[AVProVideo]`)

It does **not** understand community world assets such as YamaPlayer,
iwaSync3, VRCX, or any other third-party prefixes. Adapters for those belong
in a separate `vrclog-adapters` repository that this library does not import.

## Usage

### Read a single log file

```go
package main

import (
	"context"
	"fmt"
	"os"

	vrclog "github.com/vrclog/vrclog-go"
)

func main() {
	engine, _ := vrclog.NewEngine(vrclog.NewVRChatAdapter())
	ctx := context.Background()

	for record, err := range vrclog.ReadFile(ctx, vrclog.ReadFileConfig{Path: os.Args[1]}) {
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			break
		}
		result := engine.Process(record)
		for _, obs := range result.Observations {
			data, _ := vrclog.EncodeObservationJSON(obs)
			fmt.Println(string(data))
		}
	}
}
```

### Follow the VRChat log directory

```go
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	vrclog "github.com/vrclog/vrclog-go"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	engine, _ := vrclog.NewEngine(vrclog.NewVRChatAdapter())

	for record, err := range vrclog.Follow(ctx, vrclog.FollowConfig{}) {
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			break
		}
		result := engine.Process(record)
		for _, obs := range result.Observations {
			data, _ := vrclog.EncodeObservationJSON(obs)
			fmt.Println(string(data))
		}
	}
}
```

### Custom Adapter

Community adapters must return one of the 7 canonical `Event` types defined by
this package. The `Event` interface is sealed -- you cannot define your own
event type.

```go
type myAdapter struct{}

func (a myAdapter) ID() vrclog.AdapterID { return "my.custom" }

func (a myAdapter) Decode(r vrclog.Record) ([]vrclog.Emission, error) {
	if strings.Contains(r.Message, "[MyMod] player entered") {
		return []vrclog.Emission{{
			Rule:  "mymod_player",
			Event: vrclog.PlayerJoined{Player: vrclog.Player{DisplayName: "..."}},
		}}, nil
	}
	return nil, nil
}
```

### LogSnapshot

`CaptureLogSnapshot` captures the byte head of every currently existing
VRChat log file so a downstream consumer (e.g. a companion process
deciding what counts as "catch-up" history) can tell which bytes existed
before the snapshot was taken:

```go
snap, err := vrclog.CaptureLogSnapshot("") // "" uses DefaultLogDirectory
if err != nil {
    // handle error
}

if snap.Contains(record) {
    // record's bytes existed when the snapshot was captured
}
```

`LogSnapshot` uses path-and-size semantics: it identifies a source by its
normalized file path, not by a stable file incarnation. VRChat always
creates uniquely timestamped log filenames, so this is not a concern in
practice, but a file replaced at the exact same path with content at or
above the captured size is indistinguishable from an untouched file.

## CLI

```bash
go install github.com/vrclog/vrclog-go/cmd/vrclog@latest
```

| Command | Description |
|---------|-------------|
| `vrclog read <file>...` | Read log files and output Observations as JSONL to stdout |
| `vrclog follow [--dir <path>]` | Live-follow the VRChat log directory (Ctrl+C to stop) |
| `vrclog version` | Print version information |

## Privacy and Security

**Observation JSON output** (whether from the CLI or `EncodeObservationJSON`)
contains the same class of sensitive information as the raw VRChat logs it is
derived from:

- **Player display names** -- real usernames of VRChat players
- **VRChat user IDs** (`usr_*`) -- persistent account identifiers
- **World and instance IDs** -- instance IDs may embed the instance owner's
  user ID via patterns like `~private(usr_xxx)`
- **Media URLs** -- video/image URLs that may include time-limited signed
  authentication tokens (e.g. `sig=`, `lsig=`, `expire=`)

Treat Observation JSON with the same care as raw log files. Do not publish,
share, or commit it carelessly.

## Scope

This is a deliberately narrow library, not a general log framework.

- **Read-only**: local file access only; this library never touches the running
  VRChat process
- **No external dependencies**: standard library only (`go.sum` is empty)
- **Not supported**: YAML pattern config, plugin/WASM system, channel-based
  Watcher, replay/filter options, community-asset adapters

## License

See [LICENSE](LICENSE).
