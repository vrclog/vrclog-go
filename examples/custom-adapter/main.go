// Example: a minimal custom Adapter that detects a hypothetical log prefix
// and emits a canonical PlayerJoined event.
//
// Community adapters must return one of the 7 canonical Event types defined by
// the vrclog package. You cannot define your own Event type -- the Event
// interface is sealed (it contains unexported methods).
//
// Usage:
//
//	go run ./examples/custom-adapter testdata/logs/vrchat_full.txt
package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	vrclog "github.com/vrclog/vrclog-go"
)

// motdAdapter detects lines like "[MOTD] Welcome, PlayerName!" and emits a
// PlayerJoined event. This is a toy example showing the Adapter contract.
type motdAdapter struct{}

func (a motdAdapter) ID() vrclog.AdapterID { return "example.motd" }

func (a motdAdapter) Decode(r vrclog.Record) ([]vrclog.Emission, error) {
	const prefix = "[MOTD] Welcome, "
	idx := strings.Index(r.Message, prefix)
	if idx < 0 {
		return nil, nil
	}
	name := strings.TrimSuffix(r.Message[idx+len(prefix):], "!")
	if name == "" {
		return nil, nil
	}
	return []vrclog.Emission{{
		Rule:  "motd_welcome",
		Event: vrclog.PlayerJoined{Player: vrclog.Player{DisplayName: name}},
	}}, nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: custom-adapter <logfile>")
		os.Exit(1)
	}

	// Register both the built-in VRChat adapter and our custom adapter.
	// All registered adapters run for every record -- there is no first-match.
	engine, err := vrclog.NewEngine(vrclog.NewVRChatAdapter(), motdAdapter{})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	ctx := context.Background()
	for record, err := range vrclog.ReadFile(ctx, vrclog.ReadFileConfig{Path: os.Args[1]}) {
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		result := engine.Process(record)
		for _, obs := range result.Observations {
			data, err := vrclog.EncodeObservationJSON(obs)
			if err != nil {
				fmt.Fprintf(os.Stderr, "encode error: %v\n", err)
				continue
			}
			fmt.Println(string(data))
		}
	}
}
