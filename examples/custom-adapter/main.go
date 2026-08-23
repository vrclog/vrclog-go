// Example: a custom Adapter that detects a VRPoker-style log line and emits
// structured world-specific data through AdapterEvent.
//
// The Event interface is sealed, so external packages cannot define their own
// Event implementations. AdapterEvent is the sanctioned escape hatch for data
// that cannot map to the other canonical Event types.
//
// Usage:
//
//	go run ./examples/custom-adapter testdata/logs/vrchat_full.txt
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	vrclog "github.com/vrclog/vrclog-go"
)

// vrPokerAdapter detects lines like "[Seat]: Draw Local Hole Cards: Jc, 6d".
type vrPokerAdapter struct{}

func (a vrPokerAdapter) ID() vrclog.AdapterID { return "example.vrpoker" }

func (a vrPokerAdapter) Decode(r vrclog.Record) ([]vrclog.Emission, error) {
	const prefix = "[Seat]: Draw Local Hole Cards: "
	idx := strings.Index(r.Message, prefix)
	if idx < 0 {
		return nil, nil
	}
	cards := strings.SplitN(r.Message[idx+len(prefix):], ",", 2)
	if len(cards) != 2 {
		return nil, nil
	}
	card1 := strings.TrimSpace(cards[0])
	card2 := strings.TrimSpace(cards[1])
	if card1 == "" || card2 == "" {
		return nil, nil
	}
	data, err := json.Marshal(struct {
		Card1 string `json:"card1"`
		Card2 string `json:"card2"`
	}{Card1: card1, Card2: card2})
	if err != nil {
		return nil, err
	}
	return []vrclog.Emission{{
		Rule: "vrpoker_hole_cards",
		Event: vrclog.AdapterEvent{
			Tag:  "vrpoker.hole_cards.v1",
			Data: data,
		},
	}}, nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: custom-adapter <logfile>")
		os.Exit(1)
	}

	// Register both the built-in VRChat adapter and our custom adapter.
	// All registered adapters run for every record -- there is no first-match.
	engine, err := vrclog.NewEngine(vrclog.NewVRChatAdapter(), vrPokerAdapter{})
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
