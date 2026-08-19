package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	switch os.Args[1] {
	case "read":
		cmdRead(os.Args[2:])
	case "follow":
		cmdFollow(os.Args[2:])
	case "version":
		cmdVersion()
	default:
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: vrclog <read|follow|version> [flags]")
}
