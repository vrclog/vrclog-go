package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	// Version information (set by ldflags)
	version = "dev"
	commit  = "none"
	date    = "unknown"

	// Global flags
	verbose bool
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "vrclog",
	Short: "VRChat log parser and monitor",
	Long: `vrclog is a tool for parsing and monitoring VRChat log files.

It can parse VRChat logs to extract events like player joins/leaves,
world changes, and more. Events are output as JSON Lines for easy
processing with other tools.

This is an unofficial tool and is not affiliated with VRChat Inc.`,
	SilenceUsage: true, // Don't show usage on error
}

func init() {
	// Global flags (inherited by all subcommands)
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false,
		"Enable verbose logging")

	// Add subcommands
	rootCmd.AddCommand(tailCmd)
	rootCmd.AddCommand(parseCmd)
	rootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("vrclog %s (commit: %s, built: %s)\n", version, commit, date)
	},
}
