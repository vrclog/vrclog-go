package vrclog

import (
	"context"
	"errors"

	"github.com/vrclog/vrclog-go/pkg/vrclog/event"
)

// ParseResult represents the result of parsing a log line.
type ParseResult struct {
	// Events contains the parsed events.
	Events []event.Event

	// Matched indicates whether the parser matched the input.
	// This can be true even if Events is empty (e.g., a filter that matches but outputs nothing).
	Matched bool
}

// Parser is the interface for log line parsers.
// Implementations include DefaultParser (built-in VRChat events),
// RegexParser (custom pattern matching), and future parsers (e.g., WasmParser).
type Parser interface {
	// ParseLine parses a single log line.
	// Returns ParseResult with Matched=true if the line was recognized.
	// Returns error only for unexpected failures (not for unrecognized lines).
	ParseLine(ctx context.Context, line string) (ParseResult, error)
}

// ParserFunc is an adapter to allow ordinary functions to be used as Parsers.
type ParserFunc func(ctx context.Context, line string) (ParseResult, error)

// ParseLine implements the Parser interface.
func (f ParserFunc) ParseLine(ctx context.Context, line string) (ParseResult, error) {
	return f(ctx, line)
}

// Compile-time interface checks.
var (
	_ Parser = ParserFunc(nil)
	_ Parser = (*ParserChain)(nil)
)

// ChainMode specifies how ParserChain executes parsers.
type ChainMode int

const (
	// ChainAll executes all parsers and combines results (default).
	ChainAll ChainMode = iota

	// ChainFirst stops at the first parser that matches.
	ChainFirst

	// ChainContinueOnError skips parsers that return errors and continues.
	// Errors are collected and returned together at the end.
	ChainContinueOnError
)

// ParserChain combines multiple parsers.
// If Mode is not one of the defined constants (ChainAll, ChainFirst, ChainContinueOnError),
// the behavior defaults to ChainAll.
type ParserChain struct {
	Mode    ChainMode
	Parsers []Parser
}

// ParseLine implements the Parser interface.
//
// Context Cancellation:
// If the context is cancelled during execution, ParseLine returns immediately
// with partial results (events collected before cancellation) and the context error.
// Callers should typically discard partial results when err != nil, but the partial
// data is provided for debugging and observability purposes.
func (c *ParserChain) ParseLine(ctx context.Context, line string) (ParseResult, error) {
	var allEvents []event.Event
	var errs []error
	anyMatched := false

	for _, p := range c.Parsers {
		// Check for context cancellation
		if err := ctx.Err(); err != nil {
			// In ChainContinueOnError mode, preserve collected parser errors
			if c.Mode == ChainContinueOnError && len(errs) > 0 {
				return ParseResult{Events: allEvents, Matched: anyMatched}, errors.Join(append(errs, err)...)
			}
			return ParseResult{Events: allEvents, Matched: anyMatched}, err
		}

		// Skip nil parsers
		if p == nil {
			continue
		}

		result, err := p.ParseLine(ctx, line)
		if err != nil {
			if c.Mode == ChainContinueOnError {
				errs = append(errs, err)
				continue
			}
			return ParseResult{}, err
		}
		if result.Matched {
			anyMatched = true
			allEvents = append(allEvents, result.Events...)
			if c.Mode == ChainFirst {
				return ParseResult{Events: allEvents, Matched: true}, nil
			}
		}
	}

	// ChainContinueOnError: return collected errors at the end
	if len(errs) > 0 {
		return ParseResult{Events: allEvents, Matched: anyMatched}, errors.Join(errs...)
	}

	return ParseResult{Events: allEvents, Matched: anyMatched}, nil
}
