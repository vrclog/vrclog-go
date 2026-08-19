package vrclog

import "fmt"

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
	Component string       `json:"component,omitempty"`
	Key       string       `json:"key,omitempty"`
	Backend   MediaBackend `json:"backend,omitempty"`
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
