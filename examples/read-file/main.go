// Example: read a VRChat log file and print Observations as JSONL.
//
// Usage:
//
//	go run ./examples/read-file testdata/logs/vrchat_full.txt
package main

import (
	"context"
	"fmt"
	"os"

	vrclog "github.com/vrclog/vrclog-go"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: read-file <logfile>")
		os.Exit(1)
	}

	engine, err := vrclog.NewEngine(vrclog.NewVRChatAdapter())
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
