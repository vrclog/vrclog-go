package main

import "fmt"

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func cmdVersion() {
	fmt.Printf("vrclog version %s (commit %s, built %s)\n", version, commit, date)
}

func runVersion() string {
	return fmt.Sprintf("vrclog version %s (commit %s, built %s)\n", version, commit, date)
}
