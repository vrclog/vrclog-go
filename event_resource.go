package vrclog

import (
	"fmt"
	"net/url"
	"strings"
	"unicode"
)

const maxURLBytes = 16 * 1024

// isBidiFormat reports whether r is a Unicode bidirectional formatting
// character (category Cf): the embedding/override controls U+202A-202E
// and the isolate controls U+2066-2069. unicode.IsControl does not treat
// these as control characters, but they can be used to visually spoof
// displayed text (e.g. player names, URLs) and are rejected wherever
// this package validates or sanitizes untrusted text.
func isBidiFormat(r rune) bool {
	return (r >= '‪' && r <= '‮') || (r >= '⁦' && r <= '⁩')
}

func containsUnsafeRune(s string) bool {
	return strings.ContainsFunc(s, func(r rune) bool {
		return unicode.IsControl(r) || unicode.IsSpace(r) || isBidiFormat(r)
	})
}

// validateHTTPURL validates rawURL as a safe, absolute http(s) URL:
// non-empty, within a sane byte length, free of control/whitespace/bidi
// characters, parseable, http or https scheme, non-empty host, and no
// embedded userinfo.
func validateHTTPURL(rawURL string) error {
	if rawURL == "" {
		return fmt.Errorf("URL is required")
	}
	if len(rawURL) > maxURLBytes {
		return fmt.Errorf("URL exceeds %d bytes", maxURLBytes)
	}
	if containsUnsafeRune(rawURL) {
		return fmt.Errorf("URL contains control, whitespace, or bidi formatting characters")
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("URL is not valid: %w", err)
	}
	if !u.IsAbs() {
		return fmt.Errorf("URL must be absolute")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("URL scheme must be http or https, got %q", u.Scheme)
	}
	if u.Host == "" {
		return fmt.Errorf("URL host is required")
	}
	if u.User != nil {
		return fmt.Errorf("URL must not contain userinfo")
	}
	// containsUnsafeRune above only inspects the raw (percent-encoded)
	// string, so a control character can still slip through if it was
	// percent-encoded (e.g. %0d%0a). Re-check the decoded path, query,
	// and fragment components.
	if strings.ContainsFunc(u.Path, unicode.IsControl) {
		return fmt.Errorf("URL path must not contain percent-encoded control characters")
	}
	decodedQuery, err := url.QueryUnescape(u.RawQuery)
	if err != nil {
		return fmt.Errorf("URL query is not valid: %w", err)
	}
	if strings.ContainsFunc(decodedQuery, unicode.IsControl) {
		return fmt.Errorf("URL query must not contain percent-encoded control characters")
	}
	if strings.ContainsFunc(u.Fragment, unicode.IsControl) {
		return fmt.Errorf("URL fragment must not contain percent-encoded control characters")
	}
	return nil
}

type ResourceKind string

const (
	ResourceKindUnknown ResourceKind = "unknown"
	ResourceKindVideo   ResourceKind = "video"
	ResourceKindAudio   ResourceKind = "audio"
	ResourceKindImage   ResourceKind = "image"
	ResourceKindText    ResourceKind = "text"
)

type ResourceRole string

const (
	ResourceRoleSource        ResourceRole = "source"
	ResourceRoleResolverInput ResourceRole = "resolver_input"
	ResourceRolePlaybackInput ResourceRole = "playback_input"
	ResourceRoleResolved      ResourceRole = "resolved"
	ResourceRoleThumbnail     ResourceRole = "thumbnail"
	ResourceRoleMetadata      ResourceRole = "metadata"
)

type RemoteResource struct {
	URL  string       `json:"url"`
	Kind ResourceKind `json:"kind"`
	Role ResourceRole `json:"role"`
}

func validateRemoteResource(r RemoteResource) error {
	if err := validateHTTPURL(r.URL); err != nil {
		return fmt.Errorf("url: %w", err)
	}
	if !isValidResourceKind(r.Kind) {
		return fmt.Errorf("undefined resource kind: %q", r.Kind)
	}
	if !isValidResourceRole(r.Role) {
		return fmt.Errorf("undefined resource role: %q", r.Role)
	}
	return nil
}

func isValidResourceKind(k ResourceKind) bool {
	switch k {
	case ResourceKindUnknown, ResourceKindVideo, ResourceKindAudio, ResourceKindImage, ResourceKindText:
		return true
	}
	return false
}

func isValidResourceRole(r ResourceRole) bool {
	switch r {
	case ResourceRoleSource, ResourceRoleResolverInput, ResourceRolePlaybackInput,
		ResourceRoleResolved, ResourceRoleThumbnail, ResourceRoleMetadata:
		return true
	}
	return false
}

type ResourceURLObserved struct {
	Resource RemoteResource `json:"resource"`
	Target   *MediaTarget   `json:"target,omitempty"`
}

func (e ResourceURLObserved) Kind() EventKind { return EventKindResourceURLObserved }

func (e ResourceURLObserved) validate() error {
	if err := validateRemoteResource(e.Resource); err != nil {
		return fmt.Errorf("resource: %w", err)
	}
	if err := validateMediaTarget(e.Target); err != nil {
		return fmt.Errorf("target: %w", err)
	}
	return nil
}

func (e ResourceURLObserved) isEvent() {}

type ResourceResolved struct {
	Input  RemoteResource `json:"input"`
	Output RemoteResource `json:"output"`
	Target *MediaTarget   `json:"target,omitempty"`
}

func (e ResourceResolved) Kind() EventKind { return EventKindResourceResolved }

func (e ResourceResolved) validate() error {
	if err := validateRemoteResource(e.Input); err != nil {
		return fmt.Errorf("input: %w", err)
	}
	if err := validateRemoteResource(e.Output); err != nil {
		return fmt.Errorf("output: %w", err)
	}
	if err := validateMediaTarget(e.Target); err != nil {
		return fmt.Errorf("target: %w", err)
	}
	return nil
}

func (e ResourceResolved) isEvent() {}
