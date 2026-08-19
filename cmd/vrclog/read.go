package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	vrclog "github.com/vrclog/vrclog-go"
)

func cmdRead(args []string) {
	os.Exit(runRead(args, os.Stdout, os.Stderr))
}

func runRead(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("read", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}

	paths := fs.Args()
	if len(paths) == 0 {
		fmt.Fprintln(stderr, "usage: vrclog read <file> [<file>...]")
		return 2
	}

	engine, err := vrclog.NewEngine(vrclog.NewVRChatAdapter())
	if err != nil {
		fmt.Fprintf(stderr, "vrclog: engine init: %v\n", err)
		return 1
	}

	ctx := context.Background()
	hadFatalError := false

	for _, path := range paths {
		for record, err := range vrclog.ReadFile(ctx, vrclog.ReadFileConfig{Path: path}) {
			if err != nil {
				fmt.Fprintf(stderr, "vrclog: %s: %v\n", path, err)
				hadFatalError = true
				break
			}

			result := engine.Process(record)

			for _, obs := range result.Observations {
				jsonBytes, encErr := vrclog.EncodeObservationJSON(obs)
				if encErr != nil {
					fmt.Fprintf(stderr, "vrclog: %s:%d encode error: %v\n", path, record.Line, encErr)
					continue
				}
				jsonBytes = append(jsonBytes, '\n')
				stdout.Write(jsonBytes)
			}

			for _, diag := range result.Diagnostics {
				fmt.Fprintf(stderr, "vrclog: %s:%d %s: %s\n", path, diag.Record.Line, diag.Code, diag.Message)
			}
		}
	}

	if hadFatalError {
		return 1
	}
	return 0
}
