package vrclog

import "testing"

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
