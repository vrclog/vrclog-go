package vrclog

import (
	"strings"
	"time"
)

const headerTimestampLen = 19
const headerTimestampLayout = "2006.01.02 15:04:05"
const headerSeparator = " -  "

var levelMap = map[string]Level{
	"Log":       LevelLog,
	"Warning":   LevelWarning,
	"Error":     LevelError,
	"Debug":     LevelDebug,
	"Exception": LevelException,
}

func decodeHeader(raw string, loc *time.Location) (t time.Time, level Level, message string, ok bool) {
	if loc == nil {
		loc = time.Local
	}

	if len(raw) < headerTimestampLen {
		return time.Time{}, LevelUnknown, "", false
	}

	t, err := time.ParseInLocation(headerTimestampLayout, raw[:headerTimestampLen], loc)
	if err != nil {
		return time.Time{}, LevelUnknown, "", false
	}

	sepIdx := strings.Index(raw[headerTimestampLen:], headerSeparator)
	if sepIdx < 0 {
		return time.Time{}, LevelUnknown, "", false
	}
	sepIdx += headerTimestampLen

	levelWord := strings.TrimSpace(raw[headerTimestampLen:sepIdx])

	lvl, found := levelMap[levelWord]
	if !found {
		lvl = LevelUnknown
	}

	message = raw[sepIdx+len(headerSeparator):]

	return t, lvl, message, true
}
