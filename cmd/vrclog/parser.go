package main

import (
	"fmt"

	"github.com/vrclog/vrclog-go/pkg/vrclog"
	"github.com/vrclog/vrclog-go/pkg/vrclog/pattern"
)

// buildParser builds a Parser from pattern file paths.
// Returns nil if no patterns are specified (use default parser).
func buildParser(patternFiles []string) (vrclog.Parser, error) {
	if len(patternFiles) == 0 {
		return nil, nil
	}

	parsers := []vrclog.Parser{vrclog.DefaultParser{}}

	for i, path := range patternFiles {
		rp, err := pattern.NewRegexParserFromFile(path)
		if err != nil {
			// Error from pattern package is already sanitized (no path)
			return nil, fmt.Errorf("pattern file %d: %w", i+1, err)
		}
		parsers = append(parsers, rp)
	}

	return &vrclog.ParserChain{
		Mode:    vrclog.ChainAll,
		Parsers: parsers,
	}, nil
}
