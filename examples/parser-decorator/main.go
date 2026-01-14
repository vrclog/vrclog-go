// Package main demonstrates the Decorator pattern for extending Parser functionality.
//
// This example shows two types of decorators:
// 1. MetricsParser: Wraps a parser to collect statistics (lines, matches, errors)
// 2. TransformingParser: Wraps a parser to transform/enrich event data
//
// Key pattern: Decorator pattern - add functionality to existing parsers without modifying them.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/vrclog/vrclog-go/pkg/vrclog"
	"github.com/vrclog/vrclog-go/pkg/vrclog/event"
)

const testLogData = `2024.01.15 10:00:00 Log        -  [Behaviour] OnPlayerJoined alice
2024.01.15 10:01:00 Log        -  [Behaviour] OnPlayerJoined BOB
2024.01.15 10:02:00 Log        -  [Behaviour] Entering Room: Test World
2024.01.15 10:03:00 Log        -  [Behaviour] OnPlayerLeft Alice
2024.01.15 10:04:00 Log        -  [Behaviour] OnPlayerJoined Charlie
2024.01.15 10:05:00 Log        -  [Behaviour] OnPlayerLeft bob
2024.01.15 10:06:00 Log        -  Some unrecognized line
2024.01.15 10:07:00 Log        -  [Behaviour] OnPlayerJoined DAVE
`

func main() {
	fmt.Println("Parser Decorator Example")
	fmt.Println("========================")
	fmt.Println()

	ctx := context.Background()

	// Create temp log file
	tmpFile := createTempFile(testLogData)
	defer os.Remove(tmpFile)

	fmt.Println("Test data contains:")
	fmt.Println("  - 4 player_join events (mixed case names)")
	fmt.Println("  - 2 player_left events")
	fmt.Println("  - 1 world_join event")
	fmt.Println("  - 1 unrecognized line")
	fmt.Println()

	// ========================================
	// 1. MetricsParser - Collect Statistics
	// ========================================
	fmt.Println("1. MetricsParser Decorator")
	fmt.Println("--------------------------")
	fmt.Println("Wraps a parser to track parsing statistics")
	fmt.Println()

	// Wrap the default parser with metrics collection
	metricsParser := NewMetricsParser(vrclog.DefaultParser{})

	fmt.Println("→ Parsing with MetricsParser:")
	eventCount := 0
	for ev, err := range vrclog.ParseFile(ctx, tmpFile,
		vrclog.WithParseParser(metricsParser),
	) {
		if err != nil {
			log.Printf("Error: %v", err)
			continue
		}
		eventCount++
		_ = ev // Process event normally
	}

	// Get statistics
	total, matched, errorCount := metricsParser.Stats()
	fmt.Printf("  Collected %d events\n", eventCount)
	fmt.Println()
	fmt.Println("→ Statistics:")
	fmt.Printf("  Total lines:   %d\n", total)
	fmt.Printf("  Matched lines: %d\n", matched)
	fmt.Printf("  Errors:        %d\n", errorCount)
	fmt.Printf("  Match rate:    %.1f%%\n", float64(matched)/float64(total)*100)
	fmt.Println()

	fmt.Println("→ Use cases:")
	fmt.Println("  - Monitor parser performance")
	fmt.Println("  - Debug custom parsers")
	fmt.Println("  - Collect metrics for dashboards")
	fmt.Println()

	// ========================================
	// 2. TransformingParser - Normalize Data
	// ========================================
	fmt.Println("2. TransformingParser Decorator")
	fmt.Println("--------------------------------")
	fmt.Println("Wraps a parser to transform event data")
	fmt.Println()

	// Create a transformer that normalizes player names to Title Case
	normalizeNames := func(ev event.Event) event.Event {
		if ev.PlayerName != "" {
			// Simple title case: First letter uppercase, rest lowercase
			// Note: This implementation works for ASCII characters only
			// (VRChat usernames are typically ASCII-compatible)
			name := strings.ToLower(ev.PlayerName)
			if len(name) > 0 {
				ev.PlayerName = strings.ToUpper(string(name[0])) + name[1:]
			}
		}
		return ev
	}

	transformingParser := NewTransformingParser(
		vrclog.DefaultParser{},
		normalizeNames,
	)

	fmt.Println("→ Parsing with TransformingParser (normalize names):")
	for ev, err := range vrclog.ParseFile(ctx, tmpFile,
		vrclog.WithParseParser(transformingParser),
	) {
		if err != nil {
			log.Printf("Error: %v", err)
			continue
		}
		if ev.PlayerName != "" {
			fmt.Printf("  %s - %s\n", ev.Type, ev.PlayerName)
		}
	}
	fmt.Println()

	fmt.Println("→ Original names: alice, BOB, Charlie, DAVE")
	fmt.Println("→ Normalized:     Alice, Bob, Charlie, Dave")
	fmt.Println()

	// ========================================
	// 3. Composing Decorators
	// ========================================
	fmt.Println("3. Composing Multiple Decorators")
	fmt.Println("---------------------------------")
	fmt.Println("Stack decorators for multiple enhancements")
	fmt.Println()

	// Compose: Metrics(Transforming(Default))
	composedParser := NewMetricsParser(
		NewTransformingParser(
			vrclog.DefaultParser{},
			normalizeNames,
		),
	)

	fmt.Println("→ Parser stack:")
	fmt.Println("  1. DefaultParser    (parse VRChat events)")
	fmt.Println("  2. Transforming     (normalize names)")
	fmt.Println("  3. Metrics          (collect statistics)")
	fmt.Println()

	fmt.Println("→ Parsing with composed decorators:")
	eventsByType := make(map[event.Type][]string)
	for ev, err := range vrclog.ParseFile(ctx, tmpFile,
		vrclog.WithParseParser(composedParser),
	) {
		if err != nil {
			continue
		}
		if ev.PlayerName != "" {
			eventsByType[ev.Type] = append(eventsByType[ev.Type], ev.PlayerName)
		}
	}

	for eventType, names := range eventsByType {
		fmt.Printf("  %s: %v\n", eventType, names)
	}
	fmt.Println()

	total, matched, _ = composedParser.Stats()
	fmt.Printf("→ Metrics from composed parser:\n")
	fmt.Printf("  Total: %d, Matched: %d\n", total, matched)
	fmt.Println()

	// ========================================
	// Summary
	// ========================================
	fmt.Println("Summary:")
	fmt.Println("--------")
	fmt.Println("MetricsParser       : Track parsing statistics (lines, matches, errors)")
	fmt.Println("TransformingParser  : Transform/enrich event data (normalize, enrich)")
	fmt.Println("Composition         : Stack multiple decorators for layered functionality")
	fmt.Println()
	fmt.Println("Decorator pattern:")
	fmt.Println("  - Wraps existing parsers without modifying them")
	fmt.Println("  - Implements Parser interface (transparent)")
	fmt.Println("  - Can be composed/stacked")
	fmt.Println()
	fmt.Println("Use cases:")
	fmt.Println("  - Metrics: Monitoring, debugging, dashboards")
	fmt.Println("  - Transforming: Data normalization, enrichment, validation")
	fmt.Println("  - Custom: Logging, caching, retry logic")
}

// ============================================================
// MetricsParser - Decorator for collecting statistics
// ============================================================

// MetricsParser wraps a parser and collects statistics about parsing operations.
type MetricsParser struct {
	inner      vrclog.Parser
	mu         sync.Mutex
	totalLines int
	matchCount int
	errorCount int
}

// NewMetricsParser creates a new MetricsParser that wraps the given parser.
func NewMetricsParser(inner vrclog.Parser) *MetricsParser {
	return &MetricsParser{inner: inner}
}

// ParseLine implements the Parser interface while collecting statistics.
func (m *MetricsParser) ParseLine(ctx context.Context, line string) (vrclog.ParseResult, error) {
	// Increment total lines
	m.mu.Lock()
	m.totalLines++
	m.mu.Unlock()

	// Call inner parser
	result, err := m.inner.ParseLine(ctx, line)

	// Update statistics based on result
	m.mu.Lock()
	if err != nil {
		m.errorCount++
	}
	if result.Matched {
		m.matchCount++
	}
	m.mu.Unlock()

	return result, err
}

// Stats returns the collected statistics (total, matched, errors).
func (m *MetricsParser) Stats() (total, matched, errors int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.totalLines, m.matchCount, m.errorCount
}

// Reset clears all collected statistics.
func (m *MetricsParser) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.totalLines = 0
	m.matchCount = 0
	m.errorCount = 0
}

// Compile-time check that MetricsParser implements Parser
var _ vrclog.Parser = (*MetricsParser)(nil)

// ============================================================
// TransformingParser - Decorator for transforming events
// ============================================================

// TransformingParser wraps a parser and applies a transformation function to all events.
type TransformingParser struct {
	inner     vrclog.Parser
	transform func(event.Event) event.Event
}

// NewTransformingParser creates a new TransformingParser that wraps the given parser
// and applies the transformation function to all parsed events.
func NewTransformingParser(inner vrclog.Parser, transform func(event.Event) event.Event) *TransformingParser {
	return &TransformingParser{
		inner:     inner,
		transform: transform,
	}
}

// ParseLine implements the Parser interface while transforming events.
func (t *TransformingParser) ParseLine(ctx context.Context, line string) (vrclog.ParseResult, error) {
	// Call inner parser
	result, err := t.inner.ParseLine(ctx, line)
	if err != nil || !result.Matched {
		return result, err
	}

	// Transform all events
	for i := range result.Events {
		result.Events[i] = t.transform(result.Events[i])
	}

	return result, nil
}

// Compile-time check that TransformingParser implements Parser
var _ vrclog.Parser = (*TransformingParser)(nil)

// ============================================================
// Helper Functions
// ============================================================

// createTempFile creates a temporary VRChat log file
func createTempFile(content string) string {
	tmpFile, err := os.CreateTemp("", "output_log_*.txt")
	if err != nil {
		log.Fatal(err)
	}

	if _, err := tmpFile.WriteString(content); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		log.Fatal(err)
	}

	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpFile.Name())
		log.Fatal(err)
	}

	return tmpFile.Name()
}
