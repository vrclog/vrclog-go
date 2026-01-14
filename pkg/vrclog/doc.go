// Package vrclog provides parsing and monitoring of VRChat log files.
//
// This package allows you to:
//   - Parse VRChat log lines into structured events
//   - Monitor log files in real-time for new events
//   - Define custom event patterns via YAML configuration
//   - Build tools like join notifications, history viewers, etc.
//
// # Basic Usage
//
// To monitor VRChat logs in real-time:
//
//	ctx, cancel := context.WithCancel(context.Background())
//	defer cancel()
//
//	events, errs, err := vrclog.WatchWithOptions(ctx,
//	    vrclog.WithIncludeTypes(vrclog.EventPlayerJoin, vrclog.EventPlayerLeft),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	for {
//	    select {
//	    case event, ok := <-events:
//	        if !ok {
//	            return
//	        }
//	        switch event.Type {
//	        case vrclog.EventPlayerJoin:
//	            fmt.Printf("%s joined\n", event.PlayerName)
//	        case vrclog.EventPlayerLeft:
//	            fmt.Printf("%s left\n", event.PlayerName)
//	        case vrclog.EventWorldJoin:
//	            fmt.Printf("Joined world: %s\n", event.WorldName)
//	        }
//	    case err, ok := <-errs:
//	        if !ok {
//	            return
//	        }
//	        log.Printf("error: %v", err)
//	    }
//	}
//
// To parse a single log line:
//
//	event, err := vrclog.ParseLine(line)
//	if err != nil {
//	    log.Printf("parse error: %v", err)
//	} else if event != nil {
//	    // process event
//	}
//
// # Custom Parsers
//
// Implement the [Parser] interface for custom log parsing:
//
//	type Parser interface {
//	    ParseLine(ctx context.Context, line string) (ParseResult, error)
//	}
//
// Use [ParserChain] to combine multiple parsers:
//
//	chain := &vrclog.ParserChain{
//	    Mode:    vrclog.ChainAll,
//	    Parsers: []vrclog.Parser{vrclog.DefaultParser{}, customParser},
//	}
//
// # YAML Pattern Files
//
// For pattern-based parsing without code, use the [pattern] subpackage:
//
//	import "github.com/vrclog/vrclog-go/pkg/vrclog/pattern"
//
//	parser, err := pattern.NewRegexParserFromFile("patterns.yaml")
//
// See the [pattern] package for details on YAML format and usage.
//
// # Platform Support
//
// This package is designed for Windows where VRChat runs.
// Log file paths are auto-detected from standard Windows locations.
//
// # Disclaimer
//
// This is an unofficial tool and is not affiliated with VRChat Inc.
package vrclog
