package vrclog

import (
	"strings"
	"testing"
)

func TestPlayerJoinedValidate(t *testing.T) {
	valid := PlayerJoined{Player: Player{DisplayName: "Alice"}}
	if err := valid.validate(); err != nil {
		t.Errorf("valid PlayerJoined.validate() = %v", err)
	}
	if valid.Kind() != EventKindPlayerJoined {
		t.Errorf("Kind() = %q, want %q", valid.Kind(), EventKindPlayerJoined)
	}

	invalid := PlayerJoined{Player: Player{}}
	if err := invalid.validate(); err == nil {
		t.Error("PlayerJoined with empty DisplayName should fail validation")
	}
}

func TestPlayerLeftValidate(t *testing.T) {
	valid := PlayerLeft{Player: Player{DisplayName: "Bob"}}
	if err := valid.validate(); err != nil {
		t.Errorf("valid PlayerLeft.validate() = %v", err)
	}
	if valid.Kind() != EventKindPlayerLeft {
		t.Errorf("Kind() = %q, want %q", valid.Kind(), EventKindPlayerLeft)
	}

	invalid := PlayerLeft{Player: Player{}}
	if err := invalid.validate(); err == nil {
		t.Error("PlayerLeft with empty DisplayName should fail validation")
	}
}

func TestWorldEnteringObservedValidate(t *testing.T) {
	valid := WorldEnteringObserved{World: World{Name: "Test World"}}
	if err := valid.validate(); err != nil {
		t.Errorf("valid WorldEnteringObserved.validate() = %v", err)
	}

	invalid := WorldEnteringObserved{World: World{}}
	if err := invalid.validate(); err == nil {
		t.Error("WorldEnteringObserved with empty Name should fail validation")
	}
}

func TestWorldJoiningObservedValidate(t *testing.T) {
	valid := WorldJoiningObserved{World: World{ID: "wrld_123", InstanceID: "456~private"}}
	if err := valid.validate(); err != nil {
		t.Errorf("valid WorldJoiningObserved.validate() = %v", err)
	}

	noID := WorldJoiningObserved{World: World{InstanceID: "456"}}
	if err := noID.validate(); err == nil {
		t.Error("WorldJoiningObserved without ID should fail validation")
	}

	noInstance := WorldJoiningObserved{World: World{ID: "wrld_123"}}
	if err := noInstance.validate(); err == nil {
		t.Error("WorldJoiningObserved without InstanceID should fail validation")
	}
}

func TestResourceURLObservedValidate(t *testing.T) {
	valid := ResourceURLObserved{
		Resource: RemoteResource{URL: "https://example.com", Kind: ResourceKindVideo, Role: ResourceRoleResolverInput},
	}
	if err := valid.validate(); err != nil {
		t.Errorf("valid ResourceURLObserved.validate() = %v", err)
	}

	emptyURL := ResourceURLObserved{
		Resource: RemoteResource{URL: "", Kind: ResourceKindVideo, Role: ResourceRoleSource},
	}
	if err := emptyURL.validate(); err == nil {
		t.Error("ResourceURLObserved with empty URL should fail validation")
	}

	badKind := ResourceURLObserved{
		Resource: RemoteResource{URL: "https://x.com", Kind: "invalid_kind", Role: ResourceRoleSource},
	}
	if err := badKind.validate(); err == nil {
		t.Error("ResourceURLObserved with undefined Kind should fail validation")
	}

	badRole := ResourceURLObserved{
		Resource: RemoteResource{URL: "https://x.com", Kind: ResourceKindVideo, Role: "invalid_role"},
	}
	if err := badRole.validate(); err == nil {
		t.Error("ResourceURLObserved with undefined Role should fail validation")
	}
}

func TestResourceResolvedValidate(t *testing.T) {
	valid := ResourceResolved{
		Input:  RemoteResource{URL: "https://in.com", Kind: ResourceKindVideo, Role: ResourceRoleResolverInput},
		Output: RemoteResource{URL: "https://out.com", Kind: ResourceKindVideo, Role: ResourceRoleResolved},
	}
	if err := valid.validate(); err != nil {
		t.Errorf("valid ResourceResolved.validate() = %v", err)
	}

	emptyInput := ResourceResolved{
		Input:  RemoteResource{URL: "", Kind: ResourceKindVideo, Role: ResourceRoleResolverInput},
		Output: RemoteResource{URL: "https://out.com", Kind: ResourceKindVideo, Role: ResourceRoleResolved},
	}
	if err := emptyInput.validate(); err == nil {
		t.Error("ResourceResolved with empty Input URL should fail validation")
	}

	emptyOutput := ResourceResolved{
		Input:  RemoteResource{URL: "https://in.com", Kind: ResourceKindVideo, Role: ResourceRoleResolverInput},
		Output: RemoteResource{URL: "", Kind: ResourceKindVideo, Role: ResourceRoleResolved},
	}
	if err := emptyOutput.validate(); err == nil {
		t.Error("ResourceResolved with empty Output URL should fail validation")
	}
}

func TestMediaErrorObservedValidate(t *testing.T) {
	withCode := MediaErrorObserved{Stage: MediaStageResolve, Code: "ERR001"}
	if err := withCode.validate(); err != nil {
		t.Errorf("valid MediaErrorObserved (code).validate() = %v", err)
	}

	withMessage := MediaErrorObserved{Stage: MediaStageLoad, Message: "something failed"}
	if err := withMessage.validate(); err != nil {
		t.Errorf("valid MediaErrorObserved (message).validate() = %v", err)
	}

	withBoth := MediaErrorObserved{Stage: MediaStagePlayback, Code: "E1", Message: "msg"}
	if err := withBoth.validate(); err != nil {
		t.Errorf("valid MediaErrorObserved (both).validate() = %v", err)
	}

	badStage := MediaErrorObserved{Stage: "invalid_stage", Code: "ERR"}
	if err := badStage.validate(); err == nil {
		t.Error("MediaErrorObserved with undefined Stage should fail validation")
	}

	noCodeNoMsg := MediaErrorObserved{Stage: MediaStageUnknown}
	if err := noCodeNoMsg.validate(); err == nil {
		t.Error("MediaErrorObserved with no Code and no Message should fail validation")
	}
}

// --- Phase 6: URL, MediaTarget, and nested validation ---

func TestValidateRemoteResource_URLTooLong(t *testing.T) {
	longURL := "https://example.com/" + strings.Repeat("a", maxURLBytes)
	r := RemoteResource{URL: longURL, Kind: ResourceKindVideo, Role: ResourceRoleSource}
	if err := validateRemoteResource(r); err == nil {
		t.Error("expected error for oversized URL")
	}
}

func TestValidateRemoteResource_ControlChars(t *testing.T) {
	r := RemoteResource{URL: "https://example.com/\x01video", Kind: ResourceKindVideo, Role: ResourceRoleSource}
	if err := validateRemoteResource(r); err == nil {
		t.Error("expected error for URL containing control characters")
	}
}

func TestValidateRemoteResource_RelativeURL(t *testing.T) {
	r := RemoteResource{URL: "/relative/path", Kind: ResourceKindVideo, Role: ResourceRoleSource}
	if err := validateRemoteResource(r); err == nil {
		t.Error("expected error for relative URL")
	}
}

func TestValidateRemoteResource_UserinfoRejected(t *testing.T) {
	r := RemoteResource{URL: "https://user:pass@example.com/video", Kind: ResourceKindVideo, Role: ResourceRoleSource}
	if err := validateRemoteResource(r); err == nil {
		t.Error("expected error for URL with userinfo")
	}
}

func TestValidateRemoteResource_NonHTTPScheme(t *testing.T) {
	r := RemoteResource{URL: "ftp://example.com/video", Kind: ResourceKindVideo, Role: ResourceRoleSource}
	if err := validateRemoteResource(r); err == nil {
		t.Error("expected error for non-http(s) scheme")
	}
}

func TestValidateRemoteResource_EmptyHost(t *testing.T) {
	r := RemoteResource{URL: "https:///path", Kind: ResourceKindVideo, Role: ResourceRoleSource}
	if err := validateRemoteResource(r); err == nil {
		t.Error("expected error for empty host")
	}
}

func TestValidateRemoteResource_BidiChars(t *testing.T) {
	r := RemoteResource{URL: "https://example.com/\u202evideo", Kind: ResourceKindVideo, Role: ResourceRoleSource}
	if err := validateRemoteResource(r); err == nil {
		t.Error("expected error for URL containing bidi formatting characters")
	}
}

func TestValidateRemoteResource_PercentEncodedControlInQuery(t *testing.T) {
	r := RemoteResource{URL: "https://example.com/video?sig=abc%0d%0aInjected", Kind: ResourceKindVideo, Role: ResourceRoleSource}
	if err := validateRemoteResource(r); err == nil {
		t.Error("expected error for percent-encoded control characters in query")
	}
}

func TestValidateRemoteResource_PercentEncodedControlInPath(t *testing.T) {
	r := RemoteResource{URL: "https://example.com/%0d%0avideo", Kind: ResourceKindVideo, Role: ResourceRoleSource}
	if err := validateRemoteResource(r); err == nil {
		t.Error("expected error for percent-encoded control characters in path")
	}
}

func TestValidateRemoteResource_PercentEncodedControlInFragment(t *testing.T) {
	r := RemoteResource{URL: "https://example.com/video#%0d%0a", Kind: ResourceKindVideo, Role: ResourceRoleSource}
	if err := validateRemoteResource(r); err == nil {
		t.Error("expected error for percent-encoded control characters in fragment")
	}
}

func TestValidateMediaTarget_EmptyComponent(t *testing.T) {
	target := &MediaTarget{Component: "", Backend: MediaBackendUnknown}
	if err := validateMediaTarget(target); err == nil {
		t.Error("expected error for empty Component")
	}
}

func TestValidateMediaTarget_ComponentTooLong(t *testing.T) {
	target := &MediaTarget{Component: strings.Repeat("a", maxMediaTargetComponentBytes+1), Backend: MediaBackendUnknown}
	if err := validateMediaTarget(target); err == nil {
		t.Error("expected error for oversized Component")
	}
}

func TestValidateMediaTarget_EmptyBackend(t *testing.T) {
	target := &MediaTarget{Component: "vrchat", Backend: ""}
	if err := validateMediaTarget(target); err == nil {
		t.Error("expected error for empty Backend")
	}
}

func TestValidateMediaTarget_InvalidBackend(t *testing.T) {
	target := &MediaTarget{Component: "vrchat", Backend: "not_a_real_backend"}
	if err := validateMediaTarget(target); err == nil {
		t.Error("expected error for undefined Backend")
	}
}

func TestValidateMediaTarget_KeyTooLong(t *testing.T) {
	target := &MediaTarget{Component: "vrchat", Backend: MediaBackendUnknown, Key: strings.Repeat("a", maxMediaTargetKeyBytes+1)}
	if err := validateMediaTarget(target); err == nil {
		t.Error("expected error for oversized Key")
	}
}

func TestValidateMediaTarget_ControlChars(t *testing.T) {
	target := &MediaTarget{Component: "vrchat\x01", Backend: MediaBackendUnknown}
	if err := validateMediaTarget(target); err == nil {
		t.Error("expected error for Component containing control characters")
	}
}

func TestValidateMediaTarget_NilIsValid(t *testing.T) {
	if err := validateMediaTarget(nil); err != nil {
		t.Errorf("nil MediaTarget should be valid, got %v", err)
	}
}

func TestResourceURLObserved_NestedTargetValidation(t *testing.T) {
	ev := ResourceURLObserved{
		Resource: RemoteResource{URL: "https://example.com", Kind: ResourceKindVideo, Role: ResourceRoleSource},
		Target:   &MediaTarget{Component: "", Backend: MediaBackendUnknown},
	}
	if err := ev.validate(); err == nil {
		t.Error("expected error for invalid nested Target")
	}
}

func TestResourceResolved_NestedTargetValidation(t *testing.T) {
	ev := ResourceResolved{
		Input:  RemoteResource{URL: "https://in.example.com", Kind: ResourceKindVideo, Role: ResourceRoleResolverInput},
		Output: RemoteResource{URL: "https://out.example.com", Kind: ResourceKindVideo, Role: ResourceRoleResolved},
		Target: &MediaTarget{Component: "vrchat", Backend: "invalid"},
	}
	if err := ev.validate(); err == nil {
		t.Error("expected error for invalid nested Target")
	}
}

func TestMediaErrorObserved_NestedResourceValidation(t *testing.T) {
	badResource := RemoteResource{URL: "", Kind: ResourceKindVideo, Role: ResourceRoleSource}
	ev := MediaErrorObserved{Stage: MediaStageLoad, Code: "E1", Resource: &badResource}
	if err := ev.validate(); err == nil {
		t.Error("expected error for invalid nested Resource")
	}
}

func TestMediaErrorObserved_NestedTargetValidation(t *testing.T) {
	ev := MediaErrorObserved{Stage: MediaStageLoad, Code: "E1", Target: &MediaTarget{Component: "vrchat", Backend: ""}}
	if err := ev.validate(); err == nil {
		t.Error("expected error for invalid nested Target")
	}
}

func TestMediaErrorObserved_CodeTooLong(t *testing.T) {
	ev := MediaErrorObserved{Stage: MediaStageLoad, Code: strings.Repeat("a", maxMediaErrorCodeBytes+1)}
	if err := ev.validate(); err == nil {
		t.Error("expected error for oversized Code")
	}
}

func TestMediaErrorObserved_MessageTooLong(t *testing.T) {
	ev := MediaErrorObserved{Stage: MediaStageLoad, Message: strings.Repeat("a", maxMediaErrorMessageBytes+1)}
	if err := ev.validate(); err == nil {
		t.Error("expected error for oversized Message")
	}
}

func TestMediaErrorObserved_MessageControlCharsRejected(t *testing.T) {
	ev := MediaErrorObserved{Stage: MediaStageLoad, Message: "line one\tline two"}
	if err := ev.validate(); err == nil {
		t.Error("expected error for Message containing a tab (control character)")
	}
}
