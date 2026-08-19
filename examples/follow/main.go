// Example: live-follow the VRChat log directory and print Observations as
// JSONL. Press Ctrl+C to stop.
//
// Usage:
//
//	go run ./examples/follow /path/to/log/dir
//	go run ./examples/follow               # uses default VRChat log directory
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

	var dir string
	if len(os.Args) >= 2 {
		dir = os.Args[1]
	}

	engine, err := vrclog.NewEngine(vrclog.NewVRChatAdapter())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	for record, err := range vrclog.Follow(ctx, vrclog.FollowConfig{Directory: dir}) {
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
