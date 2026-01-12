package main

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/vrclog/vrclog-go/pkg/vrclog"
)

// ValidFormats lists all valid output formats.
var ValidFormats = map[string]bool{
	"jsonl":  true,
	"pretty": true,
}

// OutputEvent writes an event in the specified format to the writer.
func OutputEvent(format string, event vrclog.Event, out io.Writer) error {
	switch format {
	case "jsonl":
		return OutputJSON(event, out)
	case "pretty":
		return OutputPretty(event, out)
	default:
		return fmt.Errorf("unknown format: %s", format)
	}
}

// OutputJSON writes an event as JSON Lines format.
func OutputJSON(event vrclog.Event, out io.Writer) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, string(data))
	return err
}

// OutputPretty writes an event in human-readable format.
func OutputPretty(event vrclog.Event, out io.Writer) error {
	ts := event.Timestamp.Format("15:04:05")

	var err error
	switch event.Type {
	case vrclog.EventPlayerJoin:
		_, err = fmt.Fprintf(out, "[%s] + %s joined\n", ts, event.PlayerName)
	case vrclog.EventPlayerLeft:
		_, err = fmt.Fprintf(out, "[%s] - %s left\n", ts, event.PlayerName)
	case vrclog.EventWorldJoin:
		if event.WorldName != "" {
			_, err = fmt.Fprintf(out, "[%s] > Joined world: %s\n", ts, event.WorldName)
		} else {
			_, err = fmt.Fprintf(out, "[%s] > Joined instance: %s\n", ts, event.InstanceID)
		}
	default:
		// Custom events with Data field
		if len(event.Data) > 0 {
			_, err = fmt.Fprintf(out, "[%s] * %s: %s\n", ts, event.Type, formatData(event.Data))
		} else {
			_, err = fmt.Fprintf(out, "[%s] * %s\n", ts, event.Type)
		}
	}

	return err
}

// formatData formats a map as sorted key=value pairs.
// Values are quoted if they contain spaces, equals signs, quotes, or control characters.
func formatData(data map[string]string) string {
	if len(data) == 0 {
		return ""
	}

	// Sort keys for deterministic output
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(data))
	for _, k := range keys {
		v := data[k]
		parts = append(parts, fmt.Sprintf("%s=%s", quoteIfNeeded(k), quoteIfNeeded(v)))
	}
	return strings.Join(parts, " ")
}

// quoteIfNeeded quotes a value if it contains special characters or control characters.
// Returns the value unchanged if no quoting is needed.
func quoteIfNeeded(v string) string {
	if v == "" {
		return `""`
	}

	// Check for characters that require quoting
	needsQuote := false
	for _, c := range v {
		// Quote if: space, equals, quote, backslash, or any control character (< 0x20 or DEL 0x7F)
		if c == ' ' || c == '=' || c == '"' || c == '\\' || c < 0x20 || c == 0x7F {
			needsQuote = true
			break
		}
	}
	if !needsQuote {
		return v
	}

	// Escape special characters
	var sb strings.Builder
	sb.WriteByte('"')
	for _, c := range v {
		switch {
		case c == '\\':
			sb.WriteString(`\\`)
		case c == '"':
			sb.WriteString(`\"`)
		case c == '\n':
			sb.WriteString(`\n`)
		case c == '\r':
			sb.WriteString(`\r`)
		case c == '\t':
			sb.WriteString(`\t`)
		case c < 0x20 || c == 0x7F:
			// Other control characters (including DEL): escape as \xNN
			sb.WriteString(fmt.Sprintf(`\x%02x`, c))
		default:
			sb.WriteRune(c)
		}
	}
	sb.WriteByte('"')
	return sb.String()
}
