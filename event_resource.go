package vrclog

import "fmt"

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
	if r.URL == "" {
		return fmt.Errorf("resource URL is required")
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
	return validateRemoteResource(e.Resource)
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
	return nil
}

func (e ResourceResolved) isEvent() {}
