package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	vrclog "github.com/vrclog/vrclog-go"
)

func cmdFollow(args []string) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	os.Exit(runFollow(ctx, args, os.Stdout, os.Stderr))
}

func runFollow(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("follow", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dir := fs.String("dir", "", "log directory path")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	logDir := *dir
	if logDir == "" {
		d, err := vrclog.DefaultLogDirectory()
		if err != nil {
			fmt.Fprintf(stderr, "vrclog: %v\n", err)
			return 1
		}
		logDir = d
	}

	engine, err := vrclog.NewEngine(vrclog.NewVRChatAdapter())
	if err != nil {
		fmt.Fprintf(stderr, "vrclog: engine init: %v\n", err)
		return 1
	}

	hadFatalError := false
	for record, err := range vrclog.Follow(ctx, vrclog.FollowConfig{Directory: logDir}) {
		if err != nil {
			fmt.Fprintf(stderr, "vrclog: %v\n", err)
			hadFatalError = true
			break
		}

		result := engine.Process(record)

		for _, obs := range result.Observations {
			jsonBytes, encErr := vrclog.EncodeObservationJSON(obs)
			if encErr != nil {
				fmt.Fprintf(stderr, "vrclog: %s:%d encode error: %v\n", record.Path, record.Line, encErr)
				continue
			}
			jsonBytes = append(jsonBytes, '\n')
			stdout.Write(jsonBytes)
		}

		for _, diag := range result.Diagnostics {
			fmt.Fprintf(stderr, "vrclog: %s:%d %s: %s\n", record.Path, diag.Record.Line, diag.Code, diag.Message)
		}
	}

	if hadFatalError {
		return 1
	}
	return 0
}
