package vrclog

import (
	"fmt"
	"strings"
	"unicode"
)

const (
	maxMediaTargetComponentBytes = 128
	maxMediaTargetKeyBytes       = 256
	maxMediaErrorCodeBytes       = 128
	maxMediaErrorMessageBytes    = 4096
)

type MediaBackend string

const (
	MediaBackendUnknown MediaBackend = "unknown"
	MediaBackendAVPro   MediaBackend = "avpro"
	MediaBackendUnity   MediaBackend = "unity"
)

type MediaStage string

const (
	MediaStageUnknown  MediaStage = "unknown"
	MediaStageResolve  MediaStage = "resolve"
	MediaStageLoad     MediaStage = "load"
	MediaStagePlayback MediaStage = "playback"
)

type MediaTarget struct {
	Component string       `json:"component"`
	Key       string       `json:"key,omitempty"`
	Backend   MediaBackend `json:"backend"`
}

type MediaErrorObserved struct {
	Stage    MediaStage      `json:"stage"`
	Code     string          `json:"code,omitempty"`
	Message  string          `json:"message,omitempty"`
	Resource *RemoteResource `json:"resource,omitempty"`
	Target   *MediaTarget    `json:"target,omitempty"`
}

func (e MediaErrorObserved) Kind() EventKind { return EventKindMediaErrorObserved }

func (e MediaErrorObserved) validate() error {
	if !isValidMediaStage(e.Stage) {
		return fmt.Errorf("undefined media stage: %q", e.Stage)
	}
	if e.Code == "" && e.Message == "" {
		return fmt.Errorf("media error requires code or message")
	}
	if len(e.Code) > maxMediaErrorCodeBytes {
		return fmt.Errorf("error code exceeds %d bytes", maxMediaErrorCodeBytes)
	}
	if e.Code != "" && containsUnsafeControlOrBidi(e.Code) {
		return fmt.Errorf("error code contains control or bidi formatting characters")
	}
	if len(e.Message) > maxMediaErrorMessageBytes {
		return fmt.Errorf("error message exceeds %d bytes", maxMediaErrorMessageBytes)
	}
	if e.Message != "" && containsUnsafeControlOrBidi(e.Message) {
		return fmt.Errorf("error message contains control or bidi formatting characters")
	}
	if e.Resource != nil {
		if err := validateRemoteResource(*e.Resource); err != nil {
			return fmt.Errorf("resource: %w", err)
		}
	}
	if err := validateMediaTarget(e.Target); err != nil {
		return fmt.Errorf("target: %w", err)
	}
	return nil
}

func (e MediaErrorObserved) isEvent() {}

func isValidMediaStage(s MediaStage) bool {
	switch s {
	case MediaStageUnknown, MediaStageResolve, MediaStageLoad, MediaStagePlayback:
		return true
	}
	return false
}

func isValidMediaBackend(b MediaBackend) bool {
	switch b {
	case MediaBackendUnknown, MediaBackendAVPro, MediaBackendUnity:
		return true
	}
	return false
}

// containsUnsafeControlOrBidi reports whether s contains a control
// character (including tab — canonical event text fields are
// single-line and must not carry control characters that could be
// misused for column/log manipulation) or a Unicode bidi formatting
// character. Unlike containsUnsafeRune, ordinary spaces are allowed.
func containsUnsafeControlOrBidi(s string) bool {
	return strings.ContainsFunc(s, func(r rune) bool {
		return unicode.IsControl(r) || isBidiFormat(r)
	})
}

// validateMediaTarget validates an optional MediaTarget. A nil target is
// valid (the field is optional on events that carry it).
func validateMediaTarget(t *MediaTarget) error {
	if t == nil {
		return nil
	}
	if t.Component == "" {
		return fmt.Errorf("component is required")
	}
	if len(t.Component) > maxMediaTargetComponentBytes {
		return fmt.Errorf("component exceeds %d bytes", maxMediaTargetComponentBytes)
	}
	if containsUnsafeControlOrBidi(t.Component) {
		return fmt.Errorf("component contains control or bidi formatting characters")
	}
	if len(t.Key) > maxMediaTargetKeyBytes {
		return fmt.Errorf("key exceeds %d bytes", maxMediaTargetKeyBytes)
	}
	if t.Key != "" && containsUnsafeControlOrBidi(t.Key) {
		return fmt.Errorf("key contains control or bidi formatting characters")
	}
	if t.Backend == "" {
		return fmt.Errorf("backend is required")
	}
	if !isValidMediaBackend(t.Backend) {
		return fmt.Errorf("undefined media backend: %q", t.Backend)
	}
	return nil
}
