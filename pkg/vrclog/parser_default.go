package vrclog

import (
	"context"

	"github.com/vrclog/vrclog-go/internal/parser"
	"github.com/vrclog/vrclog-go/pkg/vrclog/event"
)

// DefaultParser wraps the existing internal parser for VRChat standard log events.
// It parses player_join, player_left, and world_join events.
type DefaultParser struct{}

// ParseLine implements the Parser interface.
// The context parameter is for future use (e.g., timeout/cancellation).
func (DefaultParser) ParseLine(ctx context.Context, line string) (ParseResult, error) {
	ev, err := parser.Parse(line)
	if err != nil {
		return ParseResult{}, err
	}
	if ev == nil {
		return ParseResult{Matched: false}, nil
	}
	return ParseResult{Events: []event.Event{*ev}, Matched: true}, nil
}

// Ensure DefaultParser implements Parser.
var _ Parser = DefaultParser{}
